package einvoice

import "context"

// ItemsService is saved items (`/i/v1/items`): the goods and services an invoice line can reference by `ItemID`.
type ItemsService struct{ baseService }

// List lists saved items; non-archived by default (`item.read`).
func (s *ItemsService) List(ctx context.Context, query *ListItemsQuery, opts ...RequestOption) (*Page[ListItemsItem], error) {
	return page[ListItemsItem](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/items", Query: query, Options: requestOptions(opts)})
}

// Create saves an item (`item.create`; free). Goods need `HSNCode`, services `ServiceCode`.
func (s *ItemsService) Create(ctx context.Context, params *CreateItemBody, opts ...RequestOption) (*CreateItemData, error) {
	out := &CreateItemData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/items", Body: params, Options: requestOptions(opts)}, out)
	return out, err
}

// Get gets a saved item, archived ones included (`item.read`).
func (s *ItemsService) Get(ctx context.Context, id string, opts ...RequestOption) (*GetItemData, error) {
	out := &GetItemData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/items/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Update updates a saved item; `ItemType` is immutable (`item.update`; charged). Retried only with WithIdempotencyKey.
func (s *ItemsService) Update(ctx context.Context, id string, params *UpdateItemBody, opts ...RequestOption) (*UpdateItemData, error) {
	out := &UpdateItemData{}
	o := requestOptions(opts)
	err := s.call(ctx, Call{Method: "PATCH", Path: "/i/v1/items/" + seg(id), Body: params, Idempotent: idempotencyKeyOf(o) != "", Options: o}, out)
	return out, err
}

// Delete deletes an item no invoice line references; otherwise 409 `BIZ004`: archive it (`item.archive`).
func (s *ItemsService) Delete(ctx context.Context, id string, opts ...RequestOption) (*DeleteItemData, error) {
	out := &DeleteItemData{}
	err := s.call(ctx, Call{Method: "DELETE", Path: "/i/v1/items/" + seg(id), Options: requestOptions(opts)}, out)
	return out, err
}

// Archive archives an item (`item.archive`; idempotent).
func (s *ItemsService) Archive(ctx context.Context, id string, opts ...RequestOption) (*ArchiveItemData, error) {
	out := &ArchiveItemData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/items/" + seg(id) + "/archive", Options: requestOptions(opts)}, out)
	return out, err
}

// Unarchive unarchives an item (`item.archive`; idempotent).
func (s *ItemsService) Unarchive(ctx context.Context, id string, opts ...RequestOption) (*UnarchiveItemData, error) {
	out := &UnarchiveItemData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/items/" + seg(id) + "/unarchive", Options: requestOptions(opts)}, out)
	return out, err
}

// ListUsedCodes is the HS (`Type: "goods"`) or service (`Type: "service"`) codes already used on the
// organisation's invoices (`item.read`).
func (s *ItemsService) ListUsedCodes(ctx context.Context, query *ListUsedItemCodesQuery, opts ...RequestOption) ([]ListUsedItemCodesItem, error) {
	out := []ListUsedItemCodesItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/codes/used", Query: query, Options: requestOptions(opts)}, &out)
	return out, err
}

// ReferenceService is invoice reference (`/i/v1/invoices/hsn-codes|resources|lookup|validate`): code
// lists, tax-id lookup and the dry-run validation with the tax authority.
type ReferenceService struct{ baseService }

// ListHsCodes lists HS codes of the jurisdiction (`reference.read`; free).
func (s *ReferenceService) ListHsCodes(ctx context.Context, query *ListHsCodesQuery, opts ...RequestOption) (*Page[ListHsCodesItem], error) {
	return page[ListHsCodesItem](ctx, &s.baseService, Call{Method: "GET", Path: "/i/v1/invoices/hsn-codes", Query: query, Options: requestOptions(opts)})
}

// ListHsCodeCategories lists the HS code category names (`reference.read`; free).
func (s *ReferenceService) ListHsCodeCategories(ctx context.Context, opts ...RequestOption) ([]ListHsCodeCategoriesItem, error) {
	out := []ListHsCodeCategoriesItem{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/hsn-codes/categories", Options: requestOptions(opts)}, &out)
	return out, err
}

// ListResources lists the jurisdiction's reference code lists (`reference.read`; free).
func (s *ReferenceService) ListResources(ctx context.Context, opts ...RequestOption) (*ListInvoiceReferenceListsData, error) {
	out := &ListInvoiceReferenceListsData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/resources", Options: requestOptions(opts)}, out)
	return out, err
}

// GetResource is one code list's items, e.g. `currencies`, `tax-categories` (`reference.read`; free;
// 404 for a type not served).
func (s *ReferenceService) GetResource(ctx context.Context, listType string, opts ...RequestOption) (*GetInvoiceReferenceListData, error) {
	out := &GetInvoiceReferenceListData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/resources/" + seg(listType), Options: requestOptions(opts)}, out)
	return out, err
}

// LookupTaxID looks up a tax id with the provider (`tax_id.lookup`; charged, refunded when it could
// not answer). Sends an `Idempotency-Key`, so an SDK retry is not charged twice.
func (s *ReferenceService) LookupTaxID(ctx context.Context, value string, query *LookupTaxIdQuery, opts ...RequestOption) (*LookupTaxIdData, error) {
	out := &LookupTaxIdData{}
	err := s.call(ctx, Call{Method: "GET", Path: "/i/v1/invoices/lookup/tax-id/" + seg(value), Query: query, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}

// ValidateInvoice validates a document with the tax authority without storing it
// (`invoice.validate`; charged, refunded when it could not answer). 200 whether valid or not. Sends
// an `Idempotency-Key`.
func (s *ReferenceService) ValidateInvoice(ctx context.Context, params *ValidateInvoiceBody, opts ...RequestOption) (*ValidateInvoiceData, error) {
	out := &ValidateInvoiceData{}
	err := s.call(ctx, Call{Method: "POST", Path: "/i/v1/invoices/validate", Body: params, Idempotent: true, Options: requestOptions(opts)}, out)
	return out, err
}
