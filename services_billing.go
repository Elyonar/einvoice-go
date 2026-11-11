package einvoice

import "context"

// BillingAccountsService is billing accounts (`/b/v1/billing-accounts`): the credit balance.
type BillingAccountsService struct{ baseService }

// GetMine is the organisation's billing account; an API key sees its own mode's balance (`billing.account.read`).
func (s *BillingAccountsService) GetMine(ctx context.Context, query *GetBillingAccountQuery, opts ...RequestOption) (*GetBillingAccountData, error) {
	out := &GetBillingAccountData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/billing-accounts/me", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// GetStats is balance, lifetime totals and 30-day consumption; accountID is the account's id or
// `"me"` ("" means "me") (`billing.account.read`).
func (s *BillingAccountsService) GetStats(ctx context.Context, accountID string, query *GetBillingAccountStatsQuery, opts ...RequestOption) (*GetBillingAccountStatsData, error) {
	if accountID == "" {
		accountID = "me"
	}
	out := &GetBillingAccountStatsData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/billing-accounts/" + seg(accountID) + "/stats", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// CheckBalance is whether the balance covers `Credits` (advisory; `billing.account.read`). accountID is the id or `"me"`.
func (s *BillingAccountsService) CheckBalance(ctx context.Context, accountID string, query *CheckBillingAccountBalanceQuery, opts ...RequestOption) (*CheckBillingAccountBalanceData, error) {
	out := &CheckBillingAccountBalanceData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/billing-accounts/" + seg(accountID) + "/check-balance", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// PaymentsService is payments (`/b/v1/payments`): read-only for an API key (purchases are made in the dashboard).
type PaymentsService struct{ baseService }

// List lists payments, newest first (`billing.payment.read`).
func (s *PaymentsService) List(ctx context.Context, query *ListPaymentsQuery, opts ...RequestOption) (*Page[ListPaymentsItem], error) {
	return page[ListPaymentsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/payments/history", Query: query, Options: requestOptions(opts)})
}

// Get is one payment (`billing.payment.read`).
func (s *PaymentsService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetPaymentData, error) {
	out := &GetPaymentData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/payments/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// SandboxService is the sandbox ledger (`/b/v1/sandbox`). A live key is refused with 403 `BIZ108`.
type SandboxService struct{ baseService }

// ListTransactions is the sandbox ledger's entries, newest first (`billing.ledger.read`).
func (s *SandboxService) ListTransactions(ctx context.Context, query *ListSandboxTransactionsQuery, opts ...RequestOption) (*Page[ListSandboxTransactionsItem], error) {
	return page[ListSandboxTransactionsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/sandbox/transactions", Query: query, Options: requestOptions(opts)})
}

// GetUsage is this UTC month's free sandbox allowance: granted, used, remaining (`billing.usage.read`).
func (s *SandboxService) GetUsage(ctx context.Context, opts ...RequestOption) (*GetSandboxUsageData, error) {
	out := &GetSandboxUsageData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/sandbox/usage", Options: requestOptions(opts)}, out)
	return out, err
}

// StatementsService is monthly statements (`/b/v1/statements`).
type StatementsService struct{ baseService }

// Get is one UTC month (`YYYY-MM`) of the key's own mode's ledger (`billing.statement.read`).
func (s *StatementsService) Get(ctx context.Context, period string, query *GetBillingStatementQuery, opts ...RequestOption) (*GetBillingStatementData, error) {
	out := &GetBillingStatementData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/statements/" + seg(period), Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// SubscriptionsService is subscriptions (`/b/v1/subscriptions`): read-only for an API key.
type SubscriptionsService struct{ baseService }

// GetActive is the active or past-due subscription and its allocation window (`billing.subscription.read`).
func (s *SubscriptionsService) GetActive(ctx context.Context, opts ...RequestOption) (*GetActiveSubscriptionData, error) {
	out := &GetActiveSubscriptionData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/subscriptions/active", Options: requestOptions(opts)}, out)
	return out, err
}

// List is every subscription, newest first (`billing.subscription.read`).
func (s *SubscriptionsService) List(ctx context.Context, query *ListSubscriptionsQuery, opts ...RequestOption) (*Page[ListSubscriptionsItem], error) {
	return page[ListSubscriptionsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/subscriptions/history", Query: query, Options: requestOptions(opts)})
}

// Get is one subscription (`billing.subscription.read`).
func (s *SubscriptionsService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetSubscriptionData, error) {
	out := &GetSubscriptionData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/subscriptions/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// ListRenewals is the billing periods and how each was charged (`billing.subscription.read`).
func (s *SubscriptionsService) ListRenewals(ctx context.Context, query *ListSubscriptionRenewalsQuery, opts ...RequestOption) (*Page[ListSubscriptionRenewalsItem], error) {
	return page[ListSubscriptionRenewalsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/subscriptions/renewals", Query: query, Options: requestOptions(opts)})
}

// PreviewPlanChange is what a plan change would cost now and at period end; writes nothing (`billing.subscription.read`).
func (s *SubscriptionsService) PreviewPlanChange(ctx context.Context, id string, query *PreviewSubscriptionPlanChangeQuery, opts ...RequestOption) (*PreviewSubscriptionPlanChangeData, error) {
	out := &PreviewSubscriptionPlanChangeData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/subscriptions/" + seg(id) + "/change-plan/preview", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// GetDealOffer is the deal offered to the organisation; nil when there is none (`billing.subscription.read`).
func (s *SubscriptionsService) GetDealOffer(ctx context.Context, opts ...RequestOption) (*GetDealOfferData, error) {
	var out *GetDealOfferData
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/deals/offer", Options: requestOptions(opts)}, &out)
	return out, err
}

// QuoteSubOrgSlot is what one more sub-organisation slot would cost; writes nothing (`billing.subscription.read`).
func (s *SubscriptionsService) QuoteSubOrgSlot(ctx context.Context, opts ...RequestOption) (*GetSubOrgSlotQuoteData, error) {
	out := &GetSubOrgSlotQuoteData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/sub-org-slots/quote", Options: requestOptions(opts)}, out)
	return out, err
}

// UsageService is invoice usage (`/b/v1/usage/invoices`): this month's invoices against the plan's allowance.
type UsageService struct{ baseService }

// GetInvoices is this month's invoice usage, the allowance and its thresholds; nil when the
// organisation has none (`billing.subscription.read`).
func (s *UsageService) GetInvoices(ctx context.Context, opts ...RequestOption) (*GetInvoiceUsageData, error) {
	var out *GetInvoiceUsageData
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/usage/invoices/current", Options: requestOptions(opts)}, &out)
	return out, err
}

// QuoteOverage is what an overage pack would cost now; writes nothing (`billing.subscription.read`).
func (s *UsageService) QuoteOverage(ctx context.Context, opts ...RequestOption) (*GetOverageQuoteData, error) {
	out := &GetOverageQuoteData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/usage/invoices/overage/quote", Options: requestOptions(opts)}, out)
	return out, err
}

// PlansService is the plan catalogue (`/b/v1/plans`).
type PlansService struct{ baseService }

// GetRecommendation is the plan recommended for the organisation from its usage; nil when there is
// none (`billing.subscription.read`).
func (s *PlansService) GetRecommendation(ctx context.Context, opts ...RequestOption) (*GetPlanRecommendationData, error) {
	var out *GetPlanRecommendationData
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/plans/recommendation", Options: requestOptions(opts)}, &out)
	return out, err
}

// PartnerService is the partner account (`/b/v1/partner`): an Integrated Partner's tier, seats and
// their quotes. Read-only for an API key.
type PartnerService struct{ baseService }

// Get is the partner account: tier, seats, capacity and renewal (`billing.subscription.read`).
// Businesses is the first page of the businesses the partner pays for, oldest first, 100 at most;
// GetWithQuery reads another page.
func (s *PartnerService) Get(ctx context.Context, opts ...RequestOption) (*GetPartnerData, error) {
	return s.GetWithQuery(ctx, nil, opts...)
}

// GetWithQuery is Get with the query (`billing.subscription.read`): Businesses is one page of the
// businesses the partner pays for, oldest first, and BusinessesPage says which page and how many
// there are in all. Pass Page and Limit for another (100 a page, the most, when Limit is left out).
// A nil query is Get. (einvoice-js overloads `billing.partner.get(query, options)`; Go keeps Get's
// signature and adds this method.)
func (s *PartnerService) GetWithQuery(ctx context.Context, query *GetPartnerQuery, opts ...RequestOption) (*GetPartnerData, error) {
	out := &GetPartnerData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/partner", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// GetOffer is the partner tier the organisation may buy; nil when there is none (`billing.subscription.read`).
func (s *PartnerService) GetOffer(ctx context.Context, opts ...RequestOption) (*GetPartnerOfferData, error) {
	var out *GetPartnerOfferData
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/partner/offer", Options: requestOptions(opts)}, &out)
	return out, err
}

// QuoteSeat is what one business seat on `Plan` would cost; writes nothing (`billing.subscription.read`).
func (s *PartnerService) QuoteSeat(ctx context.Context, query *GetPartnerSeatQuoteQuery, opts ...RequestOption) (*GetPartnerSeatQuoteData, error) {
	out := &GetPartnerSeatQuoteData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/partner/seats/quote", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// QuoteBasket is what seats for several businesses would cost together; writes nothing (`billing.subscription.read`).
func (s *PartnerService) QuoteBasket(ctx context.Context, query *GetPartnerBasketQuoteQuery, opts ...RequestOption) (*GetPartnerBasketQuoteData, error) {
	out := &GetPartnerBasketQuoteData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/partner/seats/basket/quote", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// QuotePlanChange is what moving one business to `Plan` would cost; writes nothing (`billing.subscription.read`).
func (s *PartnerService) QuotePlanChange(ctx context.Context, organizationID string, query *GetPartnerPlanChangeQuoteQuery, opts ...RequestOption) (*GetPartnerPlanChangeQuoteData, error) {
	out := &GetPartnerPlanChangeQuoteData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/partner/businesses/" + seg(organizationID) + "/change-plan/quote", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// TransactionsService is the credit ledger (`/b/v1/transactions`).
type TransactionsService struct{ baseService }

// List is ledger entries, newest first (`billing.ledger.read`).
func (s *TransactionsService) List(ctx context.Context, query *ListLedgerEntriesQuery, opts ...RequestOption) (*Page[ListLedgerEntriesItem], error) {
	return page[ListLedgerEntriesItem](ctx, &s.baseService, Call{Method: "GET", Path: "/b/v1/transactions", Query: query, Options: requestOptions(opts)})
}

// Get is one ledger entry (`billing.ledger.read`).
func (s *TransactionsService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetLedgerEntryData, error) {
	out := &GetLedgerEntryData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/transactions/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// GetUsageAnalytics is credits spent per day, week or month (`billing.usage.read`).
func (s *TransactionsService) GetUsageAnalytics(ctx context.Context, query *GetUsageAnalyticsQuery, opts ...RequestOption) ([]GetUsageAnalyticsItem, error) {
	out := []GetUsageAnalyticsItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/transactions/analytics/usage", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}

// GetUsageByCostCode is credits spent per cost code, net of refunds (`billing.usage.read`).
func (s *TransactionsService) GetUsageByCostCode(ctx context.Context, query *GetUsageByCostCodeQuery, opts ...RequestOption) ([]GetUsageByCostCodeItem, error) {
	out := []GetUsageByCostCodeItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/b/v1/transactions/breakdown/by-endpoint", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}

// BillingService is billing (`/b/v1`), grouped as the API tags it.
type BillingService struct {
	// Accounts: the billing account and balance.
	Accounts *BillingAccountsService
	// Payments (read-only).
	Payments *PaymentsService
	// Sandbox: the sandbox ledger and allowance.
	Sandbox *SandboxService
	// Statements: monthly statements.
	Statements *StatementsService
	// Subscriptions (read-only).
	Subscriptions *SubscriptionsService
	// Transactions: the credit ledger and usage.
	Transactions *TransactionsService
	// Usage: this month's invoice usage and overage quotes.
	Usage *UsageService
	// Plans: the plan catalogue's recommendation.
	Plans *PlansService
	// Partner: the partner account and its quotes (read-only).
	Partner *PartnerService
}

func newBillingService(base baseService) *BillingService {
	return &BillingService{
		Accounts:      &BillingAccountsService{base},
		Payments:      &PaymentsService{base},
		Sandbox:       &SandboxService{base},
		Statements:    &StatementsService{base},
		Subscriptions: &SubscriptionsService{base},
		Transactions:  &TransactionsService{base},
		Usage:         &UsageService{base},
		Plans:         &PlansService{base},
		Partner:       &PartnerService{base},
	}
}
