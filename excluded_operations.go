package einvoice

// ExcludedOperation is an operation an API key may call that the SDK deliberately has no method for, and why.
type ExcludedOperation struct {
	Method string
	Path   string
	Reason string
}

// ExcludedOperations lists the API-key operations the SDK deliberately has no method for, each with
// its reason. parity_test.go requires every API-key operation in scripts/openapi-api-key-ops.json to
// be either an SDK method or listed here, and nothing here to be missing from the snapshot.
var ExcludedOperations = []ExcludedOperation{
	{Method: "POST", Path: "/i/v1/sellers", Reason: "Always 409 BIZ205: the organisation is its only seller."},
	{Method: "PATCH", Path: "/i/v1/sellers/{id}", Reason: "Always 409 BIZ205: the seller is edited as the organisation profile in the dashboard."},
	{Method: "DELETE", Path: "/i/v1/sellers/{id}", Reason: "Always 409 BIZ205: the organisation is its only seller."},
	{Method: "POST", Path: "/i/v1/sellers/{id}/verify-tax-number", Reason: "Always 409 BIZ205: the TIN is verified in organisation settings."},
	{Method: "GET", Path: "/i/v1/sellers/setup", Reason: "Dashboard form metadata; the seller itself is Sellers.Get/List."},
	{Method: "GET", Path: "/i/v1/buyers/setup", Reason: "Dashboard form metadata; the code lists are Reference.GetResource."},
	{Method: "GET", Path: "/i/v1/invoices/setup", Reason: "Dashboard form metadata; the code lists are Reference.GetResource, the connection TaxConnection.Get."},
	{Method: "GET", Path: "/b/v1/setup", Reason: "Dashboard billing overview in one call; its parts are Billing.Accounts/Subscriptions/Sandbox."},
}
