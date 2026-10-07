package einvoice

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// PARITY: the SDK's methods and the API-key operations of the gateway's OpenAPI
// (scripts/openapi-api-key-ops.json, copied from einvoice-js by `make sync`) must agree both ways:
//   - every SDK method calls exactly one snapshot operation;
//   - every snapshot operation is called by at least one method, or is in ExcludedOperations with a
//     reason, and every exclusion names a snapshot operation no method calls;
//   - an Idempotency-Key is generated exactly on the POST/GET routes that accept one;
//   - the generated models are what the snapshot generates (`make sync-check`, run by CI).
//
// The same five assertions as einvoice-js tests/parity.test.ts. Go cannot synthesise arguments by
// reflection, so parityMethods lists one call per method; a reflection pass over every exported
// method of every service reachable from Client fails when a method is missing from the registry.

type snapshotOp struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operationId"`
	Description string `json:"description"`
	Parameters  []struct {
		In   string `json:"in"`
		Name string `json:"name"`
	} `json:"parameters"`
}

type snapshot struct {
	Count      int          `json:"count"`
	Operations []snapshotOp `json:"operations"`
}

// keyedButUndocumented: routes whose backend controller reads `Idempotency-Key` although the OpenAPI
// does not say so yet (mirrors einvoice-js: GET /i/v1/invoices/{id}/download is charged per
// download; the key dedupes the charge).
var keyedButUndocumented = []string{"GET /i/v1/invoices/{id}/download"}

type parityEntry struct {
	method string
	call   func(ctx context.Context, c *Client) error
}

var parityMethods = []parityEntry{
	{"Invoices.Create", func(ctx context.Context, c *Client) error {
		_, err := c.Invoices.Create(ctx, &CreateInvoiceBody{})
		return err
	}},
	{"Invoices.List", func(ctx context.Context, c *Client) error { _, err := c.Invoices.List(ctx, nil); return err }},
	{"Invoices.Get", func(ctx context.Context, c *Client) error { _, err := c.Invoices.Get(ctx, "ID1"); return err }},
	{"Invoices.Update", func(ctx context.Context, c *Client) error {
		_, err := c.Invoices.Update(ctx, "ID1", &UpdateInvoiceBody{})
		return err
	}},
	{"Invoices.Delete", func(ctx context.Context, c *Client) error { _, err := c.Invoices.Delete(ctx, "ID1"); return err }},
	{"Invoices.Finalise", func(ctx context.Context, c *Client) error { _, err := c.Invoices.Finalise(ctx, "ID1"); return err }},
	{"Invoices.Reopen", func(ctx context.Context, c *Client) error { _, err := c.Invoices.Reopen(ctx, "ID1"); return err }},
	{"Invoices.Revise", func(ctx context.Context, c *Client) error { _, err := c.Invoices.Revise(ctx, "ID1"); return err }},
	{"Invoices.Cancel", func(ctx context.Context, c *Client) error {
		_, err := c.Invoices.Cancel(ctx, "ID1", &CancelInvoiceBody{})
		return err
	}},
	{"Invoices.IssueCreditNote", func(ctx context.Context, c *Client) error {
		_, err := c.Invoices.IssueCreditNote(ctx, "ID1", &IssueCreditNoteBody{})
		return err
	}},
	{"Invoices.IssueDebitNote", func(ctx context.Context, c *Client) error {
		_, err := c.Invoices.IssueDebitNote(ctx, "ID1", &IssueDebitNoteBody{})
		return err
	}},
	{"Invoices.GetOverview", func(ctx context.Context, c *Client) error { _, err := c.Invoices.GetOverview(ctx, nil); return err }},
	{"Invoices.GetStatistics", func(ctx context.Context, c *Client) error { _, err := c.Invoices.GetStatistics(ctx, nil); return err }},
	{"Invoices.GetSummary", func(ctx context.Context, c *Client) error { _, err := c.Invoices.GetSummary(ctx, nil); return err }},

	{"Submissions.Submit", func(ctx context.Context, c *Client) error {
		_, err := c.Submissions.Submit(ctx, "ID1", nil)
		return err
	}},
	{"Submissions.Issue", func(ctx context.Context, c *Client) error { _, err := c.Submissions.Issue(ctx, "ID1"); return err }},
	{"Submissions.CreateAndSubmit", func(ctx context.Context, c *Client) error {
		_, err := c.Submissions.CreateAndSubmit(ctx, &CreateAndSubmitInvoiceBody{})
		return err
	}},
	{"Submissions.BatchSubmit", func(ctx context.Context, c *Client) error {
		_, err := c.Submissions.BatchSubmit(ctx, &BatchSubmitInvoicesBody{})
		return err
	}},
	{"Submissions.Retry", func(ctx context.Context, c *Client) error { _, err := c.Submissions.Retry(ctx, "ID1"); return err }},
	{"Submissions.Renumber", func(ctx context.Context, c *Client) error { _, err := c.Submissions.Renumber(ctx, "ID1"); return err }},
	{"Submissions.QueryStatus", func(ctx context.Context, c *Client) error {
		_, err := c.Submissions.QueryStatus(ctx, "ID1")
		return err
	}},
	{"Submissions.GetStatus", func(ctx context.Context, c *Client) error { _, err := c.Submissions.GetStatus(ctx, "ID1"); return err }},
	{"Submissions.RecordPayment", func(ctx context.Context, c *Client) error {
		_, err := c.Submissions.RecordPayment(ctx, "ID1", &RecordInvoicePaymentBody{})
		return err
	}},
	{"Submissions.GetAuthorityCopy", func(ctx context.Context, c *Client) error {
		_, err := c.Submissions.GetAuthorityCopy(ctx, "ID1")
		return err
	}},

	{"Output.GetDownloadLink", func(ctx context.Context, c *Client) error { _, err := c.Output.GetDownloadLink(ctx, "ID1"); return err }},
	{"Output.DownloadPDF", func(ctx context.Context, c *Client) error { _, err := c.Output.DownloadPDF(ctx, "ID1"); return err }},
	{"Output.Send", func(ctx context.Context, c *Client) error {
		_, err := c.Output.Send(ctx, "ID1", &SendInvoiceBody{})
		return err
	}},

	{"ShareLinks.Create", func(ctx context.Context, c *Client) error {
		_, err := c.ShareLinks.Create(ctx, "ID1", &CreateInvoiceShareLinkBody{})
		return err
	}},
	{"ShareLinks.List", func(ctx context.Context, c *Client) error { _, err := c.ShareLinks.List(ctx, "ID1"); return err }},
	{"ShareLinks.Revoke", func(ctx context.Context, c *Client) error {
		_, err := c.ShareLinks.Revoke(ctx, "ID1", "ID2")
		return err
	}},

	{"Items.List", func(ctx context.Context, c *Client) error { _, err := c.Items.List(ctx, nil); return err }},
	{"Items.Create", func(ctx context.Context, c *Client) error {
		_, err := c.Items.Create(ctx, &CreateItemBody{})
		return err
	}},
	{"Items.Get", func(ctx context.Context, c *Client) error { _, err := c.Items.Get(ctx, "ID1"); return err }},
	{"Items.Update", func(ctx context.Context, c *Client) error {
		_, err := c.Items.Update(ctx, "ID1", &UpdateItemBody{})
		return err
	}},
	{"Items.Delete", func(ctx context.Context, c *Client) error { _, err := c.Items.Delete(ctx, "ID1"); return err }},
	{"Items.Archive", func(ctx context.Context, c *Client) error { _, err := c.Items.Archive(ctx, "ID1"); return err }},
	{"Items.Unarchive", func(ctx context.Context, c *Client) error { _, err := c.Items.Unarchive(ctx, "ID1"); return err }},
	{"Items.ListUsedCodes", func(ctx context.Context, c *Client) error { _, err := c.Items.ListUsedCodes(ctx, nil); return err }},

	{"Reference.ListHsCodes", func(ctx context.Context, c *Client) error { _, err := c.Reference.ListHsCodes(ctx, nil); return err }},
	{"Reference.ListHsCodeCategories", func(ctx context.Context, c *Client) error {
		_, err := c.Reference.ListHsCodeCategories(ctx)
		return err
	}},
	{"Reference.ListResources", func(ctx context.Context, c *Client) error { _, err := c.Reference.ListResources(ctx); return err }},
	{"Reference.GetResource", func(ctx context.Context, c *Client) error { _, err := c.Reference.GetResource(ctx, "ID1"); return err }},
	{"Reference.LookupTaxID", func(ctx context.Context, c *Client) error {
		_, err := c.Reference.LookupTaxID(ctx, "ID1", nil)
		return err
	}},
	{"Reference.ValidateInvoice", func(ctx context.Context, c *Client) error {
		_, err := c.Reference.ValidateInvoice(ctx, &ValidateInvoiceBody{})
		return err
	}},

	{"Buyers.Create", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.Create(ctx, &CreateBuyerBody{})
		return err
	}},
	{"Buyers.List", func(ctx context.Context, c *Client) error { _, err := c.Buyers.List(ctx, nil); return err }},
	{"Buyers.Get", func(ctx context.Context, c *Client) error { _, err := c.Buyers.Get(ctx, "ID1"); return err }},
	{"Buyers.Update", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.Update(ctx, "ID1", &UpdateBuyerBody{})
		return err
	}},
	{"Buyers.Delete", func(ctx context.Context, c *Client) error { _, err := c.Buyers.Delete(ctx, "ID1"); return err }},
	{"Buyers.BulkDelete", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.BulkDelete(ctx, &BulkDeleteBuyersBody{})
		return err
	}},
	{"Buyers.VerifyTaxNumber", func(ctx context.Context, c *Client) error { _, err := c.Buyers.VerifyTaxNumber(ctx, "ID1"); return err }},
	{"Buyers.GetVerificationStatus", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.GetVerificationStatus(ctx, "ID1")
		return err
	}},
	{"Buyers.Search", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.Search(ctx, &SearchBuyersQuery{})
		return err
	}},
	{"Buyers.CheckReachability", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.CheckReachability(ctx, &CheckBuyerReachabilityQuery{})
		return err
	}},
	{"Buyers.CheckWithTaxAuthority", func(ctx context.Context, c *Client) error {
		_, err := c.Buyers.CheckWithTaxAuthority(ctx, &CheckBuyerWithTaxAuthorityBody{})
		return err
	}},

	{"Sellers.List", func(ctx context.Context, c *Client) error { _, err := c.Sellers.List(ctx, nil); return err }},
	{"Sellers.Get", func(ctx context.Context, c *Client) error { _, err := c.Sellers.Get(ctx, "ID1"); return err }},
	{"Sellers.GetVerificationStatus", func(ctx context.Context, c *Client) error {
		_, err := c.Sellers.GetVerificationStatus(ctx, "ID1")
		return err
	}},
	{"Sellers.Search", func(ctx context.Context, c *Client) error {
		_, err := c.Sellers.Search(ctx, &SearchSellersQuery{})
		return err
	}},

	{"InboundInvoices.List", func(ctx context.Context, c *Client) error { _, err := c.InboundInvoices.List(ctx, nil); return err }},
	{"InboundInvoices.Get", func(ctx context.Context, c *Client) error { _, err := c.InboundInvoices.Get(ctx, "ID1"); return err }},
	{"InboundInvoices.GetAnalytics", func(ctx context.Context, c *Client) error {
		_, err := c.InboundInvoices.GetAnalytics(ctx, nil)
		return err
	}},
	{"InboundInvoices.ListHistoryRuns", func(ctx context.Context, c *Client) error {
		_, err := c.InboundInvoices.ListHistoryRuns(ctx, nil)
		return err
	}},
	{"InboundInvoices.GetHistoryRun", func(ctx context.Context, c *Client) error {
		_, err := c.InboundInvoices.GetHistoryRun(ctx, "ID1")
		return err
	}},
	{"InboundInvoices.LoadOlder", func(ctx context.Context, c *Client) error { _, err := c.InboundInvoices.LoadOlder(ctx); return err }},

	{"IssuedHistory.List", func(ctx context.Context, c *Client) error { _, err := c.IssuedHistory.List(ctx, nil); return err }},
	{"IssuedHistory.Get", func(ctx context.Context, c *Client) error { _, err := c.IssuedHistory.Get(ctx, "ID1"); return err }},
	{"IssuedHistory.DownloadPDF", func(ctx context.Context, c *Client) error {
		_, err := c.IssuedHistory.DownloadPDF(ctx, "ID1")
		return err
	}},

	{"InvoiceSettings.Get", func(ctx context.Context, c *Client) error { _, err := c.InvoiceSettings.Get(ctx); return err }},
	{"TaxConnection.Get", func(ctx context.Context, c *Client) error { _, err := c.TaxConnection.Get(ctx); return err }},
	{"Organization.Get", func(ctx context.Context, c *Client) error { _, err := c.Organization.Get(ctx, "ID1"); return err }},
	{"Organization.GetReadiness", func(ctx context.Context, c *Client) error { _, err := c.Organization.GetReadiness(ctx); return err }},

	{"Billing.Accounts.GetMine", func(ctx context.Context, c *Client) error { _, err := c.Billing.Accounts.GetMine(ctx, nil); return err }},
	{"Billing.Accounts.GetStats", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Accounts.GetStats(ctx, "ID1", nil)
		return err
	}},
	{"Billing.Accounts.CheckBalance", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Accounts.CheckBalance(ctx, "ID1", &CheckBillingAccountBalanceQuery{Credits: 1})
		return err
	}},
	{"Billing.Payments.List", func(ctx context.Context, c *Client) error { _, err := c.Billing.Payments.List(ctx, nil); return err }},
	{"Billing.Payments.Get", func(ctx context.Context, c *Client) error { _, err := c.Billing.Payments.Get(ctx, "ID1"); return err }},
	{"Billing.Sandbox.ListTransactions", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Sandbox.ListTransactions(ctx, nil)
		return err
	}},
	{"Billing.Sandbox.GetUsage", func(ctx context.Context, c *Client) error { _, err := c.Billing.Sandbox.GetUsage(ctx); return err }},
	{"Billing.Statements.Get", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Statements.Get(ctx, "ID1", nil)
		return err
	}},
	{"Billing.Subscriptions.GetActive", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Subscriptions.GetActive(ctx)
		return err
	}},
	{"Billing.Subscriptions.List", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Subscriptions.List(ctx, nil)
		return err
	}},
	{"Billing.Subscriptions.Get", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Subscriptions.Get(ctx, "ID1")
		return err
	}},
	{"Billing.Subscriptions.ListRenewals", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Subscriptions.ListRenewals(ctx, nil)
		return err
	}},
	{"Billing.Subscriptions.PreviewPlanChange", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Subscriptions.PreviewPlanChange(ctx, "ID1", &PreviewSubscriptionPlanChangeQuery{})
		return err
	}},
	{"Billing.Transactions.List", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Transactions.List(ctx, nil)
		return err
	}},
	{"Billing.Transactions.Get", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Transactions.Get(ctx, "ID1")
		return err
	}},
	{"Billing.Transactions.GetUsageAnalytics", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Transactions.GetUsageAnalytics(ctx, nil)
		return err
	}},
	{"Billing.Transactions.GetUsageByCostCode", func(ctx context.Context, c *Client) error {
		_, err := c.Billing.Transactions.GetUsageByCostCode(ctx, nil)
		return err
	}},

	{"Webhooks.Endpoints.List", func(ctx context.Context, c *Client) error { _, err := c.Webhooks.Endpoints.List(ctx); return err }},
	{"Webhooks.Endpoints.Get", func(ctx context.Context, c *Client) error { _, err := c.Webhooks.Endpoints.Get(ctx, "ID1"); return err }},
	{"Webhooks.Endpoints.Test", func(ctx context.Context, c *Client) error {
		_, err := c.Webhooks.Endpoints.Test(ctx, "ID1", nil)
		return err
	}},
	{"Webhooks.Deliveries.List", func(ctx context.Context, c *Client) error { _, err := c.Webhooks.Deliveries.List(ctx, nil); return err }},
	{"Webhooks.Deliveries.Get", func(ctx context.Context, c *Client) error {
		_, err := c.Webhooks.Deliveries.Get(ctx, "ID1")
		return err
	}},
	{"Webhooks.Deliveries.Redeliver", func(ctx context.Context, c *Client) error {
		_, err := c.Webhooks.Deliveries.Redeliver(ctx, "ID1")
		return err
	}},
	{"Webhooks.Events.List", func(ctx context.Context, c *Client) error { _, err := c.Webhooks.Events.List(ctx, nil); return err }},
	{"Webhooks.Events.Get", func(ctx context.Context, c *Client) error { _, err := c.Webhooks.Events.Get(ctx, "ID1"); return err }},
	{"Webhooks.Events.Redeliver", func(ctx context.Context, c *Client) error {
		_, err := c.Webhooks.Events.Redeliver(ctx, "ID1", &RedeliverWebhookEventBody{})
		return err
	}},
	{"Webhooks.EventTypes.List", func(ctx context.Context, c *Client) error { _, err := c.Webhooks.EventTypes.List(ctx); return err }},
}

func loadSnapshot(t *testing.T) snapshot {
	t.Helper()
	raw, err := os.ReadFile("scripts/openapi-api-key-ops.json")
	if err != nil {
		t.Fatal(err)
	}
	var s snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func opKey(method, path string) string { return method + " " + path }

type compiledOp struct {
	op     snapshotOp
	params int
	re     *regexp.Regexp
}

// templateMatcher resolves a concrete method + path to the snapshot operation with the fewest
// placeholders among those matching (literal segments win over `{id}`).
func templateMatcher(ops []snapshotOp) func(method, path string) *snapshotOp {
	placeholder := regexp.MustCompile(`\{[^}]+\}`)
	compiled := make([]compiledOp, 0, len(ops))
	for _, op := range ops {
		literals := placeholder.Split(op.Path, -1)
		for i, l := range literals {
			literals[i] = regexp.QuoteMeta(l)
		}
		pattern := "^" + strings.Join(literals, "[^/]+") + "$"
		compiled = append(compiled, compiledOp{op: op, params: len(literals) - 1, re: regexp.MustCompile(pattern)})
	}
	return func(method, path string) *snapshotOp {
		var hits []compiledOp
		for _, c := range compiled {
			if c.op.Method == method && c.re.MatchString(path) {
				hits = append(hits, c)
			}
		}
		if len(hits) == 0 {
			return nil
		}
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].params < hits[j].params })
		op := hits[0].op
		return &op
	}
}

// reachableMethods lists `Label.Method` for every exported method of every service reachable from
// Client (Billing.* and Webhooks.* included), by reflection.
func reachableMethods(c *Client) []string {
	var out []string
	var walk func(label string, v reflect.Value, depth int)
	walk = func(label string, v reflect.Value, depth int) {
		if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
			return
		}
		if _, isService := v.Elem().Type().FieldByName("baseService"); isService {
			for i := 0; i < v.Type().NumMethod(); i++ {
				out = append(out, label+"."+v.Type().Method(i).Name)
			}
			return
		}
		if depth >= 1 {
			return
		}
		et := v.Elem().Type()
		for i := 0; i < et.NumField(); i++ {
			walk(label+"."+et.Field(i).Name, v.Elem().Field(i), depth+1)
		}
	}
	cv := reflect.ValueOf(c).Elem()
	for i := 0; i < cv.NumField(); i++ {
		name := cv.Type().Field(i).Name
		if name == "HTTP" {
			continue
		}
		walk(name, cv.Field(i), 0)
	}
	sort.Strings(out)
	return out
}

type recordedCall struct {
	method     string
	httpMethod string
	path       string
	idempotent bool
}

// recordCalls calls every SDK method once with placeholder arguments and records the request it builds.
func recordCalls(t *testing.T) []recordedCall {
	t.Helper()
	var calls []recordedCall
	for _, entry := range parityMethods {
		f := newFake(jsonAnswer(200, map[string]any{"meta": map[string]any{}, "data": nil}))
		c := newClient(t, f)
		if err := entry.call(context.Background(), c); err != nil {
			t.Fatalf("%s: %v", entry.method, err)
		}
		if f.calls() != 1 {
			t.Fatalf("%s made %d requests", entry.method, f.calls())
		}
		req := f.requests[0]
		calls = append(calls, recordedCall{method: entry.method, httpMethod: req.Method, path: req.URL.Path, idempotent: req.Header.Get("Idempotency-Key") != ""})
	}
	return calls
}

func TestParityTheRegistryCoversEveryExportedServiceMethod(t *testing.T) {
	c, _ := New(testKey)
	reachable := reachableMethods(c)
	registered := make([]string, 0, len(parityMethods))
	seen := map[string]bool{}
	for _, e := range parityMethods {
		if seen[e.method] {
			t.Fatalf("%s is registered twice", e.method)
		}
		seen[e.method] = true
		registered = append(registered, e.method)
	}
	sort.Strings(registered)
	if !reflect.DeepEqual(reachable, registered) {
		t.Fatalf("the parity registry and the reachable methods differ:\nreachable:  %v\nregistered: %v", reachable, registered)
	}
}

func TestParityTheSnapshotIsTheAPIKeySurface(t *testing.T) {
	s := loadSnapshot(t)
	if len(s.Operations) != s.Count || s.Count == 0 {
		t.Fatal(len(s.Operations), s.Count)
	}
}

func TestParityEverySDKMethodCallsExactlyOneSnapshotOperation(t *testing.T) {
	match := templateMatcher(loadSnapshot(t).Operations)
	var orphans []string
	for _, c := range recordCalls(t) {
		if match(c.httpMethod, c.path) == nil {
			orphans = append(orphans, c.method+" → "+c.httpMethod+" "+c.path)
		}
	}
	if len(orphans) != 0 {
		t.Fatal(orphans)
	}
}

func TestParityEverySnapshotOperationHasAMethodOrADocumentedExclusion(t *testing.T) {
	s := loadSnapshot(t)
	match := templateMatcher(s.Operations)
	covered := map[string]bool{}
	for _, c := range recordCalls(t) {
		op := match(c.httpMethod, c.path)
		if op == nil {
			t.Fatal(c)
		}
		covered[opKey(op.Method, op.Path)] = true
	}
	excluded := map[string]bool{}
	for _, e := range ExcludedOperations {
		excluded[opKey(e.Method, e.Path)] = true
	}
	var missing []string
	everything := map[string]bool{}
	for _, op := range s.Operations {
		k := opKey(op.Method, op.Path)
		everything[k] = true
		if !covered[k] && !excluded[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) != 0 {
		t.Fatal(missing)
	}
	for _, e := range ExcludedOperations {
		k := opKey(e.Method, e.Path)
		if !everything[k] || covered[k] || len(e.Reason) <= 10 {
			t.Fatalf("bad exclusion %+v", e)
		}
	}
}

func acceptsIdempotencyKey(op snapshotOp) bool {
	for _, p := range op.Parameters {
		if p.In == "header" && strings.EqualFold(p.Name, "idempotency-key") {
			return true
		}
	}
	return strings.Contains(op.Description, "Idempotency-Key") && !strings.Contains(op.Description, "Idempotency-Key` is ignored")
}

func TestParityTheSDKGeneratesAnIdempotencyKeyExactlyOnThePOSTGETRoutesThatAcceptOne(t *testing.T) {
	s := loadSnapshot(t)
	match := templateMatcher(s.Operations)
	expected := map[string]bool{}
	for _, op := range s.Operations {
		if acceptsIdempotencyKey(op) && (op.Method == "POST" || op.Method == "GET") {
			expected[opKey(op.Method, op.Path)] = true
		}
	}
	for _, route := range keyedButUndocumented {
		expected[route] = true
	}
	actual := map[string]bool{}
	for _, c := range recordCalls(t) {
		if c.idempotent {
			op := match(c.httpMethod, c.path)
			actual[opKey(op.Method, op.Path)] = true
		}
	}
	if !reflect.DeepEqual(sortedKeys(actual), sortedKeys(expected)) {
		t.Fatalf("keyed routes differ:\nactual:   %v\nexpected: %v", sortedKeys(actual), sortedKeys(expected))
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestParityPATCHRoutesThatAcceptAKeySendItOnlyWhenTheCallerGivesOne(t *testing.T) {
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

func TestParityTheMethodCountIsTheDocumentedSurface(t *testing.T) {
	calls := recordCalls(t)
	if len(calls) != 99 {
		t.Fatalf("%d methods: 99 methods in 23 modules at einvoice-js 0.8.0 (update CLAUDE.md and README when this changes)", len(calls))
	}
	modules := map[string]bool{}
	for _, c := range calls {
		modules[c.method[:strings.LastIndex(c.method, ".")]] = true
	}
	if len(modules) != 23 {
		t.Fatalf("%d modules", len(modules))
	}
}

func TestParityTypesGenIsWhatTheSnapshotGenerates(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; CI runs `make sync-check` instead")
	}
	gen := "../elyonar-sdk/scripts/gen-types.mjs"
	if _, err := os.Stat(gen); err != nil {
		t.Skip("no local einvoice-js checkout at ../elyonar-sdk; CI runs `make sync-check` instead")
	}
	out, err := exec.Command(node, gen, "--lang", "go", "--snapshot", "scripts/openapi-api-key-ops.json", "--out", "types_gen.go", "--check").CombinedOutput()
	if err != nil {
		t.Fatalf("types_gen.go is stale: %v\n%s", err, out)
	}
}
