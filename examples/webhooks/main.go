// @recipe webhooks Receive webhooks
// @summary Verify the Yona-Signature of each delivery over the raw body, dispatch by event type, and inspect deliveries with the SDK. Endpoints are registered in the dashboard: an API key can read and test them, not create them.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()

	// @step secret Keep the endpoint secret
	// @text Register your endpoint URL in the dashboard (Developers, Webhooks); it shows the signing secret (whsec_…) once. Keep it in your environment.
	secret := os.Getenv("YONA_WEBHOOK_SECRET")
	if secret == "" {
		secret = "whsec_local_example_secret"
	}

	// @step read-raw-body Read the raw body
	// @text The signature covers the exact bytes Yona sent. Read the body as bytes; never verify a re-serialised object.
	readRawBody := func(r *http.Request) ([]byte, error) {
		defer r.Body.Close()
		return io.ReadAll(r.Body)
	}

	// @step dispatch Dispatch by event type
	// @text Deduplicate on event.ID: a redelivery carries the same id. Answer 2xx quickly and do slow work afterwards.
	seen := map[string]bool{}
	handleEvent := func(event *einvoice.WebhookEvent) {
		if seen[event.ID] {
			return
		}
		seen[event.ID] = true
		var data struct {
			InvoiceID string `json:"invoiceId"`
		}
		_ = event.DecodeData(&data)
		switch event.Type {
		case einvoice.WebhookEventInvoiceAccepted:
			fmt.Println("accepted", data.InvoiceID)
		case einvoice.WebhookEventInvoiceRejected:
			fmt.Println("rejected", data.InvoiceID)
		case einvoice.WebhookEventInvoiceReceived:
			fmt.Println("a supplier sent you an invoice")
		default:
			fmt.Println("ignored", event.Type)
		}
	}

	// @step handler Verify every delivery
	// @text VerifyWebhook checks the HMAC-SHA256 signature and the timestamp (±300 s) and returns the parsed event. Refuse anything it rejects with 400.
	mux := http.NewServeMux()
	mux.HandleFunc("/webhooks/yona", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, err := readRawBody(r)
		if err != nil {
			http.Error(w, "unreadable body", http.StatusBadRequest)
			return
		}
		event, err := einvoice.VerifyWebhook(raw, r.Header, secret)
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
		handleEvent(event)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	url := "http://" + listener.Addr().String() + "/webhooks/yona"

	// @step test-locally Test your handler locally
	// @text SignWebhookPayload signs a payload exactly as Yona does, so you can exercise the handler before going live. A tampered body is refused.
	body, _ := json.Marshal(map[string]any{
		"id": "evt_local_1", "type": "invoice.accepted", "version": 1, "createdAt": time.Now().UTC().Format(time.RFC3339),
		"mode": "sandbox", "organizationId": "org_local", "data": map[string]any{"invoiceId": "inv_local"}, "test": true,
	})
	deliver := func(payload []byte) int {
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		req.Header.Set(einvoice.WebhookSignatureHeader, einvoice.SignWebhookPayload(body, []string{secret}, 0))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	signed := deliver(body)
	tampered := deliver(bytes.Replace(body, []byte("accepted"), []byte("rejected"), 1))
	fmt.Println("signed delivery", signed, "| tampered delivery", tampered)
	if signed != http.StatusOK || tampered != http.StatusBadRequest {
		log.Fatal("the handler did not verify as expected")
	}
	_ = server.Shutdown(ctx)

	// @step list-endpoints List your endpoints
	// @text The key reads the endpoints registered for its organisation and mode.
	// @op listWebhookEndpoints
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	endpoints, err := yona.Webhooks.Endpoints.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("endpoints", len(endpoints))

	// @step list-deliveries Inspect deliveries
	// @text Every delivery and its status: delivered, retrying or failed with the reason.
	// @op listWebhookDeliveries
	deliveries, err := yona.Webhooks.Deliveries.List(ctx, &einvoice.ListWebhookDeliveriesQuery{Limit: einvoice.Ptr(int64(10))})
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range deliveries.Data {
		lastStatus := "-"
		if d.LastHTTPStatus != nil {
			lastStatus = fmt.Sprint(*d.LastHTTPStatus)
		}
		fmt.Println(d.Type, d.Status, lastStatus)
	}

	// @step redeliver Redeliver a failed delivery
	// @text After fixing your endpoint, ask for one more attempt.
	// @op redeliverWebhookDelivery
	redelivered := "nothing to redeliver"
	for _, d := range deliveries.Data {
		if d.Status == einvoice.WebhookDeliveryDtoStatusFailed {
			if _, err := yona.Webhooks.Deliveries.Redeliver(ctx, d.ID); err != nil {
				log.Fatal(err)
			}
			redelivered = d.ID
			break
		}
	}
	fmt.Println("redelivered", redelivered)
}
