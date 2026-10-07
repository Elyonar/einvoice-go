// Runs examples/<id>/main.go as an integrator would: the key comes only from YONA_API_KEY. Used by
// scripts/smoke_remote. Not shipped. The key is passed through the child's environment and never printed.
//
//	go run ./scripts/run_examples            (YONA_API_KEY, optional YONA_BASE_URL) every example
//	go run ./scripts/run_examples webhooks   one example
//
// YONA_BASE_URL points the examples at another gateway without changing a line of them: the runner
// links each example with the SDK's default host overridden (see scripts/internal/examples).
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/elyonar/einvoice-go/scripts/internal/examples"
)

func main() {
	apiKey := os.Getenv("YONA_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Set YONA_API_KEY")
		os.Exit(2)
	}
	root, err := examples.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ids := os.Args[1:]
	if len(ids) == 0 {
		if ids, err = examples.IDs(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	failed := 0
	for _, id := range ids {
		r := examples.Run(root, id, apiKey, os.Getenv("YONA_BASE_URL"), 180*time.Second)
		status := "OK  "
		if !r.OK {
			status = "FAIL"
			failed++
		}
		fmt.Printf("%s example %s (%d ms)\n", status, id, r.Millis)
		if !r.OK {
			lines := strings.Split(strings.TrimSpace(r.Output), "\n")
			if len(lines) > 20 {
				lines = lines[len(lines)-20:]
			}
			for _, l := range lines {
				fmt.Printf("     %s\n", l)
			}
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}
