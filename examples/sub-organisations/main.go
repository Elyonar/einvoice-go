// @recipe sub-organisations Sub-organisations
// @summary For an organisation that runs several businesses: list its sub-organisations with an API key, see which are ready for live and connected to the tax authority, and create one with a live key.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()

	// @step client Create the client
	// @text Use a key of the parent organisation. No key preset includes the two sub-organisation permissions: an Owner or Admin ticks them under Sub-organisations when creating the key in the dashboard. Listing works in sandbox and live; creating needs a live key.
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// @step list List the sub-organisations
	// @text Newest first, 100 a page at most, with the total in Pagination. Search keeps the ones whose name holds the text. The permission is organization.children.read.
	// @op listChildOrganizations
	firstPage, err := yona.Organization.ListSubOrganizations(ctx, &einvoice.ListSubOrganizationsQuery{Limit: einvoice.Ptr(100.0)})
	if err != nil {
		// 403 AUTH019: the key does not hold organization.children.read. 404 RES001: sub-organisations are
		// not switched on here. 409 BIZ014: this organisation is itself a sub-organisation.
		var permission *einvoice.PermissionError
		var notFound *einvoice.NotFoundError
		var conflict *einvoice.ConflictError
		switch {
		case errors.As(err, &permission) && permission.ErrorCode == "AUTH019":
			fmt.Println("this key cannot list sub-organisations:", permission.ErrorCode)
		case errors.As(err, &notFound) && notFound.ErrorCode == "RES001":
			fmt.Println("this key cannot list sub-organisations:", notFound.ErrorCode)
		case errors.As(err, &conflict) && conflict.ErrorCode == "BIZ014":
			fmt.Println("this key cannot list sub-organisations:", conflict.ErrorCode)
		default:
			log.Fatal(err)
		}
		return
	}
	fmt.Println(firstPage.Pagination.Total, "sub-organisations")

	// @step paginate Walk every page
	// @text An organisation can hold many sub-organisations. Paginate fetches page after page for you.
	// @op listChildOrganizations
	var all []einvoice.ListChildOrganizationsItem
	enough := errors.New("enough")
	query := &einvoice.ListSubOrganizationsQuery{Limit: einvoice.Ptr(100.0)}
	err = einvoice.Paginate(ctx, func(ctx context.Context, page int64) (*einvoice.Page[einvoice.ListChildOrganizationsItem], error) {
		query.Page = einvoice.Ptr(float64(page))
		return yona.Organization.ListSubOrganizations(ctx, query)
	}, func(sub einvoice.ListChildOrganizationsItem) error {
		all = append(all, sub)
		if len(all) >= 300 {
			return enough
		}
		return nil
	})
	if err != nil && !errors.Is(err, enough) {
		log.Fatal(err)
	}
	fmt.Println("read", len(all))

	// @step readiness See which are ready for live
	// @text The same page, with each sub-organisation's checklist, live status and next step. IDs keeps only the ones you name (100 at most), given as a list.
	// @op listChildOrganizationsReadiness
	var ids []string
	for _, sub := range firstPage.Data {
		ids = append(ids, sub.ID)
	}
	readinessQuery := &einvoice.ListSubOrganizationsReadinessQuery{Limit: einvoice.Ptr(100.0)}
	if len(ids) > 0 {
		readinessQuery = &einvoice.ListSubOrganizationsReadinessQuery{IDs: ids}
	}
	readiness, err := yona.Organization.ListSubOrganizationsReadiness(ctx, readinessQuery)
	if err != nil {
		log.Fatal(err)
	}
	for _, sub := range readiness.Data {
		fmt.Println(sub.Name, sub.LiveStatus, sub.NextStep)
	}

	// @step tax-connection Check their tax connections
	// @text One item for each sub-organisation named: the state of its connection to the tax authority in the key's mode, and whether a submission would pass. The permission is tax_connection.read. An id that is not one of your sub-organisations is left out.
	// @op listChildTaxConnections
	if len(ids) > 0 {
		connections, err := yona.TaxConnection.ListForSubOrganizations(ctx, &einvoice.SubOrganizationTaxConnectionsQuery{IDs: ids})
		if err != nil {
			log.Fatal(err)
		}
		for _, c := range connections {
			fmt.Println(c.OrganizationID, c.State, c.SubmissionsAllowed)
		}
	}

	// @step create Create a sub-organisation
	// @text With a live key that holds organization.children.create, this answers the new organisation; its Status is pending_verification until it completes its own onboarding. A name is unique under its parent, so a repeat answers 409 RES002 and nothing is created twice. A sandbox key is refused, which is what this step shows in sandbox; with a live key it creates nothing here.
	// @op createOrganization
	if yona.Mode() == einvoice.ModeSandbox {
		created, err := yona.Organization.CreateSubOrganization(ctx, &einvoice.CreateSubOrganizationBody{BusinessName: "Acme Retail Ltd", Email: "accounts@acme.example"})
		var refused *einvoice.PermissionError
		switch {
		case err == nil:
			log.Fatal("a sandbox key created ", created.ID)
		case !errors.As(err, &refused):
			log.Fatal(err)
		}
		fmt.Println("refused in sandbox:", refused.ErrorCode)
	}
}
