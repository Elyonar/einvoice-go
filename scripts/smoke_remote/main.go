// REMOTE SMOKE: verify your own sandbox setup against the real API. Not shipped. Run it yourself:
//
//		YONA_API_KEY=sk_test_… make smoke-remote
//
//	  - Refuses any key that is not a sandbox key (sk_test_): nothing here ever runs in live.
//	  - Talks to the SDK's default host (https://gp.useyona.com). Another host only when YONA_BASE_URL is
//	    set explicitly in the environment; there is no other way to change it.
//	  - Runs only sandbox-safe calls: mode detection, free reads across every module, then the
//	    first-invoice example end to end, which CREATES SANDBOX DATA in your organisation (a buyer, an
//	    item and an invoice submitted to the tax authority's sandbox).
//	  - Prints a table method → OK/FAIL with the error code and request id of a failure. The key is never
//	    printed (nor passed on the command line: the example gets it through its environment).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/elyonar/einvoice-go"
	"github.com/elyonar/einvoice-go/scripts/internal/examples"
)

type row struct{ name, result, note string }

var rows []row

// check runs one call; known: error codes that are a correct answer for a fresh sandbox (OK, with the code noted).
func check(name string, fn func() error, known ...string) bool {
	err := fn()
	if err == nil {
		rows = append(rows, row{name, "OK", ""})
		return true
	}
	var ae *einvoice.APIError
	if errors.As(err, &ae) {
		for _, k := range known {
			if ae.ErrorCode == k {
				rows = append(rows, row{name, "OK", fmt.Sprintf("%d %s (expected in a sandbox)", ae.Status, ae.ErrorCode)})
				return false
			}
		}
		requestID := ae.RequestID
		if requestID == "" {
			requestID = "-"
		}
		rows = append(rows, row{name, "FAIL", fmt.Sprintf("%d %s requestId %s", ae.Status, ae.ErrorCode, requestID)})
		return false
	}
	msg := err.Error()
	if len(msg) > 140 {
		msg = msg[:140]
	}
	rows = append(rows, row{name, "FAIL", fmt.Sprintf("%T: %s", err, msg)})
	return false
}

func main() {
	apiKey := strings.TrimSpace(os.Getenv("YONA_API_KEY"))
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Set YONA_API_KEY to a sandbox key: YONA_API_KEY=sk_test_… make smoke-remote")
		os.Exit(2)
	}
	if !strings.HasPrefix(apiKey, "sk_test_") {
		fmt.Fprintln(os.Stderr, "Refused: smoke-remote runs only with a sandbox key (sk_test_…). The key was not used.")
		os.Exit(2)
	}
	explicitBase := strings.TrimSpace(os.Getenv("YONA_BASE_URL"))
	baseURL := einvoice.DefaultBaseURL
	if explicitBase != "" {
		baseURL = strings.TrimRight(explicitBase, "/")
	}
	if !strings.HasPrefix(baseURL, "https://") && !regexp.MustCompile(`^http://(localhost|127\.0\.0\.1)(:\d+)?$`).MatchString(baseURL) {
		fmt.Fprintln(os.Stderr, "Refused: YONA_BASE_URL must be https:// (or http://localhost).")
		os.Exit(2)
	}
	yona, err := einvoice.New(apiKey, einvoice.WithBaseURL(baseURL), einvoice.WithAssertMode(einvoice.ModeSandbox))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Refused: %v\n", err)
		os.Exit(2)
	}
	suffix := ""
	if explicitBase != "" {
		suffix = " (YONA_BASE_URL)"
	}
	fmt.Printf("smoke-remote → %s%s, sandbox key (not printed)\n", baseURL, suffix)
	fmt.Println("Reads first; then the first-invoice example, which CREATES SANDBOX DATA in your organisation:")
	fmt.Println("a buyer, an item and an invoice submitted to the tax authority's sandbox.")
	fmt.Println()

	ctx := context.Background()
	period := time.Now().UTC().Format("2006-01")
	limit5 := einvoice.Ptr(5.0)

	check("mode (sk_test_ → sandbox)", func() error {
		if yona.Mode() != einvoice.ModeSandbox {
			return fmt.Errorf("mode is %s", yona.Mode())
		}
		return nil
	})

	// ── reads across every module (free; nothing is written) ──
	check("Organization.GetReadiness", func() error { _, err := yona.Organization.GetReadiness(ctx); return err })
	check("InvoiceSettings.Get", func() error { _, err := yona.InvoiceSettings.Get(ctx); return err })
	check("TaxConnection.Get", func() error { _, err := yona.TaxConnection.Get(ctx); return err })
	check("Reference.ListResources", func() error { _, err := yona.Reference.ListResources(ctx); return err })
	check("Reference.ListHsCodes", func() error {
		_, err := yona.Reference.ListHsCodes(ctx, &einvoice.ListHsCodesQuery{Limit: limit5})
		return err
	})
	check("Reference.ListHsCodeCategories", func() error { _, err := yona.Reference.ListHsCodeCategories(ctx); return err })
	var sellers *einvoice.Page[einvoice.ListSellersItem]
	if check("Sellers.List", func() (err error) { sellers, err = yona.Sellers.List(ctx, nil); return }) && len(sellers.Data) > 0 {
		check("Sellers.Get", func() error { _, err := yona.Sellers.Get(ctx, sellers.Data[0].ID); return err })
	}
	var buyers *einvoice.Page[einvoice.ListBuyersItem]
	if check("Buyers.List", func() (err error) {
		buyers, err = yona.Buyers.List(ctx, &einvoice.ListBuyersQuery{Limit: limit5})
		return
	}) && len(buyers.Data) > 0 {
		check("Buyers.Get", func() error { _, err := yona.Buyers.Get(ctx, buyers.Data[0].ID); return err })
	}
	var items *einvoice.Page[einvoice.ListItemsItem]
	if check("Items.List", func() (err error) { items, err = yona.Items.List(ctx, &einvoice.ListItemsQuery{Limit: limit5}); return }) && len(items.Data) > 0 {
		check("Items.Get", func() error { _, err := yona.Items.Get(ctx, items.Data[0].ID); return err })
	}
	var invoices *einvoice.Page[einvoice.ListInvoicesItem]
	if check("Invoices.List", func() (err error) {
		invoices, err = yona.Invoices.List(ctx, &einvoice.ListInvoicesQuery{Limit: limit5})
		return
	}) && len(invoices.Data) > 0 {
		firstID := invoices.Data[0].ID
		check("Invoices.Get", func() error { _, err := yona.Invoices.Get(ctx, firstID); return err })
		check("Submissions.GetStatus", func() error { _, err := yona.Submissions.GetStatus(ctx, firstID); return err })
		check("ShareLinks.List", func() error { _, err := yona.ShareLinks.List(ctx, firstID); return err })
	}
	check("Invoices.GetOverview", func() error { _, err := yona.Invoices.GetOverview(ctx, nil); return err })
	check("Invoices.GetStatistics", func() error { _, err := yona.Invoices.GetStatistics(ctx, nil); return err })
	check("Invoices.GetSummary", func() error { _, err := yona.Invoices.GetSummary(ctx, nil); return err })
	check("InboundInvoices.List", func() error {
		_, err := yona.InboundInvoices.List(ctx, &einvoice.ListInboundInvoicesQuery{Limit: limit5})
		return err
	})
	check("InboundInvoices.GetAnalytics", func() error { _, err := yona.InboundInvoices.GetAnalytics(ctx, nil); return err })
	// BIZ221: retrieving invoice history is not available yet on this deployment.
	check("InboundInvoices.ListHistoryRuns", func() error { _, err := yona.InboundInvoices.ListHistoryRuns(ctx, nil); return err }, "BIZ221")
	check("IssuedHistory.List", func() error {
		_, err := yona.IssuedHistory.List(ctx, &einvoice.ListIssuedHistoryQuery{Limit: limit5})
		return err
	})
	var account *einvoice.GetBillingAccountData
	if check("Billing.Accounts.GetMine", func() (err error) { account, err = yona.Billing.Accounts.GetMine(ctx, nil); return }) && account.OrganizationID != "" {
		check("Organization.Get", func() error { _, err := yona.Organization.Get(ctx, account.OrganizationID); return err })
	}
	check("Billing.Accounts.GetStats", func() error { _, err := yona.Billing.Accounts.GetStats(ctx, "", nil); return err })
	if account != nil && account.ID != "" {
		check("Billing.Accounts.CheckBalance", func() error {
			_, err := yona.Billing.Accounts.CheckBalance(ctx, account.ID, &einvoice.CheckBillingAccountBalanceQuery{Credits: 1})
			return err
		})
	}
	check("Billing.Payments.List", func() error { _, err := yona.Billing.Payments.List(ctx, nil); return err })
	check("Billing.Sandbox.GetUsage", func() error { _, err := yona.Billing.Sandbox.GetUsage(ctx); return err })
	check("Billing.Sandbox.ListTransactions", func() error {
		_, err := yona.Billing.Sandbox.ListTransactions(ctx, &einvoice.ListSandboxTransactionsQuery{Limit: limit5})
		return err
	})
	check("Billing.Statements.Get", func() error { _, err := yona.Billing.Statements.Get(ctx, period, nil); return err })
	check("Billing.Subscriptions.List", func() error { _, err := yona.Billing.Subscriptions.List(ctx, nil); return err })
	check("Billing.Subscriptions.ListRenewals", func() error { _, err := yona.Billing.Subscriptions.ListRenewals(ctx, nil); return err })
	check("Billing.Transactions.List", func() error {
		_, err := yona.Billing.Transactions.List(ctx, &einvoice.ListLedgerEntriesQuery{Limit: limit5})
		return err
	})
	check("Billing.Transactions.GetUsageAnalytics", func() error { _, err := yona.Billing.Transactions.GetUsageAnalytics(ctx, nil); return err })
	check("Billing.Transactions.GetUsageByCostCode", func() error { _, err := yona.Billing.Transactions.GetUsageByCostCode(ctx, nil); return err })
	check("Webhooks.Endpoints.List", func() error { _, err := yona.Webhooks.Endpoints.List(ctx); return err })
	check("Webhooks.Deliveries.List", func() error {
		_, err := yona.Webhooks.Deliveries.List(ctx, &einvoice.ListWebhookDeliveriesQuery{Limit: einvoice.Ptr(int64(5))})
		return err
	})
	check("Webhooks.Events.List", func() error {
		_, err := yona.Webhooks.Events.List(ctx, &einvoice.ListWebhookEventsQuery{Limit: einvoice.Ptr(int64(5))})
		return err
	})
	check("Webhooks.EventTypes.List", func() error { _, err := yona.Webhooks.EventTypes.List(ctx); return err })

	// ── the first-invoice example, end to end (creates sandbox data) ──
	root, err := examples.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	exampleBase := ""
	if explicitBase != "" {
		exampleBase = baseURL
	}
	ex := examples.Run(root, "first-invoice", apiKey, exampleBase, 180*time.Second)
	var tailLines []string
	for _, l := range strings.Split(strings.TrimSpace(ex.Output), "\n") {
		if l != "" {
			tailLines = append(tailLines, l)
		}
	}
	tail := strings.Join(tailLines, " | ")
	if len(tail) > 240 {
		tail = tail[len(tail)-240:]
	}
	if ex.OK {
		rows = append(rows, row{"example first-invoice (creates sandbox data)", "OK", fmt.Sprintf("%d s", (ex.Millis+500)/1000)})
	} else {
		rows = append(rows, row{"example first-invoice (creates sandbox data)", "FAIL", tail})
	}

	// ── report ──
	width := len("method")
	for _, r := range rows {
		if len(r.name) > width {
			width = len(r.name)
		}
	}
	fmt.Printf("%-*s  result  detail\n", width, "method")
	failed := 0
	for _, r := range rows {
		fmt.Printf("%-*s  %-6s  %s\n", width, r.name, r.result, r.note)
		if r.result == "FAIL" {
			failed++
		}
	}
	fmt.Printf("\n%d OK, %d FAIL. Quote a requestId to Yona support for any FAIL.\n", len(rows)-failed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
