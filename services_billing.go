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
}

func newBillingService(base baseService) *BillingService {
	return &BillingService{
		Accounts:      &BillingAccountsService{base},
		Payments:      &PaymentsService{base},
		Sandbox:       &SandboxService{base},
		Statements:    &StatementsService{base},
		Subscriptions: &SubscriptionsService{base},
		Transactions:  &TransactionsService{base},
	}
}
