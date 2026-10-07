package einvoice

import "context"

// OrganizationService is the key's organisation (`/a/v1/organizations`): its profile and readiness.
// Read-only for an API key.
type OrganizationService struct{ baseService }

// Get is the organisation's profile; orgID must be the key's own organisation (404 otherwise; `organization.read`).
func (s *OrganizationService) Get(ctx context.Context, orgID string, opts ...RequestOption) (*GetOrganizationData, error) {
	out := &GetOrganizationData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/a/v1/organizations/" + seg(orgID), Options: requestOptions(opts)}, out)
	return out, err
}

// GetReadiness is the onboarding checklist, the live-access status and the next step (`organization.read`).
func (s *OrganizationService) GetReadiness(ctx context.Context, opts ...RequestOption) (*GetOrganizationReadinessData, error) {
	out := &GetOrganizationReadinessData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/a/v1/organizations/me/readiness", Options: requestOptions(opts)}, out)
	return out, err
}

// InvoiceSettingsService is invoice settings (`/i/v1/invoice-settings`): read-only for an API key.
type InvoiceSettingsService struct{ baseService }

// Get is the organisation's invoice settings in the key's mode (`invoice.read`).
func (s *InvoiceSettingsService) Get(ctx context.Context, opts ...RequestOption) (*GetInvoiceSettingsData, error) {
	out := &GetInvoiceSettingsData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoice-settings", Options: requestOptions(opts)}, out)
	return out, err
}

// TaxConnectionService is the tax connection (`/i/v1/tax-connection`): the organisation's link to the tax authority.
type TaxConnectionService struct{ baseService }

// Get is the connection's state in the key's mode, its key and the next step (`tax_connection.read`).
func (s *TaxConnectionService) Get(ctx context.Context, opts ...RequestOption) (*GetTaxConnectionData, error) {
	out := &GetTaxConnectionData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/tax-connection", Options: requestOptions(opts)}, out)
	return out, err
}
