package einvoice

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
