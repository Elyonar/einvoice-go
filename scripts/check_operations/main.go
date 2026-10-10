// guides/operations.json → compiled and run snippets: the proof that every template the portal's API
// playground shows for Go builds and works. Not shipped. Run by operations_test.go (`make test`).
//
//	go run ./scripts/check_operations          check every operation
//	go run ./scripts/check_operations --keep   keep the generated module and print where it is
//
// Each template is rendered as the portal renders it: `{{path:<name>}}` → "id-1" (the n-th path parameter "id-n"), `{{query}}` → a
// raw string literal of the operation's required query parameters (their schema examples) or `{}`,
// `{{body}}` → a raw string literal of the example body built from the request schema. The setup's
// import block heads its own file (without "encoding/json" when the template does not use `json.`,
// as the portal drops it) and the setup's statements, then the template, become the body of one
// function; the harness alone appends `_ = result`. The files form a temporary module that replaces
// github.com/elyonar/einvoice-go with this checkout; it must pass `go vet`, then each function runs
// in its own process against a local fake gateway, which must see exactly the operation's method
// and path (and a JSON body when the operation takes one).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/elyonar/einvoice-go/scripts/internal/examples"
	"github.com/elyonar/einvoice-go/scripts/internal/schemaexample"
)

// fakeKey has the shape of a sandbox key and opens nothing: the snippets only reach the fake gateway.
const fakeKey = "sk_test_abcdefghijklmnop_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type entry struct {
	Module   string `json:"module"`
	Method   string `json:"method"`
	Template string `json:"template"`
}

type operationsFile struct {
	Setup      string           `json:"setup"`
	Operations map[string]entry `json:"operations"`
}

type snapshotOp struct {
	Method      string           `json:"method"`
	Path        string           `json:"path"`
	OperationID string           `json:"operationId"`
	Parameters  []map[string]any `json:"parameters"`
	RequestBody map[string]any   `json:"requestBody"`
}

type snapshot struct {
	Operations []snapshotOp              `json:"operations"`
	Schemas    map[string]map[string]any `json:"schemas"`
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "check_operations: "+format+"\n", args...)
	os.Exit(1)
}

func readJSON(path string, v any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail("%v", err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		fail("%s: %v", path, err)
	}
}

func rawLiteral(opID, what string, v any, indent string) string {
	b, err := schemaexample.Marshal(v, indent)
	if err != nil {
		fail("%s: %s: %v", opID, what, err)
	}
	if strings.Contains(string(b), "`") {
		fail("%s: the sample %s holds a backtick, which a Go raw string cannot", opID, what)
	}
	return "`" + string(b) + "`"
}

func sampleQuery(b schemaexample.Builder, op snapshotOp) string {
	q := schemaexample.OrderedMap{}
	for _, p := range op.Parameters {
		required, _ := p["required"].(bool)
		if p["in"] != "query" || !required {
			continue
		}
		name, _ := p["name"].(string)
		schema, _ := p["schema"].(map[string]any)
		v, err := b.Of(schema)
		if err != nil {
			fail("%s: %v", op.OperationID, err)
		}
		q = append(q, schemaexample.KV{Key: name, Value: v})
	}
	return rawLiteral(op.OperationID, "query", q, "")
}

func sampleBody(b schemaexample.Builder, op snapshotOp) string {
	content, _ := op.RequestBody["content"].(map[string]any)
	j, _ := content["application/json"].(map[string]any)
	schema, _ := j["schema"].(map[string]any)
	if schema == nil {
		fail("%s: the request body has no application/json schema", op.OperationID)
	}
	v, err := b.Of(schema)
	if err != nil {
		fail("%s: %v", op.OperationID, err)
	}
	return rawLiteral(op.OperationID, "body", v, "  ")
}

var pathPlaceholder = regexp.MustCompile(`\{\{path:([^}]+)\}\}`)

// samplePath is the value of the n-th path placeholder of an operation: "id-1", "id-2", … so a
// template that swaps two path arguments reaches the wrong path.
func samplePath(n int) string { return fmt.Sprintf("id-%d", n+1) }

// pathOrder: the path parameter names of op in the order its path has them.
func pathOrder(op snapshotOp) map[string]int {
	out := map[string]int{}
	for i, m := range opPlaceholder.FindAllStringSubmatch(op.Path, -1) {
		out[m[1]] = i
	}
	return out
}

var opPlaceholder = regexp.MustCompile(`\{([^}]+)\}`)

func render(b schemaexample.Builder, op snapshotOp, template string) string {
	order := pathOrder(op)
	out := pathPlaceholder.ReplaceAllStringFunc(template, func(m string) string {
		name := pathPlaceholder.FindStringSubmatch(m)[1]
		n, ok := order[name]
		if !ok {
			fail("%s: {{path:%s}} is not a path parameter of %s", op.OperationID, name, op.Path)
		}
		return strconv.Quote(samplePath(n))
	})
	if strings.Contains(out, "{{query}}") {
		out = strings.ReplaceAll(out, "{{query}}", sampleQuery(b, op))
	}
	if strings.Contains(out, "{{body}}") {
		out = strings.ReplaceAll(out, "{{body}}", sampleBody(b, op))
	}
	if strings.Contains(out, "{{") {
		fail("%s: an unknown placeholder is left in %q", op.OperationID, out)
	}
	return out
}

// splitSetup separates the setup's import block from its statements.
func splitSetup(setup string) (string, string) {
	if !strings.HasPrefix(setup, "import (\n") {
		fail("the setup does not start with an import block")
	}
	end := strings.Index(setup, "\n)\n")
	if end < 0 {
		fail("the setup's import block is not closed")
	}
	return setup[:end+2], strings.TrimSpace(setup[end+3:])
}

func withoutJSONImport(imports string) string {
	var kept []string
	for _, l := range strings.Split(imports, "\n") {
		if strings.TrimSpace(l) != `"encoding/json"` {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

func run(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		fail("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

type seen struct {
	method, path, contentType string
	body                      []byte
}

func main() {
	keep := len(os.Args) > 1 && os.Args[1] == "--keep"
	root, err := examples.Root()
	if err != nil {
		fail("%v", err)
	}
	var file operationsFile
	readJSON(filepath.Join(root, "guides", "operations.json"), &file)
	var snap snapshot
	readJSON(filepath.Join(root, "scripts", "openapi-api-key-ops.json"), &snap)
	ops := map[string]snapshotOp{}
	for _, op := range snap.Operations {
		ops[op.OperationID] = op
	}
	b := schemaexample.Builder{Schemas: snap.Schemas}
	imports, setupBody := splitSetup(file.Setup)

	dir, err := os.MkdirTemp("", "einvoice-go-operations-")
	if err != nil {
		fail("%v", err)
	}
	if keep {
		fmt.Println("module kept at", dir)
	} else {
		defer os.RemoveAll(dir)
	}
	goMod := "module yonaoperationscheck\n\ngo 1.22\n\nrequire github.com/elyonar/einvoice-go v0.0.0\n\nreplace github.com/elyonar/einvoice-go => " + root + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		fail("%v", err)
	}

	var ids []string
	for id := range file.Operations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var registry strings.Builder
	for i, id := range ids {
		op, ok := ops[id]
		if !ok {
			fail("%s is not in the snapshot", id)
		}
		snippet := render(b, op, file.Operations[id].Template)
		fileImports := imports
		if !strings.Contains(snippet, "json.") {
			fileImports = withoutJSONImport(imports)
		}
		fn := fmt.Sprintf("op%03d", i)
		src := fmt.Sprintf("// %s\npackage main\n\n%s\n\nfunc %s() {\n%s\n%s\n_ = result\n}\n", id, fileImports, fn, setupBody, snippet)
		if err := os.WriteFile(filepath.Join(dir, fn+".go"), []byte(src), 0o644); err != nil {
			fail("%v", err)
		}
		fmt.Fprintf(&registry, "\t%q: %s,\n", id, fn)
	}
	mainSrc := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nvar operations = map[string]func(){\n" + registry.String() + "}\n\n" +
		"func main() {\n\tf, ok := operations[os.Args[1]]\n\tif !ok {\n\t\tfmt.Fprintln(os.Stderr, \"unknown operation\", os.Args[1])\n\t\tos.Exit(2)\n\t}\n\tf()\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		fail("%v", err)
	}

	// 1. Every snippet compiles and passes go vet.
	run(dir, "go", "vet", ".")

	// 2. Every snippet runs: the JSON decodes into the SDK's typed params and the call reaches the
	// operation's route on a fake gateway.
	var mu sync.Mutex
	var requests []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, seen{r.Method, r.URL.Path, r.Header.Get("Content-Type"), body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"statusCode":200,"success":true,"message":"Success","errors":[],"timestamp":"t","requestId":"req-check"},"data":null}`))
	}))
	defer srv.Close()
	bin := filepath.Join(dir, "operations-check")
	run(dir, "go", "build", "-o", bin, "-ldflags", "-X github.com/elyonar/einvoice-go.defaultBaseURLOverride="+srv.URL, ".")
	for _, id := range ids {
		op := ops[id]
		mu.Lock()
		requests = nil
		mu.Unlock()
		cmd := exec.Command(bin, id)
		cmd.Env = append(os.Environ(), "YONA_API_KEY="+fakeKey)
		if out, err := cmd.CombinedOutput(); err != nil {
			fail("%s: the snippet failed: %v\n%s", id, err, out)
		}
		mu.Lock()
		got := requests
		mu.Unlock()
		n := -1
		wantPath := opPlaceholder.ReplaceAllStringFunc(op.Path, func(string) string { n++; return samplePath(n) })
		if len(got) != 1 || got[0].method != op.Method || got[0].path != wantPath {
			fail("%s: expected one %s %s, the gateway saw %+v", id, op.Method, wantPath, got)
		}
		if op.RequestBody != nil && (!json.Valid(got[0].body) || !strings.HasPrefix(got[0].contentType, "application/json")) {
			fail("%s: expected a JSON body, got %q (%s)", id, got[0].body, got[0].contentType)
		}
	}
	fmt.Printf("guides/operations.json: %d snippets pass go vet and run against a fake gateway\n", len(ids))
}
