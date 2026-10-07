package einvoice

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Yona-Signature verification: the shared vectors (tests/vectors/webhook-signature.json, produced by
// the backend signer) that every Yona SDK verifies, plus the Go-specific input handling.

type vectorCase struct {
	Name             string `json:"name"`
	Secret           string `json:"secret"`
	Header           string `json:"header"`
	Body             string `json:"body"`
	Now              *int64 `json:"now"`
	ToleranceSeconds *int64 `json:"toleranceSeconds"`
	Expect           string `json:"expect"`
}

type vectors struct {
	Secret           string `json:"secret"`
	PreviousSecret   string `json:"previousSecret"`
	OtherSecret      string `json:"otherSecret"`
	T                int64  `json:"t"`
	ToleranceSeconds int64  `json:"toleranceSeconds"`
	BodyUTF8         string `json:"bodyUtf8"`
	BodyBase64       string `json:"bodyBase64"`
	BodyBytes        int    `json:"bodyBytes"`
	BodyReserialised string `json:"bodyReserialised"`
	V1               string `json:"v1"`
	V1Previous       string `json:"v1Previous"`
	Header           string `json:"header"`
	HeaderRotating   string `json:"headerRotating"`
	Event            struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Mode string `json:"mode"`
	} `json:"event"`
	Cases       []vectorCase `json:"cases"`
	ParseHeader []struct {
		Header     string   `json:"header"`
		Timestamp  int64    `json:"timestamp"`
		Signatures []string `json:"signatures"`
	} `json:"parseHeader"`
}

func loadVectors(t *testing.T) (vectors, []byte) {
	t.Helper()
	raw, err := os.ReadFile("tests/vectors/webhook-signature.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	body, err := base64.StdEncoding.DecodeString(v.BodyBase64)
	if err != nil {
		t.Fatal(err)
	}
	return v, body
}

// backendV1 is the receiver recipe of the API's webhook documentation, reimplemented independently.
func backendV1(secret string, t int64, raw []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(t, 10) + "."))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func signatureHeaders(header string) http.Header {
	h := http.Header{}
	if header != "" {
		h.Set("yona-signature", header)
	}
	return h
}

var vectorMessages = map[string]string{
	"signature": "verification failed",
	"tolerance": "tolerance",
	"parse":     "has ",
	"missing":   "Missing Yona-Signature",
	"secret":    "secret is required",
}

func TestVectorsAreConsistentWithThemselves(t *testing.T) {
	v, body := loadVectors(t)
	if string(body) != v.BodyUTF8 || len(body) != v.BodyBytes {
		t.Fatal("body")
	}
	if v.V1 != backendV1(v.Secret, v.T, body) || v.V1Previous != backendV1(v.PreviousSecret, v.T, body) {
		t.Fatal("v1")
	}
	if v.Header != "t="+strconv.FormatInt(v.T, 10)+",v1="+v.V1 || v.HeaderRotating != v.Header+",v1="+v.V1Previous {
		t.Fatal("header")
	}
}

func TestMatchesTheBackendSignerByteForByte(t *testing.T) {
	v, body := loadVectors(t)
	if SignWebhookPayload(body, []string{v.Secret}, v.T) != v.Header {
		t.Fatal("header")
	}
	if SignWebhookPayload(body, []string{v.Secret, v.PreviousSecret}, v.T) != v.HeaderRotating {
		t.Fatal("rotating header")
	}
	if ComputeWebhookSignature(body, v.Secret, v.T) != v.V1 || ComputeWebhookSignature([]byte(v.BodyUTF8), v.Secret, v.T) != v.V1 {
		t.Fatal("v1")
	}
}

func TestSharedVectorCases(t *testing.T) {
	v, body := loadVectors(t)
	secrets := map[string]string{"current": v.Secret, "previous": v.PreviousSecret, "other": v.OtherSecret, "empty": ""}
	headers := map[string]string{"header": v.Header, "rotating": v.HeaderRotating}
	for _, c := range v.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			header, ok := headers[c.Header]
			if !ok {
				header = c.Header
			}
			payload := body
			if c.Body == "reserialised" {
				payload = []byte(v.BodyReserialised)
			}
			now := v.T
			if c.Now != nil {
				now = *c.Now
			}
			opts := []VerifyOption{WithNow(now)}
			if c.ToleranceSeconds != nil {
				opts = append(opts, WithTolerance(*c.ToleranceSeconds))
			}
			event, err := VerifyWebhook(payload, signatureHeaders(header), secrets[c.Secret], opts...)
			if c.Expect == "ok" {
				if err != nil {
					t.Fatal(err)
				}
				if event.ID != v.Event.ID || string(event.Type) != v.Event.Type || string(event.Mode) != v.Event.Mode {
					t.Fatalf("%+v", event)
				}
				return
			}
			var we *WebhookError
			if !errors.As(err, &we) {
				t.Fatalf("expected a WebhookError, got %T %v", err, err)
			}
			contains(t, err.Error(), vectorMessages[c.Expect])
		})
	}
}

func TestParsesTheHeaderIgnoringUnknownSchemesAndJunkParts(t *testing.T) {
	v, _ := loadVectors(t)
	for _, p := range v.ParseHeader {
		ts, sigs, err := ParseSignatureHeader(p.Header)
		if err != nil || ts != p.Timestamp || !reflect.DeepEqual(sigs, p.Signatures) {
			t.Fatal(ts, sigs, err)
		}
	}
	if _, _, err := ParseSignatureHeader("t=99999999999999999999,v1=aa"); err == nil || !strings.Contains(err.Error(), "invalid t") {
		t.Fatal(err)
	}
}

// ── Go-specific inputs ──

func TestGoInputsAcceptHTTPHeaderInAnyNameCaseAndTheEventDecodes(t *testing.T) {
	v, body := loadVectors(t)
	for _, name := range []string{"YONA-SIGNATURE", "Yona-Signature", "yona-signature"} {
		h := http.Header{}
		h.Set(name, v.Header)
		event, err := VerifyWebhook(body, h, v.Secret, WithNow(v.T))
		if err != nil || event.ID == "" {
			t.Fatal(name, err)
		}
	}
	// Real request headers (as net/http canonicalises them) work too.
	req, _ := http.NewRequest("POST", "/webhooks/yona", nil)
	req.Header.Set(WebhookSignatureHeader, v.Header)
	event, err := VerifyWebhook(body, req.Header, v.Secret, WithNow(v.T))
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != WebhookEventInvoiceSigned || event.Version != 1 || event.OrganizationID == "" || event.Test {
		t.Fatalf("%+v", event)
	}
	var data struct {
		InvoiceID string `json:"invoiceId"`
		Note      string `json:"note"`
	}
	if err := event.DecodeData(&data); err != nil || data.InvoiceID != "0192f3a0-0a1b-7c2d-9e3f-4a5b6c7d8e9f" || data.Note != "naïve €" {
		t.Fatal(data, err)
	}
	if m := event.DataMap(); m["invoiceId"] != data.InvoiceID {
		t.Fatal(m)
	}
}

func TestGoInputsRefuseANonJSONBodyAndNilHeaders(t *testing.T) {
	v, body := loadVectors(t)
	raw := []byte("not json")
	_, err := VerifyWebhook(raw, signatureHeaders(SignWebhookPayload(raw, []string{v.Secret}, v.T)), v.Secret, WithNow(v.T))
	var we *WebhookError
	if !errors.As(err, &we) {
		t.Fatal(err)
	}
	contains(t, err.Error(), "not JSON")
	_, err = VerifyWebhook(body, nil, v.Secret, WithNow(v.T))
	if !errors.As(err, &we) {
		t.Fatal(err)
	}
	contains(t, err.Error(), "Missing")
	var sdkErr Error
	if !errors.As(err, &sdkErr) {
		t.Fatal("WebhookError implements einvoice.Error")
	}
}

func TestGoInputsUseTheCurrentClockByDefault(t *testing.T) {
	v, body := loadVectors(t)
	now := time.Now().Unix()
	header := SignWebhookPayload(body, []string{v.Secret}, now)
	if _, err := VerifyWebhook(body, signatureHeaders(header), v.Secret); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(SignWebhookPayload(body, []string{v.Secret}, 0), "t=") {
		t.Fatal("a zero timestamp means now")
	}
	if _, err := VerifyWebhook(body, signatureHeaders(v.Header), v.Secret); err == nil {
		t.Fatal("a 2026 timestamp is outside the tolerance now")
	}
}

func TestWebhookEventTypeCatalogue(t *testing.T) {
	if WebhookEventInvoiceAccepted != "invoice.accepted" || WebhookEventBillingSubscriptionTrialEnding != "billing.subscription.trial_ending" {
		t.Fatal("catalogue constants")
	}
	if WebhookEventWebhookEndpointFlagged != "webhook_endpoint.flagged" {
		t.Fatal("catalogue end")
	}
}
