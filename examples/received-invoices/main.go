// @recipe received-invoices Received invoices
// @summary List the invoices other businesses issued to you, filter them, and walk every page with Paginate.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/elyonar/einvoice-go"
)

// text prints an optional string field: its value, or "-" when the API sent null.
func text(p *string) string {
	if p == nil {
		return "-"
	}
	return *p
}

func main() {
	ctx := context.Background()

	// @step client Create the client
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// @step connection Check the tax connection
	// @text Received invoices arrive through your connection to the tax authority; until it is connected the list is empty.
	// @op getTaxConnection
	connection, err := yona.TaxConnection.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("tax connection", connection.Mode, connection.Kind, connection.State)

	// @step list Read the first page
	// @text Newest first. Sync tells you whether the mirror is still filling (Syncing) and when it last read the tax authority.
	// @op listInboundInvoices
	first, err := yona.InboundInvoices.List(ctx, &einvoice.ListInboundInvoicesQuery{Limit: einvoice.Ptr(20.0)})
	if err != nil {
		log.Fatal(err)
	}
	total := int64(len(first.Data.Items))
	if first.Pagination != nil {
		total = first.Pagination.Total
	}
	fmt.Println("received", total, "syncing", first.Data.Sync.Syncing)
	for _, inv := range first.Data.Items {
		supplier := inv.SupplierName
		if supplier == nil {
			supplier = inv.SupplierTIN
		}
		payable := 0.0
		if inv.PayableAmountMinor != nil {
			payable = *inv.PayableAmountMinor
		}
		fmt.Println(inv.Irn, text(supplier), payable, text(inv.Currency), inv.PaymentStatus)
	}

	// @step filter Filter
	// @text Filter by seller name or TIN, payment status, issue date, amount or currency.
	// @op listInboundInvoices
	unpaid, err := yona.InboundInvoices.List(ctx, &einvoice.ListInboundInvoicesQuery{
		PaymentStatus: einvoice.Ptr(einvoice.ListInboundInvoicesQueryPaymentStatusUnpaid),
		Sort:          einvoice.Ptr(einvoice.ListInboundInvoicesQuerySortDescIssueDate),
		Limit:         einvoice.Ptr(20.0),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("unpaid on this page", len(unpaid.Data.Items))

	// @step paginate Walk every page
	// @text Paginate fetches page after page for you. Received invoices return an object with Items, so map each page to its items.
	// @op listInboundInvoices
	count := 0
	query := &einvoice.ListInboundInvoicesQuery{Limit: einvoice.Ptr(50.0)}
	err = einvoice.Paginate(ctx, func(ctx context.Context, page int64) (*einvoice.Page[einvoice.InboundInvoiceViewDto], error) {
		query.Page = einvoice.Ptr(float64(page))
		res, err := yona.InboundInvoices.List(ctx, query)
		if err != nil {
			return nil, err
		}
		out := &einvoice.Page[einvoice.InboundInvoiceViewDto]{Data: res.Data.Items, RequestID: res.RequestID}
		if res.Pagination != nil {
			out.Pagination = *res.Pagination
		}
		return out, nil
	}, func(inv einvoice.InboundInvoiceViewDto) error {
		count++
		if count <= 3 {
			fmt.Println(inv.Irn)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("walked", count, "received invoices")

	// @step detail Open one
	// @op getInboundInvoice
	if len(first.Data.Items) > 0 {
		one, err := yona.InboundInvoices.Get(ctx, first.Data.Items[0].ID)
		if err != nil {
			log.Fatal(err)
		}
		detail, _ := json.MarshalIndent(one, "", "  ")
		fmt.Println(string(detail))
	}

	// @step analytics Summarise
	// @text Totals by status and seller for a period.
	// @op getInboundInvoiceAnalytics
	analytics, err := yona.InboundInvoices.GetAnalytics(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	summary, _ := json.MarshalIndent(analytics, "", "  ")
	fmt.Println(string(summary))

	// @step paginate-buyers The same for any list
	// @text Every list method returns a Page with Data and Pagination, so Paginate (or Collect) works on it directly.
	// @op listBuyers
	buyersQuery := &einvoice.ListBuyersQuery{Limit: einvoice.Ptr(100.0)}
	buyers, err := einvoice.Collect(ctx, func(ctx context.Context, page int64) (*einvoice.Page[einvoice.BuyerViewDto], error) {
		buyersQuery.Page = einvoice.Ptr(float64(page))
		return yona.Buyers.List(ctx, buyersQuery)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("buyers", len(buyers))
}
