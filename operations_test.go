package einvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// OPERATIONS: guides/operations.json tells the portal's API playground how this SDK calls each
// API-key operation, so a request built in the playground can be shown as Go. It is derived from the
// parity source, never written by hand: the operation of every method is the request recordCalls
// (parity_test.go) sees parityMethods make, matched to the snapshot by templateMatcher; the template
// is built from the method's Go signature (reflection), and the order of its path arguments is proved
// by calling it with distinct values and reading them back from the request path.
//
//	make operations         rewrite guides/operations.json (go test -run TestOperationsMapIsCurrent -write-operations)
//	make operations-check   the freshness and compile checks alone
//
// scripts/check_operations renders every template with sample values, compiles the snippets with
// `go vet` in a temporary module that replaces this one, and runs each against a local fake gateway.

var writeOperations = flag.Bool("write-operations", false, "rewrite guides/operations.json from the parity source")

// operationsPackage is what a Go developer installs (the module path is the registry name).
const operationsPackage = "github.com/elyonar/einvoice-go"

// operationsSetup imports the SDK and builds the client named `yona` from YONA_API_KEY, as the
// examples do. The portal shows it once, above the call; a call without a JSON body or query does not
// use encoding/json, so the portal drops that import line when the template has no `json.`.
const operationsSetup = `import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/elyonar/einvoice-go"
)

ctx := context.Background()
yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
if err != nil {
	log.Fatal(err)
}`

// preferredMethod names, for an operation several methods call, the plain one-call method the map
// shows. TestOperationsEveryOperationCalledByTwoMethodsHasAPreferredOne keeps it exact.
var preferredMethod = map[string]string{
	// Output.DownloadPDF is the same route with `Accept: application/pdf` and ?format=pdf; the
	// operation answers the JSON download link, which is GetDownloadLink.
	"downloadInvoice": "Output.GetDownloadLink",
}

type operationLiteral struct {
	String string `json:"string"`
	Body   string `json:"body"`
}

type operationEntry struct {
	Module   string `json:"module"`
	Method   string `json:"method"`
	Template string `json:"template"`
}

type operationsFile struct {
	Language      string                    `json:"language"`
	Package       string                    `json:"package"`
	SDKVersion    string                    `json:"sdkVersion"`
	GeneratedFrom string                    `json:"generatedFrom"`
	Setup         string                    `json:"setup"`
	Literal       operationLiteral          `json:"literal"`
	Operations    map[string]operationEntry `json:"operations"`
}

func serialiseOperations(t *testing.T, f operationsFile) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// snapshotHasBody: operationId → the operation takes a JSON request body.
func snapshotHasBody(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("scripts/openapi-api-key-ops.json")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Operations []struct {
			OperationID string          `json:"operationId"`
			RequestBody json.RawMessage `json:"requestBody"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, op := range s.Operations {
		out[op.OperationID] = len(op.RequestBody) > 0 && string(op.RequestBody) != "null"
	}
	return out
}

// methodOperations maps each parity method to the operationId it calls, from recordCalls.
func methodOperations(t *testing.T) map[string]snapshotOp {
	t.Helper()
	match := templateMatcher(loadSnapshot(t).Operations)
	out := map[string]snapshotOp{}
	for _, c := range recordCalls(t) {
		op := match(c.httpMethod, c.path)
		if op == nil {
			t.Fatalf("%s calls no snapshot operation", c.method)
		}
		out[c.method] = *op
	}
	return out
}

// chosenMethods: operationId → the method the map names (the only one, or preferredMethod's).
func chosenMethods(t *testing.T, byMethod map[string]snapshotOp) map[string]string {
	t.Helper()
	candidates := map[string][]string{}
	for m, op := range byMethod {
		candidates[op.OperationID] = append(candidates[op.OperationID], m)
	}
	out := map[string]string{}
	for opID, ms := range candidates {
		sort.Strings(ms)
		if len(ms) == 1 {
			out[opID] = ms[0]
			continue
		}
		pick, ok := preferredMethod[opID]
		if !ok {
			t.Fatalf("%s is called by %v: name the plain one-call method in preferredMethod", opID, ms)
		}
		found := false
		for _, m := range ms {
			found = found || m == pick
		}
		if !found {
			t.Fatalf("preferredMethod[%s] = %s is not one of %v", opID, pick, ms)
		}
		out[opID] = pick
	}
	for opID := range preferredMethod {
		if len(candidates[opID]) < 2 {
			t.Fatalf("preferredMethod[%s]: the operation is no longer called by several methods", opID)
		}
	}
	return out
}

// methodValue finds `Label.Method` (e.g. "Billing.Accounts.GetMine") on a client.
func methodValue(t *testing.T, c *Client, label string) (reflect.Value, string) {
	t.Helper()
	parts := strings.Split(label, ".")
	v := reflect.ValueOf(c)
	for _, field := range parts[:len(parts)-1] {
		v = v.Elem().FieldByName(field)
		if !v.IsValid() {
			t.Fatalf("%s: no field %s", label, field)
		}
	}
	m := v.MethodByName(parts[len(parts)-1])
	if !m.IsValid() {
		t.Fatalf("%s: no such method", label)
	}
	return m, v.Type().Elem().Name()
}

// declaredParamTypes: "InvoicesService.Create" → the type names of its parameters as the source
// spells them (`CreateInvoiceBody`, an alias reflection would report as `CreateInvoiceDto`).
func declaredParamTypes(t *testing.T) map[string][]string {
	t.Helper()
	files, err := filepath.Glob("services_*.go")
	if err != nil || len(files) == 0 {
		t.Fatal("no services_*.go", err)
	}
	out := map[string][]string{}
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			recv, ok := star.X.(*ast.Ident)
			if !ok {
				continue
			}
			var types []string
			for _, field := range fn.Type.Params.List {
				name := ""
				if p, ok := field.Type.(*ast.StarExpr); ok {
					if id, ok := p.X.(*ast.Ident); ok {
						name = id.Name
					}
				}
				n := len(field.Names)
				if n == 0 {
					n = 1
				}
				for i := 0; i < n; i++ {
					types = append(types, name)
				}
			}
			out[recv.Name+"."+fn.Name.Name] = types
		}
	}
	return out
}

var pathPlaceholder = regexp.MustCompile(`\{([^}]+)\}`)

var (
	contextType       = reflect.TypeOf((*context.Context)(nil)).Elem()
	requestOptionType = reflect.TypeOf(RequestOption(nil))
)

// templateFor builds the Go call of label for op from the method's signature.
func templateFor(t *testing.T, label string, op snapshotOp, hasBody bool) string {
	t.Helper()
	f := newFake(jsonAnswer(200, map[string]any{"meta": map[string]any{}, "data": nil}))
	c := newClient(t, f)
	m, receiver := methodValue(t, c, label)
	mt := m.Type()
	if mt.NumIn() < 2 || mt.In(0) != contextType || !mt.IsVariadic() || mt.In(mt.NumIn()-1).Elem() != requestOptionType {
		t.Fatalf("%s: expected (ctx, …, ...RequestOption), got %s", label, mt)
	}
	if mt.NumOut() != 2 {
		t.Fatalf("%s: expected (result, error), got %s", label, mt)
	}

	// The arguments between ctx and the options: path strings, then at most one *Query or *Body.
	args := []reflect.Value{reflect.ValueOf(context.Background())}
	var strs int
	var param reflect.Type
	for i := 1; i < mt.NumIn()-1; i++ {
		in := mt.In(i)
		switch {
		case in.Kind() == reflect.String && param == nil:
			args = append(args, reflect.ValueOf(fmt.Sprintf("zzarg%d", strs)))
			strs++
		case in.Kind() == reflect.Pointer && in.Elem().Kind() == reflect.Struct && param == nil:
			param = in.Elem()
			args = append(args, reflect.New(param))
		default:
			t.Fatalf("%s: unexpected parameter %d of %s", label, i, mt)
		}
	}

	// Which placeholder each string argument fills: call with distinct values, read the path back.
	out := m.Call(args)
	if err, _ := out[1].Interface().(error); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	if f.calls() != 1 {
		t.Fatalf("%s made %d requests", label, f.calls())
	}
	actual := strings.Split(f.requests[0].URL.Path, "/")
	declared := strings.Split(op.Path, "/")
	if len(actual) != len(declared) {
		t.Fatalf("%s: path %s does not have the shape of %s", label, f.requests[0].URL.Path, op.Path)
	}
	names := make([]string, strs)
	for i, seg := range declared {
		ph := pathPlaceholder.FindStringSubmatch(seg)
		if ph == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(actual[i], "zzarg%d", &n); err != nil || n >= strs || names[n] != "" {
			t.Fatalf("%s: placeholder {%s} is filled with %q", label, ph[1], actual[i])
		}
		names[n] = ph[1]
	}
	var specPath []string
	for _, p := range op.Parameters {
		if p.In == "path" {
			specPath = append(specPath, p.Name)
		}
	}
	sortedNames := append([]string(nil), names...)
	sort.Strings(sortedNames)
	sort.Strings(specPath)
	if strings.Join(sortedNames, ",") != strings.Join(specPath, ",") {
		t.Fatalf("%s: path arguments %v, the snapshot's path parameters %v", label, names, specPath)
	}

	// The struct parameter is the query or the body: its name says which, and the snapshot agrees.
	var lines []string
	callArgs := []string{"ctx"}
	for _, n := range names {
		callArgs = append(callArgs, "{{path:"+n+"}}")
	}
	if param != nil {
		// the parameter's type as the signature spells it (the alias, not what it aliases)
		typeName := ""
		if sig := declaredParamTypes(t)[receiver+"."+label[strings.LastIndex(label, ".")+1:]]; len(sig) == mt.NumIn() {
			typeName = sig[1+strs]
		}
		if typeName == "" {
			t.Fatalf("%s: the declared type of its struct parameter is not found in services_*.go", label)
		}
		if reflect.TypeOf(Client{}).PkgPath() != param.PkgPath() {
			t.Fatalf("%s: %s is not an einvoice type", label, param)
		}
		// body or query: the snapshot decides, and the declared name must agree
		var variable, placeholder, suffix string
		if hasBody {
			variable, placeholder, suffix = "params", "{{body}}", "Body"
		} else {
			if !hasQueryParameter(op) {
				t.Fatalf("%s takes %s but %s has neither a body nor query parameters", label, typeName, op.OperationID)
			}
			variable, placeholder, suffix = "query", "{{query}}", "Query"
		}
		if !strings.HasSuffix(typeName, suffix) {
			t.Fatalf("%s: %s is the operation's %s but is not named *%s", label, typeName, strings.ToLower(suffix), suffix)
		}
		lines = append(lines,
			"var "+variable+" einvoice."+typeName,
			"if err := json.Unmarshal([]byte("+placeholder+"), &"+variable+"); err != nil {",
			"\tlog.Fatal(err)",
			"}",
		)
		callArgs = append(callArgs, "&"+variable)
	} else if hasBody {
		t.Fatalf("%s takes no body but %s has one", label, op.OperationID)
	}
	lines = append(lines,
		"result, err := yona."+label+"("+strings.Join(callArgs, ", ")+")",
		"if err != nil {",
		"\tlog.Fatal(err)",
		"}",
	)
	return strings.Join(lines, "\n")
}

func hasQueryParameter(op snapshotOp) bool {
	for _, p := range op.Parameters {
		if p.In == "query" {
			return true
		}
	}
	return false
}

func buildOperations(t *testing.T, sha string) operationsFile {
	t.Helper()
	byMethod := methodOperations(t)
	bodies := snapshotHasBody(t)
	ops := map[string]snapshotOp{}
	for _, op := range byMethod {
		ops[op.OperationID] = op
	}
	entries := map[string]operationEntry{}
	for opID, label := range chosenMethods(t, byMethod) {
		dot := strings.LastIndex(label, ".")
		entries[opID] = operationEntry{
			Module:   label[:dot],
			Method:   label[dot+1:],
			Template: templateFor(t, label, ops[opID], bodies[opID]),
		}
	}
	return operationsFile{
		Language:      "go",
		Package:       operationsPackage,
		SDKVersion:    Version,
		GeneratedFrom: sha,
		Setup:         operationsSetup,
		Literal:       operationLiteral{String: "go", Body: "json-string"},
		Operations:    entries,
	}
}

func loadOperations(t *testing.T) (operationsFile, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile("guides/operations.json")
	if err != nil {
		t.Fatal("guides/operations.json is missing: run `make operations` and commit it")
	}
	var f operationsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	return f, generic
}

func TestOperationsMapIsCurrent(t *testing.T) {
	if *writeOperations {
		sha := "0000000"
		if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil && len(bytes.TrimSpace(out)) > 0 {
			sha = string(bytes.TrimSpace(out))
		}
		f := buildOperations(t, sha)
		if err := os.WriteFile("guides/operations.json", []byte(serialiseOperations(t, f)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("guides/operations.json: %d operations (from %s)", len(f.Operations), sha)
		return
	}
	committed, _ := loadOperations(t)
	fresh := buildOperations(t, committed.GeneratedFrom)
	raw, _ := os.ReadFile("guides/operations.json")
	if serialiseOperations(t, fresh) != string(raw) {
		t.Fatal("guides/operations.json is stale: run `make operations` and commit it")
	}
}

func TestOperationsMapHasExactlyTheContractSchema(t *testing.T) {
	f, generic := loadOperations(t)
	if got := strings.Join(keysOf(generic), ","); got != "generatedFrom,language,literal,operations,package,sdkVersion,setup" {
		t.Fatal(got)
	}
	if f.Language != "go" || f.Package != operationsPackage || f.SDKVersion != Version {
		t.Fatalf("%s %s %s", f.Language, f.Package, f.SDKVersion)
	}
	if !regexp.MustCompile(`^[0-9a-f]{7,40}$`).MatchString(f.GeneratedFrom) {
		t.Fatal(f.GeneratedFrom)
	}
	if f.Literal != (operationLiteral{String: "go", Body: "json-string"}) {
		t.Fatal(f.Literal)
	}
	for _, want := range []string{`"github.com/elyonar/einvoice-go"`, "ctx := context.Background()", `yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))`, "if err != nil {\n\tlog.Fatal(err)\n}"} {
		if !strings.Contains(f.Setup, want) {
			t.Fatalf("the setup lacks %q", want)
		}
	}
	for opID, e := range generic["operations"].(map[string]any) {
		if got := strings.Join(keysOf(e.(map[string]any)), ","); got != "method,module,template" {
			t.Fatalf("%s: %s", opID, got)
		}
	}
}

func TestOperationsMapCoversExactlyTheNonExcludedOperationsWithTheParityMethods(t *testing.T) {
	f, _ := loadOperations(t)
	excluded := map[string]bool{}
	for _, e := range ExcludedOperations {
		excluded[opKey(e.Method, e.Path)] = true
	}
	var want []string
	for _, op := range loadSnapshot(t).Operations {
		if !excluded[opKey(op.Method, op.Path)] {
			want = append(want, op.OperationID)
		}
	}
	sort.Strings(want)
	var got []string
	for opID := range f.Operations {
		got = append(got, opID)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("operations differ:\nmap:      %v\nsnapshot: %v", got, want)
	}
	byMethod := methodOperations(t)
	for opID, e := range f.Operations {
		label := e.Module + "." + e.Method
		op, ok := byMethod[label]
		if !ok {
			t.Fatalf("%s: %s is not a parity method", opID, label)
		}
		if op.OperationID != opID {
			t.Fatalf("%s: %s calls %s", opID, label, op.OperationID)
		}
	}
}

func TestOperationsTemplatesUseExactlyTheirOperationsPlaceholders(t *testing.T) {
	f, _ := loadOperations(t)
	bodies := snapshotHasBody(t)
	ops := map[string]snapshotOp{}
	for _, op := range loadSnapshot(t).Operations {
		ops[op.OperationID] = op
	}
	placeholder := regexp.MustCompile(`\{\{([^}]*)\}\}`)
	for opID, e := range f.Operations {
		op := ops[opID]
		var want []string
		for _, p := range op.Parameters {
			if p.In == "path" {
				want = append(want, "path:"+p.Name)
			}
		}
		if bodies[opID] {
			want = append(want, "body")
		}
		// a method may leave an optional query out; it may only take one the operation has
		if strings.Contains(e.Template, "{{query}}") {
			if !hasQueryParameter(op) {
				t.Fatalf("%s: {{query}} but the operation has no query parameters", opID)
			}
			want = append(want, "query")
		}
		var got []string
		for _, m := range placeholder.FindAllStringSubmatch(e.Template, -1) {
			got = append(got, m[1])
		}
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: placeholders %v, want %v", opID, got, want)
		}
		if !strings.Contains(e.Template, "result, err := yona."+e.Module+"."+e.Method+"(ctx") {
			t.Fatalf("%s: the template does not assign the call to result: %s", opID, e.Template)
		}
	}
}

func TestOperationsEveryOperationCalledByTwoMethodsHasAPreferredOne(t *testing.T) {
	chosenMethods(t, methodOperations(t))
}

// The rendered snippets compile (go vet) and run against a local fake gateway: scripts/check_operations.
func TestOperationsSnippetsCompileAndRun(t *testing.T) {
	out, err := exec.Command("go", "run", "./scripts/check_operations").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
