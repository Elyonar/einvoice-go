package einvoice

import "context"

// ListSubOrganizationsQuery is the query of Organization.ListSubOrganizations: a page, a name
// search, or the sub-organisations named in IDs. It is the API's ListChildOrganizationsQuery with
// `ids` as a list.
type ListSubOrganizationsQuery struct {
	Page *float64 `json:"page,omitempty"`

	// Limit is the page size: 100 at most, and 100 when left out.
	Limit *float64 `json:"limit,omitempty"`

	// Search keeps the sub-organisations whose name holds it.
	Search *string `json:"search,omitempty"`

	// IDs keeps only these sub-organisations: at most 100. An id that is not one of the
	// organisation's sub-organisations is left out of the answer.
	IDs IDs `json:"ids,omitempty"`
}

// ListSubOrganizationsReadinessQuery is the query of Organization.ListSubOrganizationsReadiness:
// the API's ListChildOrganizationsReadinessQuery with `ids` as a list.
type ListSubOrganizationsReadinessQuery struct {
	Page *float64 `json:"page,omitempty"`

	// Limit is the page size: 100 at most, and 100 when left out.
	Limit *float64 `json:"limit,omitempty"`

	// Search keeps the sub-organisations whose name holds it.
	Search *string `json:"search,omitempty"`

	// IDs keeps only these sub-organisations: at most 100.
	IDs IDs `json:"ids,omitempty"`
}

// CreateSubOrganizationBody is the body of Organization.CreateSubOrganization: the API's
// CreateOrganizationBody without `country`, which a sub-organisation takes from its parent.
// (einvoice-js names it CreateSubOrganizationParams.)
type CreateSubOrganizationBody struct {
	// BusinessName is the new organisation's name, unique under its parent.
	BusinessName string `json:"businessName"`

	// Email becomes the organisation's contact email (lower-cased).
	Email string `json:"email"`

	// PhoneNumber is E.164; spaces, dashes, dots and brackets are removed before validation.
	PhoneNumber *string `json:"phoneNumber,omitempty"`

	Website *string `json:"website,omitempty"`
}

// SubOrganizationTaxConnectionsQuery is the query of TaxConnection.ListForSubOrganizations: the
// API's ListChildTaxConnectionsQuery with `ids` as a list.
type SubOrganizationTaxConnectionsQuery struct {
	// IDs are the sub-organisations: at most 100, each once.
	IDs IDs `json:"ids"`
}

// OrganizationService is the key's organisation (`/a/v1/organizations`): its profile and readiness,
// and its sub-organisations. The profile is read-only for an API key.
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

// ListSubOrganizations is a page of the organisation's sub-organisations, newest first, with the
// total in Pagination (`organization.children.read`; sandbox and live). Search keeps the ones whose
// name holds it; IDs keeps only the ones named (at most 100). Without Limit a page holds 100, the
// most. No key preset includes `organization.children.read`: it is added to a key by name, in the
// dashboard's Create API Key dialog. 409 `BIZ014` when the key's organisation is itself a
// sub-organisation.
func (s *OrganizationService) ListSubOrganizations(ctx context.Context, query *ListSubOrganizationsQuery, opts ...RequestOption) (*Page[ListChildOrganizationsItem], error) {
	return page[ListChildOrganizationsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/a/v1/organizations/me/children", Query: query, Options: requestOptions(opts)})
}

// ListSubOrganizationsReadiness is the same page as ListSubOrganizations, with each
// sub-organisation's onboarding checklist, live status and next step: which of a parent's
// businesses still have something to do (`organization.children.read`; sandbox and live). Whether
// each is connected to the tax authority is TaxConnection.ListForSubOrganizations with the same ids.
func (s *OrganizationService) ListSubOrganizationsReadiness(ctx context.Context, query *ListSubOrganizationsReadinessQuery, opts ...RequestOption) (*Page[ListChildOrganizationsReadinessItem], error) {
	return page[ListChildOrganizationsReadinessItem](ctx, &s.baseService, Call{Method: "GET", Path: "/a/v1/organizations/me/children/readiness", Query: query, Options: requestOptions(opts)})
}

// CreateSubOrganization creates a sub-organisation of the key's organisation
// (`organization.children.create`, which no key preset includes; a live key only: a sandbox key is
// refused with 403 `BIZ003`). It is recorded under the parent's longest-standing Owner and starts
// as `pending_verification` until its own onboarding is completed. No `Idempotency-Key` is sent and
// the call is not retried: a name is unique under its parent, so a repeated request answers 409
// `RES002`, and the organisation is found with ListSubOrganizations and Search. 402 `BIZ016` when
// the plan allows no more organisations; at most 60 a minute for each key.
func (s *OrganizationService) CreateSubOrganization(ctx context.Context, params *CreateSubOrganizationBody, opts ...RequestOption) (*CreateOrganizationData, error) {
	out := &CreateOrganizationData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/a/v1/organizations", Body: params, Options: requestOptions(opts)}, out)
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
// It makes no identifier lookup; GetWithQuery does. State can be `awaiting_authority_listing`
// (see GetWithQuery).
func (s *TaxConnectionService) Get(ctx context.Context, opts ...RequestOption) (*GetTaxConnectionData, error) {
	return s.GetWithQuery(ctx, nil, opts...)
}

// GetWithQuery is Get with the query (`tax_connection.read`): `Identifier: "true"` also resolves
// which number the tax authority holds for the organisation (`EinvoicingIdentifier`, or
// `IdentifierNextAction` when it holds none); `Fresh: "true"` asks the authority again. Both are
// rationed: pass them only when that answer is needed. A nil query is Get. (einvoice-js overloads
// `taxConnection.get(query, options)`; Go keeps Get's signature and adds this method.)
//
// State can be `awaiting_authority_listing`: the business asked to connect, and the tax authority
// does not list it under any of its numbers yet. Yona asks the authority again every 6 hours
// (`Fresh: "true"` asks now) and carries on by itself once it is listed; Listing says since when,
// when it was last asked and is asked next, and what the last answer was. Invoices issued meanwhile
// are held and reported once the connection completes.
func (s *TaxConnectionService) GetWithQuery(ctx context.Context, query *GetTaxConnectionQuery, opts ...RequestOption) (*GetTaxConnectionData, error) {
	out := &GetTaxConnectionData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/tax-connection", Query: query, Options: requestOptions(opts)}, out)
	return out, err
}

// ListForSubOrganizations is, for a parent organisation, the state of each named sub-organisation's
// connection to the tax authority in the key's mode, and whether a submission would pass
// (`tax_connection.read`). IDs are the ids Organization.ListSubOrganizations answers: at most 100,
// each once (400 `VAL001` otherwise). An id that is not one of the organisation's sub-organisations
// is left out of the answer. Only the state is shown, never the sub-organisation's tax id or key.
func (s *TaxConnectionService) ListForSubOrganizations(ctx context.Context, query *SubOrganizationTaxConnectionsQuery, opts ...RequestOption) ([]ListChildTaxConnectionsItem, error) {
	out := []ListChildTaxConnectionsItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/tax-connection/children", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}
