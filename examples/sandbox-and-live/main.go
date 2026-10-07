// @recipe sandbox-and-live Sandbox and live
// @summary The key decides the mode: an sk_test_ key works in sandbox and an sk_live_ key in live, on the same host. Going live changes only the key.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()

	// @step client Create the client and read its mode
	// @text There is no host or mode to configure. The SDK reads the mode from the key prefix.
	// @quickstart
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(yona.Mode()) // "sandbox" for sk_test_, "live" for sk_live_

	// @step first-call Make your first call
	// @text The readiness check lists what your organisation still needs before it can invoice in this mode.
	// @op getOrganizationReadiness
	// @quickstart
	readiness, err := yona.Organization.GetReadiness(ctx)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := json.MarshalIndent(readiness, "", "  ")
	fmt.Println(string(out))

	// @step assert-mode Guard against the wrong key
	// @text Pass WithAssertMode so a deployment refuses to start with a key of the other mode. The error never echoes the key.
	other := einvoice.ModeLive
	if yona.Mode() == einvoice.ModeLive {
		other = einvoice.ModeSandbox
	}
	_, err = einvoice.New(os.Getenv("YONA_API_KEY"), einvoice.WithAssertMode(other))
	var refused *einvoice.ConfigError
	if !errors.As(err, &refused) {
		log.Fatal("WithAssertMode did not refuse a key of the other mode")
	}
	fmt.Println("refused:", refused.Message)

	// @step sandbox-allowance See your sandbox allowance
	// @text Sandbox invoices are free within an allowance; submissions go to the tax authority's sandbox, never to the live register.
	// @op getSandboxUsage
	if yona.Mode() == einvoice.ModeSandbox {
		usage, err := yona.Billing.Sandbox.GetUsage(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("sandbox credits", usage.CreditsUsed, "used of", usage.CreditsLimit, "until", usage.PeriodEnd)
	}

	// @step tax-connection Check the tax connection
	// @text Each mode has its own connection to the tax authority. Live needs the organisation approved for live and its live connection made in the dashboard.
	// @op getTaxConnection
	connection, err := yona.TaxConnection.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("tax connection", connection.Mode, connection.Kind, connection.State)

	// @step go-live Go live
	// @text Create a live key in the dashboard and deploy it as YONA_API_KEY in place of the test key. Nothing else in your code changes: the same calls, the same host.
	client, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	if client.Mode() == einvoice.ModeLive {
		fmt.Println("live: submissions are reported to the tax authority")
	} else {
		fmt.Println("sandbox: deploy an sk_live_ key to go live")
	}
}
