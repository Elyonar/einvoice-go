# einvoice-go

The official Go SDK for the [Yona](https://useyona.com) e-invoicing API: create and manage invoices,
report them to the tax authority, and work with buyers, items, received invoices, billing and
webhooks. Standard library only, Go 1.22+.

Every Yona SDK has the same modules and methods; in Go the names are `PascalCase`
(`client.Invoices.IssueCreditNote`).

## Install

```bash
go get github.com/elyonar/einvoice-go
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()
	client, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// 1. A buyer
	buyer, err := client.Buyers.Create(ctx, &einvoice.CreateBuyerBody{
		Name:      "Acme Nigeria Ltd",
		TaxID:     einvoice.Ptr("12345678-0001"),
		Email:     einvoice.Ptr("accounts@acme.ng"),
		PartyType: einvoice.Ptr(einvoice.CreateBuyerDtoPartyTypeCompany),
		Address:   &einvoice.BuyerAddressDto{Line1: "1 Marina", City: "Lagos", Country: "NG"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. A saved item
	item, err := client.Items.Create(ctx, &einvoice.CreateItemBody{
		Name:            "Laptop",
		ItemType:        einvoice.CreateItemDtoItemTypeGoods,
		HSNCode:         einvoice.Ptr("8471.30"),
		ProductCategory: "Machinery",
		UnitCode:        "EA",
		UnitPriceMinor:  "45000000",
		Currency:        "NGN",
		TaxCategory:     "STANDARD_VAT",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 3. A draft invoice
	invoice, err := client.Invoices.Create(ctx, &einvoice.CreateInvoiceBody{
		InvoiceKind: "B2B",
		InvoiceDate: "2026-10-05",
		Currency:    "NGN",
		BuyerID:     einvoice.Ptr(buyer.ID),
		LineItems:   []einvoice.InvoiceLineItemDto{{ItemID: einvoice.Ptr(item.ID), Quantity: einvoice.AmountNumber(2)}},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 4. Finalise it and report it to the tax authority
	if _, err := client.Invoices.Finalise(ctx, invoice.ID); err != nil {
		log.Fatal(err)
	}
	if _, err := client.Submissions.Submit(ctx, invoice.ID, nil); err != nil {
		log.Fatal(err)
	}
	status, err := client.Submissions.GetStatus(ctx, invoice.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.Status)
}
```

Every method takes a `context.Context` first; cancel it or give it a deadline and the request stops.

## Sandbox and live

The key is the only configuration. An `sk_test_…` key works in the sandbox and an `sk_live_…` key
in live, on the same host; `client.Mode()` tells you which. Going live means deploying a live key,
nothing else changes.

To make a deployment refuse a key of the wrong kind:

```go
client, err := einvoice.New(key, einvoice.WithAssertMode(einvoice.ModeLive))
```

## Responses and pagination

Methods return the API's answer as a typed struct (`invoice.InvoiceNumber`). Optional fields are
pointers; `einvoice.Ptr(v)` makes one when building a request. List methods return a page:

```go
page, err := client.Buyers.List(ctx, &einvoice.ListBuyersQuery{Limit: einvoice.Ptr(50.0)})
page.Data       // []einvoice.BuyerViewDto
page.Pagination // Total, Page, PageSize, TotalPages, HasNext, HasPrevious
```

`Paginate` walks every page for you:

```go
query := &einvoice.ListBuyersQuery{Limit: einvoice.Ptr(100.0)}
err := einvoice.Paginate(ctx, func(ctx context.Context, page int64) (*einvoice.Page[einvoice.BuyerViewDto], error) {
	query.Page = einvoice.Ptr(float64(page))
	return client.Buyers.List(ctx, query)
}, func(buyer einvoice.BuyerViewDto) error {
	fmt.Println(buyer.Name)
	return nil
})
```

Received invoices and issued history return `*Paginated[D]` with the items under `Data.Items`. PDF
downloads return `*BinaryResponse` (`Data`, `ContentType`, `FileName`). Quantities and prices the API
accepts as a string or a number are `einvoice.Amount` (`AmountString("25000.00")`, `AmountNumber(2)`).
Two answers come in one of two shapes (`IssueCreditNoteData`, `IssueDebitNoteData`); decode them with
their `As…()` methods.

## Errors

A refused request returns a typed error you can match with `errors.As`:

| Status | Type |
|---|---|
| 400, 422 | `*ValidationError` |
| 401 | `*AuthenticationError` |
| 402 | `*InsufficientCreditsError` |
| 403 | `*PermissionError` |
| 404 | `*NotFoundError` |
| 409 | `*ConflictError` |
| 429 | `*RateLimitError` |
| 5xx | `*ServerError` |

All of them unwrap to `*APIError`, which carries `Status`, `ErrorCode`, `Errors` (per field),
`RequestID` and `RetryAfter`:

```go
_, err := client.Invoices.Create(ctx, params)
var validation *einvoice.ValidationError
var apiErr *einvoice.APIError
switch {
case errors.As(err, &validation):
	for _, e := range validation.Errors {
		fmt.Println(e.Field, e.Message)
	}
case errors.As(err, &apiErr):
	fmt.Println(apiErr.Status, apiErr.ErrorCode, apiErr.RequestID) // quote RequestID to support
}
```

Branch on `ErrorCode`, not on the message. Problems before the API answers are `*TimeoutError`,
`*ConnectionError` and `*ConfigError`; a cancelled context comes back as `ctx.Err()`.

## Retries and idempotency

Reads, and writes that carry an `Idempotency-Key`, are retried automatically on network errors,
timeouts, 408, 429 and 5xx, with backoff and respecting `Retry-After`. Other writes are never retried.

Where the API accepts an `Idempotency-Key` (creating an invoice, submitting, sending, downloading…)
the SDK generates one per call, so a retry is never applied or charged twice. Pass your own to make a
retry safe across restarts:

```go
client.Invoices.Create(ctx, params, einvoice.WithIdempotencyKey("order-"+orderID))
```

Tune the defaults with `WithTimeout` and `WithRetry` when creating the client, or per call with
`WithRequestTimeout`, `WithRequestHeaders` and `WithMaxRetries`.

## Webhooks

```go
http.HandleFunc("/webhooks/yona", func(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	event, err := einvoice.VerifyWebhook(raw, r.Header, os.Getenv("YONA_WEBHOOK_SECRET"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if event.Type == einvoice.WebhookEventInvoiceAccepted {
		var data struct{ InvoiceID string `json:"invoiceId"` }
		_ = event.DecodeData(&data)
	}
	w.WriteHeader(http.StatusOK)
})
```

`VerifyWebhook` checks the `Yona-Signature` header over the raw body and returns the event. Always
pass the raw bytes, and deduplicate on `event.ID`. `SignWebhookPayload` signs a payload the way Yona
does, so you can test your handler locally. Endpoints are registered in the dashboard; the SDK can
list and test them and inspect deliveries.

## Guides

[`examples/`](examples/) holds complete, runnable walkthroughs: your first invoice, webhooks, errors
and retries, sandbox and live, received invoices. Run them against your sandbox key with
`YONA_API_KEY=sk_test_… make examples`.

## API reference

Every method takes `ctx` first and optional trailing `...RequestOption`. Request and query types are
named after the operation (`*CreateInvoiceBody`, `*ListInvoicesQuery`); a query may be nil.

| Module | Methods |
|---|---|
| `client.Invoices` | `Create`, `List`, `Get`, `Update`, `Delete`, `Finalise`, `Reopen`, `Revise`, `Cancel`, `IssueCreditNote`, `IssueDebitNote`, `GetOverview`, `GetStatistics`, `GetSummary` |
| `client.Submissions` | `Submit`, `Issue`, `CreateAndSubmit`, `BatchSubmit`, `Retry`, `Renumber`, `QueryStatus`, `GetStatus`, `RecordPayment`, `GetAuthorityCopy` |
| `client.Output` | `GetDownloadLink`, `DownloadPDF`, `Send` |
| `client.ShareLinks` | `Create`, `List`, `Revoke` |
| `client.Items` | `List`, `Create`, `Get`, `Update`, `Delete`, `Archive`, `Unarchive`, `ListUsedCodes` |
| `client.Reference` | `ListHsCodes`, `ListHsCodeCategories`, `ListResources`, `GetResource`, `LookupTaxID`, `ValidateInvoice` |
| `client.Buyers` | `Create`, `List`, `Get`, `Update`, `Delete`, `BulkDelete`, `VerifyTaxNumber`, `GetVerificationStatus`, `Search`, `CheckReachability`, `CheckWithTaxAuthority` |
| `client.Sellers` | `List`, `Get`, `GetVerificationStatus`, `Search` |
| `client.InboundInvoices` | `List`, `Get`, `GetAnalytics`, `ListHistoryRuns`, `GetHistoryRun` |
| `client.IssuedHistory` | `List`, `Get`, `DownloadPDF` |
| `client.InvoiceSettings`, `client.TaxConnection` | `Get` |
| `client.Organization` | `Get`, `GetReadiness` |
| `client.Billing.Accounts` | `GetMine`, `GetStats`, `CheckBalance` |
| `client.Billing.Payments` | `List`, `Get` |
| `client.Billing.Sandbox` | `ListTransactions`, `GetUsage` |
| `client.Billing.Statements` | `Get` |
| `client.Billing.Subscriptions` | `GetActive`, `List`, `Get`, `ListRenewals`, `PreviewPlanChange` |
| `client.Billing.Transactions` | `List`, `Get`, `GetUsageAnalytics`, `GetUsageByCostCode` |
| `client.Webhooks.Endpoints` | `List`, `Get`, `Test` |
| `client.Webhooks.Deliveries` | `List`, `Get`, `Redeliver` |
| `client.Webhooks.Events` | `List`, `Get`, `Redeliver` |
| `client.Webhooks.EventTypes` | `List` |
| package `einvoice` | `VerifyWebhook`, `SignWebhookPayload`, `ComputeWebhookSignature`, `ParseSignatureHeader`, `Paginate`, `Collect` |

Account management (users, roles, API keys, purchases, webhook endpoint settings) is done in the
Yona dashboard, not through the API key.

## Configuration

| Option | Purpose |
|---|---|
| `WithTimeout(d)` | per-attempt timeout (default 30 s) |
| `WithRetry(RetryConfig{…})` | `MaxRetries` (2), `BaseDelay` (500 ms), `MaxDelay` (8 s), `MaxRetryAfter` (60 s) |
| `WithHeaders(map)` | headers sent on every request |
| `WithHTTPClient(*http.Client)` | your own client (proxy, TLS) |
| `WithBaseURL(url)` | another gateway, for local development only; the key decides sandbox or live |

## Development

```bash
make test        # go test ./... -cover
make lint        # gofmt + go vet
make examples    # run the examples against your sandbox key (YONA_API_KEY)
make sync        # regenerate the typed models from the API definition
```

## License

MIT
