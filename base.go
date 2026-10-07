package einvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// ═══════════════════════════════════════════
// Common response shapes
// ═══════════════════════════════════════════

// PaginationMeta is `meta.pagination` on a list answer (wire names in the json tags).
type PaginationMeta struct {
	Total int64 `json:"total"`
	// Page is the 1-based page number.
	Page int64 `json:"page"`
	// PageSize is the number of items per page (the `limit` query parameter).
	PageSize int64 `json:"pageSize"`
	// TotalPages is `ceil(total / pageSize)`; an empty set is 0 pages.
	TotalPages   int64  `json:"totalPages"`
	HasNext      bool   `json:"hasNext"`
	HasPrevious  bool   `json:"hasPrevious"`
	NextPage     string `json:"nextPage,omitempty"`
	PreviousPage string `json:"previousPage,omitempty"`
	CurrentPage  string `json:"currentPage,omitempty"`
}

// Page is a list answer: the items and `meta.pagination`.
type Page[T any] struct {
	Data       []T
	Pagination PaginationMeta
	// RequestID is the request id of the answer.
	RequestID string
}

// Paginated is an answer whose `data` is an object that also carries `meta.pagination` (received
// invoices, issued history): `Data.Items` holds the page.
type Paginated[D any] struct {
	Data       D
	Pagination *PaginationMeta
	// RequestID is the request id of the answer.
	RequestID string
}

// BinaryResponse is a binary download (a PDF).
type BinaryResponse struct {
	// Data is the file's bytes.
	Data []byte
	// ContentType is `Content-Type`, e.g. `application/pdf`.
	ContentType string
	// FileName is the file name from `Content-Disposition`, when sent.
	FileName string
	// RequestID is the request id of the answer.
	RequestID string
}

// ═══════════════════════════════════════════
// Per-request options
// ═══════════════════════════════════════════

// RequestOptions are the per-request options every service method takes as trailing RequestOption
// values: WithRequestTimeout, WithRequestHeaders, WithIdempotencyKey, WithMaxRetries.
type RequestOptions struct {
	// Timeout for this request (headers and body of each attempt).
	Timeout time.Duration
	// Headers are extra headers for this request. `Authorization` cannot be overridden.
	Headers map[string]string
	// IdempotencyKey is the `Idempotency-Key` for a write that accepts one (8–128 of `A–Z a–z 0–9 _ -`).
	// When omitted on such a write the SDK generates one, so its own retries are never charged twice;
	// pass your own to make a retry across process restarts safe. Ignored by routes that do not accept a key.
	IdempotencyKey string
	// MaxRetries is the maximum retries for this request (overrides the client's RetryConfig.MaxRetries).
	MaxRetries *int
}

// RequestOption configures one call.
type RequestOption func(*RequestOptions)

// WithIdempotencyKey sets the `Idempotency-Key` of a write that accepts one (8–128 of `A–Z a–z 0–9 _ -`).
func WithIdempotencyKey(key string) RequestOption {
	return func(o *RequestOptions) { o.IdempotencyKey = key }
}

// WithRequestTimeout sets the timeout of this request.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(o *RequestOptions) { o.Timeout = d }
}

// WithRequestHeaders adds headers to this request. `Authorization` cannot be overridden.
func WithRequestHeaders(h map[string]string) RequestOption {
	return func(o *RequestOptions) { o.Headers = h }
}

// WithMaxRetries sets the maximum retries of this request (0 disables them).
func WithMaxRetries(n int) RequestOption {
	return func(o *RequestOptions) { o.MaxRetries = &n }
}

// requestOptions applies the options; nil when none was given.
func requestOptions(opts []RequestOption) *RequestOptions {
	if len(opts) == 0 {
		return nil
	}
	o := &RequestOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// idempotencyKeyOf is the caller's key, "" when none: PATCH routes send a key only then.
func idempotencyKeyOf(o *RequestOptions) string {
	if o == nil {
		return ""
	}
	return o.IdempotencyKey
}

// ═══════════════════════════════════════════
// Service plumbing
// ═══════════════════════════════════════════

// seg encodes one path segment.
func seg(value string) string {
	return url.PathEscape(value)
}

// baseService is the shared plumbing of every service: it unwraps the `{ meta, data }` envelope.
type baseService struct {
	http *HttpClient
}

// call makes one call and decodes `data` into out (left untouched on a 204 or `null`).
func (s *baseService) call(ctx context.Context, c Call, out any) error {
	res, err := s.http.Request(ctx, c)
	if err != nil {
		return err
	}
	return decodeData(res.Data, out)
}

// decodeData unmarshals the envelope's data into out, when there is one.
func decodeData(data json.RawMessage, out any) error {
	if out == nil || len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &APIError{Status: 200, Message: "The API's answer does not have the expected shape: " + err.Error(), Body: string(data)}
	}
	return nil
}

// page makes a list call and returns the items and `meta.pagination`.
func page[T any](ctx context.Context, s *baseService, c Call) (*Page[T], error) {
	c.Idempotent = false
	c.Binary = false
	res, err := s.http.Request(ctx, c)
	if err != nil {
		return nil, err
	}
	out := &Page[T]{Data: []T{}, RequestID: res.RequestID}
	if err := decodeData(res.Data, &out.Data); err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = []T{}
	}
	if res.Meta != nil && res.Meta.Pagination != nil {
		out.Pagination = *res.Meta.Pagination
	} else {
		out.Pagination = PaginationMeta{Page: 1}
	}
	return out, nil
}

// paginated makes a call whose `data` is an object and whose `meta` also carries pagination.
func paginated[D any](ctx context.Context, s *baseService, c Call) (*Paginated[D], error) {
	c.Idempotent = false
	c.Binary = false
	c.Body = nil
	res, err := s.http.Request(ctx, c)
	if err != nil {
		return nil, err
	}
	out := &Paginated[D]{RequestID: res.RequestID}
	if err := decodeData(res.Data, &out.Data); err != nil {
		return nil, err
	}
	if res.Meta != nil {
		out.Pagination = res.Meta.Pagination
	}
	return out, nil
}

// binary makes a binary download. When the route answers JSON instead (a binary route answering the
// envelope), the unwrapped `data` is handed back as the bytes, with its content type.
func (s *baseService) binary(ctx context.Context, c Call) (*BinaryResponse, error) {
	c.Binary = true
	c.Body = nil
	headers := map[string]string{"Accept": "application/pdf"}
	for k, v := range c.Headers {
		headers[k] = v
	}
	c.Headers = headers
	res, err := s.http.Request(ctx, c)
	if err != nil {
		return nil, err
	}
	if res.Binary != nil {
		return res.Binary, nil
	}
	return &BinaryResponse{Data: []byte(res.Data), ContentType: res.Header.Get("Content-Type"), RequestID: res.RequestID}, nil
}

// Ptr returns a pointer to v, for the optional fields of the generated query and body structs:
// `&einvoice.ListBuyersQuery{Limit: einvoice.Ptr(50.0)}`.
func Ptr[T any](v T) *T {
	return &v
}
