package einvoice

import "context"

// OutputService is output (`/i/v1/invoices/:id/download|send`): the invoice PDF and sending it to the buyer.
type OutputService struct{ baseService }

// GetDownloadLink is a short-lived download link to the invoice PDF (`invoice.download_pdf`; charged
// as one download). Sends an `Idempotency-Key`, so an SDK retry is not charged twice; the backend
// binds a key to the invoice it first downloaded (reusing it for another invoice is 409 `BIZ107`).
func (s *OutputService) GetDownloadLink(ctx context.Context, id string, opts ...RequestOption) (*DownloadInvoiceData, error) {
	out := &DownloadInvoiceData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/" + seg(id) + "/download", Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// DownloadPDF is the invoice PDF itself (`invoice.download_pdf`; same route with
// `Accept: application/pdf`; charged as one download). Sends an `Idempotency-Key`, like GetDownloadLink.
func (s *OutputService) DownloadPDF(ctx context.Context, id string, opts ...RequestOption) (*BinaryResponse, error) {
	return s.binary(ctx, Call{Method: "GET", Path: "/i/v1/invoices/" + seg(id) + "/download", Query: map[string]string{"format": "pdf"}, Idempotent: true, Options: requestOptions(opts)})
}

// Send emails the invoice to the buyer's address on record (`invoice.send_to_buyer`; 202 = queued).
// In sandbox no email leaves; `ShareURL` previews the PDF. Sends an `Idempotency-Key`.
func (s *OutputService) Send(ctx context.Context, id string, params *SendInvoiceBody, opts ...RequestOption) (*SendInvoiceData, error) {
	out := &SendInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(id) + "/send", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// ShareLinksService is share links (`/i/v1/invoices/:id/share-links`): public, revocable links to an invoice.
type ShareLinksService struct{ baseService }

// Create creates a share link (`invoice.share`).
func (s *ShareLinksService) Create(ctx context.Context, invoiceID string, params *CreateInvoiceShareLinkBody, opts ...RequestOption) (*CreateInvoiceShareLinkData, error) {
	out := &CreateInvoiceShareLinkData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/" + seg(invoiceID) + "/share-links", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// List lists an invoice's share links (`invoice.share`).
func (s *ShareLinksService) List(ctx context.Context, invoiceID string, opts ...RequestOption) ([]ListInvoiceShareLinksItem, error) {
	out := []ListInvoiceShareLinksItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/" + seg(invoiceID) + "/share-links", Options: requestOptions(opts)}, &out)
	return out, err
}

// Revoke revokes a share link (`invoice.share`).
func (s *ShareLinksService) Revoke(ctx context.Context, invoiceID, linkID string, opts ...RequestOption) (*RevokeInvoiceShareLinkData, error) {
	out := &RevokeInvoiceShareLinkData{}
	err := s.call(ctx, Call{Method: "DELETE", Path: "/i/v1/invoices/" + seg(invoiceID) + "/share-links/" + seg(linkID), Options: requestOptions(opts)}, out)
	return out, err
}
