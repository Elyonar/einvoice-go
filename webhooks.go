package einvoice

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WebhookSignatureHeader is the signature header Yona sends on every webhook delivery.
const WebhookSignatureHeader = "Yona-Signature"

// The other headers of a webhook delivery (names are case-insensitive).
const (
	WebhookEventIDHeader         = "Yona-Event-Id"
	WebhookEventTypeHeader       = "Yona-Event-Type"
	WebhookDeliveryIDHeader      = "Yona-Delivery-Id"
	WebhookDeliveryAttemptHeader = "Yona-Delivery-Attempt"
)

// DefaultToleranceSeconds: a delivery whose `t` is further than this from the receiver's clock is refused.
const DefaultToleranceSeconds int64 = 300

// HeaderGetter is anything that answers a header by name, case-insensitively: http.Header does.
type HeaderGetter interface {
	Get(name string) string
}

// VerifyOption configures VerifyWebhook.
type VerifyOption func(*verifyOptions)

type verifyOptions struct {
	tolerance int64
	now       int64
	hasNow    bool
}

// WithTolerance sets the maximum distance (seconds) between the signature's `t` and now (default 300). Replay protection.
func WithTolerance(seconds int64) VerifyOption {
	return func(o *verifyOptions) { o.tolerance = seconds }
}

// WithNow sets "now" in unix seconds (for tests).
func WithNow(unix int64) VerifyOption {
	return func(o *verifyOptions) {
		o.now = unix
		o.hasNow = true
	}
}

// ComputeWebhookSignature computes one `v1` value: hex HMAC-SHA256, keyed with the endpoint secret
// as UTF-8, over `<t> + "." + <raw body>`.
func ComputeWebhookSignature(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// SignWebhookPayload builds a `Yona-Signature` header for a payload, for testing your own webhook
// handler. Pass two secrets to imitate a rotation overlap (the current one first). A timestamp of 0
// means now.
func SignWebhookPayload(payload []byte, secrets []string, timestamp int64) string {
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	parts := []string{"t=" + strconv.FormatInt(timestamp, 10)}
	for _, s := range secrets {
		parts = append(parts, "v1="+ComputeWebhookSignature(payload, s, timestamp))
	}
	return strings.Join(parts, ",")
}

var signatureTimestamp = regexp.MustCompile(`^\d+$`)

// ParseSignatureHeader returns the `t` and every `v1` of a `Yona-Signature` header.
func ParseSignatureHeader(header string) (timestamp int64, signatures []string, err error) {
	hasT := false
	for _, part := range strings.Split(header, ",") {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(part[:eq])
		value := strings.TrimSpace(part[eq+1:])
		switch key {
		case "t":
			if !signatureTimestamp.MatchString(value) {
				return 0, nil, &WebhookError{Message: "Yona-Signature has an invalid t"}
			}
			n, perr := strconv.ParseInt(value, 10, 64)
			if perr != nil {
				return 0, nil, &WebhookError{Message: "Yona-Signature has an invalid t"}
			}
			timestamp, hasT = n, true
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if !hasT {
		return 0, nil, &WebhookError{Message: "Yona-Signature has no t"}
	}
	if len(signatures) == 0 {
		return 0, nil, &WebhookError{Message: "Yona-Signature has no v1 signature"}
	}
	return timestamp, signatures, nil
}

// VerifyWebhook verifies a webhook delivery and returns the parsed event.
//
// Yona signs `t + "." + <the exact bytes of the body>` with HMAC-SHA256 and the endpoint secret, and
// sends `Yona-Signature: t=<unix seconds>,v1=<hex>`; while a rotated secret overlaps the header
// carries a second `v1=`. This accepts the delivery when any `v1` matches (constant-time compare) and
// `t` is within the tolerance. Pass the RAW body: a re-serialised object will not verify.
// Deduplicate on event.ID: a redelivery carries the same id.
//
// payload is the raw request body; headers the request headers (r.Header); secret the endpoint's
// signing secret (`whsec_…`) from the dashboard. It returns a *WebhookError when the header is
// missing or malformed, `t` is outside the tolerance, no signature matches, or the body is not JSON.
//
//	http.HandleFunc("/webhooks/yona", func(w http.ResponseWriter, r *http.Request) {
//		raw, _ := io.ReadAll(r.Body)
//		event, err := einvoice.VerifyWebhook(raw, r.Header, os.Getenv("YONA_WEBHOOK_SECRET"))
//		if err != nil {
//			w.WriteHeader(http.StatusBadRequest)
//			return
//		}
//		if event.Type == einvoice.WebhookEventInvoiceAccepted { … }
//		w.WriteHeader(http.StatusOK)
//	})
func VerifyWebhook(payload []byte, headers HeaderGetter, secret string, opts ...VerifyOption) (*WebhookEvent, error) {
	if secret == "" {
		return nil, &WebhookError{Message: "A webhook secret is required"}
	}
	header := ""
	if headers != nil {
		header = headers.Get(WebhookSignatureHeader)
	}
	if header == "" {
		return nil, &WebhookError{Message: "Missing Yona-Signature header"}
	}
	timestamp, signatures, err := ParseSignatureHeader(header)
	if err != nil {
		return nil, err
	}
	o := verifyOptions{tolerance: DefaultToleranceSeconds}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	now := o.now
	if !o.hasNow {
		now = time.Now().Unix()
	}
	delta := now - timestamp
	if delta < 0 {
		delta = -delta
	}
	if delta > o.tolerance {
		return nil, &WebhookError{Message: fmt.Sprintf("Yona-Signature timestamp is outside the %ds tolerance", o.tolerance)}
	}
	expected := []byte(ComputeWebhookSignature(payload, secret, timestamp))
	matched := false
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), expected) {
			matched = true
		}
	}
	if !matched {
		return nil, &WebhookError{Message: "Webhook signature verification failed"}
	}
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, &WebhookError{Message: "The webhook body is not JSON"}
	}
	return &event, nil
}
