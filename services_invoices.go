package einvoice

import "context"

// InvoicesService is invoices (`/i/v1/invoices`): drafts, their lifecycle, credit and debit notes,
// and the organisation's invoice statistics. Submission to the tax authority is SubmissionsService.
type InvoicesService struct{ baseService }

// Create creates a draft invoice (`invoice.create`; charged as one invoice creation). The number is
// allocated from the series when InvoiceNumber is omitted. Sends an `Idempotency-Key` (generated
// unless WithIdempotencyKey is given), so a retry is never charged twice.
func (s *InvoicesService) Create(ctx context.Context, params *CreateInvoiceBody, opts ...RequestOption) (*CreateInvoiceData, error) {
	out := &CreateInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// List lists invoices, newest first, without LineItems (`invoice.read`).
func (s *InvoicesService) List(ctx context.Context, query *ListInvoicesQuery, opts ...RequestOption) (*Page[ListInvoicesItem], error) {
	return page[ListInvoicesItem](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/invoices", Query: query, Options: requestOptions(opts)})
}

// Get gets one invoice with its lines (`invoice.read`).
func (s *InvoicesService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetInvoiceData, error) {
	out := &GetInvoiceData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Update updates a draft (`invoice.update_draft`).
func (s *InvoicesService) Update(ctx context.Context, id string, params *UpdateInvoiceBody, opts ...RequestOption) (*UpdateInvoiceData, error) {
	out := &UpdateInvoiceData{}
	err := s.call(ctx, Call{Method: "PATCH", Path: "/i/v1/invoices/" + seg(id), Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// Delete deletes a draft permanently; its number is never reused (`invoice.delete_draft`).
func (s *InvoicesService) Delete(ctx context.Context, id string, opts ...RequestOption) (*DeleteInvoiceData, error) {
	out := &DeleteInvoiceData{}
	err := s.call(ctx, Call{Method: "DELETE", Path: "/i/v1/invoices/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Finalise finalises a draft: freezes the seller and buyer onto it; 422 `VAL001` when the
// jurisdiction's checks refuse it (`invoice.finalise`).
func (s *InvoicesService) Finalise(ctx context.Context, id string, opts ...RequestOption) (*FinaliseInvoiceData, error) {
	out := &FinaliseInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/finalise", Options: requestOptions(opts)}, out)
	return out, err
}

// Reopen reopens a finalised invoice as a draft: allowed while its submission was rejected, or when
// it was never submitted (`invoice.reopen`).
func (s *InvoicesService) Reopen(ctx context.Context, id string, opts ...RequestOption) (*ReopenInvoiceData, error) {
	out := &ReopenInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/reopen", Options: requestOptions(opts)}, out)
	return out, err
}

// Revise revises an issued invoice that was never reported and is unpaid: back to a draft with the
// same number and the next version (`invoice.reopen`).
func (s *InvoicesService) Revise(ctx context.Context, id string, opts ...RequestOption) (*ReviseInvoiceData, error) {
	out := &ReviseInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/revise", Options: requestOptions(opts)}, out)
	return out, err
}

// Cancel cancels an invoice (`invoice.cancel`; 202): voided locally when the tax authority does not
// hold it, otherwise by a credit note. A draft is 409 `BIZ201` (delete it instead). Sends an
// `Idempotency-Key`.
func (s *InvoicesService) Cancel(ctx context.Context, id string, params *CancelInvoiceBody, opts ...RequestOption) (*CancelInvoiceData, error) {
	out := &CancelInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/cancel", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// IssueCreditNote issues a credit note against a registered invoice (`invoice.cancel`). With
// `Preview: true` nothing is written or charged. Sends an `Idempotency-Key`. The answer is one of
// two shapes: decode it with AsCreditNotePreviewAnswerDto or AsCreditNoteIssuedDto.
func (s *InvoicesService) IssueCreditNote(ctx context.Context, id string, params *IssueCreditNoteBody, opts ...RequestOption) (*IssueCreditNoteData, error) {
	out := &IssueCreditNoteData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/credit-notes", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// IssueDebitNote issues a debit note (added lines) against a registered invoice and submits it
// (`invoice.create` + `invoice.submit`). With `Preview: true` nothing is written. Sends an
// `Idempotency-Key`. Decode the answer with AsDebitNotePreviewAnswerDto or AsDebitNoteIssuedDto.
func (s *InvoicesService) IssueDebitNote(ctx context.Context, id string, params *IssueDebitNoteBody, opts ...RequestOption) (*IssueDebitNoteData, error) {
	out := &IssueDebitNoteData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/debit-notes", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// GetOverview is the dashboard overview: KPIs, trend, alerts and readiness (`invoice.stats.read`).
func (s *InvoicesService) GetOverview(ctx context.Context, query *GetInvoiceOverviewQuery, opts ...RequestOption) (*GetInvoiceOverviewData, error) {
	out := &GetInvoiceOverviewData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/overview", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// GetStatistics is the invoice statistics for a period (`invoice.stats.read`).
func (s *InvoicesService) GetStatistics(ctx context.Context, query *GetInvoiceStatisticsQuery, opts ...RequestOption) (*GetInvoiceStatisticsData, error) {
	out := &GetInvoiceStatisticsData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/statistics", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// GetSummary is the counts behind the invoice list's summary cards (`invoice.stats.read`).
func (s *InvoicesService) GetSummary(ctx context.Context, query *GetInvoiceListSummaryQuery, opts ...RequestOption) (*GetInvoiceListSummaryData, error) {
	out := &GetInvoiceListSummaryData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/summary", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// SubmissionsService is submissions (`/i/v1/invoices/…`): reporting invoices to the tax authority
// and following them.
type SubmissionsService struct{ baseService }

// Submit queues a submission of a finalised invoice (`invoice.submit`; 202, or 200 `Replayed: true`
// while one is in flight). `Finalise: "true"` finalises a draft first. Sends an `Idempotency-Key`.
func (s *SubmissionsService) Submit(ctx context.Context, id string, query *SubmitInvoiceQuery, opts ...RequestOption) (*SubmitInvoiceData, error) {
	out := &SubmitInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/submit", Query: query, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// Issue issues a draft: finalised and submitted when the organisation reports automatically (202),
// otherwise finalised and sent to the buyer (200, `NotReportedReason`). Sends an `Idempotency-Key`.
func (s *SubmissionsService) Issue(ctx context.Context, id string, opts ...RequestOption) (*IssueInvoiceData, error) {
	out := &IssueInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/issue", Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// CreateAndSubmit creates an invoice and submits it (`invoice.create` + `invoice.submit`). When the
// submit is refused the draft is kept and `SubmitRefusal` says why. Sends an `Idempotency-Key`.
func (s *SubmissionsService) CreateAndSubmit(ctx context.Context, params *CreateAndSubmitInvoiceBody, opts ...RequestOption) (*CreateAndSubmitInvoiceData, error) {
	out := &CreateAndSubmitInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/create-and-submit", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// BatchSubmit submits 1–100 invoices, each independently (`invoice.submit`). Not retried by the SDK.
func (s *SubmissionsService) BatchSubmit(ctx context.Context, params *BatchSubmitInvoicesBody, opts ...RequestOption) (*BatchSubmitInvoicesData, error) {
	out := &BatchSubmitInvoicesData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/batch-submit", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// Retry resumes a submission that is on hold, with its existing charge hold (`invoice.retry`; 202).
func (s *SubmissionsService) Retry(ctx context.Context, id string, opts ...RequestOption) (*RetryInvoiceSubmissionData, error) {
	out := &RetryInvoiceSubmissionData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/retry", Options: requestOptions(opts)}, out)
	return out, err
}

// Renumber gives an invoice whose submission is on hold because the tax authority already holds its
// number the next number of its series and queues a new submission (`invoice.submit`). Sends an
// `Idempotency-Key`.
func (s *SubmissionsService) Renumber(ctx context.Context, id string, opts ...RequestOption) (*RenumberInvoiceData, error) {
	out := &RenumberInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/renumber", Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// QueryStatus asks the tax authority now for the invoice's state (`invoice.query_status`). Sends an `Idempotency-Key`.
func (s *SubmissionsService) QueryStatus(ctx context.Context, id string, opts ...RequestOption) (*QueryInvoiceStatusData, error) {
	out := &QueryInvoiceStatusData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/query-status", Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// GetStatus is the submission status as last recorded (`invoice.read`).
func (s *SubmissionsService) GetStatus(ctx context.Context, id string, opts ...RequestOption) (*GetInvoiceStatusData, error) {
	out := &GetInvoiceStatusData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/" + seg(id) + "/status", Options: requestOptions(opts)}, out)
	return out, err
}

// RecordPayment records a payment against an issued invoice (`invoice.record_payment`). Charged once
// per status report relayed to the tax authority; a no-op change is free. Not retried by the SDK.
func (s *SubmissionsService) RecordPayment(ctx context.Context, id string, params *RecordInvoicePaymentBody, opts ...RequestOption) (*RecordInvoicePaymentData, error) {
	out := &RecordInvoicePaymentData{}
	err := s.call(ctx, Call{Method: "PATCH", Path: "/i/v1/invoices/" + seg(id) + "/payment-status", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// GetAuthorityCopy is the tax authority's own decrypted copy of a registered invoice (`invoice.download_authority_copy`).
func (s *SubmissionsService) GetAuthorityCopy(ctx context.Context, id string, opts ...RequestOption) (*GetInvoiceAuthorityCopyData, error) {
	out := &GetInvoiceAuthorityCopyData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/" + seg(id) + "/authority-download", Options: requestOptions(opts)}, out)
	return out, err
}
