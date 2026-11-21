# Changelog

All notable changes to this project will be documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## 0.2.3 (2026-11-21)

In line with `@useyona/einvoice-js` 0.9.3 (the Yona API as of 21 Nov 2026): the 151 operations an
API key may call (138 before), 13 documented exclusions; 141 methods in 28 modules. No method is
removed or renamed. In the generated models one type is renamed (its earlier name still compiles)
and four fields are now pointers: see Changed.

### Added

- Managed onboarding: a parent organisation's live key takes each business it creates from created
  to waiting for approval. `client.Organization.VerifySubOrganizationTaxID`,
  `SetSubOrganizationContact`, `CreateSubOrganizationSetupLink`, `RevokeSubOrganizationSetupLink`
  and `SubmitSubOrganization`, with the new capability `organization.children.onboard` (in no
  preset). `client.Organization.GetGroupTerms` says whether the organisation has accepted the
  managed onboarding terms that allow it.
- The setup link's `URL` is answered once; a retried `CreateSubOrganizationSetupLink` answers the
  same link without it (`URL` nil, `Replayed` true).
- `CreateSubOrganizationBody.ExternalReference`, your own id for the business: the same reference
  with the same name answers the business already made. `Intent` is there too.
- Eight webhook types the parent hears: `organization.child.tax_id_verified`,
  `.onboarding_submitted`, `.phone_verified`, `.live_access_approved`, `.live_access_rejected`
  (`einvoice.OrganizationChildStepData`), and `.tax_connection_action_required`
  (`einvoice.OrganizationChildTaxConnectionActionRequiredData`), `.tax_connection_connected` and
  `.tax_connection_suspended` (`einvoice.OrganizationChildTaxConnectionSuspendedData`), each with
  its `WebhookEventOrganizationChild…` constant.
- `client.Billing.Accounts.GetCreditCosts`, `client.Billing.Subscriptions.ListAddOns` and
  `QuoteAddOn`, `client.Collections.GetSummary` and `GetDailySeries`, and the new module
  `client.Webhooks.Health` with `Get` and `GetDaily`: reads the API has opened to API keys.

### Changed

- `client.Organization.CreateSubOrganization` now sends an `Idempotency-Key` (yours from
  `WithIdempotencyKey`, or one the SDK makes), so a retried call answers the business already made,
  with `Replayed` true, and the SDK retries it like every keyed write.
- The generated models follow the API. No method is removed or renamed, and everything adds except
  the following.
  - Four fields are now pointers, because the API can answer `null` for them: check for nil before
    reading. `OverageQuoteDto.CapPercent` and `PricePercent` (`*int64`) and `PeriodEnd` (`*string`)
    are nil on a top-up; `SubOrgSlotQuoteDto.PercentBp` (`*int64`) is nil for a seat.
  - The API renamed a schema: the type of `AccountPlanDto.Entitlements` is now
    `AccountEntitlementsDto`, with the same three fields. `PlanEntitlementsDto` remains as a
    deprecated alias of it, so code that names it still compiles; it goes with the next breaking
    release.
- The payment kind gains the values `add_on_slots`, `metered_statement` and `metered_topup`
  (`PaymentDtoKind`, `PaymentDetailDtoKind`, `StatementPaymentDtoKind` and the
  `ListPaymentsQueryKind` filter).
- `guides/operations.json` covers 138 operations (125 before).

## 0.2.2 (2026-11-12)

In line with `@useyona/einvoice-js` 0.9.2: held invoices, and the tax connection waiting for the
tax authority to list the business. No new methods; every change adds.

### Added

- Held invoices. While an organisation's connection to the tax authority is still in progress, an
  invoice it issues is no longer refused: `Submissions.Issue` answers `Reporting` `held`
  (`IssueAnswerDtoReportingHeld`) with `NotReportedReason` `awaiting_connection`, the invoice is
  sent to the buyer, and Yona reports it automatically once the connection completes. Until then
  the invoice's `Reporting.State` is `held`. `Invoices.List` and `Invoices.GetSummary` take
  `Reporting` `held`, and the summary counts them in `Held`. `Reporting.Reason` `release_refused`
  says one could not be reported when the connection completed: report it with `Submissions.Submit`.
- The tax connection's `State` can be `awaiting_authority_listing`: the business asked to connect
  and the tax authority does not list it yet. Yona asks again every 6 hours and connects it by
  itself. `Listing` (`Since`, `CheckedAt`, `NextCheckAt`, `Answer`) says where that stands, nil in
  every other state, and `ConnectWaits` says whether connecting now would wait this way. The same
  state appears on `TaxConnection.ListForSubOrganizations` and in the invoice setup.
- `einvoice.InvoiceFinalisedData`: the payload of `invoice.finalised`, which may carry `Reporting`
  (`State`, `Reason`) so a webhook handler can tell a held invoice from one being reported.

## 0.2.1 (2026-11-11)

In line with `@useyona/einvoice-js` 0.9.1 (the Yona API as of 11 Nov 2026): the 138 operations an
API key may call (134 before), 13 documented exclusions.

### Added

- `client.Organization.ListSubOrganizations`, `ListSubOrganizationsReadiness` and
  `CreateSubOrganization`: a parent organisation's sub-organisations, how ready each is for live,
  and creating one. They need `organization.children.read` / `organization.children.create`, which
  no key preset includes; creating needs a live key (a sandbox key is refused with 403 `BIZ003`),
  sends no `Idempotency-Key` and is not retried (a repeat answers 409 `RES002`).
- `client.TaxConnection.ListForSubOrganizations(ctx, query, opts...)`: the connection state of up to
  100 sub-organisations, and whether a submission would pass.
- `einvoice.IDs`, a `[]string` for the `ids` of those queries (`IDs: []string{"a", "b"}`): it reaches
  the API as one comma-separated value (`ids=a,b`), never as a repeated key.
- `client.Billing.Partner.GetWithQuery(ctx, query, opts...)`: `Page` and `Limit` page the partner's
  businesses, and the answer carries `BusinessesPage`. `Get(ctx, opts...)` keeps its signature.
- `einvoice.CodeListItems[T]` with `StateOption`, `LGAOption` and `CountryOption`: the typed items of
  `client.Reference.GetResource(ctx, "states" | "lgas" | "countries")`. A buyer's Nigerian address
  takes those codes: 400 `VAL202` (state) or `VAL203` (LGA) otherwise.
- `einvoice.OrganizationChildCreatedData`: the payload of `organization.child.created`, which
  carries `keyId` when an API key created the sub-organisation.
- Two examples, also in `guides/guides.json`: `states-and-lgas` and `sub-organisations`.

### Changed

- The generated models follow the API; every change adds. The organisation address gains `lga`, the
  organisation `onboardingProfile`, readiness `profile` and `liveRejection`, the billing plan
  `coveredBy`, payments `netMinor`, `vatMinor`, `serviceFeeItem` and the kind `service_fee`, and
  plan-change quotes the reason `renewal_in_progress`.
- `guides/operations.json` covers 125 operations (121 before); `getPartner` is shown as
  `Billing.Partner.GetWithQuery`, which takes the operation's query.

## 0.2.0 (2026-11-04)

In line with `@useyona/einvoice-js` 0.9.0: the 134 operations an API key may call, 13 documented
exclusions.

### Added

- `client.Collections`: the bank accounts that collect an invoice's payment and what they received (accounts:
  list, create, get, status, deactivate; the fee quote; payments; buyer credits; exceptions: list,
  resolve; settlement accounts; banks). They answer only where collections are switched on for the
  key's mode; elsewhere 409 `BIZ120`.
- `client.Billing.Usage` (`GetInvoices`, `QuoteOverage`), `client.Billing.Plans` (`GetRecommendation`), `client.Billing.Partner` (`Get`, `GetOffer`, `QuoteSeat`, `QuoteBasket`, `QuotePlanChange`), and `client.Billing.Subscriptions.GetDealOffer` and `QuoteSubOrgSlot`. Answers that can be empty are `nil`.
- `client.TaxConnection.GetWithQuery(ctx, query, opts...)`: with `Identifier: "true"` or `Fresh: "true"`, which number the tax authority holds for the organisation, its TIN or its 13-digit Tax ID. `Get(ctx, opts...)` keeps its signature.
- `guides/operations.json`: for each of the 98 API-key operations the SDK calls, the module, method
  and a Go call template the portal's API playground renders (`make operations`). Derived from the
  parity registry; `make test` checks it is current and compiles and runs every rendered snippet.
  Not shipped in the package; no code change.

### Changed

- **Breaking:** the generated models follow the API. Plan-change quotes carry `daysLeft` and `periodDays`,
  and no longer `creditMinor`, `windowsInPeriod` or `windowsUnissued`. Payment, ledger and plan-change
  reason enums gain values. The organisation gains `einvoicingTaxId`, the billing account `plan`, invoices
  `createdViaClientId`, and the tax connection `einvoicingIdentifier`, `identifierSource` and
  `identifierNextAction`.
- The payments service's reads (bank account, transfers) are excluded until payments launches in
  production.

### Fixed

- `make sync` runs the generator on macOS (the einvoice-js checkout is resolved with `pwd -P`).

## 0.1.1 (2026-10-07)

### Changed

- A shorter README for integrators: install, quick start, sandbox and live, responses, errors, retries,
  webhooks, guides, the API reference and configuration. No code change.

## 0.1.0 (2026-10-07)

The first release of the Go SDK, at parity with `@useyona/einvoice-js` 0.8.x: 99 methods in 23
modules over the 106 operations an API key may call, 8 documented exclusions. The SDKs are
versioned independently; the einvoice-js commit (or tag) pinned in `scripts/sync.sh` records which
JS release this one tracks.

### Added

- `einvoice.New(apiKey, ...Option)`: the mode follows the key prefix (`sk_test_` sandbox, `sk_live_`
  live) on one permanent host; `WithAssertMode`, `WithTimeout`, `WithRetry(RetryConfig{...})`,
  `WithHeaders`, `WithHTTPClient`, `WithTransport` (a recorder in tests), `WithBaseURL` (advanced).
- Modules `Invoices`, `Submissions`, `Output`, `ShareLinks`, `Items`, `Reference`, `Buyers`, `Sellers`,
  `InboundInvoices`, `IssuedHistory`, `InvoiceSettings`, `TaxConnection`, `Organization`,
  `Billing.{Accounts, Payments, Sandbox, Statements, Subscriptions, Transactions}`,
  `Webhooks.{Endpoints, Deliveries, Events, EventTypes}`; `Paginate` and `Collect`.
- The error types (`*APIError` and its wrappers by status for `errors.As`, `*TimeoutError`,
  `*ConnectionError`, `*ConfigError`, `*WebhookError`; all implement `einvoice.Error`).
- Retries with backoff and `Retry-After` for GET/PUT/DELETE and keyed writes; generated
  `Idempotency-Key`s on the routes that accept one; one deadline per attempt covering headers and body;
  a cancelled `context.Context` is never retried.
- `VerifyWebhook`, `SignWebhookPayload`, `ComputeWebhookSignature`, `ParseSignatureHeader`, verified
  against the backend's test vectors; the `WebhookEventType` catalogue.
- Generated models for every request and response shape (`types_gen.go`, from einvoice-js's
  generator) and the hand-written `Amount` for `string | number` values.
- The five guide recipes in `examples/` and their export to `guides/guides.json`.
