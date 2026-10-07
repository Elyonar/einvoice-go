package einvoice

// Client is the client of the Yona e-invoicing API (the EInvoice client of the other SDKs). It
// covers what an API key may call: invoicing, buyers, items, received invoices, billing reads and
// webhooks. Users, invitations, roles, API keys and organisation management are done in the dashboard.
//
// The key decides the mode: `sk_test_…` is sandbox and `sk_live_…` is live, on the same host.
//
//	client, err := einvoice.New(os.Getenv("YONA_API_KEY"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	client.Mode() // einvoice.ModeSandbox or einvoice.ModeLive
//	invoice, err := client.Invoices.Create(ctx, &einvoice.CreateInvoiceBody{…})
type Client struct {
	// Invoices: drafts, lifecycle, credit and debit notes, statistics.
	Invoices *InvoicesService
	// Submissions to the tax authority and their status.
	Submissions *SubmissionsService
	// Output: the invoice PDF and sending it to the buyer.
	Output *OutputService
	// ShareLinks: revocable share links to an invoice.
	ShareLinks *ShareLinksService
	// Items: saved items (goods and services).
	Items *ItemsService
	// Reference: code lists, HS codes, tax-id lookup and dry-run validation.
	Reference *ReferenceService
	// Buyers.
	Buyers *BuyersService
	// Sellers: the organisation's seller (read-only).
	Sellers *SellersService
	// InboundInvoices: invoices other businesses issued to the organisation.
	InboundInvoices *InboundInvoicesService
	// IssuedHistory: invoices the organisation issued outside Yona that the tax authority holds.
	IssuedHistory *IssuedHistoryService
	// InvoiceSettings (read-only).
	InvoiceSettings *InvoiceSettingsService
	// TaxConnection: the connection to the tax authority (read-only).
	TaxConnection *TaxConnectionService
	// Organization: the key's organisation (read-only).
	Organization *OrganizationService
	// Billing reads: account, ledger, statements, subscriptions, payments, sandbox allowance.
	Billing *BillingService
	// Webhooks: endpoints (read, test), deliveries, events and the event catalogue.
	Webhooks *WebhooksService
	// HTTP is the transport, for advanced use.
	HTTP *HttpClient
}

// New creates a client. It returns a *ConfigError for a missing or malformed key, or when
// WithAssertMode does not match the key. Options: WithBaseURL, WithAssertMode, WithTimeout,
// WithRetry, WithHeaders, WithHTTPClient, WithTransport.
func New(apiKey string, opts ...Option) (*Client, error) {
	http, err := NewHttpClient(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	base := baseService{http: http}
	return &Client{
		Invoices:        &InvoicesService{base},
		Submissions:     &SubmissionsService{base},
		Output:          &OutputService{base},
		ShareLinks:      &ShareLinksService{base},
		Items:           &ItemsService{base},
		Reference:       &ReferenceService{base},
		Buyers:          &BuyersService{base},
		Sellers:         &SellersService{base},
		InboundInvoices: &InboundInvoicesService{base},
		IssuedHistory:   &IssuedHistoryService{base},
		InvoiceSettings: &InvoiceSettingsService{base},
		TaxConnection:   &TaxConnectionService{base},
		Organization:    &OrganizationService{base},
		Billing:         newBillingService(base),
		Webhooks:        newWebhooksService(base),
		HTTP:            http,
	}, nil
}

// Mode is ModeSandbox for an `sk_test_` key, ModeLive for an `sk_live_` key.
func (c *Client) Mode() Mode { return c.HTTP.Mode() }

// BaseURL is the host requests go to.
func (c *Client) BaseURL() string { return c.HTTP.BaseURL() }
