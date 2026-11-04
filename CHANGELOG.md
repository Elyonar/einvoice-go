# Changelog

All notable changes to this project will be documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
