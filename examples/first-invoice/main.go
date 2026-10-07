// @recipe first-invoice Your first invoice
// @summary Create a buyer and an item, draft an invoice, finalise it, submit it to the tax authority, follow its status and download the PDF and the QR payload.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()

	// @step client Create the client
	// @text Pass only your API key. An sk_test_ key works in sandbox and an sk_live_ key in live, on the same host.
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	refBytes := make([]byte, 4)
	_, _ = rand.Read(refBytes)
	ref := hex.EncodeToString(refBytes)

	// @step create-buyer Create a buyer
	// @text A buyer is the party you invoice. A B2B buyer carries its tax id (TIN), which the tax authority checks on submission. A buyer can never carry your own organisation's TIN (the authority refuses SAME_PARTY_TIN); the sandbox accepts any well-formed TIN. A TIN is unique per organisation: on 409 RES002, reuse the buyer you already have.
	// @op createBuyer
	taxID := "12345678-0001"
	buyer, err := yona.Buyers.Create(ctx, &einvoice.CreateBuyerBody{
		Name:      "Acme Nigeria Ltd " + ref,
		PartyType: einvoice.Ptr(einvoice.CreateBuyerDtoPartyTypeCompany),
		TaxID:     einvoice.Ptr(taxID),
		Email:     einvoice.Ptr("accounts." + ref + "@example.com"),
		Address:   &einvoice.BuyerAddressDto{Line1: "1 Marina", City: "Lagos", Country: "NG"},
	})
	var conflict *einvoice.ConflictError
	if errors.As(err, &conflict) && conflict.ErrorCode == "RES002" {
		found, listErr := yona.Buyers.List(ctx, &einvoice.ListBuyersQuery{Search: einvoice.Ptr(taxID), Limit: einvoice.Ptr(1.0)})
		if listErr != nil || len(found.Data) == 0 {
			log.Fatal(err)
		}
		buyer, err = &found.Data[0], nil
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("buyer", buyer.ID)

	// @step find-hs-code Find an HS code
	// @text Goods carry an HS code. Search the reference list for one that fits your product.
	// @op listHsCodes
	hsCodes, err := yona.Reference.ListHsCodes(ctx, &einvoice.ListHsCodesQuery{Limit: einvoice.Ptr(1.0)})
	if err != nil {
		log.Fatal(err)
	}
	hsnCode := "8471.30"
	if len(hsCodes.Data) > 0 {
		hsnCode = hsCodes.Data[0].Code
	}

	// @step create-item Create an item
	// @text Save what you sell once and reuse it on every invoice. Amounts are integer minor units (kobo for NGN), sent as strings.
	// @op createItem
	item, err := yona.Items.Create(ctx, &einvoice.CreateItemBody{
		Name:            "Laptop " + ref,
		ItemType:        einvoice.CreateItemDtoItemTypeGoods,
		HSNCode:         einvoice.Ptr(hsnCode),
		ProductCategory: "Machinery",
		UnitCode:        "EA",
		UnitPriceMinor:  "150000000",
		Currency:        "NGN",
		TaxCategory:     "STANDARD_VAT",
		SKU:             einvoice.Ptr("LAPTOP-" + ref),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("item", item.ID)

	// @step create-invoice Draft the invoice
	// @text A draft can be edited freely. The SDK sends an Idempotency-Key, so a retried create is never charged twice.
	// @op createInvoice
	draft, err := yona.Invoices.Create(ctx, &einvoice.CreateInvoiceBody{
		InvoiceKind: "B2B",
		InvoiceDate: time.Now().Format("2006-01-02"),
		Currency:    "NGN",
		BuyerID:     einvoice.Ptr(buyer.ID),
		LineItems:   []einvoice.InvoiceLineItemDto{{ItemID: einvoice.Ptr(item.ID), Quantity: einvoice.AmountNumber(2)}},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("draft", draft.InvoiceNumber, draft.Totals.PayableMinor)

	// @step finalise Finalise it
	// @text Finalising freezes the seller and buyer onto the invoice and runs the jurisdiction's checks.
	// @op finaliseInvoice
	finalised, err := yona.Invoices.Finalise(ctx, draft.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("finalised", finalised.Status)

	// @step submit Submit it to the tax authority
	// @text Submission is asynchronous: the answer is the queued submission. Webhooks tell you when it settles.
	// @op submitInvoice
	submitted, err := yona.Submissions.Submit(ctx, draft.ID, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("submitted", submitted.Invoice.Status)

	// @step wait Wait for the outcome
	// @text Poll the status as last recorded (or listen for invoice.accepted / invoice.rejected webhooks). A rejection carries the authority's reasons in submissions[].rejectReasons: print them and stop, since a rejected invoice cannot be queried or downloaded.
	// @op getInvoiceStatus
	settled := map[einvoice.SubmissionStatusDtoStatus]bool{
		einvoice.SubmissionStatusDtoStatusSigned:      true,
		einvoice.SubmissionStatusDtoStatusTransmitted: true,
		einvoice.SubmissionStatusDtoStatusAccepted:    true,
		einvoice.SubmissionStatusDtoStatusRejected:    true,
		einvoice.SubmissionStatusDtoStatusFailed:      true,
	}
	status, err := yona.Submissions.GetStatus(ctx, draft.ID)
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 45 && !settled[status.Status]; i++ {
		time.Sleep(2 * time.Second)
		if status, err = yona.Submissions.GetStatus(ctx, draft.ID); err != nil {
			log.Fatal(err)
		}
	}
	reference := ""
	if status.AuthorityReference != nil {
		reference = *status.AuthorityReference
	}
	fmt.Println("status", status.Status, reference)
	if status.Status == einvoice.SubmissionStatusDtoStatusRejected {
		for _, submission := range status.Submissions {
			for _, reason := range submission.RejectReasons {
				field := ""
				if reason.Field != nil {
					field = *reason.Field
				}
				fmt.Println("  refused:", reason.Code, field, reason.Message)
			}
		}
		log.Fatal("the tax authority rejected the invoice; fix what it named and submit again")
	}

	// @step query-status Ask the tax authority directly
	// @text QueryStatus asks the authority now instead of reading the last recorded state.
	// @op queryInvoiceStatus
	live, err := yona.Submissions.QueryStatus(ctx, draft.ID)
	if err != nil {
		log.Fatal(err)
	}
	authorityState := ""
	if live.AuthorityState != nil {
		authorityState = string(*live.AuthorityState)
	}
	fmt.Println("authority says", live.Status, authorityState)

	// @step download-pdf Download the PDF
	// @text The PDF comes back as bytes in a BinaryResponse. GetDownloadLink returns a short-lived URL instead.
	// @op downloadInvoice
	pdf, err := yona.Output.DownloadPDF(ctx, draft.ID)
	if err != nil {
		log.Fatal(err)
	}
	name := pdf.FileName
	if name == "" {
		name = draft.InvoiceNumber + ".pdf"
	}
	file := filepath.Join(os.TempDir(), name)
	if err := os.WriteFile(file, pdf.Data, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("pdf", file, len(pdf.Data), "bytes")

	// @step qr Read the QR payload
	// @text Once registered, the invoice carries the verification payload to print as its QR code.
	// @op getInvoice
	invoice, err := yona.Invoices.Get(ctx, draft.ID)
	if err != nil {
		log.Fatal(err)
	}
	if invoice.Verification != nil {
		fmt.Printf("qr %d chars\n", len(invoice.Verification.Payload))
	} else {
		fmt.Println("qr not registered yet")
	}
}
