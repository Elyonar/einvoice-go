package einvoice

import "context"

// InboundInvoicesService is received invoices (`/i/v1/inbound-invoices`): what other businesses
// issued to the organisation.
type InboundInvoicesService struct{ baseService }

// List lists received invoices (`invoice.inbound.read`; free). `Tier: "history"` lists those
// retrieved by a history run. `Data.Items` holds the page; `Pagination` is `meta.pagination`.
func (s *InboundInvoicesService) List(ctx context.Context, query *ListInboundInvoicesQuery, opts ...RequestOption) (*Paginated[ListInboundInvoicesData], error) {
	return paginated[ListInboundInvoicesData](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/inbound-invoices", Query: query, Options: requestOptions(opts)})
}

// Get gets a received invoice; `Document` is read from the gateway on first view (`invoice.inbound.read`).
func (s *InboundInvoicesService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetInboundInvoiceData, error) {
	out := &GetInboundInvoiceData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/inbound-invoices/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// GetAnalytics is analytics over received invoices, with the list's filters (`invoice.inbound.read`; free).
func (s *InboundInvoicesService) GetAnalytics(ctx context.Context, query *GetInboundInvoiceAnalyticsQuery, opts ...RequestOption) (*GetInboundInvoiceAnalyticsData, error) {
	out := &GetInboundInvoiceAnalyticsData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/inbound-invoices/analytics", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// ListHistoryRuns is the history retrievals, newest first, and the active one (`invoice.inbound.read`; free).
func (s *InboundInvoicesService) ListHistoryRuns(ctx context.Context, query *ListInboundHistoryRunsQuery, opts ...RequestOption) (*Paginated[ListInboundHistoryRunsData], error) {
	return paginated[ListInboundHistoryRunsData](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/inbound-invoices/history-runs", Query: query, Options: requestOptions(opts)})
}

// GetHistoryRun is one history retrieval, for polling its progress (`invoice.inbound.read`; free).
func (s *InboundInvoicesService) GetHistoryRun(ctx context.Context, runID string, opts ...RequestOption) (*GetInboundHistoryRunData, error) {
	out := &GetInboundHistoryRunData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/inbound-invoices/history-runs/" + seg(runID), Options: requestOptions(opts)}, out)
	return out, err
}

// LoadOlder is kept for older clients: it reads and charges nothing and answers that nothing older
// belongs in the Received tab. Older invoices come from history runs (started in the dashboard).
//
// Deprecated: use ListHistoryRuns and List with `Tier: "history"`.
func (s *InboundInvoicesService) LoadOlder(ctx context.Context, opts ...RequestOption) (*LoadOlderInboundInvoicesData, error) {
	out := &LoadOlderInboundInvoicesData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/inbound-invoices/load-older", Options: requestOptions(opts)}, out)
	return out, err
}

// IssuedHistoryService is issued history (`/i/v1/issued-history`): invoices the organisation issued
// that NRS holds, from before Yona.
type IssuedHistoryService struct{ baseService }

// List lists invoices issued outside Yona that the tax authority holds (`invoice.inbound.read`; free).
func (s *IssuedHistoryService) List(ctx context.Context, query *ListIssuedHistoryQuery, opts ...RequestOption) (*Paginated[ListIssuedHistoryData], error) {
	return paginated[ListIssuedHistoryData](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/issued-history", Query: query, Options: requestOptions(opts)})
}

// Get gets one; `Document` is read and charged once per invoice on first open (`invoice.inbound.read`).
func (s *IssuedHistoryService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetIssuedHistoryInvoiceData, error) {
	out := &GetIssuedHistoryInvoiceData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/issued-history/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// DownloadPDF is the PDF printed from the authority copy (`invoice.inbound.read`; charged once per
// invoice if not yet read).
func (s *IssuedHistoryService) DownloadPDF(ctx context.Context, id string, opts ...RequestOption) (*BinaryResponse, error) {
	return s.binary(ctx, Call{Method: "GET", Path: "/i/v1/issued-history/" + seg(id) + "/pdf", Options: requestOptions(opts)})
}
