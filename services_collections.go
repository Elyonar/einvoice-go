package einvoice

import "context"

// CollectionsService is collections (`/b/v1/collection`): accounts that collect an invoice's payment
// by bank transfer, the payments they received and how each was reconciled, buyer credit, the
// exceptions a person must resolve, and the settlement account the money is paid out to. Each answer
// is the key's own mode; where collections are not switched on for it, 409 `BIZ120`.
type CollectionsService struct{ baseService }

// ListAccounts is the organisation's collection accounts, newest first (`collection.account.read`).
func (s *CollectionsService) ListAccounts(ctx context.Context, query *ListCollectionAccountsQuery, opts ...RequestOption) (*Page[ListCollectionAccountsItem], error) {
	return page[ListCollectionAccountsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/collection/accounts", Query: query, Options: requestOptions(opts)})
}

// CreateAccount opens a collection account for an invoice (`collection.account.create`). Not
// retried: the route takes no `Idempotency-Key`.
func (s *CollectionsService) CreateAccount(ctx context.Context, params *CreateCollectionAccountBody, opts ...RequestOption) (*CreateCollectionAccountData, error) {
	out := &CreateCollectionAccountData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/b/v1/collection/accounts", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// GetAccount is one collection account (`collection.account.read`).
func (s *CollectionsService) GetAccount(ctx context.Context, id string, opts ...RequestOption) (*GetCollectionAccountData, error) {
	out := &GetCollectionAccountData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/collection/accounts/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// GetAccountStatus is the provider's state of a collection account (`collection.account.read`).
func (s *CollectionsService) GetAccountStatus(ctx context.Context, id string, opts ...RequestOption) (*GetCollectionAccountStatusData, error) {
	out := &GetCollectionAccountStatusData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/collection/accounts/" + seg(id) + "/status", Options: requestOptions(opts)}, out)
	return out, err
}

// DeactivateAccount deactivates a collection account; it stops taking payments
// (`collection.account.deactivate`). Not retried: the route takes no `Idempotency-Key`.
func (s *CollectionsService) DeactivateAccount(ctx context.Context, id string, params *DeactivateCollectionAccountBody, opts ...RequestOption) (*DeactivateCollectionAccountData, error) {
	out := &DeactivateCollectionAccountData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/b/v1/collection/accounts/" + seg(id) + "/deactivate", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// QuoteFee estimates the provider's fee on a collection before the account is opened
// (`collection.account.create`).
func (s *CollectionsService) QuoteFee(ctx context.Context, query *QuoteCollectionFeeQuery, opts ...RequestOption) ([]QuoteCollectionFeeItem, error) {
	out := []QuoteCollectionFeeItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/collection/quote", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}

// ListPayments is the organisation's collection payments, newest first (`collection.payment.read`).
func (s *CollectionsService) ListPayments(ctx context.Context, query *ListCollectionPaymentsQuery, opts ...RequestOption) (*Page[ListCollectionPaymentsItem], error) {
	return page[ListCollectionPaymentsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/collection/payments", Query: query, Options: requestOptions(opts)})
}

// ListBuyerCredits is buyer credit: each buyer's paid remainder not yet applied or refunded
// (`collection.payment.read`).
func (s *CollectionsService) ListBuyerCredits(ctx context.Context, query *ListCollectionBuyerCreditsQuery, opts ...RequestOption) (*Page[ListCollectionBuyerCreditsItem], error) {
	return page[ListCollectionBuyerCreditsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/collection/credits", Query: query, Options: requestOptions(opts)})
}

// ListExceptions is the payments a person must resolve (`collection.exception.read`).
func (s *CollectionsService) ListExceptions(ctx context.Context, query *ListCollectionExceptionsQuery, opts ...RequestOption) (*Page[ListCollectionExceptionsItem], error) {
	return page[ListCollectionExceptionsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/collection/exceptions", Query: query, Options: requestOptions(opts)})
}

// ResolveException resolves an exception: points its remainder at an invoice or a buyer, or records
// it as handled (`collection.exception.resolve`). paymentID is the exception's payment. Not retried:
// the route takes no `Idempotency-Key`.
func (s *CollectionsService) ResolveException(ctx context.Context, paymentID string, params *ResolveCollectionExceptionBody, opts ...RequestOption) (*ResolveCollectionExceptionData, error) {
	out := &ResolveCollectionExceptionData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/b/v1/collection/exceptions/" + seg(paymentID) + "/resolve", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// ListSettlementAccounts is the active settlement account of this mode, as a page of zero or one
// (`collection.settlement_account.read`).
func (s *CollectionsService) ListSettlementAccounts(ctx context.Context, query *ListCollectionSettlementAccountsQuery, opts ...RequestOption) (*Page[ListCollectionSettlementAccountsItem], error) {
	return page[ListCollectionSettlementAccountsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/collection/settlement-accounts", Query: query, Options: requestOptions(opts)})
}

// GetSettlementAccount is the active settlement account, by the organisation's seller id
// (`collection.settlement_account.read`).
func (s *CollectionsService) GetSettlementAccount(ctx context.Context, sellerID string, opts ...RequestOption) (*GetCollectionSettlementAccountData, error) {
	out := &GetCollectionSettlementAccountData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/collection/settlement-accounts/" + seg(sellerID), Options: requestOptions(opts)}, out)
	return out, err
}

// ListBanks is the rail's bank list for this mode, for choosing a settlement bank
// (`collection.settlement_account.read`).
func (s *CollectionsService) ListBanks(ctx context.Context, query *ListCollectionBanksQuery, opts ...RequestOption) ([]ListCollectionBanksItem, error) {
	out := []ListCollectionBanksItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/collection/banks", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}
