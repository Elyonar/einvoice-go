package einvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// Shared fixtures: a Transport that records every request and answers from a script (the last
// answer repeats), the way the TS tests mock fetch and the Python tests mock httpx.

const testKey = "sk_test_abcdefghijklmnop_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const liveKey = "sk_live_qrstuvwxyz234567_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// answer is one scripted response: a status + headers + body, an error, or a handler.
type answer struct {
	status  int
	headers map[string]string
	body    []byte
	err     error
	fn      func(*http.Request) (*http.Response, error)
}

func (a answer) response() *http.Response {
	h := http.Header{}
	for k, v := range a.headers {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: a.status, Header: h, Body: io.NopCloser(bytes.NewReader(a.body))}
}

func jsonAnswer(status int, body any, headers ...map[string]string) answer {
	a := answer{status: status, headers: map[string]string{"Content-Type": "application/json"}}
	if body != nil {
		a.body, _ = json.Marshal(body)
	}
	for _, h := range headers {
		for k, v := range h {
			a.headers[k] = v
		}
	}
	return a
}

func okAnswer(data any, meta ...map[string]any) answer {
	m := map[string]any{"statusCode": 200, "success": true, "message": "Success", "errors": []any{}, "timestamp": "t", "requestId": "req-ok"}
	for _, extra := range meta {
		for k, v := range extra {
			m[k] = v
		}
	}
	return jsonAnswer(200, map[string]any{"meta": m, "data": data})
}

func refusal(status int, code string, extra map[string]any, headers map[string]string) answer {
	m := map[string]any{
		"statusCode": status, "success": false, "message": "refused " + code, "errorCode": code,
		"errors": []any{map[string]any{"name": "is required"}}, "timestamp": "t", "requestId": "req-err",
	}
	for k, v := range extra {
		m[k] = v
	}
	return jsonAnswer(status, map[string]any{"meta": m, "data": nil}, headers)
}

func failing(err error) answer { return answer{err: err} }

func handler(fn func(*http.Request) (*http.Response, error)) answer { return answer{fn: fn} }

// blocking never answers: it waits for the request's context to end.
func blocking() answer {
	return handler(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
}

// stalledBody sends its first bytes, then blocks until the request's context ends.
type stalledBody struct {
	ctx   context.Context
	first []byte
	sent  bool
}

func (b *stalledBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		if len(b.first) > 0 {
			return copy(p, b.first), nil
		}
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *stalledBody) Close() error { return nil }

func stalled(status int, first string) answer {
	return handler(func(req *http.Request) (*http.Response, error) {
		h := http.Header{}
		h.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: status, Header: h, Body: &stalledBody{ctx: req.Context(), first: []byte(first)}}, nil
	})
}

type fakeTransport struct {
	mu       sync.Mutex
	answers  []answer
	requests []*http.Request
	bodies   []string
	sleeps   []time.Duration
}

func newFake(answers ...answer) *fakeTransport {
	return &fakeTransport{answers: answers}
}

func (f *fakeTransport) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.bodies = append(f.bodies, string(body))
	var a answer
	if len(f.answers) > 1 {
		a, f.answers = f.answers[0], f.answers[1:]
	} else if len(f.answers) == 1 {
		a = f.answers[0]
	}
	f.mu.Unlock()
	if a.fn != nil {
		return a.fn(req)
	}
	if a.err != nil {
		return nil, a.err
	}
	return a.response(), nil
}

func (f *fakeTransport) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeTransport) recordSleep(_ context.Context, d time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sleeps = append(f.sleeps, d)
	return nil
}

// newHTTP builds a transport-level client over the fake, with a 1 ms base delay and recorded sleeps.
func newHTTP(t *testing.T, f *fakeTransport, opts ...Option) *HttpClient {
	t.Helper()
	all := append([]Option{WithTransport(f), WithRetry(RetryConfig{BaseDelay: time.Millisecond})}, opts...)
	c, err := NewHttpClient(testKey, all...)
	if err != nil {
		t.Fatalf("NewHttpClient: %v", err)
	}
	c.sleep = f.recordSleep
	return c
}

// newClient builds a Client over the fake, like newHTTP.
func newClient(t *testing.T, f *fakeTransport, opts ...Option) *Client {
	t.Helper()
	all := append([]Option{WithTransport(f), WithRetry(RetryConfig{BaseDelay: time.Millisecond})}, opts...)
	c, err := New(testKey, all...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.HTTP.sleep = f.recordSleep
	return c
}

func mustJSON(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("not JSON: %s", raw)
	}
	return v
}

func equalJSON(t *testing.T, got json.RawMessage, want any) {
	t.Helper()
	w, _ := json.Marshal(want)
	g, _ := json.Marshal(mustJSON(t, got))
	if string(g) != string(w) {
		t.Fatalf("got %s, want %s", g, w)
	}
}

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}
