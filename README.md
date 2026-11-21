# einvoice-go

The official Go SDK for the [Yona](https://useyona.com) e-invoicing API: create and manage invoices,
report them to the tax authority, and work with buyers, items, received invoices, billing,
collections and webhooks. Standard library only, Go 1.22+.

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

Some codes to branch on:

| Status | `ErrorCode` | When |
|---|---|---|
| 400 | `VAL202`, `VAL203` | a buyer's address: a state that is not on the list (`VAL202`); an LGA that is not on the list, or of another state (`VAL203`) |
| 402 | `BIZ016` | `Organization.CreateSubOrganization`: the plan allows no more organisations |
| 403 | `AUTH019` | the key does not hold the capability the route needs |
| 403 | `BIZ003` | a sandbox key creating a sub-organisation |

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
list and test them, inspect deliveries and read how they have been arriving (`client.Webhooks.Health`).

## Guides

[`examples/`](examples/) holds seven complete, runnable walkthroughs: your first invoice, webhooks,
errors and retries, sandbox and live, received invoices, states and LGAs (the two lists, an LGA
picker filtered by state, a buyer saved with the codes, `VAL202` and `VAL203`) and sub-organisations
(listing them with a key, their readiness for live and their tax connections, creating one with a
live key). Run them against your sandbox key with `YONA_API_KEY=sk_test_… make examples`.

## API reference

Every method takes `ctx` first and optional trailing `...RequestOption`. Request and query types are
named after the operation (`*CreateInvoiceBody`, `*ListInvoicesQuery`); a query may be nil. 141
methods in 28 modules: einvoice-js's 139, plus `TaxConnection.GetWithQuery` and
`Billing.Partner.GetWithQuery` (see below).

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
| `client.InboundInvoices` | `List`, `Get`, `GetAnalytics`, `ListHistoryRuns`, `GetHistoryRun`, `LoadOlder` |
| `client.IssuedHistory` | `List`, `Get`, `DownloadPDF` |
| `client.InvoiceSettings` | `Get` |
| `client.TaxConnection` | `Get`, `GetWithQuery`, `ListForSubOrganizations` |
| `client.Organization` | `Get`, `GetReadiness`, `ListSubOrganizations`, `ListSubOrganizationsReadiness`, `CreateSubOrganization`, `GetGroupTerms`, `VerifySubOrganizationTaxID`, `SetSubOrganizationContact`, `CreateSubOrganizationSetupLink`, `RevokeSubOrganizationSetupLink`, `SubmitSubOrganization` |
| `client.Billing.Accounts` | `GetMine`, `GetStats`, `CheckBalance`, `GetCreditCosts` |
| `client.Billing.Payments` | `List`, `Get` |
| `client.Billing.Sandbox` | `ListTransactions`, `GetUsage` |
| `client.Billing.Statements` | `Get` |
| `client.Billing.Subscriptions` | `GetActive`, `List`, `Get`, `ListRenewals`, `PreviewPlanChange`, `GetDealOffer`, `QuoteSubOrgSlot`, `ListAddOns`, `QuoteAddOn` |
| `client.Billing.Transactions` | `List`, `Get`, `GetUsageAnalytics`, `GetUsageByCostCode` |
| `client.Billing.Usage` | `GetInvoices`, `QuoteOverage` |
| `client.Billing.Plans` | `GetRecommendation` |
| `client.Billing.Partner` | `Get`, `GetWithQuery`, `GetOffer`, `QuoteSeat`, `QuoteBasket`, `QuotePlanChange` |
| `client.Collections` | `ListAccounts`, `CreateAccount`, `GetAccount`, `GetAccountStatus`, `DeactivateAccount`, `QuoteFee`, `ListPayments`, `ListBuyerCredits`, `GetSummary`, `GetDailySeries`, `ListExceptions`, `ResolveException`, `ListSettlementAccounts`, `GetSettlementAccount`, `ListBanks` |
| `client.Webhooks.Endpoints` | `List`, `Get`, `Test` |
| `client.Webhooks.Deliveries` | `List`, `Get`, `Redeliver` |
| `client.Webhooks.Events` | `List`, `Get`, `Redeliver` |
| `client.Webhooks.EventTypes` | `List` |
| `client.Webhooks.Health` | `Get`, `GetDaily` |
| package `einvoice` | `VerifyWebhook`, `SignWebhookPayload`, `ComputeWebhookSignature`, `ParseSignatureHeader`, `Paginate`, `Collect`, `CodeListItems` |

- `TaxConnection.GetWithQuery(ctx, &einvoice.GetTaxConnectionQuery{Identifier: einvoice.Ptr(einvoice.GetTaxConnectionQueryIdentifierTrue)})`
  also says which number the tax authority holds for the organisation, its TIN or its 13-digit Tax
  ID (`EinvoicingIdentifier`, or `IdentifierNextAction` when it holds none); `Fresh` asks again.
  Both are rationed. `Get(ctx, ...RequestOption)` is unchanged and makes no lookup (einvoice-js
  overloads `taxConnection.get`; Go adds a method instead).
- Held invoices: while an organisation's connection to the tax authority is still in progress, an
  invoice it issues is not refused. `Submissions.Issue` answers `Reporting: "held"` with
  `NotReportedReason: "awaiting_connection"`, the invoice is sent to the buyer, and Yona reports it
  automatically once the connection completes. Until then the invoice's `Reporting.State` is
  `held`; `Invoices.List` with `Reporting: einvoice.Ptr(einvoice.ListInvoicesQueryReportingHeld)`
  lists them and `Invoices.GetSummary` counts them in `Held`. If one cannot be reported when the
  connection completes, its `Reporting.Reason` is `release_refused`: report it yourself with
  `Submissions.Submit`.
- `TaxConnection.Get`: `State` can be `awaiting_authority_listing`. The business asked to connect
  and the tax authority does not list it under any of its numbers yet. Yona asks the authority
  again every 6 hours and carries on by itself once the business is listed. `Listing` then holds
  `Since`, `CheckedAt`, `NextCheckAt` and the last `Answer` (`unknown`, `elsewhere`, `unanswered`
  or `held_elsewhere`); it is nil in every other state. `ConnectWaits` says whether connecting now
  would wait like this instead of being refused. The `tax_connection.action_required` webhook is
  sent for this state too, and `invoice.finalised` can carry `reporting` (decode it into
  `einvoice.InvoiceFinalisedData`), so a handler can tell a held invoice from one being reported.
- `Reference.GetResource(ctx, "states")` and `GetResource(ctx, "lgas")` have a fixed item shape,
  which `einvoice.CodeListItems` decodes: a state is `StateOption` `{Code, Name}` (`NG-LA` Lagos)
  and an LGA is `LGAOption` `{Code, Name, StateCode}` (`NG-LA-IKE` Ikeja, in `NG-LA`);
  `"countries"` is `CountryOption` `{Code, Name}`. For a State and LGA picker, read both once and
  filter the LGAs by the chosen state's `Code`:
  `lgas, err := einvoice.CodeListItems[einvoice.LGAOption](list)`.
- `Buyers`: for a Nigerian address, `Address.State` and `Address.Lga` are a state and an LGA from
  those lists: the code, or the exact name, which is stored as the code. A value that is not on the
  list is refused with 400 `VAL202` (state) or `VAL203` (LGA, also when it belongs to another
  state). Both are optional, and free text for other countries.
- Sub-organisations: an organisation that runs several businesses holds them as sub-organisations.
  No key preset includes `organization.children.read` or `organization.children.create`: an Owner
  or Admin adds them to a key by name, under Sub-organisations in the dashboard's Create API Key
  dialog.
  - `Organization.ListSubOrganizations(ctx, &einvoice.ListSubOrganizationsQuery{Page, Limit, Search, IDs})`
    (`GET /a/v1/organizations/me/children`) answers a page, newest first, with the total in
    `Pagination`; 100 a page at most. `IDs` (a list, at most 100) keeps only the ones named. It
    works in sandbox and live, and with `Paginate`.
  - `Organization.ListSubOrganizationsReadiness` (`GET /a/v1/organizations/me/children/readiness`)
    answers the same page with each sub-organisation's checklist, `LiveStatus` and `NextStep`.
  - `Organization.CreateSubOrganization(ctx, &einvoice.CreateSubOrganizationBody{BusinessName, Email, PhoneNumber, Website, ExternalReference})`
    (`POST /a/v1/organizations`) needs a live key (a sandbox key is refused with 403 `BIZ003`). The
    new organisation is recorded under the parent's longest-standing Owner and is
    `pending_verification` until it completes its own onboarding. An `Idempotency-Key` is sent
    (yours from `WithIdempotencyKey`, or one the SDK makes), so a retried call answers the
    organisation already made, with `Replayed` true; the same key with other details is 409
    `BIZ107`. `ExternalReference` is your own id for the business: the same reference with the same
    name also answers the business already made. A name is unique under its parent (409 `RES002`).
    402 `BIZ016` when the plan allows no more organisations; 60 calls a minute for each key.
  - `TaxConnection.ListForSubOrganizations(ctx, &einvoice.SubOrganizationTaxConnectionsQuery{IDs: ids})`
    (`GET /i/v1/tax-connection/children?ids=`, `tax_connection.read`) is for a parent organisation:
    the state of each named sub-organisation's connection in the key's mode, and whether a
    submission would pass. At most 100 ids, each once. An id that is not one of the organisation's
    sub-organisations is left out of the answer.
  - `IDs` is an `einvoice.IDs`, a `[]string`: `IDs: []string{"a", "b"}`. It is sent as one
    comma-separated value (`ids=a,b`), as the API reads it, never as a repeated key.
  - The parent's `organization.child.created` webhook carries `keyId` when a key created the
    sub-organisation: decode it into `einvoice.OrganizationChildCreatedData`.
  - A key that also holds `organization.children.onboard` takes the businesses it creates through
    managed onboarding: see [Onboarding your businesses](#onboarding-your-businesses-managed-onboarding).
- `Billing.Partner.GetWithQuery(ctx, &einvoice.GetPartnerQuery{Page: einvoice.Ptr(int64(2)), Limit: einvoice.Ptr(int64(50))})`
  (`GET /b/v1/partner?page=&limit=`) reads another page of the businesses the partner pays for;
  the answer's `BusinessesPage` says which page and how many there are in all. `Get(ctx,
  ...RequestOption)` is unchanged and answers the first page (einvoice-js overloads
  `billing.partner.get`; Go adds a method instead).
- `Collections` answers only where collections are switched on for the key's mode; elsewhere 409
  `BIZ120`. Its writes (`CreateAccount`, `DeactivateAccount`, `ResolveException`) take no
  `Idempotency-Key` and are not retried.
- `Collections.GetSummary(ctx, &einvoice.GetCollectionSummaryQuery{From, To})`
  (`GET /b/v1/collection/summary`) is the money received, fees, payouts and reconciliation between
  `From` and `To` (366 days at most), beside the window before it;
  `Collections.GetDailySeries` (`GET /b/v1/collection/summary/daily`) is the money received for
  each UTC day of the window. Both need `collection.payment.read`.
- `Billing.Accounts.GetCreditCosts` (`GET /b/v1/billing-accounts/me/credit-costs`,
  `billing.account.read`) is what each action costs the organisation, in credits, under its own
  plan and terms.
- `Billing.Subscriptions.ListAddOns` (`GET /b/v1/subscription/add-ons`) is the slot add-ons the
  organisation's plan offers and how many it holds;
  `Billing.Subscriptions.QuoteAddOn(ctx, kind, &einvoice.GetAddOnQuoteQuery{Slots: 5})`
  (`GET /b/v1/subscription/add-ons/{kind}/quote`) is what that many more of one would cost now.
  Both need `billing.subscription.read`; buying is done in the Yona app.
- `Webhooks.Health.Get(ctx, &einvoice.GetWebhookHealthQuery{From, To})` (`GET /n/v1/webhook-health`)
  counts the deliveries between `From` and `To` (31 days at most) for each endpoint and event
  type; `Webhooks.Health.GetDaily` (`GET /n/v1/webhook-health/daily`) counts them for each UTC day,
  and `EndpointID` keeps one endpoint. Both need `webhook.delivery.read`.
- `Billing.Usage.GetInvoices`, `Billing.Plans.GetRecommendation`, `Billing.Partner.GetOffer` and
  `Billing.Subscriptions.GetDealOffer` return nil (and no error) when the API answers `null`.
- The payments service's reads (bank account, transfers) join with its launch; until then they are
  in `ExcludedOperations`.

Account management (users, roles, API keys, purchases, webhook endpoint settings) is done in the
Yona dashboard, not through the API key. So is managing a sub-organisation (its profile, members,
onboarding, tax connection): in the dashboard, signed in to that sub-organisation. A key lists
sub-organisations, creates one and, when it holds `organization.children.onboard`, takes the ones
it creates through managed onboarding.

### Onboarding your businesses (managed onboarding)

A live key that also holds `organization.children.onboard` (in no preset; an Owner or Admin adds
it, after accepting the managed onboarding terms in the Yona app) takes each business it creates
from created to waiting for approval:

```go
business, err := client.Organization.CreateSubOrganization(ctx, &einvoice.CreateSubOrganizationBody{
	BusinessName:      "Bola Stores Limited",
	Email:             "accounts@bolastores.example",
	ExternalReference: einvoice.Ptr("crm-0042"), // your own id: the same reference again answers the same business
})
if err != nil {
	log.Fatal(err)
}
if _, err := client.Organization.VerifySubOrganizationTaxID(ctx, business.ID, &einvoice.VerifyChildOrganizationTaxIdBody{
	BusinessName: "Bola Stores Limited",
	TaxNo:        "1234567890123",
}); err != nil {
	log.Fatal(err)
}
if _, err := client.Organization.SetSubOrganizationContact(ctx, business.ID, &einvoice.SetChildOrganizationContactBody{
	PhoneNumber: "+2348031234567",
}); err != nil {
	log.Fatal(err)
}
link, err := client.Organization.CreateSubOrganizationSetupLink(ctx, business.ID, &einvoice.CreateChildOrganizationSetupLinkBody{
	ExpiresInDays: einvoice.Ptr(14.0),
})
if err != nil {
	log.Fatal(err)
}
// link.URL is answered once (it is nil on a repeated call): send it to the business owner now.
if _, err := client.Organization.SubmitSubOrganization(ctx, business.ID); err != nil {
	log.Fatal(err)
}
```

| Method | Route | Capability |
|---|---|---|
| `GetGroupTerms` | `GET /a/v1/organizations/me/group-terms` | `organization.children.read` |
| `VerifySubOrganizationTaxID` | `POST /a/v1/organizations/me/children/{childId}/onboarding/tax-id` | `organization.children.onboard` |
| `SetSubOrganizationContact` | `PUT /a/v1/organizations/me/children/{childId}/onboarding/contact` | `organization.children.onboard` |
| `CreateSubOrganizationSetupLink` | `POST /a/v1/organizations/me/children/{childId}/setup-link` | `organization.children.onboard` |
| `RevokeSubOrganizationSetupLink` | `DELETE /a/v1/organizations/me/children/{childId}/setup-link` | `organization.children.onboard` |
| `SubmitSubOrganization` | `POST /a/v1/organizations/me/children/{childId}/onboarding/submit` | `organization.children.onboard` |

- `Organization.GetGroupTerms` says whether the organisation has accepted the managed onboarding
  terms (`Accepted` is true only for the current version). An API key cannot accept them.
- The business owner opens the link, proves the business phone with a code sent by SMS and, once
  Yona has approved the business, gives the consent that lets Yona report its e-invoices.
  `SubmitSubOrganization` can be called before the owner has done anything: the business goes to
  review by itself when the phone is verified.
- Every call can be sent again. `CreateSubOrganization` and `CreateSubOrganizationSetupLink` send
  an `Idempotency-Key`; a repeated Tax ID check, contact or submit answers where the business
  stands, with `Replayed` true where nothing was done. A nil body for
  `CreateSubOrganizationSetupLink` gives the link the default life of 14 days (30 at most).
- A business the key may not onboard (not yours, one its own people set up, one Yona has approved)
  answers 404 `RES001`. After approval the key can only make a new setup link.
  `RevokeSubOrganizationSetupLink` withdraws the working link at once.
- The parent hears each step on its webhooks: `organization.child.tax_id_verified`,
  `.onboarding_submitted`, `.phone_verified`, `.live_access_approved`, `.live_access_rejected`
  (decode into `einvoice.OrganizationChildStepData`), and the business's tax connection:
  `.tax_connection_action_required` (`einvoice.OrganizationChildTaxConnectionActionRequiredData`),
  `.tax_connection_connected` (`einvoice.OrganizationChildStepData` too) and
  `.tax_connection_suspended` (`einvoice.OrganizationChildTaxConnectionSuspendedData`).

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
make operations  # regenerate guides/operations.json (the portal playground's Go call per operation)
make release     # prepare a release: version, CHANGELOG, guides, checks (DRY=1 to preview)
```

`guides/operations.json` is generated from the parity test's method registry, never edited by hand;
`make test` fails when it is stale or when any of its rendered snippets does not compile (`go vet`)
and run against a local fake gateway.

## License

MIT
