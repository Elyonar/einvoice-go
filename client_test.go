package einvoice

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// The client, case for case with einvoice-js tests/einvoice.test.ts.

func TestClientNeedsOnlyTheKeyModeAndHostFollowFromIt(t *testing.T) {
	c, err := New(testKey)
	if err != nil || c.Mode() != ModeSandbox || c.BaseURL() != "https://gp.useyona.com" {
		t.Fatal(c, err)
	}
	live, _ := New(liveKey)
	if live.Mode() != ModeLive {
		t.Fatal(live.Mode())
	}
	local, _ := New(liveKey, WithBaseURL("http://localhost:3000"))
	if local.BaseURL() != "http://localhost:3000" {
		t.Fatal(local.BaseURL())
	}
	var ce *ConfigError
	if _, err := New(testKey, WithAssertMode(ModeLive)); !errors.As(err, &ce) {
		t.Fatal(err)
	}
	if c.HTTP == nil || c.Billing.Accounts == nil || c.Webhooks.EventTypes == nil {
		t.Fatal("services missing")
	}
}

func TestClientHasNoUserRoleInvitationAPIKeyOrOrganisationManagementServices(t *testing.T) {
	ct := reflect.TypeOf(Client{})
	for _, gone := range []string{"Users", "Roles", "Invitations", "APIKeys", "ApiKeys", "Organizations", "Collection"} {
		if _, found := ct.FieldByName(gone); found {
			t.Fatalf("%s should not exist", gone)
		}
	}
	if len(ExcludedOperations) == 0 {
		t.Fatal("ExcludedOperations is empty")
	}
}

func TestClientUnwrapsDataAndPagesIntoDataAndPagination(t *testing.T) {
	pagination := map[string]any{"total": 1, "page": 1, "pageSize": 20, "totalPages": 1, "hasNext": false, "hasPrevious": false}
	f := newFake(okAnswer([]map[string]any{{"id": "b1"}}, map[string]any{"pagination": pagination, "requestId": "r1"}))
	pg, err := newClient(t, f).Buyers.List(context.Background(), &ListBuyersQuery{Page: Ptr(1.0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.Data) != 1 || pg.Data[0].ID != "b1" || pg.RequestID != "r1" {
		t.Fatalf("%+v", pg)
	}
	if pg.Pagination != (PaginationMeta{Total: 1, Page: 1, PageSize: 20, TotalPages: 1}) {
		t.Fatalf("%+v", pg.Pagination)
	}
	if f.requests[0].URL.String() != "https://gp.useyona.com/i/v1/buyers?page=1" {
		t.Fatal(f.requests[0].URL.String())
	}

	f = newFake(jsonAnswer(200, map[string]any{"meta": map[string]any{}, "data": nil}))
	empty, err := newClient(t, f).Items.List(context.Background(), nil)
	if err != nil || len(empty.Data) != 0 || empty.Data == nil || empty.Pagination.Total != 0 {
		t.Fatalf("%+v %v", empty, err)
	}

	f = newFake(okAnswer(map[string]any{"id": "inv"}))
	inv, err := newClient(t, f).Invoices.Get(context.Background(), "inv")
	if err != nil || inv.ID != "inv" {
		t.Fatal(inv, err)
	}

	// An answer of the wrong shape is reported, not silently zeroed.
	f = newFake(okAnswer(map[string]any{"id": "inv"}))
	_, err = newClient(t, f).Buyers.List(context.Background(), nil)
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatal(err)
	}
	contains(t, err.Error(), "expected shape")
}

func TestClientReceivedAndIssuedHistoryKeepDataAndMetaPagination(t *testing.T) {
	pagination := map[string]any{"total": 0, "page": 1, "pageSize": 20, "totalPages": 0, "hasNext": false, "hasPrevious": false}
	f := newFake(okAnswer(map[string]any{"items": []any{}}, map[string]any{"pagination": pagination}))
	tier := ListInboundInvoicesQueryTierHistory
	res, err := newClient(t, f).InboundInvoices.List(context.Background(), &ListInboundInvoicesQuery{Tier: &tier})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Data.Items) != 0 || res.Pagination == nil || res.Pagination.PageSize != 20 || res.RequestID != "req-ok" {
		t.Fatalf("%+v", res)
	}
	if f.requests[0].URL.Query().Get("tier") != "history" {
		t.Fatal(f.requests[0].URL.String())
	}
	f = newFake(okAnswer(map[string]any{"items": []any{}}))
	hist, err := newClient(t, f).IssuedHistory.List(context.Background(), nil)
	if err != nil || hist.Pagination != nil {
		t.Fatal(hist, err)
	}
}

func TestClientDownloadPDFAsksForThePDFAndReturnsTheBinary(t *testing.T) {
	f := newFake(answer{status: 200, body: []byte("%PDF"), headers: map[string]string{"Content-Type": "application/pdf"}})
	file, err := newClient(t, f).Output.DownloadPDF(context.Background(), "inv")
	if err != nil || string(file.Data) != "%PDF" || file.ContentType != "application/pdf" {
		t.Fatal(file, err)
	}
	req := f.requests[0]
	if req.Method != "GET" || req.URL.Path != "/i/v1/invoices/inv/download" || req.URL.Query().Get("format") != "pdf" {
		t.Fatal(req.URL.String())
	}
	if req.Header.Get("Accept") != "application/pdf" || req.Header.Get("Idempotency-Key") == "" {
		t.Fatal(req.Header)
	}
}

func TestClientBothDownloadsSendAnIdempotencyKey(t *testing.T) {
	f := newFake(okAnswer(map[string]any{}))
	c := newClient(t, f)
	if _, err := c.Output.GetDownloadLink(context.Background(), "inv"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Output.DownloadPDF(context.Background(), "inv", WithIdempotencyKey("download-inv-1")); err != nil {
		t.Fatal(err)
	}
	if f.requests[0].URL.Path != "/i/v1/invoices/inv/download" || f.requests[0].Header.Get("Idempotency-Key") == "" {
		t.Fatal(f.requests[0])
	}
	if f.requests[1].Header.Get("Idempotency-Key") != "download-inv-1" {
		t.Fatal(f.requests[1].Header)
	}
}

func TestClientABinaryRouteAnsweringJSONHandsTheJSONBackAsBytes(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"downloadUrl": "u"}))
	file, err := newClient(t, f).Output.DownloadPDF(context.Background(), "inv")
	if err != nil || file.ContentType != "application/json" || file.RequestID != "req-ok" {
		t.Fatal(file, err)
	}
	equalJSON(t, json.RawMessage(file.Data), map[string]any{"downloadUrl": "u"})
}

func TestClientEncodesPathSegments(t *testing.T) {
	f := newFake(okAnswer(map[string]any{}))
	if _, err := newClient(t, f).Reference.LookupTaxID(context.Background(), "12345678/0001", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.requests[0].URL.EscapedPath(); got != "/i/v1/invoices/lookup/tax-id/12345678%2F0001" {
		t.Fatal(got)
	}
}

func TestClientWebhooksEndpointsTestSendsAnEmptyBodyByDefault(t *testing.T) {
	f := newFake(okAnswer(map[string]any{}))
	c := newClient(t, f)
	if _, err := c.Webhooks.Endpoints.Test(context.Background(), "ep", nil); err != nil {
		t.Fatal(err)
	}
	if f.bodies[0] != "{}" || f.requests[0].Header.Get("Content-Type") != "application/json" {
		t.Fatal(f.bodies[0])
	}
	if _, err := c.Webhooks.Endpoints.Test(context.Background(), "ep", &TestWebhookEndpointBody{Type: Ptr("invoice.signed")}); err != nil {
		t.Fatal(err)
	}
	if f.bodies[1] != `{"type":"invoice.signed"}` {
		t.Fatal(f.bodies[1])
	}
}

func TestClientBillingAccountsGetStatsDefaultsToTheCallersOwnAccount(t *testing.T) {
	f := newFake(okAnswer(map[string]any{}))
	if _, err := newClient(t, f).Billing.Accounts.GetStats(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	if f.requests[0].URL.Path != "/b/v1/billing-accounts/me/stats" {
		t.Fatal(f.requests[0].URL.Path)
	}
}

func TestClientPATCHUpdatesSendAKeyOnlyWhenTheCallerGivesOne(t *testing.T) {
	f := newFake(okAnswer(map[string]any{}))
	c := newClient(t, f)
	ctx := context.Background()
	if _, err := c.Buyers.Update(ctx, "b1", &UpdateBuyerBody{Name: Ptr("X")}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Buyers.Update(ctx, "b1", &UpdateBuyerBody{Name: Ptr("X")}, WithIdempotencyKey("caller-key-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Items.Update(ctx, "i1", &UpdateItemBody{}, WithIdempotencyKey("caller-key-2")); err != nil {
		t.Fatal(err)
	}
	if f.requests[0].Header.Get("Idempotency-Key") != "" || f.requests[1].Header.Get("Idempotency-Key") != "caller-key-1" || f.requests[2].Header.Get("Idempotency-Key") != "caller-key-2" {
		t.Fatal("PATCH keys")
	}
}

func TestClientRequestOptionsReachTheTransport(t *testing.T) {
	f := newFake(refusal(503, "SYS001", nil, nil))
	c := newClient(t, f)
	_, err := c.Invoices.Get(context.Background(), "x", WithMaxRetries(0), WithRequestHeaders(map[string]string{"X-Trace": "t1"}), WithRequestTimeout(time.Second))
	var se *ServerError
	if !errors.As(err, &se) || f.calls() != 1 || f.requests[0].Header.Get("X-Trace") != "t1" {
		t.Fatal(err, f.calls())
	}
}

func TestClientIssueCreditNoteDecodesEitherShape(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"preview": map[string]any{}}))
	data, err := newClient(t, f).Invoices.IssueCreditNote(context.Background(), "inv", &IssueCreditNoteBody{})
	if err != nil || len(data.RawMessage) == 0 {
		t.Fatal(data, err)
	}
	if _, err := data.AsCreditNotePreviewAnswerDto(); err != nil {
		t.Fatal(err)
	}
}

func TestClientTaxConnectionGetKeepsTheOptionsOnlyCallOf08AndGetWithQueryTakesTheR151Query(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"state": "connected"}))
	c := newClient(t, f)
	ctx := context.Background()
	if _, err := c.TaxConnection.Get(ctx, WithRequestHeaders(map[string]string{"X-Trace": "t1"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.TaxConnection.GetWithQuery(ctx, &GetTaxConnectionQuery{Identifier: Ptr(GetTaxConnectionQueryIdentifierTrue)}, WithRequestHeaders(map[string]string{"X-Trace": "t2"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.TaxConnection.GetWithQuery(ctx, &GetTaxConnectionQuery{Fresh: Ptr(GetTaxConnectionQueryFreshTrue)}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.TaxConnection.Get(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.TaxConnection.GetWithQuery(ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"", "identifier=true", "fresh=true", "", ""}
	for i, q := range want {
		if got := f.requests[i].URL.RawQuery; got != q || f.requests[i].URL.Path != "/i/v1/tax-connection" {
			t.Fatalf("call %d: %s?%s", i, f.requests[i].URL.Path, got)
		}
	}
	if f.requests[0].Header.Get("X-Trace") != "t1" || f.requests[1].Header.Get("X-Trace") != "t2" {
		t.Fatal("options lost")
	}
}

func TestClientListsSubOrganisationsAndTheirReadinessAsPagesJoiningAListOfIDs(t *testing.T) {
	pagination := map[string]any{"total": 2, "page": 1, "pageSize": 100, "totalPages": 1, "hasNext": false, "hasPrevious": false}
	f := newFake(okAnswer([]map[string]any{{"id": "c1"}, {"id": "c2"}}, map[string]any{"pagination": pagination}))
	c := newClient(t, f)
	ctx := context.Background()
	pg, err := c.Organization.ListSubOrganizations(ctx, &ListSubOrganizationsQuery{Search: Ptr("acme"), Limit: Ptr(50.0)})
	if err != nil || len(pg.Data) != 2 || pg.Data[1].ID != "c2" || pg.Pagination.Total != 2 || pg.Pagination.PageSize != 100 || pg.RequestID != "req-ok" {
		t.Fatalf("%+v %v", pg, err)
	}
	if _, err := c.Organization.ListSubOrganizations(ctx, &ListSubOrganizationsQuery{IDs: []string{"c1", "c2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Organization.ListSubOrganizations(ctx, &ListSubOrganizationsQuery{IDs: IDs{"c1,c2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Organization.ListSubOrganizations(ctx, nil); err != nil {
		t.Fatal(err)
	}
	rd, err := c.Organization.ListSubOrganizationsReadiness(ctx, &ListSubOrganizationsReadinessQuery{IDs: IDs{"c1"}, Page: Ptr(2.0)})
	if err != nil || len(rd.Data) != 2 || rd.Data[0].ID != "c1" {
		t.Fatalf("%+v %v", rd, err)
	}
	want := []string{
		"GET /a/v1/organizations/me/children?limit=50&search=acme",
		"GET /a/v1/organizations/me/children?ids=c1%2Cc2",
		"GET /a/v1/organizations/me/children?ids=c1%2Cc2",
		"GET /a/v1/organizations/me/children",
		"GET /a/v1/organizations/me/children/readiness?page=2&ids=c1",
	}
	for i, w := range want {
		got := f.requests[i].Method + " " + f.requests[i].URL.Path
		if q := f.requests[i].URL.RawQuery; q != "" {
			got += "?" + q
		}
		if got != w {
			t.Fatalf("call %d: %s, want %s", i, got, w)
		}
	}
}

func TestClientAListOfIDsReachesTheAPIAsOneCommaSeparatedValueNotARepeatedKey(t *testing.T) {
	f := newFake(okAnswer([]map[string]any{{"organizationId": "a", "state": "connected", "submissionsAllowed": true}}))
	list, err := newClient(t, f).TaxConnection.ListForSubOrganizations(context.Background(), &SubOrganizationTaxConnectionsQuery{IDs: []string{"a", "b"}})
	if err != nil || len(list) != 1 || list[0].OrganizationID != "a" || !list[0].SubmissionsAllowed {
		t.Fatalf("%+v %v", list, err)
	}
	u := f.requests[0].URL
	if ids := u.Query()["ids"]; u.Path != "/i/v1/tax-connection/children" || len(ids) != 1 || ids[0] != "a,b" {
		t.Fatal(u.String())
	}

	// In a map query too, and an empty list sends nothing.
	if qs, err := encodeQuery(map[string]any{"ids": IDs{"a", "b"}, "tag": []string{"x", "y"}}); err != nil || qs != "ids=a%2Cb&tag=x&tag=y" {
		t.Fatal(qs, err)
	}
	if qs, err := encodeQuery(&ListSubOrganizationsQuery{IDs: IDs{}}); err != nil || qs != "" {
		t.Fatal(qs, err)
	}

	// The portal's playground hands a query over as JSON, where ids is the API's string.
	var q SubOrganizationTaxConnectionsQuery
	if err := json.Unmarshal([]byte(`{"ids":"a,b"}`), &q); err != nil || len(q.IDs) != 2 || q.IDs.String() != "a,b" {
		t.Fatal(q, err)
	}
	if err := json.Unmarshal([]byte(`{"ids":["c"]}`), &q); err != nil || len(q.IDs) != 1 || q.IDs[0] != "c" {
		t.Fatal(q, err)
	}
	if err := json.Unmarshal([]byte(`{"ids":""}`), &q); err != nil || len(q.IDs) != 0 {
		t.Fatal(q, err)
	}
	if err := json.Unmarshal([]byte(`{"ids":7}`), &q); err == nil {
		t.Fatal("a number is not a list of ids")
	}
	if raw, err := json.Marshal(SubOrganizationTaxConnectionsQuery{IDs: IDs{"a", "b"}}); err != nil || string(raw) != `{"ids":"a,b"}` {
		t.Fatal(string(raw), err)
	}
}

// The hand-written sub-organisation queries and body are the generated ones with `ids` as a list
// and without `country`: a parameter the API adds must be added to them too.
func TestClientTheSubOrganisationParamsCarryTheOperationsOwnFields(t *testing.T) {
	names := func(v any, without string) string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			if name := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]; name != without {
				out = append(out, name)
			}
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	pairs := [][2]string{
		{names(ListSubOrganizationsQuery{}, ""), names(ListChildOrganizationsQuery{}, "")},
		{names(ListSubOrganizationsReadinessQuery{}, ""), names(ListChildOrganizationsReadinessQuery{}, "")},
		{names(SubOrganizationTaxConnectionsQuery{}, ""), names(ListChildTaxConnectionsQuery{}, "")},
		{names(CreateSubOrganizationBody{}, ""), names(CreateOrganizationBody{}, "country")},
	}
	for i, p := range pairs {
		if p[0] != p[1] || p[0] == "" {
			t.Fatalf("pair %d: %q, the operation has %q", i, p[0], p[1])
		}
	}
}

func TestClientCreateSubOrganizationPostsTheBodyWithNoIdempotencyKeyAndSoNoRetry(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"id": "c9", "type": "secondary", "status": "pending_verification"}))
	c, err := New(liveKey, WithTransport(f))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := c.Organization.CreateSubOrganization(ctx, &CreateSubOrganizationBody{BusinessName: "Acme Retail Ltd", Email: "accounts@acme.example"})
	if err != nil || created.ID != "c9" || created.Type != "secondary" || created.Status != "pending_verification" {
		t.Fatalf("%+v %v", created, err)
	}
	req := f.requests[0]
	if req.Method != "POST" || req.URL.Path != "/a/v1/organizations" || req.Header.Get("Idempotency-Key") != "" {
		t.Fatal(req.Method, req.URL.Path, req.Header.Get("Idempotency-Key"))
	}
	if body := f.bodies[0]; body != `{"businessName":"Acme Retail Ltd","email":"accounts@acme.example"}` {
		t.Fatal(body)
	}

	f = newFake(refusal(503, "SYS001", nil, nil), refusal(503, "SYS001", nil, nil))
	_, err = newClient(t, f).Organization.CreateSubOrganization(ctx, &CreateSubOrganizationBody{BusinessName: "Acme Retail Ltd", Email: "accounts@acme.example"})
	var se *ServerError
	if !errors.As(err, &se) || f.calls() != 1 {
		t.Fatal(err, f.calls())
	}
}

func TestClientBillingPartnerGetKeepsTheOptionsOnlyCallOf09AndGetWithQueryTakesThePageOfBusinesses(t *testing.T) {
	f := newFake(okAnswer(map[string]any{"businessesPage": map[string]any{"page": 2, "limit": 50, "total": 120}}))
	c := newClient(t, f)
	ctx := context.Background()
	if _, err := c.Billing.Partner.Get(ctx, WithRequestHeaders(map[string]string{"X-Trace": "t1"})); err != nil {
		t.Fatal(err)
	}
	partner, err := c.Billing.Partner.GetWithQuery(ctx, &GetPartnerQuery{Page: Ptr(int64(2)), Limit: Ptr(int64(50))}, WithRequestHeaders(map[string]string{"X-Trace": "t2"}))
	if err != nil || partner.BusinessesPage.Page != 2 || partner.BusinessesPage.Limit != 50 || partner.BusinessesPage.Total != 120 {
		t.Fatalf("%+v %v", partner, err)
	}
	if _, err := c.Billing.Partner.Get(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Billing.Partner.GetWithQuery(ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"", "page=2&limit=50", "", ""}
	for i, q := range want {
		if got := f.requests[i].URL.RawQuery; got != q || f.requests[i].URL.Path != "/b/v1/partner" {
			t.Fatalf("call %d: %s?%s", i, f.requests[i].URL.Path, got)
		}
	}
	if f.requests[0].Header.Get("X-Trace") != "t1" || f.requests[1].Header.Get("X-Trace") != "t2" {
		t.Fatal("options lost")
	}
}

func TestClientCodeListItemsTypesTheStatesAndLGALists(t *testing.T) {
	f := newFake(
		okAnswer(map[string]any{"type": "lgas", "version": 1, "fetchedAt": "2026-11-10T00:00:00Z", "stale": false, "items": []map[string]any{
			{"code": "NG-LA-IKE", "name": "Ikeja", "stateCode": "NG-LA"},
			{"code": "NG-AB-ABN", "name": "Aba North", "stateCode": "NG-AB"},
		}}),
		okAnswer(map[string]any{"type": "states", "version": 1, "fetchedAt": "2026-11-10T00:00:00Z", "stale": false, "items": []map[string]any{{"code": "NG-LA", "name": "Lagos"}}}),
		okAnswer(map[string]any{"type": "countries", "version": 1, "fetchedAt": "2026-11-10T00:00:00Z", "stale": false, "items": []map[string]any{{"code": "NG", "name": "Nigeria"}}}),
		okAnswer(map[string]any{"type": "currencies", "version": 1, "fetchedAt": "2026-11-10T00:00:00Z", "stale": false, "items": []any{}}),
	)
	c := newClient(t, f)
	ctx := context.Background()
	list, err := c.Reference.GetResource(ctx, "lgas")
	if err != nil {
		t.Fatal(err)
	}
	lgas, err := CodeListItems[LGAOption](list)
	if err != nil || len(lgas) != 2 || lgas[0] != (LGAOption{Code: "NG-LA-IKE", Name: "Ikeja", StateCode: "NG-LA"}) {
		t.Fatalf("%+v %v", lgas, err)
	}
	list, _ = c.Reference.GetResource(ctx, "states")
	states, err := CodeListItems[StateOption](list)
	if err != nil || len(states) != 1 || states[0] != (StateOption{Code: "NG-LA", Name: "Lagos"}) {
		t.Fatalf("%+v %v", states, err)
	}
	list, _ = c.Reference.GetResource(ctx, "countries")
	countries, err := CodeListItems[CountryOption](list)
	if err != nil || len(countries) != 1 || countries[0].Code != "NG" || countries[0].Name != "Nigeria" {
		t.Fatalf("%+v %v", countries, err)
	}
	list, _ = c.Reference.GetResource(ctx, "currencies")
	if empty, err := CodeListItems[StateOption](list); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("%+v %v", empty, err)
	}
	if none, err := CodeListItems[StateOption](nil); err != nil || none != nil {
		t.Fatalf("%+v %v", none, err)
	}
	if _, err := CodeListItems[int](&GetInvoiceReferenceListData{Items: []map[string]any{{"code": "NG"}}}); err == nil {
		t.Fatal("an item that is not a T is an error")
	}
	if f.requests[0].URL.Path != "/i/v1/invoices/resources/lgas" || f.requests[1].URL.Path != "/i/v1/invoices/resources/states" {
		t.Fatal(f.requests[0].URL.Path, f.requests[1].URL.Path)
	}
}

func TestClientHasNoPaymentsServiceMethodsUntilPaymentsLaunchesInProduction(t *testing.T) {
	ct := reflect.TypeOf(Client{})
	for _, gone := range []string{"BankAccount", "Transfers", "PaymentsAccount"} {
		if _, found := ct.FieldByName(gone); found {
			t.Fatalf("%s should not exist", gone)
		}
	}
	n := 0
	for _, e := range ExcludedOperations {
		if strings.HasPrefix(e.Path, "/p/") {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("%d excluded payments operations", n)
	}
}

func TestClientNullableBillingAnswersAreNil(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(c *Client) (bool, error){
		"Usage.GetInvoices": func(c *Client) (bool, error) {
			v, err := c.Billing.Usage.GetInvoices(ctx)
			return v == nil, err
		},
		"Plans.GetRecommendation": func(c *Client) (bool, error) {
			v, err := c.Billing.Plans.GetRecommendation(ctx)
			return v == nil, err
		},
		"Partner.GetOffer": func(c *Client) (bool, error) {
			v, err := c.Billing.Partner.GetOffer(ctx)
			return v == nil, err
		},
		"Subscriptions.GetDealOffer": func(c *Client) (bool, error) {
			v, err := c.Billing.Subscriptions.GetDealOffer(ctx)
			return v == nil, err
		},
	}
	for name, call := range calls {
		isNil, err := call(newClient(t, newFake(jsonAnswer(200, map[string]any{"meta": map[string]any{}, "data": nil}))))
		if err != nil || !isNil {
			t.Fatalf("%s on null: nil=%v %v", name, isNil, err)
		}
		isNil, err = call(newClient(t, newFake(okAnswer(map[string]any{}))))
		if err != nil || isNil {
			t.Fatalf("%s on an object: nil=%v %v", name, isNil, err)
		}
	}
}

func TestClientCollectionsWritesAreNotRetriedAndListsDecodePlainArrays(t *testing.T) {
	f := newFake(refusal(503, "SYS001", nil, nil), refusal(503, "SYS001", nil, nil))
	c := newClient(t, f)
	ctx := context.Background()
	_, err := c.Collections.CreateAccount(ctx, &CreateCollectionAccountBody{})
	var se *ServerError
	if !errors.As(err, &se) || f.calls() != 1 || f.requests[0].Header.Get("Idempotency-Key") != "" {
		t.Fatal(err, f.calls())
	}

	f = newFake(okAnswer([]map[string]any{{"uuid": "058", "name": "A bank", "sortCode": "058"}}))
	banks, err := newClient(t, f).Collections.ListBanks(ctx, &ListCollectionBanksQuery{Jurisdiction: ListCollectionBanksQueryJurisdictionNG})
	if err != nil || len(banks) != 1 || banks[0].SortCode != "058" || f.requests[0].URL.RawQuery != "jurisdiction=NG" {
		t.Fatal(banks, err, f.requests[0].URL.RawQuery)
	}
}

// ── Paginate ──

func TestPaginateWalksEveryPageUntilHasNextIsFalse(t *testing.T) {
	pages := []*Page[int]{
		{Data: []int{1, 2}, Pagination: PaginationMeta{HasNext: true}},
		{Data: []int{3}, Pagination: PaginationMeta{HasNext: false}},
	}
	var seen []int64
	var out []int
	err := Paginate(context.Background(), func(_ context.Context, page int64) (*Page[int], error) {
		seen = append(seen, page)
		return pages[page-1], nil
	}, func(n int) error {
		out = append(out, n)
		return nil
	})
	if err != nil || !reflect.DeepEqual(out, []int{1, 2, 3}) || !reflect.DeepEqual(seen, []int64{1, 2}) {
		t.Fatal(err, out, seen)
	}
}

func TestPaginateStopsOnAnEmptyPageEvenIfHasNextIsTrue(t *testing.T) {
	calls := 0
	out, err := Collect(context.Background(), func(context.Context, int64) (*Page[int], error) {
		calls++
		return &Page[int]{Data: []int{}, Pagination: PaginationMeta{HasNext: true}}, nil
	})
	if err != nil || len(out) != 0 || calls != 1 {
		t.Fatal(err, out, calls)
	}
}

func TestPaginatePropagatesErrorsAndStopsWhenFnAsks(t *testing.T) {
	boom := errors.New("boom")
	err := Paginate(context.Background(), func(context.Context, int64) (*Page[int], error) { return nil, boom }, func(int) error { return nil })
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	stop := errors.New("stop")
	calls := 0
	err = Paginate(context.Background(), func(context.Context, int64) (*Page[int], error) {
		calls++
		return &Page[int]{Data: []int{1, 2}, Pagination: PaginationMeta{HasNext: true}}, nil
	}, func(int) error { return stop })
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatal(err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = Paginate(ctx, func(context.Context, int64) (*Page[int], error) { t.Fatal("must not be called"); return nil, nil }, func(int) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPaginateOverARealList(t *testing.T) {
	f := newFake(
		okAnswer([]map[string]any{{"id": "b1"}}, map[string]any{"pagination": map[string]any{"hasNext": true}}),
		okAnswer([]map[string]any{{"id": "b2"}}, map[string]any{"pagination": map[string]any{"hasNext": false}}),
	)
	c := newClient(t, f)
	query := &ListBuyersQuery{Limit: Ptr(1.0)}
	buyers, err := Collect(context.Background(), func(ctx context.Context, page int64) (*Page[BuyerViewDto], error) {
		query.Page = Ptr(float64(page))
		return c.Buyers.List(ctx, query)
	})
	if err != nil || len(buyers) != 2 || buyers[1].ID != "b2" {
		t.Fatal(err, buyers)
	}
	if f.requests[1].URL.RawQuery != "page=2&limit=1" {
		t.Fatal(f.requests[1].URL.RawQuery)
	}
}
