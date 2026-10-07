package einvoice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ═══════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════

// DefaultBaseURL is the production gateway, for sandbox and live keys alike (the key's prefix picks
// the mode).
//
// THIS NAME IS PERMANENT. It is compiled into every installed copy of the SDK, so infrastructure
// moves by repointing DNS behind it, never by changing it here: 0.7.0 broke for everyone when its
// built-in `gateway.useyona.com` stopped serving.
const DefaultBaseURL = "https://gp.useyona.com"

// defaultBaseURLOverride lets the development tooling (scripts/run_examples) point an example at
// another gateway without the example naming a host: it is set at link time with
// `-ldflags "-X github.com/elyonar/einvoice-go.defaultBaseURLOverride=https://…"`. The SDK never
// reads the environment.
var defaultBaseURLOverride string

func defaultBaseURL() string {
	if defaultBaseURLOverride != "" {
		return defaultBaseURLOverride
	}
	return DefaultBaseURL
}

// apiKeyPattern is a Yona API key: `sk_test_` (sandbox) or `sk_live_` (live), a 16-character public
// id (base32, lower case), then a 43-character secret (base64url).
var apiKeyPattern = regexp.MustCompile(`^sk_(test|live)_([a-z2-7]{16})_([A-Za-z0-9_-]{43})$`)

// idempotencyKeyPattern is what the API accepts in `Idempotency-Key`.
var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

var retryableStatus = map[int]bool{408: true, 429: true, 500: true, 502: true, 503: true, 504: true}
var idempotentMethods = map[string]bool{"GET": true, "HEAD": true, "PUT": true, "DELETE": true}

// Mode is the key's mode: `sk_test_…` is ModeSandbox, `sk_live_…` is ModeLive, on the same host.
type Mode string

const (
	// ModeSandbox is the mode of an `sk_test_` key.
	ModeSandbox Mode = "sandbox"
	// ModeLive is the mode of an `sk_live_` key.
	ModeLive Mode = "live"
)

// ═══════════════════════════════════════════
// Configuration
// ═══════════════════════════════════════════

// Transport sends one HTTP request; *http.Client satisfies it. Tests inject a recorder.
type Transport interface {
	Do(*http.Request) (*http.Response, error)
}

// RetryConfig is the retry behaviour. A nil MaxRetries and zero durations keep the defaults.
type RetryConfig struct {
	// MaxRetries is the number of retries after the first attempt (default 2; Ptr(0) disables). Only
	// requests that are safe to repeat are retried: GET, PUT and DELETE, and writes that carry an
	// `Idempotency-Key`. Retried on a network error, a timeout, 408, 429 and 5xx.
	MaxRetries *int
	// BaseDelay is the first backoff delay (default 500 ms); it doubles each retry, with jitter.
	BaseDelay time.Duration
	// MaxDelay is the ceiling of one backoff delay (default 8 s).
	MaxDelay time.Duration
	// MaxRetryAfter is the longest `Retry-After` the SDK will wait on 429/503 before retrying
	// (default 60 s). A longer one is returned at once, as the error with RetryAfter set.
	MaxRetryAfter time.Duration
}

// Default values of the client.
const (
	DefaultTimeout       = 30 * time.Second
	DefaultMaxRetries    = 2
	DefaultBaseDelay     = 500 * time.Millisecond
	DefaultMaxDelay      = 8 * time.Second
	DefaultMaxRetryAfter = 60 * time.Second
)

// clientConfig is what the functional options fill.
type clientConfig struct {
	baseURL    string
	assertMode Mode
	timeout    time.Duration
	retry      RetryConfig
	headers    map[string]string
	transport  Transport
}

// Option configures New and NewHttpClient.
type Option func(*clientConfig)

// WithBaseURL overrides the API host (advanced: a local gateway in development). The default is the
// production gateway for both modes; never switch hosts by mode.
func WithBaseURL(baseURL string) Option {
	return func(c *clientConfig) { c.baseURL = baseURL }
}

// WithAssertMode makes New fail unless the key is of this mode, so a deployment refuses to start
// with a key of the other mode.
func WithAssertMode(mode Mode) Option {
	return func(c *clientConfig) { c.assertMode = mode }
}

// WithTimeout sets the request timeout (default 30 s). It bounds each attempt, headers and body.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.timeout = d }
}

// WithRetry sets the retry behaviour.
func WithRetry(r RetryConfig) Option {
	return func(c *clientConfig) { c.retry = r }
}

// WithHeaders adds headers sent on every request. `Authorization` cannot be overridden.
func WithHeaders(h map[string]string) Option {
	return func(c *clientConfig) { c.headers = h }
}

// WithHTTPClient sends with this *http.Client (its own Timeout is not needed: the SDK bounds each attempt).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) { c.transport = hc }
}

// WithTransport sends with this Transport (a recorder in tests, a wrapped client in production).
func WithTransport(t Transport) Option {
	return func(c *clientConfig) { c.transport = t }
}

// ═══════════════════════════════════════════
// Internal request shape
// ═══════════════════════════════════════════

// Call is how one API call is made. Services build it; integrators normally never touch it.
type Call struct {
	Method string
	Path   string
	// Query is a `<Op>Query` struct (encoded from its json tags), url.Values, map[string]any or map[string]string.
	Query any
	// Body is marshalled as JSON when not nil.
	Body any
	// Idempotent: the route honours `Idempotency-Key`, so the SDK sends one (generated unless the caller gave one).
	Idempotent bool
	// Binary: expect a binary body (a PDF) instead of the JSON envelope.
	Binary bool
	// Headers set by the service (e.g. `Accept`).
	Headers map[string]string
	// Options are the caller's per-request options.
	Options *RequestOptions
}

// ResponseMeta is `meta` of a successful answer.
type ResponseMeta struct {
	StatusCode int  `json:"statusCode"`
	Success    bool `json:"success"`
	// Message is copy for humans. Never branch on it.
	Message   string           `json:"message"`
	Errors    []map[string]any `json:"errors"`
	Timestamp string           `json:"timestamp"`
	// RequestID is the request id (also the `x-request-id` header).
	RequestID string `json:"requestId"`
	// Pagination is present on list answers.
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

// RawResponse is a successful answer as the transport hands it to the services: the envelope's
// `data` still as JSON, its `meta`, the status and headers, or the binary file.
type RawResponse struct {
	Status int
	// Data is the envelope's `data` (or the whole body when it was not the envelope); nil on 204 or an empty body.
	Data json.RawMessage
	// Meta is the envelope's `meta`, when the body was the envelope.
	Meta *ResponseMeta
	// RequestID is `meta.requestId`, falling back to the `x-request-id` header.
	RequestID string
	Header    http.Header
	// Binary is set for a binary answer (Call.Binary and a non-JSON content type).
	Binary *BinaryResponse
}

// ═══════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════

// ModeOfAPIKey returns the mode a key belongs to, or a *ConfigError for a malformed key (never echoing it).
func ModeOfAPIKey(apiKey string) (Mode, error) {
	m := apiKeyPattern.FindStringSubmatch(strings.TrimSpace(apiKey))
	if m == nil {
		return "", &ConfigError{Message: "The API key is malformed: expected sk_test_<16 characters>_<43 characters> (sandbox) or " +
			"sk_live_… (live). Copy it again from the dashboard (API keys)."}
	}
	if m[1] == "live" {
		return ModeLive, nil
	}
	return ModeSandbox, nil
}

// GenerateIdempotencyKey returns a fresh idempotency key (a UUID v4).
func GenerateIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on a supported platform; degrade to math/rand rather than panic.
		for i := range b {
			b[i] = byte(mathrand.Intn(256))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

var digits = regexp.MustCompile(`^\d+$`)

// ParseRetryAfter reads seconds from a `Retry-After` header (delta-seconds or an HTTP date, relative
// to now). ok is false for an empty or unreadable value.
func ParseRetryAfter(value string, now time.Time) (seconds int, ok bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	if digits.MatchString(trimmed) {
		n, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	at, err := http.ParseTime(trimmed)
	if err != nil {
		return 0, false
	}
	delta := math.Ceil(at.Sub(now).Seconds())
	if delta < 0 {
		delta = 0
	}
	return int(delta), true
}

// unpackErrors turns `{ field: message }` wire entries into ErrorDetail{Field, Message}.
func unpackErrors(errs any) []ErrorDetail {
	out := []ErrorDetail{}
	list, ok := errs.([]any)
	if !ok {
		return out
	}
	for _, entry := range list {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, ErrorDetail{Field: k, Message: stringOf(obj[k])})
		}
	}
	return out
}

// stringOf renders a JSON scalar as JavaScript's String() would (1 → "1", true → "true").
func stringOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return "null"
	default:
		return fmt.Sprint(x)
	}
}

var fileNamePattern = regexp.MustCompile(`(?i)filename\*?=(?:UTF-8'')?"?([^";]+)"?`)

// attemptOutcome is one attempt's result: a parsed success, or a refusal to retry or return.
type attemptOutcome struct {
	value  *RawResponse
	status int
	err    error
}

type attemptFailure int

const (
	failureTimeout attemptFailure = iota
	failureConnection
)

type attemptError struct {
	kind attemptFailure
	err  error
}

func (e *attemptError) Error() string { return e.err.Error() }

// ═══════════════════════════════════════════
// HTTP client
// ═══════════════════════════════════════════

// HttpClient is the transport: authentication, the envelope, errors, idempotency keys, retries and
// timeouts. Services call Request; integrators normally never touch it (it is Client.HTTP).
type HttpClient struct {
	baseURL        string
	mode           Mode
	apiKey         string
	timeout        time.Duration
	maxRetries     int
	baseDelay      time.Duration
	maxDelay       time.Duration
	maxRetryAfter  time.Duration
	defaultHeaders map[string]string
	transport      Transport
	// sleep waits d or returns ctx.Err() as soon as ctx is done. Tests replace it to record waits.
	sleep func(ctx context.Context, d time.Duration) error
	// jitter returns a number in [0, 1).
	jitter func() float64
}

// NewHttpClient builds the transport. New calls it; use it directly only to make raw calls.
func NewHttpClient(apiKey string, opts ...Option) (*HttpClient, error) {
	cfg := clientConfig{}
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, &ConfigError{Message: `An API key is required: einvoice.New("sk_test_…").`}
	}
	mode, err := ModeOfAPIKey(apiKey)
	if err != nil {
		return nil, err
	}
	if cfg.assertMode != "" && cfg.assertMode != mode {
		prefix := "sk_test_"
		if mode == ModeLive {
			prefix = "sk_live_"
		}
		return nil, &ConfigError{Message: fmt.Sprintf("This client was asserted to be %s, but the API key is a %s key (%s…).", cfg.assertMode, mode, prefix)}
	}
	base := cfg.baseURL
	if base == "" {
		base = defaultBaseURL()
	}
	if u, err := url.Parse(base); err != nil || u.Scheme == "" || u.Host == "" {
		return nil, &ConfigError{Message: "baseUrl is not a URL: " + base}
	}
	c := &HttpClient{
		baseURL:        strings.TrimRight(base, "/"),
		mode:           mode,
		apiKey:         strings.TrimSpace(apiKey),
		timeout:        cfg.timeout,
		maxRetries:     DefaultMaxRetries,
		baseDelay:      cfg.retry.BaseDelay,
		maxDelay:       cfg.retry.MaxDelay,
		maxRetryAfter:  cfg.retry.MaxRetryAfter,
		defaultHeaders: cfg.headers,
		transport:      cfg.transport,
		sleep:          sleepCtx,
		jitter:         mathrand.Float64,
	}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	if cfg.retry.MaxRetries != nil {
		c.maxRetries = *cfg.retry.MaxRetries
	}
	if c.maxRetries < 0 {
		c.maxRetries = 0
	}
	if c.baseDelay <= 0 {
		c.baseDelay = DefaultBaseDelay
	}
	if c.maxDelay <= 0 {
		c.maxDelay = DefaultMaxDelay
	}
	if c.maxRetryAfter <= 0 {
		c.maxRetryAfter = DefaultMaxRetryAfter
	}
	if c.transport == nil {
		c.transport = &http.Client{}
	}
	return c, nil
}

// BaseURL is the host requests go to.
func (c *HttpClient) BaseURL() string { return c.baseURL }

// Mode is the key's mode, from its prefix.
func (c *HttpClient) Mode() Mode { return c.mode }

// Request sends one API call and returns the parsed success, or an error: an *APIError wrapper for a
// refusal, *TimeoutError, *ConnectionError, *ConfigError, or ctx.Err() when the caller cancelled.
func (c *HttpClient) Request(ctx context.Context, call Call) (*RawResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ro := call.Options
	if ro == nil {
		ro = &RequestOptions{}
	}
	target, err := c.buildURL(call.Path, call.Query)
	if err != nil {
		return nil, err
	}
	timeout := ro.Timeout
	if timeout <= 0 {
		timeout = c.timeout
	}
	headers, err := c.buildHeaders(call, ro)
	if err != nil {
		return nil, err
	}
	keyed := headers.Get("Idempotency-Key") != ""
	retryable := idempotentMethods[call.Method] || keyed
	maxRetries := 0
	if retryable {
		maxRetries = c.maxRetries
		if ro.MaxRetries != nil {
			maxRetries = *ro.MaxRetries
		}
	}
	var body []byte
	if call.Body != nil {
		body, err = marshalJSON(call.Body)
		if err != nil {
			return nil, &ConfigError{Message: "the request body cannot be encoded as JSON: " + err.Error()}
		}
	}

	for attempt := 0; ; attempt++ {
		outcome, err := c.attemptOnce(ctx, target, call.Method, headers, body, timeout, call.Binary)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			var ae *attemptError
			if !asAttemptError(err, &ae) {
				return nil, err // an *APIError (a success body that is not JSON): never retried
			}
			var failure error
			if ae.kind == failureTimeout {
				failure = &TimeoutError{Timeout: timeout}
			} else {
				failure = &ConnectionError{BaseURL: c.baseURL, Err: ae.err}
			}
			if attempt < maxRetries {
				if err := c.sleep(ctx, c.backoff(attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, failure
		}
		if outcome.value != nil {
			return outcome.value, nil
		}
		status := outcome.status
		if attempt < maxRetries && retryableStatus[status] {
			var apiErr *APIError
			wait := 0
			if asAPIError(outcome.err, &apiErr) {
				wait = apiErr.RetryAfter
			}
			if wait > 0 && (status == 429 || status == 503) {
				if time.Duration(wait)*time.Second > c.maxRetryAfter {
					return nil, outcome.err
				}
				if err := c.sleep(ctx, time.Duration(wait)*time.Second); err != nil {
					return nil, err
				}
			} else if err := c.sleep(ctx, c.backoff(attempt)); err != nil {
				return nil, err
			}
			continue
		}
		return nil, outcome.err
	}
}

func asAttemptError(err error, target **attemptError) bool {
	ae, ok := err.(*attemptError)
	if ok {
		*target = ae
	}
	return ok
}

func asAPIError(err error, target **APIError) bool {
	for err != nil {
		if ae, ok := err.(*APIError); ok {
			*target = ae
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// marshalJSON encodes without HTML escaping and without the trailing newline json.Encoder adds.
func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (c *HttpClient) buildHeaders(call Call, ro *RequestOptions) (http.Header, error) {
	h := http.Header{}
	h.Set("Accept", "application/json")
	h.Set("User-Agent", userAgent)
	for _, layer := range []map[string]string{c.defaultHeaders, call.Headers, ro.Headers} {
		for k, v := range layer {
			h.Set(k, v)
		}
	}
	for name := range h {
		switch strings.ToLower(name) {
		case "authorization", "x-api-key", "idempotency-key":
			h.Del(name)
		}
	}
	h.Set("Authorization", "Bearer "+c.apiKey)
	if call.Body != nil {
		h.Set("Content-Type", "application/json")
	}
	if call.Idempotent {
		key := ro.IdempotencyKey
		if key == "" {
			key = GenerateIdempotencyKey()
		}
		if !idempotencyKeyPattern.MatchString(key) {
			return nil, &ConfigError{Message: "idempotencyKey must be 8–128 characters of A–Z, a–z, 0–9, _ or -."}
		}
		h.Set("Idempotency-Key", key)
	}
	return h, nil
}

// attemptOnce makes one attempt: the request AND reading its body, all under one deadline, so a body
// that stalls after the headers arrived times out like a request that never answered.
func (c *HttpClient) attemptOnce(ctx context.Context, target, method string, headers http.Header, body []byte, timeout time.Duration, binary bool) (*attemptOutcome, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(attemptCtx, method, target, reader)
	if err != nil {
		return nil, &ConfigError{Message: "the request could not be built: " + err.Error()}
	}
	req.Header = headers.Clone()
	if body != nil {
		req.ContentLength = int64(len(body))
	}
	classify := func(err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attemptCtx.Err() != nil {
			return &attemptError{kind: failureTimeout, err: err}
		}
		return &attemptError{kind: failureConnection, err: err}
	}
	resp, err := c.transport.Do(req)
	if err != nil {
		return nil, classify(err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, classify(err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		value, err := c.parseSuccess(resp, payload, binary)
		if err != nil {
			return nil, err
		}
		return &attemptOutcome{value: value}, nil
	}
	return &attemptOutcome{status: resp.StatusCode, err: c.parseError(resp, payload)}, nil
}

func (c *HttpClient) parseSuccess(resp *http.Response, payload []byte, binary bool) (*RawResponse, error) {
	headerRequestID := resp.Header.Get("X-Request-Id")
	contentType := resp.Header.Get("Content-Type")
	if binary && !strings.Contains(contentType, "json") {
		file := &BinaryResponse{Data: payload, ContentType: contentType, RequestID: headerRequestID}
		if file.ContentType == "" {
			file.ContentType = "application/octet-stream"
		}
		if m := fileNamePattern.FindStringSubmatch(resp.Header.Get("Content-Disposition")); m != nil {
			name := m[1]
			if decoded, err := url.PathUnescape(name); err == nil {
				name = decoded
			}
			file.FileName = name
		}
		return &RawResponse{Status: resp.StatusCode, RequestID: headerRequestID, Header: resp.Header, Binary: file}, nil
	}
	if resp.StatusCode == 204 {
		return &RawResponse{Status: 204, RequestID: headerRequestID, Header: resp.Header}, nil
	}
	text := bytes.TrimSpace(payload)
	if len(text) == 0 {
		return &RawResponse{Status: resp.StatusCode, RequestID: headerRequestID, Header: resp.Header}, nil
	}
	if !json.Valid(text) {
		return nil, &APIError{
			Status:    resp.StatusCode,
			Message:   fmt.Sprintf("The API answered %d with a body that is not JSON", resp.StatusCode),
			RequestID: headerRequestID,
			Body:      string(payload),
		}
	}
	if text[0] == '{' {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(text, &envelope); err == nil {
			rawMeta, hasMeta := envelope["meta"]
			data, hasData := envelope["data"]
			if hasMeta && hasData {
				meta := &ResponseMeta{}
				_ = json.Unmarshal(rawMeta, meta) // best effort: a field of the wrong shape leaves the others filled
				requestID := meta.RequestID
				if requestID == "" {
					requestID = headerRequestID
				}
				return &RawResponse{Status: resp.StatusCode, Data: data, Meta: meta, RequestID: requestID, Header: resp.Header}, nil
			}
		}
	}
	return &RawResponse{Status: resp.StatusCode, Data: json.RawMessage(text), RequestID: headerRequestID, Header: resp.Header}, nil
}

func (c *HttpClient) parseError(resp *http.Response, payload []byte) error {
	retryAfter, _ := ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	headerRequestID := resp.Header.Get("X-Request-Id")
	var parsed any
	var body any
	if len(bytes.TrimSpace(payload)) > 0 {
		if err := json.Unmarshal(payload, &parsed); err != nil {
			parsed = nil
			body = string(payload)
		} else {
			body = parsed
		}
	}
	var meta map[string]any
	if obj, ok := parsed.(map[string]any); ok {
		meta, _ = obj["meta"].(map[string]any)
	}
	e := &APIError{
		Status:     resp.StatusCode,
		Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
		Errors:     unpackErrors(meta["errors"]),
		RequestID:  headerRequestID,
		RetryAfter: retryAfter,
		Body:       body,
	}
	if msg, ok := meta["message"].(string); ok && msg != "" {
		e.Message = msg
	}
	if code, ok := meta["errorCode"].(string); ok {
		e.ErrorCode = code
	}
	if id, ok := meta["requestId"].(string); ok && id != "" {
		e.RequestID = id
	}
	return apiErrorFor(e)
}

func (c *HttpClient) buildURL(path string, query any) (string, error) {
	target := c.baseURL + path
	qs, err := encodeQuery(query)
	if err != nil {
		return "", err
	}
	if qs != "" {
		if strings.Contains(target, "?") {
			target += "&" + qs
		} else {
			target += "?" + qs
		}
	}
	return target, nil
}

func (c *HttpClient) backoff(attempt int) time.Duration {
	base := float64(c.baseDelay) * math.Pow(2, float64(attempt))
	if base > float64(c.maxDelay) {
		base = float64(c.maxDelay)
	}
	return time.Duration(base/2 + c.jitter()*(base/2))
}

// sleepCtx waits d, or returns ctx.Err() as soon as ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ═══════════════════════════════════════════
// Query encoding
// ═══════════════════════════════════════════

type queryPair struct{ key, value string }

// encodeQuery renders a query in the order its fields are declared: nil pointers and empty strings
// are skipped, bools are "true"/"false", slices repeat the key. It accepts a `<Op>Query` struct (or
// a pointer to one), url.Values, map[string]any and map[string]string (maps are sorted by key).
func encodeQuery(query any) (string, error) {
	if query == nil {
		return "", nil
	}
	var pairs []queryPair
	switch q := query.(type) {
	case url.Values:
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range q[k] {
				if v != "" {
					pairs = append(pairs, queryPair{k, v})
				}
			}
		}
	case map[string]string:
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if q[k] != "" {
				pairs = append(pairs, queryPair{k, q[k]})
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pairs = appendQueryValue(pairs, k, reflect.ValueOf(q[k]))
		}
	default:
		v := reflect.ValueOf(query)
		for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if v.IsNil() {
				return "", nil
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return "", &ConfigError{Message: fmt.Sprintf("a query must be a struct, url.Values or a map, not %T", query)}
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name := f.Name
			if tag, ok := f.Tag.Lookup("json"); ok {
				parts := strings.Split(tag, ",")
				if parts[0] == "-" {
					continue
				}
				if parts[0] != "" {
					name = parts[0]
				}
			}
			pairs = appendQueryValue(pairs, name, v.Field(i))
		}
	}
	if len(pairs) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for i, p := range pairs {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(url.QueryEscape(p.key))
		sb.WriteByte('=')
		sb.WriteString(url.QueryEscape(p.value))
	}
	return sb.String(), nil
}

func appendQueryValue(pairs []queryPair, key string, v reflect.Value) []queryPair {
	if !v.IsValid() {
		return pairs
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return pairs
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return appendScalar(pairs, key, string(v.Bytes()))
		}
		for i := 0; i < v.Len(); i++ {
			pairs = appendQueryValue(pairs, key, v.Index(i))
		}
		return pairs
	case reflect.String:
		return appendScalar(pairs, key, v.String())
	case reflect.Bool:
		return appendScalar(pairs, key, strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return appendScalar(pairs, key, strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return appendScalar(pairs, key, strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		return appendScalar(pairs, key, strconv.FormatFloat(v.Float(), 'f', -1, 64))
	default:
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return appendScalar(pairs, key, s.String())
		}
		return appendScalar(pairs, key, fmt.Sprint(v.Interface()))
	}
}

func appendScalar(pairs []queryPair, key, value string) []queryPair {
	if value == "" {
		return pairs
	}
	return append(pairs, queryPair{key, value})
}
