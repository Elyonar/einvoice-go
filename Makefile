.PHONY: sync sync-check test lint examples guides guides-check clean

sync:               ## copy the snapshot and vectors from einvoice-js and regenerate types_gen.go (needs git + Node 22)
	sh scripts/sync.sh

sync-check:         ## CI: fail when snapshot, vectors or models are not what the pinned einvoice-js commit produces
	sh scripts/sync.sh --check

test:               ## unit tests with coverage, parity and guides included
	go test ./... -cover

lint:               ## gofmt + go vet (+ golangci-lint when installed)
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt: these files need formatting:"; echo "$$out"; exit 1; fi
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not installed; skipped"; fi

examples:           ## run every example (YONA_API_KEY; YONA_BASE_URL to point elsewhere)
	go run ./scripts/run_examples

guides:             ## examples/ -> guides/guides.json
	go run ./scripts/export_guides

guides-check:       ## fail when guides/guides.json is stale
	go run ./scripts/export_guides --check


clean:
	go clean ./...
	rm -f coverage.out
