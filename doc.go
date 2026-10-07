// Package einvoice is the official Go SDK for the Yona e-invoicing API, the port of
// @useyona/einvoice-js module for module. It covers exactly what an API key may call: invoicing,
// submissions to the tax authority, output and share links, items, reference data, buyers, the
// read-only seller, received invoices and issued history, invoice settings, the tax connection, the
// organisation (read), billing reads and webhooks (read, test, redeliver). Users, invitations, roles,
// API keys, organisation management, collections, purchases and webhook-endpoint writes are done by
// a signed-in user in the dashboard and are not in the SDK.
//
// The key decides the mode: an `sk_test_…` key is the sandbox and an `sk_live_…` key is live, on the
// same host. There is nothing else to configure:
//
//	client, err := einvoice.New(os.Getenv("YONA_API_KEY"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	invoice, err := client.Invoices.Create(ctx, &einvoice.CreateInvoiceBody{…})
//
// Every refusal of the API is a typed error; narrow it with errors.As:
//
//	var ve *einvoice.ValidationError
//	if errors.As(err, &ve) { … }
//	var ae *einvoice.APIError
//	if errors.As(err, &ae) { fmt.Println(ae.Status, ae.ErrorCode, ae.RequestID) }
//
// The request and response shapes (CreateInvoiceBody, ListInvoicesQuery, InvoiceViewDto …) are
// generated from the API's OpenAPI into types_gen.go; never edit that file. Webhook deliveries are
// verified with VerifyWebhook. Zero dependencies outside the standard library.
package einvoice
