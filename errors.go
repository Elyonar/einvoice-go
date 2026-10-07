package einvoice

import (
	"fmt"
	"time"
)

// Error is implemented by every error the SDK returns, so `var e einvoice.Error; errors.As(err, &e)`
// tells an SDK error from any other. Narrow further with the concrete types below.
type Error interface {
	error
	einvoiceError()
}

// ErrorDetail is one entry of the API's `meta.errors`: the wire shape is a single-key object
// `{ "<field-or-code>": "<message>" }`; the SDK gives it as Field + Message.
type ErrorDetail struct {
	// Field is the field (or code) the message is about: the single key of the wire object.
	Field string
	// Message is the human-readable message.
	Message string
}

// APIError is the API's refusal of a request. Every refusal carries the envelope
// `{ meta: { statusCode, errorCode, message, errors, requestId }, data: null }`. The SDK returns a
// typed wrapper chosen by status (ValidationError, NotFoundError …), each of which unwraps to the
// *APIError, so both of these work:
//
//	var nf *einvoice.NotFoundError
//	if errors.As(err, &nf) { … }
//	var ae *einvoice.APIError
//	if errors.As(err, &ae) {
//		fmt.Println(ae.Status, ae.ErrorCode, ae.RequestID) // quote RequestID to support
//	}
type APIError struct {
	// Status is the HTTP status.
	Status int
	// ErrorCode is `meta.errorCode` (e.g. `VAL001`, `RES001`, `AUTH019`, `BIZ001`, `SYS005`).
	// Branch on this, never on the message. Empty when the body was not the envelope.
	ErrorCode string
	// Message is `meta.message`, or `HTTP <status>` when the body was not the envelope.
	Message string
	// Errors is `meta.errors`, unpacked.
	Errors []ErrorDetail
	// RequestID is `meta.requestId`, falling back to the `x-request-id` header. Quote it in a support request.
	RequestID string
	// RetryAfter is the seconds the API asked the caller to wait (`Retry-After`); 0 when it sent none.
	RetryAfter int
	// Body is the parsed response body (or the raw text when it was not JSON), for anything the fields above do not carry.
	Body any
}

func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("einvoice: %s (%d %s)", e.Message, e.Status, e.ErrorCode)
	}
	return fmt.Sprintf("einvoice: %s (%d)", e.Message, e.Status)
}

func (e *APIError) einvoiceError() {}

// ValidationError is 400 or 422: a field out of shape or not accepted (`VAL…`).
type ValidationError struct{ *APIError }

// AuthenticationError is 401: the key is missing, malformed, revoked or expired (`AUTH…`).
type AuthenticationError struct{ *APIError }

// InsufficientCreditsError is 402: not enough credits for the charge (`BIZ001`).
type InsufficientCreditsError struct{ *APIError }

// PermissionError is 403: the key lacks a capability (`AUTH019`), the route does not accept an API
// key (`AUTH018`), or a business rule forbids it.
type PermissionError struct{ *APIError }

// NotFoundError is 404: the resource does not exist or is not this organisation's (`RES001`).
type NotFoundError struct{ *APIError }

// ConflictError is 409: the resource's state does not allow it (e.g. `BIZ004`, `BIZ201`, `BIZ205`).
type ConflictError struct{ *APIError }

// RateLimitError is 429: rate limited (`SYS005`). RetryAfter holds the seconds to wait.
type RateLimitError struct{ *APIError }

// ServerError is 5xx: the API or a provider behind it failed (`SYS001`). 503 may carry RetryAfter.
type ServerError struct{ *APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *ValidationError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *AuthenticationError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *InsufficientCreditsError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *PermissionError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *NotFoundError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *ConflictError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *RateLimitError) Unwrap() error { return e.APIError }

// Unwrap exposes the *APIError to errors.As.
func (e *ServerError) Unwrap() error { return e.APIError }

// apiErrorFor wraps an APIError in the type matching its HTTP status.
func apiErrorFor(e *APIError) error {
	switch e.Status {
	case 400, 422:
		return &ValidationError{e}
	case 401:
		return &AuthenticationError{e}
	case 402:
		return &InsufficientCreditsError{e}
	case 403:
		return &PermissionError{e}
	case 404:
		return &NotFoundError{e}
	case 409:
		return &ConflictError{e}
	case 429:
		return &RateLimitError{e}
	}
	if e.Status >= 500 {
		return &ServerError{e}
	}
	return e
}

// TimeoutError means the request did not finish within the timeout (headers and body).
type TimeoutError struct {
	// Timeout is the deadline that was exceeded.
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("einvoice: request timed out after %s", e.Timeout)
}

func (e *TimeoutError) einvoiceError() {}

// ConnectionError means the request never reached the API (DNS, TLS, connection reset …).
type ConnectionError struct {
	// BaseURL is the host that could not be reached.
	BaseURL string
	// Err is the transport's error.
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("einvoice: could not reach %s: %v", e.BaseURL, e.Err)
}

// Unwrap returns the transport's error.
func (e *ConnectionError) Unwrap() error { return e.Err }

func (e *ConnectionError) einvoiceError() {}

// ConfigError means the SDK is misconfigured: a missing or malformed API key, a mode mismatch, a bad option.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string { return "einvoice: " + e.Message }

func (e *ConfigError) einvoiceError() {}

// WebhookError means a webhook request failed signature verification or could not be parsed.
type WebhookError struct {
	Message string
}

func (e *WebhookError) Error() string { return "einvoice: " + e.Message }

func (e *WebhookError) einvoiceError() {}
