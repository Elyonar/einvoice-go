package einvoice

import "context"

// BuyersService is buyers (`/i/v1/buyers`): the parties the organisation invoices.
type BuyersService struct{ baseService }

// Create creates a buyer (`buyer.create`; charged per buyer).
func (s *BuyersService) Create(ctx context.Context, params *CreateBuyerBody, opts ...RequestOption) (*CreateBuyerData, error) {
	out := &CreateBuyerData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/buyers", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// List lists buyers, filtered by `Search` and `TaxIDStatus` (`buyer.read`).
func (s *BuyersService) List(ctx context.Context, query *ListBuyersQuery, opts ...RequestOption) (*Page[ListBuyersItem], error) {
	return page[ListBuyersItem](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/buyers", Query: query, Options: requestOptions(opts)})
}

// Get gets a buyer, archived ones included (`buyer.read`).
func (s *BuyersService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetBuyerData, error) {
	out := &GetBuyerData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/buyers/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Update updates a buyer; `null` clears an optional field (`buyer.update`; charged when the edit
// changes something, free for a no-op). Retried only with WithIdempotencyKey.
func (s *BuyersService) Update(ctx context.Context, id string, params *UpdateBuyerBody, opts ...RequestOption) (*UpdateBuyerData, error) {
	out := &UpdateBuyerData{}
	o := requestOptions(opts)
	err := s.call(ctx, Call{Method: "PATCH", Path: "/i/v1/buyers/" + seg(id), Body: params, Idempotent: idempotencyKeyOf(o) != "", Options: o}, out)
	return out, err
}

// Delete deletes a buyer, or archives it when an issued invoice names it (`Outcome` says which; `buyer.delete`).
func (s *BuyersService) Delete(ctx context.Context, id string, opts ...RequestOption) (*DeleteBuyerData, error) {
	out := &DeleteBuyerData{}
	err := s.call(ctx, Call{Method: "DELETE", Path: "/i/v1/buyers/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// BulkDelete deletes up to 100 buyers, each like a single delete (`buyer.delete`).
func (s *BuyersService) BulkDelete(ctx context.Context, params *BulkDeleteBuyersBody, opts ...RequestOption) (*BulkDeleteBuyersData, error) {
	out := &BulkDeleteBuyersData{}
	err := s.call(ctx, Call{Method: "DELETE", Path: "/i/v1/buyers/bulk", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// VerifyTaxNumber starts a tax-number verification (`buyer.verify_tax_id`; 202, or 200 with the one pending).
func (s *BuyersService) VerifyTaxNumber(ctx context.Context, id string, opts ...RequestOption) (*VerifyBuyerTaxNumberData, error) {
	out := &VerifyBuyerTaxNumberData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/buyers/" + seg(id) + "/verify-tax-number", Options: requestOptions(opts)}, out)
	return out, err
}

// GetVerificationStatus is the latest verification and the buyer's tax-id status (`buyer.read`).
func (s *BuyersService) GetVerificationStatus(ctx context.Context, id string, opts ...RequestOption) (*GetBuyerVerificationStatusData, error) {
	out := &GetBuyerVerificationStatusData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/buyers/" + seg(id) + "/verification-status", Options: requestOptions(opts)}, out)
	return out, err
}

// Search searches buyers by name, legal name or tax id (`Q` ≥ 2 characters; `buyer.read`).
func (s *BuyersService) Search(ctx context.Context, query *SearchBuyersQuery, opts ...RequestOption) ([]SearchBuyersItem, error) {
	out := []SearchBuyersItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/buyers/search", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}

// CheckReachability is whether the e-invoicing network reaches a TIN, from the cache (`buyer.read`; free).
func (s *BuyersService) CheckReachability(ctx context.Context, query *CheckBuyerReachabilityQuery, opts ...RequestOption) (*CheckBuyerReachabilityData, error) {
	out := &CheckBuyerReachabilityData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/buyers/reachability", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// CheckWithTaxAuthority asks the tax authority about a TIN now (`buyer.read`; charged, refunded when
// unanswered). Sends an `Idempotency-Key`.
func (s *BuyersService) CheckWithTaxAuthority(ctx context.Context, params *CheckBuyerWithTaxAuthorityBody, opts ...RequestOption) (*CheckBuyerWithTaxAuthorityData, error) {
	out := &CheckBuyerWithTaxAuthorityData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/buyers/reachability/check", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// SellersService is sellers (`/i/v1/sellers`): read-only. The organisation is its only seller;
// creating, editing, deleting or verifying a seller is always 409 `BIZ205`, so the SDK has no method for it.
type SellersService struct{ baseService }

// List is the organisation's seller, as a list of one (`seller.read`).
func (s *SellersService) List(ctx context.Context, query *ListSellersQuery, opts ...RequestOption) (*Page[ListSellersItem], error) {
	return page[ListSellersItem](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/sellers", Query: query, Options: requestOptions(opts)})
}

// Get gets the seller by id (`seller.read`).
func (s *SellersService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetSellerData, error) {
	out := &GetSellerData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/sellers/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// GetVerificationStatus is the organisation's TIN status (`seller.read`).
func (s *SellersService) GetVerificationStatus(ctx context.Context, id string, opts ...RequestOption) (*GetSellerVerificationStatusData, error) {
	out := &GetSellerVerificationStatusData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/sellers/" + seg(id) + "/verification-status", Options: requestOptions(opts)}, out)
	return out, err
}

// Search is the seller when it matches `Q`, else an empty list (`seller.read`).
func (s *SellersService) Search(ctx context.Context, query *SearchSellersQuery, opts ...RequestOption) ([]SearchSellersItem, error) {
	out := []SearchSellersItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/sellers/search", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}
