package einvoice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"testing"
	"time"
)

// The transport, case for case with einvoice-js tests/client/http-client.test.ts.

// ── configuration ──

func TestConfigDefaultsToTheProductionGatewayForBothModes(t *testing.T) {
	if DefaultBaseURL != "https://gp.useyona.com" {
		t.Fatal(DefaultBaseURL)
	}
	for _, key := range []string{testKey, liveKey} {
		c, err := NewHttpClient(key)
		if err != nil || c.BaseURL() != "https://gp.useyona.com" {
			t.Fatalf("%v %v", c, err)
		}
	}
}

func TestConfigReadsTheModeFromTheKeyPrefix(t *testing.T) {
	c, _ := NewHttpClient(testKey)
	if c.Mode() != ModeSandbox {
		t.Fatal(c.Mode())
	}
	c, _ = NewHttpClient(liveKey)
	if c.Mode() != ModeLive {
		t.Fatal(c.Mode())
	}
	if m, err := ModeOfAPIKey(" " + liveKey + " "); err != nil || m != ModeLive {
		t.Fatal(m, err)
	}
}

func TestConfigAcceptsABaseURLOverrideAndStripsTrailingSlashes(t *testing.T) {
	c, err := NewHttpClient(testKey, WithBaseURL("http://127.0.0.1:3000//"))
	if err != nil || c.BaseURL() != "http://127.0.0.1:3000" {
		t.Fatal(c, err)
	}
	var ce *ConfigError
	if _, err := NewHttpClient(testKey, WithBaseURL("not a url")); !errors.As(err, &ce) {
		t.Fatalf("expected ConfigError, got %v", err)
	}
}

func TestConfigRejectsAMissingOrMalformedKeyEarlyWithoutEchoingIt(t *testing.T) {
	var ce *ConfigError
	if _, err := NewHttpClient(""); !errors.As(err, &ce) {
		t.Fatalf("expected ConfigError, got %v", err)
	}
	if _, err := New(""); !errors.As(err, &ce) {
		t.Fatalf("expected ConfigError, got %v", err)
	}
	bad := []string{
		"sk_prod_abcdefghijklmnop_" + repeat("A", 43), "sk_test_short", "pk_test_x", testKey + "x",
		"sk_test_ABCDEFGHIJKLMNOP_" + repeat("A", 43),
	}
	for _, key := range bad {
		_, err := NewHttpClient(key)
		if !errors.As(err, &ce) {
			t.Fatalf("%q accepted", key)
		}
		if contains := regexp.MustCompile(regexp.QuoteMeta(key)); contains.MatchString(err.Error()) {
			t.Fatalf("the error echoes the key: %v", err)
		}
		var sdkErr Error
		if !errors.As(err, &sdkErr) {
			t.Fatal("ConfigError should implement einvoice.Error")
		}
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestConfigAssertModeFailsOnAMismatchAndPassesOnAMatch(t *testing.T) {
	_, err := NewHttpClient(testKey, WithAssertMode(ModeLive))
	if err == nil {
		t.Fatal("expected an error")
	}
	contains(t, err.Error(), "asserted to be live")
	_, err = NewHttpClient(liveKey, WithAssertMode(ModeSandbox))
	if err == nil {
		t.Fatal("expected an error")
	}
	contains(t, err.Error(), "sk_live_")
	c, err := NewHttpClient(liveKey, WithAssertMode(ModeLive))
	if err != nil || c.Mode() != ModeLive {
		t.Fatal(c, err)
	}
}

func TestConfigRetryDefaultsAndOverrides(t *testing.T) {
	c, _ := NewHttpClient(testKey)
	if c.timeout != DefaultTimeout || c.maxRetries != DefaultMaxRetries || c.baseDelay != DefaultBaseDelay || c.maxDelay != DefaultMaxDelay || c.maxRetryAfter != DefaultMaxRetryAfter {
		t.Fatalf("defaults: %+v", c)
	}
	c, _ = NewHttpClient(testKey, WithTimeout(5*time.Second), WithRetry(RetryConfig{MaxRetries: Ptr(0), MaxRetryAfter: 10 * time.Second}))
	if c.timeout != 5*time.Second || c.maxRetries != 0 || c.maxRetryAfter != 10*time.Second || c.baseDelay != DefaultBaseDelay {
		t.Fatalf("overrides: %+v", c)
	}
}

// ── requests ──

func TestRequestSendsBearerAuthJSONAndTheQueryAndUnwrapsTheEnvelope(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"id": "x"}))
	c := newHTTP(t, f, WithHeaders(map[string]string{"X-Extra": "1", "Authorization": "Bearer nope"}))
	type query struct {
		Page  int      `json:"page"`
		Empty string   `json:"empty"`
		Skip  *string  `json:"skip,omitempty"`
		Flag  bool     `json:"flag"`
		IDs   []string `json:"ids"`
	}
	res, err := c.Request(context.Background(), Call{
		Method: "POST", Path: "/i/v1/buyers", Body: map[string]string{"name": "A"},
		Query:   query{Page: 2, Flag: true, IDs: []string{"a", "b"}},
		Options: &RequestOptions{Headers: map[string]string{"authorization": "Bearer override", "x-api-key": "legacy"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	equalJSON(t, res.Data, map[string]any{"id": "x"})
	if res.RequestID != "req-ok" {
		t.Fatal(res.RequestID)
	}
	req := f.requests[0]
	if got := req.URL.String(); got != "https://gp.useyona.com/i/v1/buyers?page=2&flag=true&ids=a&ids=b" {
		t.Fatal(got)
	}
	if got := req.Header.Values("Authorization"); len(got) != 1 || got[0] != "Bearer "+testKey {
		t.Fatal(got)
	}
	if req.Header.Get("X-Api-Key") != "" {
		t.Fatal("x-api-key kept")
	}
	if req.Header.Get("Content-Type") != "application/json" || req.Header.Get("X-Extra") != "1" || req.Header.Get("Accept") != "application/json" {
		t.Fatal(req.Header)
	}
	if req.Header.Get("User-Agent") != "einvoice-go/"+Version {
		t.Fatal(req.Header.Get("User-Agent"))
	}
	if f.bodies[0] != `{"name":"A"}` {
		t.Fatal(f.bodies[0])
	}
}

func TestRequestEncodesQueriesFromStructsMapsAndValues(t *testing.T) {
	// A generated query struct: nil pointers skipped, enums as their string, floats without a trailing .0.
	status := ListInvoicesQueryStatusDraft
	q, err := encodeQuery(&ListInvoicesQuery{Status: &status, Page: Ptr(2.0), Limit: Ptr(50.0)})
	if err != nil || q != "status=draft&page=2&limit=50" {
		t.Fatal(q, err)
	}
	if q, _ := encodeQuery((*ListInvoicesQuery)(nil)); q != "" {
		t.Fatal(q)
	}
	if q, _ := encodeQuery(&ListInvoicesQuery{}); q != "" {
		t.Fatal(q)
	}
	// Maps are sorted by key; nil, "" skipped; slices repeat.
	q, _ = encodeQuery(map[string]any{"page": 2, "empty": "", "skip": nil, "flag": true, "ids": []string{"a", "b"}})
	if q != "flag=true&ids=a&ids=b&page=2" {
		t.Fatal(q)
	}
	q, _ = encodeQuery(url.Values{"b": {"2"}, "a": {"1", ""}})
	if q != "a=1&b=2" {
		t.Fatal(q)
	}
	q, _ = encodeQuery(map[string]string{"format": "pdf", "x": ""})
	if q != "format=pdf" {
		t.Fatal(q)
	}
	if q, _ := encodeQuery(map[string]any{"search": "a b&c"}); q != "search=a+b%26c" {
		t.Fatal(q)
	}
	if _, err := encodeQuery(42); err == nil {
		t.Fatal("an int is not a query")
	}
}

func TestRequestAnswersANonEnvelopeJSONBodyAndA204AsTheyAre(t *testing.T) {
	f := newFake(jsonAnswer(200, []int{1, 2}))
	res, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	if err != nil {
		t.Fatal(err)
	}
	equalJSON(t, res.Data, []int{1, 2})
	if res.Meta != nil {
		t.Fatal("no meta expected")
	}

	f = newFake(answer{status: 204, headers: map[string]string{"X-Request-Id": "r204"}})
	res, err = newHTTP(t, f).Request(context.Background(), Call{Method: "DELETE", Path: "/x"})
	if err != nil || res.Data != nil || res.RequestID != "r204" || res.Status != 204 {
		t.Fatalf("%+v %v", res, err)
	}

	f = newFake(answer{status: 200})
	res, err = newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	if err != nil || res.Data != nil {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRequestASuccessBodyThatIsNotJSONIsAnError(t *testing.T) {
	f := newFake(answer{status: 200, body: []byte("<html>")})
	_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	if err == nil {
		t.Fatal("expected an error")
	}
	contains(t, err.Error(), "not JSON")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 200 || ae.Body != "<html>" {
		t.Fatalf("%v", err)
	}
}

func TestRequestReturnsABinaryDownloadWithItsTypeAndFileName(t *testing.T) {
	f := newFake(answer{status: 200, body: []byte{37, 80, 68, 70}, headers: map[string]string{
		"Content-Type": "application/pdf", "Content-Disposition": `attachment; filename="INV-1.pdf"`, "X-Request-Id": "rpdf",
	}})
	res, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/pdf", Binary: true})
	if err != nil || res.Binary == nil {
		t.Fatal(res, err)
	}
	if !reflect.DeepEqual(res.Binary.Data, []byte{37, 80, 68, 70}) || res.Binary.ContentType != "application/pdf" || res.Binary.FileName != "INV-1.pdf" || res.Binary.RequestID != "rpdf" {
		t.Fatalf("%+v", res.Binary)
	}
	f = newFake(answer{status: 200, body: []byte{1}})
	res, err = newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/pdf", Binary: true})
	if err != nil || res.Binary.ContentType != "application/octet-stream" || res.Binary.FileName != "" {
		t.Fatalf("%+v %v", res.Binary, err)
	}
	f = newFake(answer{status: 200, body: []byte{1}, headers: map[string]string{"Content-Type": "application/pdf", "Content-Disposition": "attachment; filename*=UTF-8''INV%201.pdf"}})
	res, _ = newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/pdf", Binary: true})
	if res.Binary.FileName != "INV 1.pdf" {
		t.Fatal(res.Binary.FileName)
	}
}

func TestRequestABinaryRouteAnsweringJSONIsParsedAsTheEnvelope(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"downloadUrl": "u"}))
	res, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/pdf", Binary: true})
	if err != nil || res.Binary != nil {
		t.Fatal(res, err)
	}
	equalJSON(t, res.Data, map[string]any{"downloadUrl": "u"})
}

// ── errors ──

func TestErrorsMapEachStatusToItsTypeWithErrorCodeErrorsAndRequestID(t *testing.T) {
	cases := []struct {
		status int
		code   string
		check  func(error) bool
	}{
		{400, "VAL001", func(e error) bool { var x *ValidationError; return errors.As(e, &x) }},
		{422, "VAL001", func(e error) bool { var x *ValidationError; return errors.As(e, &x) }},
		{401, "AUTH004", func(e error) bool { var x *AuthenticationError; return errors.As(e, &x) }},
		{402, "BIZ001", func(e error) bool { var x *InsufficientCreditsError; return errors.As(e, &x) }},
		{403, "AUTH019", func(e error) bool { var x *PermissionError; return errors.As(e, &x) }},
		{404, "RES001", func(e error) bool { var x *NotFoundError; return errors.As(e, &x) }},
		{409, "BIZ205", func(e error) bool { var x *ConflictError; return errors.As(e, &x) }},
		{418, "BIZ999", func(e error) bool { _, plain := e.(*APIError); return plain }},
	}
	for _, tc := range cases {
		f := newFake(refusal(tc.status, tc.code, nil, nil))
		_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "POST", Path: "/x", Body: map[string]any{}})
		if err == nil || !tc.check(err) {
			t.Fatalf("%d: wrong type %T %v", tc.status, err, err)
		}
		var ae *APIError
		if !errors.As(err, &ae) {
			t.Fatalf("%d: not an APIError", tc.status)
		}
		if ae.Status != tc.status || ae.ErrorCode != tc.code || ae.Message != "refused "+tc.code || ae.RequestID != "req-err" {
			t.Fatalf("%d: %+v", tc.status, ae)
		}
		if !reflect.DeepEqual(ae.Errors, []ErrorDetail{{Field: "name", Message: "is required"}}) {
			t.Fatalf("%d: %+v", tc.status, ae.Errors)
		}
		var sdkErr Error
		if !errors.As(err, &sdkErr) {
			t.Fatal("API errors implement einvoice.Error")
		}
		contains(t, err.Error(), tc.code)
	}
}

func TestErrorsFallBackToTheXRequestIDHeaderAndHTTPStatusWhenTheBodyIsNotTheEnvelope(t *testing.T) {
	f := newFake(answer{status: 502, body: []byte("Bad Gateway"), headers: map[string]string{"X-Request-Id": "hdr-id"}})
	_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "POST", Path: "/x"})
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatalf("%T %v", err, err)
	}
	if se.Message != "HTTP 502" || se.RequestID != "hdr-id" || se.ErrorCode != "" || len(se.Errors) != 0 || se.Body != "Bad Gateway" {
		t.Fatalf("%+v", se.APIError)
	}

	f = newFake(jsonAnswer(404, map[string]any{"meta": map[string]any{"errors": "odd", "message": ""}}))
	_, err = newHTTP(t, f).Request(context.Background(), Call{Method: "POST", Path: "/x"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Message != "HTTP 404" {
		t.Fatal(err)
	}

	f = newFake(jsonAnswer(404, map[string]any{"meta": map[string]any{"errors": []any{nil, "x", map[string]any{"a": 1}}}}))
	_, err = newHTTP(t, f).Request(context.Background(), Call{Method: "POST", Path: "/x"})
	if !errors.As(err, &ae) || !reflect.DeepEqual(ae.Errors, []ErrorDetail{{Field: "a", Message: "1"}}) {
		t.Fatal(err)
	}
}

func TestErrorsA429ExposesRetryAfter(t *testing.T) {
	f := newFake(refusal(429, "SYS005", nil, map[string]string{"Retry-After": "7"}))
	_, err := newHTTP(t, f, WithRetry(RetryConfig{MaxRetries: Ptr(0)})).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var rl *RateLimitError
	if !errors.As(err, &rl) || rl.RetryAfter != 7 {
		t.Fatal(err)
	}
}

// ── retries ──

func TestRetriesAGETOn503HonouringRetryAfterThenSucceeds(t *testing.T) {
	f := newFake(refusal(503, "SYS001", nil, map[string]string{"Retry-After": "3"}), okAnswer(map[string]any{"n": 1}))
	res, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	if err != nil {
		t.Fatal(err)
	}
	equalJSON(t, res.Data, map[string]any{"n": 1})
	if f.calls() != 2 || !reflect.DeepEqual(f.sleeps, []time.Duration{3 * time.Second}) {
		t.Fatal(f.calls(), f.sleeps)
	}
}

func TestRetries429OnAGETWithRetryAfterAndGivesUpAfterMaxRetries(t *testing.T) {
	f := newFake(refusal(429, "SYS005", nil, map[string]string{"Retry-After": "1"}))
	_, err := newHTTP(t, f, WithRetry(RetryConfig{MaxRetries: Ptr(2), BaseDelay: time.Millisecond})).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var rl *RateLimitError
	if !errors.As(err, &rl) || f.calls() != 3 {
		t.Fatal(err, f.calls())
	}
}

func TestRetriesReturnAtOnceWhenRetryAfterExceedsMaxRetryAfter(t *testing.T) {
	f := newFake(refusal(429, "SYS005", nil, map[string]string{"Retry-After": "3600"}))
	_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.RetryAfter != 3600 || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
}

func TestRetriesBackOffOnA500WithoutRetryAfter(t *testing.T) {
	f := newFake(refusal(500, "SYS001", nil, nil), okAnswer(map[string]any{}))
	c := newHTTP(t, f)
	if _, err := c.Request(context.Background(), Call{Method: "DELETE", Path: "/x"}); err != nil {
		t.Fatal(err)
	}
	if f.calls() != 2 || len(f.sleeps) != 1 {
		t.Fatal(f.calls(), f.sleeps)
	}
	// The backoff is min(maxDelay, base·2^attempt)/2 + jitter·(that/2).
	c.jitter = func() float64 { return 0 }
	if c.backoff(0) != 500*time.Microsecond || c.backoff(1) != time.Millisecond {
		t.Fatal(c.backoff(0), c.backoff(1))
	}
	c.jitter = func() float64 { return 0.999 }
	if d := c.backoff(20); d > c.maxDelay || d < c.maxDelay/2 {
		t.Fatal(d)
	}
}

func TestRetriesNeverRetryAPOSTWithoutAnIdempotencyKey(t *testing.T) {
	f := newFake(refusal(503, "SYS001", nil, nil))
	_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "POST", Path: "/x", Body: map[string]any{}})
	var se *ServerError
	if !errors.As(err, &se) || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
	g := newFake(failing(errors.New("fetch failed")))
	_, err = newHTTP(t, g).Request(context.Background(), Call{Method: "PATCH", Path: "/x", Body: map[string]any{}})
	var ce *ConnectionError
	if !errors.As(err, &ce) || g.calls() != 1 {
		t.Fatal(err, g.calls())
	}
}

func TestRetriesGenerateOneIdempotencyKeyForAKeyedPOSTAndReuseItOnEveryRetry(t *testing.T) {
	f := newFake(failing(errors.New("fetch failed")), refusal(502, "SYS001", nil, nil), okAnswer(map[string]any{"id": "inv"}))
	if _, err := newHTTP(t, f).Request(context.Background(), Call{Method: "POST", Path: "/i/v1/invoices", Body: map[string]any{}, Idempotent: true}); err != nil {
		t.Fatal(err)
	}
	if f.calls() != 3 {
		t.Fatal(f.calls())
	}
	keys := map[string]bool{}
	for _, r := range f.requests {
		keys[r.Header.Get("Idempotency-Key")] = true
	}
	if len(keys) != 1 || !idempotencyKeyPattern.MatchString(f.requests[0].Header.Get("Idempotency-Key")) {
		t.Fatal(keys)
	}
}

func TestRetriesUseTheCallerIdempotencyKeyRefuseAMalformedOneAndDropARawHeaderCopy(t *testing.T) {
	f := newFake(okAnswer(map[string]any{}))
	c := newHTTP(t, f)
	_, err := c.Request(context.Background(), Call{Method: "POST", Path: "/x", Idempotent: true, Options: &RequestOptions{IdempotencyKey: "order-42-create", Headers: map[string]string{"idempotency-key": "raw"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.requests[0].Header.Values("Idempotency-Key"); len(got) != 1 || got[0] != "order-42-create" {
		t.Fatal(got)
	}
	var ce *ConfigError
	if _, err := c.Request(context.Background(), Call{Method: "POST", Path: "/x", Idempotent: true, Options: &RequestOptions{IdempotencyKey: "bad key!"}}); !errors.As(err, &ce) {
		t.Fatal(err)
	}
	if _, err := c.Request(context.Background(), Call{Method: "POST", Path: "/x", Options: &RequestOptions{IdempotencyKey: "ignored-here"}}); err != nil {
		t.Fatal(err)
	}
	if f.requests[1].Header.Get("Idempotency-Key") != "" {
		t.Fatal("a key was sent on a route that does not accept one")
	}
}

func TestRetriesPerRequestMaxRetriesOverridesTheClient(t *testing.T) {
	f := newFake(refusal(503, "SYS001", nil, nil))
	_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x", Options: &RequestOptions{MaxRetries: Ptr(0)}})
	var se *ServerError
	if !errors.As(err, &se) || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
}

func TestRetriesTimeOutAndRetryAGETTheFinalFailureIsATimeoutError(t *testing.T) {
	f := newFake(blocking())
	c := newHTTP(t, f, WithTimeout(5*time.Millisecond), WithRetry(RetryConfig{MaxRetries: Ptr(1), BaseDelay: time.Millisecond}))
	_, err := c.Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var te *TimeoutError
	if !errors.As(err, &te) || te.Timeout != 5*time.Millisecond || f.calls() != 2 {
		t.Fatalf("%T %v %d", err, err, f.calls())
	}
	contains(t, err.Error(), "timed out")
	// A per-request timeout wins over the client's.
	f = newFake(blocking())
	c = newHTTP(t, f, WithTimeout(time.Minute), WithRetry(RetryConfig{MaxRetries: Ptr(0)}))
	_, err = c.Request(context.Background(), Call{Method: "GET", Path: "/x", Options: &RequestOptions{Timeout: 5 * time.Millisecond}})
	if !errors.As(err, &te) || te.Timeout != 5*time.Millisecond {
		t.Fatal(err)
	}
}

func TestRetriesAConnectionErrorOnAGETIsRetriedThenReturnedAsAConnectionError(t *testing.T) {
	cause := errors.New("getaddrinfo ENOTFOUND")
	f := newFake(failing(cause))
	_, err := newHTTP(t, f, WithRetry(RetryConfig{MaxRetries: Ptr(1), BaseDelay: time.Millisecond})).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var ce *ConnectionError
	if !errors.As(err, &ce) || f.calls() != 2 || !errors.Is(err, cause) {
		t.Fatal(err, f.calls())
	}
	contains(t, err.Error(), "ENOTFOUND")
	contains(t, err.Error(), "gp.useyona.com")
}

func TestRetriesACallerCancellationIsNeverRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := newFake(handler(func(req *http.Request) (*http.Response, error) {
		cancel()
		<-req.Context().Done()
		return nil, req.Context().Err()
	}))
	c := newHTTP(t, f)
	_, err := c.Request(ctx, Call{Method: "GET", Path: "/x"})
	if !errors.Is(err, context.Canceled) || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
	// An already-cancelled context never reaches the transport.
	_, err = c.Request(ctx, Call{Method: "GET", Path: "/x"})
	if !errors.Is(err, context.Canceled) || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
}

func TestRetriesACallerCancellationDuringTheRetryAfterWaitReturnsAtOnce(t *testing.T) {
	f := newFake(refusal(429, "SYS005", nil, map[string]string{"Retry-After": "30"}))
	c, err := NewHttpClient(testKey, WithTransport(f)) // the real sleep
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	started := time.Now()
	_, err = c.Request(ctx, Call{Method: "GET", Path: "/x"})
	if !errors.Is(err, context.Canceled) || time.Since(started) > time.Second || f.calls() != 1 {
		t.Fatal(err, time.Since(started), f.calls())
	}
}

func TestRetriesTheRealSleepReturnsAtOnceOnAnAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, 10*time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRetriesABodyThatStallsAfterTheHeadersTimesOutAndIsRetriedLikeAnyTimeout(t *testing.T) {
	f := newFake(stalled(200, `{"meta":`))
	c := newHTTP(t, f, WithTimeout(20*time.Millisecond), WithRetry(RetryConfig{MaxRetries: Ptr(1), BaseDelay: time.Millisecond}))
	_, err := c.Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var te *TimeoutError
	if !errors.As(err, &te) || f.calls() != 2 {
		t.Fatal(err, f.calls())
	}
}

func TestRetriesAStalledErrorBodyTimesOutTooInsteadOfHanging(t *testing.T) {
	f := newFake(stalled(500, ""))
	c := newHTTP(t, f, WithTimeout(20*time.Millisecond), WithRetry(RetryConfig{MaxRetries: Ptr(0)}))
	_, err := c.Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatal(err)
	}
}

func TestRetriesANonJSONSuccessBodyIsAnAPIErrorNeverRetriedAsAConnectionError(t *testing.T) {
	f := newFake(answer{status: 200, body: []byte("<html>"), headers: map[string]string{"Content-Type": "text/html"}})
	_, err := newHTTP(t, f).Request(context.Background(), Call{Method: "GET", Path: "/x"})
	var ae *APIError
	if !errors.As(err, &ae) || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
}

func TestRetriesTheRealSleepWaits(t *testing.T) {
	started := time.Now()
	if err := sleepCtx(context.Background(), 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 4*time.Millisecond {
		t.Fatal(time.Since(started))
	}
}

// ── helpers ──

func TestParseRetryAfterReadsSecondsAndHTTPDates(t *testing.T) {
	if _, ok := ParseRetryAfter("", time.Now()); ok {
		t.Fatal("empty should not parse")
	}
	if n, ok := ParseRetryAfter("12", time.Now()); !ok || n != 12 {
		t.Fatal(n, ok)
	}
	if _, ok := ParseRetryAfter("garbage", time.Now()); ok {
		t.Fatal("garbage should not parse")
	}
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	if n, ok := ParseRetryAfter("Mon, 05 Oct 2026 10:00:30 GMT", now); !ok || n != 30 {
		t.Fatal(n, ok)
	}
	if n, ok := ParseRetryAfter("Mon, 05 Oct 2026 09:00:00 GMT", now); !ok || n != 0 {
		t.Fatal(n, ok)
	}
}

func TestGenerateIdempotencyKeyMakesValidUniqueUUIDv4s(t *testing.T) {
	a := GenerateIdempotencyKey()
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(a) {
		t.Fatal(a)
	}
	if GenerateIdempotencyKey() == a {
		t.Fatal("not unique")
	}
	if !idempotencyKeyPattern.MatchString(a) {
		t.Fatal("not an acceptable Idempotency-Key")
	}
}

func TestAmountKeepsTheWireBytesAndAcceptsStringsAndNumbers(t *testing.T) {
	type line struct {
		Quantity  Amount  `json:"quantity"`
		UnitPrice *Amount `json:"unitPrice,omitempty"`
	}
	var l line
	if err := json.Unmarshal([]byte(`{"quantity":"2.50","unitPrice":1500}`), &l); err != nil {
		t.Fatal(err)
	}
	if !l.Quantity.IsString() || l.Quantity.String() != "2.50" || l.UnitPrice.IsString() || l.UnitPrice.String() != "1500" {
		t.Fatalf("%+v", l)
	}
	if f, err := l.Quantity.Float64(); err != nil || f != 2.5 {
		t.Fatal(f, err)
	}
	out, _ := json.Marshal(l)
	if string(out) != `{"quantity":"2.50","unitPrice":1500}` {
		t.Fatal(string(out))
	}
	out, _ = json.Marshal(line{Quantity: AmountNumber(3), UnitPrice: Ptr(AmountString("25000.00"))})
	if string(out) != `{"quantity":3,"unitPrice":"25000.00"}` {
		t.Fatal(string(out))
	}
	if out, _ := json.Marshal(line{}); string(out) != `{"quantity":null}` {
		t.Fatal(string(out))
	}
	var zero Amount
	if err := json.Unmarshal([]byte(`null`), &zero); err != nil || !zero.IsZero() || zero.String() != "" {
		t.Fatal(err, zero)
	}
	if _, err := zero.Float64(); err == nil {
		t.Fatal("the zero value has no number")
	}
	if err := json.Unmarshal([]byte(`true`), &zero); err == nil {
		t.Fatal("a bool is not an Amount")
	}
	if err := json.Unmarshal([]byte(`{"a":1}`), &zero); err == nil {
		t.Fatal("an object is not an Amount")
	}
}

func TestConfigTheLinkTimeOverrideRepointsTheDefaultHostOnly(t *testing.T) {
	defer func() { defaultBaseURLOverride = "" }()
	defaultBaseURLOverride = "http://localhost:4010/"
	c, err := NewHttpClient(testKey)
	if err != nil || c.BaseURL() != "http://localhost:4010" {
		t.Fatal(c, err)
	}
	explicit, _ := NewHttpClient(testKey, WithBaseURL("https://example.test"))
	if explicit.BaseURL() != "https://example.test" {
		t.Fatal(explicit.BaseURL())
	}
	defaultBaseURLOverride = ""
	plain, _ := NewHttpClient(testKey)
	if plain.BaseURL() != DefaultBaseURL {
		t.Fatal(plain.BaseURL())
	}
}
