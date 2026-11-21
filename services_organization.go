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
// CreateOrganizationBody without `country`, which a sub-organisation takes from its parent, and
// without `groupOnboarding`, the signed-in person's field (a key that holds
// `organization.children.onboard` makes a managed business without it).
// (einvoice-js names it CreateSubOrganizationParams.)
type CreateSubOrganizationBody struct {
	// BusinessName is the new organisation's name, unique under its parent.
	BusinessName string `json:"businessName"`

	// Email becomes the organisation's contact email (lower-cased).
	Email string `json:"email"`

	// PhoneNumber is E.164; spaces, dashes, dots and brackets are removed before validation.
	PhoneNumber *string `json:"phoneNumber,omitempty"`

	// Website is the new organisation's website: an https URL of at most 255 characters.
	Website *string `json:"website,omitempty"`

	// Intent is which rail the organisation comes for: core (the default), access, or both.
	Intent *CreateOrganizationDtoIntent `json:"intent,omitempty"`

	// ExternalReference is your own id for the business: 1 to 64 of letters, digits, `.`, `_`, `:`
	// and `-`. It is unique among the parent's sub-organisations and never changed; the same
	// reference with the same name answers the business already made.
	ExternalReference *string `json:"externalReference,omitempty"`
}

// SubOrganizationTaxConnectionsQuery is the query of TaxConnection.ListForSubOrganizations: the
// API's ListChildTaxConnectionsQuery with `ids` as a list.
type SubOrganizationTaxConnectionsQuery struct {
	// IDs are the sub-organisations: at most 100, each once.
	IDs IDs `json:"ids"`
}

// OrganizationService is the key's organisation (`/a/v1/organizations`): its profile and readiness,
// its sub-organisations, and the managed onboarding of the businesses it creates. The profile is
// read-only for an API key.
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
// as `pending_verification` until its own onboarding is completed. Sends an `Idempotency-Key`
// (generated unless WithIdempotencyKey is given), so a retried call answers the organisation
// already made, with `Replayed: true`; the same key with other details is 409 `BIZ107`.
// ExternalReference is your own id for the business (1 to 64 of letters, digits, `.`, `_`, `:`,
// `-`): unique among the parent's sub-organisations, never changed, and the same reference with
// the same name also answers the business already made. A name is unique under its parent (409
// `RES002`). 402 `BIZ016` when the plan allows no more organisations; at most 60 a minute for each
// key.
//
// With a key that also holds `organization.children.onboard`, in an organisation that has accepted
// the managed onboarding terms, the business is created for managed onboarding
// (`OnboardingManaged: true`, `Origin: "parent_key"`): the same key then takes it through
// VerifySubOrganizationTaxID, SetSubOrganizationContact, CreateSubOrganizationSetupLink and
// SubmitSubOrganization.
func (s *OrganizationService) CreateSubOrganization(ctx context.Context, params *CreateSubOrganizationBody, opts ...RequestOption) (*CreateOrganizationData, error) {
	out := &CreateOrganizationData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/a/v1/organizations", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// GetGroupTerms is where the organisation stands with the managed onboarding terms, which let it
// onboard its businesses (`organization.children.read`; sandbox and live). Accepted is true only
// for the current version. An Owner or Admin accepts them in the Yona app; an API key cannot.
func (s *OrganizationService) GetGroupTerms(ctx context.Context, opts ...RequestOption) (*GetGroupTermsData, error) {
	out := &GetGroupTermsData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/a/v1/organizations/me/group-terms", Options: requestOptions(opts)}, out)
	return out, err
}

// VerifySubOrganizationTaxID is managed onboarding, step 2: it verifies the Tax ID of a business
// this key created (`organization.children.onboard`, in no preset; a live key only). The
// registry's name for the number is compared with BusinessName. The same number, already verified,
// answers the stored result with `Replayed: true` and makes no second lookup. No `Idempotency-Key`
// is sent and the call is not retried.
//
// 404 `RES001` for any business the key may not onboard: not the organisation's, one its own
// people set up, or one Yona has approved. 409 `BIZ004` with `errors[0].reason`
// `group_terms_not_accepted` or `in_review`; 422 `BIZ007` the name does not match; 409 `BIZ009`
// the Tax ID is verified for another organisation; 429 `BIZ002` the day's checks are used up.
func (s *OrganizationService) VerifySubOrganizationTaxID(ctx context.Context, subOrganizationID string, params *VerifyChildOrganizationTaxIdBody, opts ...RequestOption) (*VerifyChildOrganizationTaxIdData, error) {
	out := &VerifyChildOrganizationTaxIdData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/a/v1/organizations/me/children/" + seg(subOrganizationID) + "/onboarding/tax-id", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// SetSubOrganizationContact is managed onboarding, step 3: it sets the contact details of a
// business this key is onboarding (`organization.children.onboard`). PhoneNumber is the business
// owner's own phone: the setup link's codes go to it and nowhere else. The same values again
// change nothing and never undo the phone check; a changed number clears it and stops the working
// setup link. 409 `RES002` (`errors[0].phoneNumber`) when another business being onboarded uses
// the number.
func (s *OrganizationService) SetSubOrganizationContact(ctx context.Context, subOrganizationID string, params *SetChildOrganizationContactBody, opts ...RequestOption) (*SetChildOrganizationContactData, error) {
	out := &SetChildOrganizationContactData{}
	err := s.call(ctx, Call{Method: "PUT", Path: "/a/v1/organizations/me/children/" + seg(subOrganizationID) + "/onboarding/contact", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// CreateSubOrganizationSetupLink is managed onboarding, step 4: it creates the setup link the
// business owner finishes on (`organization.children.onboard`). URL is answered ONCE: Yona keeps
// only a fingerprint of it, so store it or send it straight away. A link works for 14 days unless
// ExpiresInDays says otherwise (30 at most). A business has one working link: a new one stops the
// one before it. A nil params sends `{}`, and the link takes the default life.
//
// Sends an `Idempotency-Key` (generated unless WithIdempotencyKey is given), so a retried call
// answers the same link with a nil URL and `Replayed: true`, and makes no second link. This is
// also the one call the key may make for a business Yona has approved, when its owner lost the
// link before finishing.
func (s *OrganizationService) CreateSubOrganizationSetupLink(ctx context.Context, subOrganizationID string, params *CreateChildOrganizationSetupLinkBody, opts ...RequestOption) (*CreateChildOrganizationSetupLinkData, error) {
	var body any = params
	if params == nil {
		body = struct{}{}
	}
	out := &CreateChildOrganizationSetupLinkData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/a/v1/organizations/me/children/" + seg(subOrganizationID) + "/setup-link", Body: body, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// RevokeSubOrganizationSetupLink withdraws the working setup link of a business at once
// (`organization.children.onboard`). Revoked is false, and nothing changes, when there was no
// working link.
func (s *OrganizationService) RevokeSubOrganizationSetupLink(ctx context.Context, subOrganizationID string, opts ...RequestOption) (*RevokeChildOrganizationSetupLinkData, error) {
	out := &RevokeChildOrganizationSetupLinkData{}
	err := s.call(ctx, Call{Method: "DELETE", Path: "/a/v1/organizations/me/children/" + seg(subOrganizationID) + "/setup-link", Options: requestOptions(opts)}, out)
	return out, err
}

// SubmitSubOrganization is managed onboarding, step 5: it asks Yona to review a business this key
// is onboarding (`organization.children.onboard`). It can be called before the owner has verified
// the phone: State is then `awaiting_phone_verification`, and the business goes to review by
// itself when the owner enters the code on the setup link. With the phone verified, State is
// `in_review`. A business already submitted answers where it stands, with `Replayed: true`. No
// `Idempotency-Key` is sent and the call is not retried. 409 `BIZ010` (`errors[].missing`: `tin`,
// `contact_info`) when an earlier step is missing. Yona approves; a key never does.
func (s *OrganizationService) SubmitSubOrganization(ctx context.Context, subOrganizationID string, opts ...RequestOption) (*SubmitChildOrganizationOnboardingData, error) {
	out := &SubmitChildOrganizationOnboardingData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/a/v1/organizations/me/children/" + seg(subOrganizationID) + "/onboarding/submit", Options: requestOptions(opts)}, out)
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
