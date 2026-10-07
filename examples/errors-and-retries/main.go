// @recipe errors-and-retries Errors and safe retries
// @summary Catch the SDK's typed errors, branch on ErrorCode, quote the RequestID to support, honour RetryAfter, and use idempotency keys so a retried write is never applied or charged twice.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()

	// @step client Configure retries
	// @text The SDK retries GET, PUT and DELETE, and writes that carry an Idempotency-Key, on network errors, timeouts, 408, 429 and 5xx. Other writes are never retried for you.
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"),
		einvoice.WithTimeout(30*time.Second),
		einvoice.WithRetry(einvoice.RetryConfig{MaxRetries: einvoice.Ptr(3), BaseDelay: 500 * time.Millisecond, MaxRetryAfter: 30 * time.Second}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// @step validation Read a validation error
	// @text A 400 or 422 is a *ValidationError. Branch on ErrorCode, show Errors per field, and keep RequestID for support.
	// @op createInvoice
	_, err = yona.Invoices.Create(ctx, &einvoice.CreateInvoiceBody{InvoiceKind: "B2B", InvoiceDate: "2026-10-05", Currency: "NGN", LineItems: []einvoice.InvoiceLineItemDto{}})
	var validation *einvoice.ValidationError
	if !errors.As(err, &validation) {
		log.Fatal("expected a validation error, got ", err)
	}
	fmt.Println(validation.Status, validation.ErrorCode, validation.RequestID)
	for _, e := range validation.Errors {
		fmt.Printf("  %s: %s\n", e.Field, e.Message)
	}

	// @step not-found Handle a missing resource
	// @text Every API error unwraps to *APIError, so one errors.As can handle them all; narrow by type first.
	// @op getInvoice
	_, err = yona.Invoices.Get(ctx, "0192f3a0-0000-7000-8000-000000000000")
	var notFound *einvoice.NotFoundError
	var apiErr *einvoice.APIError
	switch {
	case errors.As(err, &notFound):
		fmt.Println("not found", notFound.ErrorCode, notFound.RequestID)
	case errors.As(err, &apiErr):
		fmt.Println("refused", apiErr.Status, apiErr.ErrorCode)
	default:
		log.Fatal("expected a not-found error, got ", err)
	}

	// @step idempotency Make a write safe to retry
	// @text Pass your own idempotency key (stored with your order, for example) so a retry after a crash or a timeout returns the first result instead of creating a second invoice.
	// @op createInvoice
	idempotencyKey := "order-" + einvoice.GenerateIdempotencyKey()
	params := &einvoice.CreateInvoiceBody{
		InvoiceKind: "B2C",
		InvoiceDate: time.Now().Format("2006-01-02"),
		Currency:    "NGN",
		LineItems: []einvoice.InvoiceLineItemDto{{
			ItemName:        einvoice.Ptr("Consulting hour"),
			Description:     einvoice.Ptr("Consulting hour"),
			UnitCode:        einvoice.Ptr("EA"),
			Quantity:        einvoice.AmountNumber(1),
			UnitPrice:       einvoice.Ptr(einvoice.AmountString("25000.00")),
			TaxCategory:     einvoice.Ptr("STANDARD_VAT"),
			ISICCode:        einvoice.Ptr("6201"),
			ServiceCategory: einvoice.Ptr("Professional services"),
		}},
	}
	first, err := yona.Invoices.Create(ctx, params, einvoice.WithIdempotencyKey(idempotencyKey))
	if err != nil {
		log.Fatal(err)
	}
	again, err := yona.Invoices.Create(ctx, params, einvoice.WithIdempotencyKey(idempotencyKey))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("same invoice on retry:", first.ID == again.ID)
	if first.ID != again.ID {
		log.Fatal("an idempotent retry created a second invoice")
	}

	// @step retry-after Wait out a rate limit
	// @text When the SDK's own retries are exhausted, a 429 or 503 carries RetryAfter (seconds). Retry only what is safe: reads, or writes with an idempotency key.
	withRetry := func(call func() (*einvoice.Page[einvoice.ListInvoicesItem], error), attempts int) (*einvoice.Page[einvoice.ListInvoicesItem], error) {
		for attempt := 1; ; attempt++ {
			page, err := call()
			var rateLimited *einvoice.RateLimitError
			var server *einvoice.ServerError
			var timeout *einvoice.TimeoutError
			var connection *einvoice.ConnectionError
			transient := errors.As(err, &rateLimited) || errors.As(err, &server) || errors.As(err, &timeout) || errors.As(err, &connection)
			if err == nil || !transient || attempt >= attempts {
				return page, err
			}
			seconds := 1 << attempt
			var refusal *einvoice.APIError
			if errors.As(err, &refusal) && refusal.RetryAfter > 0 {
				seconds = refusal.RetryAfter
			}
			time.Sleep(time.Duration(seconds) * time.Second)
		}
	}
	page, err := withRetry(func() (*einvoice.Page[einvoice.ListInvoicesItem], error) {
		return yona.Invoices.List(ctx, &einvoice.ListInvoicesQuery{Limit: einvoice.Ptr(5.0)})
	}, 3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("invoices", page.Pagination.Total, "request", page.RequestID)

	// @step cleanup Delete the draft
	// @text Drafts can be deleted; their number is never reused.
	// @op deleteInvoice
	if _, err := yona.Invoices.Delete(ctx, first.ID); err != nil {
		log.Fatal(err)
	}
}
