# einvoice-go

Official Go SDK for the Yona e-invoicing API. It covers exactly what an API key may call:
invoicing, submissions to the tax authority, output and share links, items, reference data, buyers,
the read-only seller, received invoices and issued history, invoice settings, the tax connection, the
organisation (read), billing reads and webhooks (read, test, redeliver).

The same surface as [`@useyona/einvoice-js`](https://github.com/Elyonar/einvoice-js), module for
module, with the same names in Go's `PascalCase`.

## Features

- **One key, nothing else to configure.** `sk_test_…` is the sandbox, `sk_live_…` is live, on the same host.
- **Typed.** Every request and response shape is a struct generated from the API's OpenAPI (`types_gen.go`).
- **Safe retries.** GET, PUT, DELETE and writes carrying an `Idempotency-Key` are retried on network
  errors, timeouts, 408, 429 and 5xx, honouring `Retry-After`. The SDK generates the key on the
  routes that accept one, so its own retry is never charged twice.
- **Typed errors.** `*ValidationError`, `*NotFoundError`, `*RateLimitError`… all unwrap to `*APIError`
  with `Status`, `ErrorCode`, `Errors`, `RequestID` and `RetryAfter`; narrow with `errors.As`.
- **Webhooks.** `VerifyWebhook` checks the `Yona-Signature` over the raw body in constant time.
- **Light.** Standard library only. Go 1.22+.

## Installation

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
	fmt.Println(client.Mode()) // einvoice.ModeSandbox for sk_test_… keys, einvoice.ModeLive for sk_live_… keys

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

	// 2. A saved item (optional: a line can also carry its own description, unit code and codes)
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

	// 3. A draft invoice: InvoiceKind and TaxCategory, never a tax percent
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

	// 4. Finalise and report it to the tax authority
	if _, err := client.Invoices.Finalise(ctx, invoice.ID); err != nil {
		log.Fatal(err)
	}
	if _, err := client.Submissions.Submit(ctx, invoice.ID, nil); err != nil { // 202: queued
		log.Fatal(err)
	}
	status, err := client.Submissions.GetStatus(ctx, invoice.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.Status)
}
```

That is all the configuration an integration needs: the key. The SDK talks to the production
gateway for both modes; the key's prefix decides whether you are in the sandbox or live.

To fail fast when a deployment is given the wrong key:

```go
client, err := einvoice.New(key, einvoice.WithAssertMode(einvoice.ModeLive)) // *ConfigError for an sk_test_ key
```

A malformed key is refused by `New` with a `*ConfigError` (the key is never echoed).

Every method takes a `context.Context` first: cancel it or give it a deadline and the request stops;
a cancelled context is never retried and comes back as `ctx.Err()`.

## Guides

The recipes in [`examples/`](examples/) are the developer guides of the Yona portal (Developers,
Overview): your first invoice, webhooks, errors and retries, sandbox and live, received invoices.
Each runs end to end against a sandbox key (`YONA_API_KEY=sk_test_… make examples`).

## Verifying your setup

```bash
YONA_API_KEY=sk_test_… make smoke-remote
```

runs free reads across every module and then the first-invoice example against the real API, with
your own sandbox key, and prints a table method → OK/FAIL with the error code and request id of any
failure. It refuses a live key and never prints the key. The example creates sandbox data in your
organisation: a buyer, an item and an invoice submitted to the tax authority's sandbox.

## Responses and pagination

Methods return a pointer to the API's `data` as a struct with the wire names in its json tags
(`invoice.InvoiceNumber`). Optional and nullable fields are pointers; `einvoice.Ptr(v)` makes one
when building a request. List methods return a `*Page[T]`:

```go
page, err := client.Buyers.List(ctx, &einvoice.ListBuyersQuery{Limit: einvoice.Ptr(50.0)})
page.Data       // []einvoice.BuyerViewDto
page.Pagination // Total, Page, PageSize, TotalPages, HasNext, HasPrevious
page.RequestID
```

`Paginate` walks every page for you; `Collect` gathers them into one slice:

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

Received invoices and issued history answer an object with `Items`, so they return a `*Paginated[D]`:
`res.Data.Items` and `res.Pagination` (a pointer; nil when the answer carried none).

PDF downloads return a `*BinaryResponse`: `Data` (bytes), `ContentType`, `FileName`, `RequestID`.

A few answers are one of two shapes (`IssueCreditNoteData`, `IssueDebitNoteData`): they keep the
JSON and decode on demand with `AsCreditNotePreviewAnswerDto()` / `AsCreditNoteIssuedDto()`. Values
the API types as `string | number` (quantities, unit prices) are `einvoice.Amount`: `String()` keeps
the exact text, `Float64()` parses it; build one with `AmountString` or `AmountNumber`.

## Errors

Every refusal of the API is a typed wrapper chosen by status, each unwrapping to `*APIError`:

| Status | Type | Typical `ErrorCode` |
|---|---|---|
| 400, 422 | `*ValidationError` | `VAL…` |
| 401 | `*AuthenticationError` | `AUTH…` (a revoked, expired or malformed key) |
| 402 | `*InsufficientCreditsError` | `BIZ001` |
| 403 | `*PermissionError` | `AUTH019` (capability), `AUTH018` (user-only route) |
| 404 | `*NotFoundError` | `RES001` |
| 409 | `*ConflictError` | `BIZ…` (state), `RES002` (duplicate) |
| 429 | `*RateLimitError` | `SYS005`, with `RetryAfter` |
| 5xx | `*ServerError` | `SYS001` |

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

Branch on `ErrorCode`, never on the message. Outside the API: `*TimeoutError`, `*ConnectionError`
(unwraps to the transport's error), `*ConfigError`, `*WebhookError`. Every error the SDK returns
implements the `einvoice.Error` interface; a cancelled context is returned as `ctx.Err()`.

## Retries and idempotency

```go
client, err := einvoice.New(key,
	einvoice.WithTimeout(30*time.Second),
	einvoice.WithRetry(einvoice.RetryConfig{MaxRetries: einvoice.Ptr(3), BaseDelay: 500 * time.Millisecond, MaxDelay: 8 * time.Second, MaxRetryAfter: 30 * time.Second}),
)
```

The SDK retries GET, PUT and DELETE, and writes that carry an `Idempotency-Key`, on network errors,
timeouts, 408, 429 and 5xx, with exponential backoff and jitter; a `Retry-After` on 429/503 is
waited out up to `MaxRetryAfter` (a longer one is returned at once, with `RetryAfter` set). Other
writes are never retried for you. `WithTimeout` bounds each attempt, headers and body.

On routes that accept an `Idempotency-Key` (`Invoices.Create`, `Submissions.Submit`, `Output.Send`,
the downloads…) the SDK generates one per call, so its own retries are never applied or charged
twice. Pass your own to make a retry across process restarts safe:

```go
client.Invoices.Create(ctx, params, einvoice.WithIdempotencyKey("order-"+orderID))
```

The other per-call options are `WithRequestTimeout`, `WithRequestHeaders` and `WithMaxRetries`.

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

Yona signs `t + "." + raw body` with HMAC-SHA256 and sends `Yona-Signature: t=…,v1=…` (a second
`v1=` while a rotated secret overlaps). `VerifyWebhook` accepts the delivery when a signature matches
in constant time and `t` is within ±300 s (`WithTolerance`, `WithNow`), and returns the parsed
`*WebhookEvent`. Pass the raw body bytes, never a re-serialised object. Deduplicate on `event.ID`.
`SignWebhookPayload` signs a payload exactly as Yona does, for testing your handler. Endpoints are
registered in the dashboard; the key can list and test them (`client.Webhooks.Endpoints`), read
deliveries and redeliver.

## What an API key cannot do

Users, invitations, roles, API keys, organisation management, collections, purchases and webhook
endpoint writes are done by a signed-in user in the dashboard; the API answers 403 `AUTH018` to a
key. The seller is the organisation itself (read-only; `Sellers.Create/Update/Delete` are 409
`BIZ205` and have no method). `ExcludedOperations` lists the API-key operations the SDK deliberately
has no method for, with the reason.

## API reference

Every method takes `ctx context.Context` first and optional trailing `...RequestOption`. Query and
body parameters are the generated structs named after the operation (`*ListInvoicesQuery`,
`*CreateInvoiceBody`); a query may be nil.

#### `client.Invoices`
`Create(params)`, `List(query)`, `Get(id)`, `Update(id, params)`, `Delete(id)`, `Finalise(id)`, `Reopen(id)`, `Revise(id)`, `Cancel(id, params)`, `IssueCreditNote(id, params)`, `IssueDebitNote(id, params)`, `GetOverview(query)`, `GetStatistics(query)`, `GetSummary(query)`

#### `client.Submissions`
`Submit(id, query)`, `Issue(id)`, `CreateAndSubmit(params)`, `BatchSubmit(params)`, `Retry(id)`, `Renumber(id)`, `QueryStatus(id)`, `GetStatus(id)`, `RecordPayment(id, params)`, `GetAuthorityCopy(id)`

#### `client.Output`
`GetDownloadLink(id)`, `DownloadPDF(id)` → `*BinaryResponse`, `Send(id, params)`

#### `client.ShareLinks`
`Create(invoiceID, params)`, `List(invoiceID)`, `Revoke(invoiceID, linkID)`

#### `client.Items`
`List(query)`, `Create(params)`, `Get(id)`, `Update(id, params)`, `Delete(id)`, `Archive(id)`, `Unarchive(id)`, `ListUsedCodes(query)`

#### `client.Reference`
`ListHsCodes(query)`, `ListHsCodeCategories()`, `ListResources()`, `GetResource(listType)`, `LookupTaxID(value, query)`, `ValidateInvoice(params)`

#### `client.Buyers`
`Create(params)`, `List(query)`, `Get(id)`, `Update(id, params)`, `Delete(id)`, `BulkDelete(params)`, `VerifyTaxNumber(id)`, `GetVerificationStatus(id)`, `Search(query)`, `CheckReachability(query)`, `CheckWithTaxAuthority(params)`

#### `client.Sellers`
`List(query)`, `Get(id)`, `GetVerificationStatus(id)`, `Search(query)`

#### `client.InboundInvoices`
`List(query)` → `*Paginated`, `Get(id)`, `GetAnalytics(query)`, `ListHistoryRuns(query)` → `*Paginated`, `GetHistoryRun(runID)`, `LoadOlder()` (deprecated)

#### `client.IssuedHistory`
`List(query)` → `*Paginated`, `Get(id)`, `DownloadPDF(id)` → `*BinaryResponse`

#### `client.InvoiceSettings` · `client.TaxConnection`
`Get()`

#### `client.Organization`
`Get(orgID)`, `GetReadiness()`

#### `client.Billing.Accounts`
`GetMine(query)`, `GetStats(accountID, query)` (`""` is `me`), `CheckBalance(accountID, query)`

#### `client.Billing.Payments`
`List(query)`, `Get(id)`

#### `client.Billing.Sandbox`
`ListTransactions(query)`, `GetUsage()`

#### `client.Billing.Statements`
`Get(period, query)`

#### `client.Billing.Subscriptions`
`GetActive()`, `List(query)`, `Get(id)`, `ListRenewals(query)`, `PreviewPlanChange(id, query)`

#### `client.Billing.Transactions`
`List(query)`, `Get(id)`, `GetUsageAnalytics(query)`, `GetUsageByCostCode(query)`

#### `client.Webhooks.Endpoints`
`List()`, `Get(id)`, `Test(id, params)` (nil sends `{}`)

#### `client.Webhooks.Deliveries`
`List(query)`, `Get(id)`, `Redeliver(id)`

#### `client.Webhooks.Events`
`List(query)`, `Get(id)`, `Redeliver(id, params)`

#### `client.Webhooks.EventTypes`
`List()`

#### Webhook verification (package level)
`VerifyWebhook(payload, headers, secret, ...VerifyOption)` with `WithTolerance(seconds)`, `WithNow(unix)`; `SignWebhookPayload(payload, secrets, timestamp)` (0 = now); `ComputeWebhookSignature(payload, secret, timestamp)`; `ParseSignatureHeader(header)`

## Advanced: `WithBaseURL`, `WithTransport`

`WithBaseURL` points the client at another gateway (a local one in development). It never changes
the mode: the key does. Never switch hosts by mode in your own code. `WithHTTPClient` sends with
your own `*http.Client`; `WithTransport` with anything that has `Do(*http.Request)` (a recorder in
tests). `client.HTTP.Request(ctx, einvoice.Call{...})` makes a raw call with the SDK's envelope,
retry and idempotency handling.

## Development

```bash
make test           # go test ./... -cover, parity and guides included
make lint           # gofmt + go vet (+ golangci-lint when installed)
make sync           # copy the snapshot and vectors from einvoice-js and regenerate types_gen.go (needs git + Node 22)
make sync-check     # CI: the three must match the einvoice-js commit (or tag) pinned in scripts/sync.sh
make guides         # examples/ → guides/guides.json (a test fails when it is stale)
make examples       # run the examples (YONA_API_KEY; YONA_BASE_URL to point elsewhere)
make smoke-remote   # verify your own sandbox key (see "Verifying your setup")
```

The models (`types_gen.go`), the API snapshot and the webhook test vectors come from
[`einvoice-js`](https://github.com/Elyonar/einvoice-js), the source of truth for every Yona SDK, at
the commit (or tag) pinned in `scripts/sync.sh`; `parity_test.go` fails when a method and the
snapshot disagree. The SDKs are versioned independently (`version.go`).

## License

MIT
