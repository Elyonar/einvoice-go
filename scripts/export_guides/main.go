// examples/<id>/main.go → guides/guides.json, the developer guides the portal shows (ruling R78). Not shipped.
//
//	go run ./scripts/export_guides            write guides/guides.json
//	go run ./scripts/export_guides --check    exit 1 unless the committed file equals a fresh export
//	                                          (every field but generatedFrom, the commit it was made at)
//
// An example is annotated with comment lines (the same directives as einvoice-js):
//
//	// @recipe <id> <Title>       header: id = the directory name
//	// @summary <text>            header
//	// @step <id> <Title>         a step: its code runs until the next @step
//	// @text <guidance>           optional, after @step (may repeat; joined with a space)
//	// @op <operationId>          optional: the API operation of the step (scripts/openapi-api-key-ops.json)
//	// @quickstart                optional: the step is part of the README-sized quick start
//
// Everything before the first @step (package, imports, `func main() {` and its setup) is hoisted into
// the first step's code; the quick start gets it with the imports it does not use pruned, and the
// braces it leaves open closed. The curl of a step is generated from its @op: method, path with its
// {placeholders}, the required query parameters, and an example body built from the request schema.
// It never carries a real key: $YONA_API_KEY.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/elyonar/einvoice-go/scripts/internal/examples"
	"github.com/elyonar/einvoice-go/scripts/internal/schemaexample"
)

const (
	host    = "https://gp.useyona.com"
	install = "go get github.com/elyonar/einvoice-go"
)

var directive = regexp.MustCompile(`^\s*//\s*@(recipe|summary|step|text|op|quickstart)\b\s*(.*)$`)

// ── the output schema (the portal codes against it) ──

type step struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Text        *string `json:"text"`
	SDK         string  `json:"sdk"`
	Curl        *string `json:"curl"`
	OperationID *string `json:"operationId"`
}

type recipe struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Steps   []step `json:"steps"`
}

type guides struct {
	SDKVersion    string   `json:"sdkVersion"`
	GeneratedFrom string   `json:"generatedFrom"`
	Install       string   `json:"install"`
	QuickStart    string   `json:"quickStart"`
	Recipes       []recipe `json:"recipes"`
}

// ── the snapshot ──

type snapshotOp struct {
	Method      string           `json:"method"`
	Path        string           `json:"path"`
	OperationID string           `json:"operationId"`
	Description string           `json:"description"`
	Parameters  []map[string]any `json:"parameters"`
	RequestBody map[string]any   `json:"requestBody"`
}

type snapshot struct {
	Operations []snapshotOp              `json:"operations"`
	Schemas    map[string]map[string]any `json:"schemas"`
}

var (
	root string
	snap snapshot
	ops  map[string]snapshotOp
)

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// ── example bodies from the schema (scripts/internal/schemaexample) ──

func marshal(v any, indent string) ([]byte, error) { return schemaexample.Marshal(v, indent) }

func exampleOf(schema map[string]any) any {
	value, err := schemaexample.Builder{Schemas: snap.Schemas}.Of(schema)
	if err != nil {
		fail("%v", err)
	}
	return value
}

// acceptsIdempotencyKey mirrors parity_test.go: the route reads an Idempotency-Key.
func acceptsIdempotencyKey(op snapshotOp) bool {
	if op.Method != "POST" && op.Method != "GET" {
		return false
	}
	for _, p := range op.Parameters {
		name, _ := p["name"].(string)
		if p["in"] == "header" && strings.EqualFold(name, "idempotency-key") {
			return true
		}
	}
	return strings.Contains(op.Description, "Idempotency-Key") && !strings.Contains(op.Description, "Idempotency-Key` is ignored")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// encodeURIComponent percent-encodes as JavaScript does (spaces as %20, `!'()*~` kept).
func encodeURIComponent(s string) string {
	var sb strings.Builder
	for _, b := range []byte(s) {
		switch {
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9', strings.IndexByte("-_.!~*'()", b) >= 0:
			sb.WriteByte(b)
		default:
			fmt.Fprintf(&sb, "%%%02X", b)
		}
	}
	return sb.String()
}

// stringOf renders a JSON scalar as JavaScript's String() would.
func stringOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, _ := marshal(x, "")
		return string(b)
	}
}

func curlFor(operationID string) string {
	op, ok := ops[operationID]
	if !ok {
		fail("@op %s is not an API-key operation of the snapshot", operationID)
	}
	var query []string
	for _, p := range op.Parameters {
		required, _ := p["required"].(bool)
		if p["in"] != "query" || !required {
			continue
		}
		name, _ := p["name"].(string)
		schema, _ := p["schema"].(map[string]any)
		value := exampleOf(schema)
		rendered := name
		if value != nil {
			rendered = stringOf(value)
		}
		query = append(query, encodeURIComponent(name)+"="+encodeURIComponent(rendered))
	}
	target := host + op.Path
	if len(query) > 0 {
		target += "?" + strings.Join(query, "&")
	}
	first := fmt.Sprintf(`curl "%s"`, target)
	if op.Method != "GET" {
		first = fmt.Sprintf(`curl -X %s "%s"`, op.Method, target)
	}
	lines := []string{first, `  -H "Authorization: Bearer $YONA_API_KEY"`}
	var bodySchema map[string]any
	if content, ok := op.RequestBody["content"].(map[string]any); ok {
		if j, ok := content["application/json"].(map[string]any); ok {
			bodySchema, _ = j["schema"].(map[string]any)
		}
	}
	if acceptsIdempotencyKey(op) {
		lines = append(lines, `  -H "Idempotency-Key: $(uuidgen)"`)
	}
	if bodySchema != nil {
		body, err := marshal(exampleOf(bodySchema), "  ")
		if err != nil {
			fail("%s: %v", operationID, err)
		}
		lines = append(lines, `  -H "Content-Type: application/json"`, "  -d "+shellQuote(string(body)))
	}
	return strings.Join(lines, " \\\n")
}

// ── parsing ──

func trimBlank(lines []string) []string {
	a, b := 0, len(lines)
	for a < b && strings.TrimSpace(lines[a]) == "" {
		a++
	}
	for b > a && strings.TrimSpace(lines[b-1]) == "" {
		b--
	}
	return lines[a:b]
}

type parsedStep struct {
	id, title  string
	text       []string
	op         string
	quickstart bool
	code       []string
	started    bool
}

type parsedRecipe struct {
	id, title, summary string
}

var stepID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func splitFirst(value string) (string, string) {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.TrimSpace(parts[1])
}

func parseExample(id, source string) (parsedRecipe, string, []*parsedStep) {
	var rec parsedRecipe
	var header []string
	var steps []*parsedStep
	var current *parsedStep
	for _, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		if m := directive.FindStringSubmatch(line); m != nil {
			kind, value := m[1], strings.TrimSpace(m[2])
			switch kind {
			case "recipe", "summary":
				if current != nil {
					fail("%s: @%s must come before the first @step", id, kind)
				}
				if kind == "recipe" {
					rec.id, rec.title = splitFirst(value)
				} else {
					rec.summary = value
				}
			case "step":
				sid, title := splitFirst(value)
				current = &parsedStep{id: sid, title: title}
				steps = append(steps, current)
			default:
				if current == nil {
					fail("%s: @%s outside a step", id, kind)
				}
				if current.started {
					fail("%s/%s: @%s must directly follow @step", id, current.id, kind)
				}
				switch kind {
				case "text":
					current.text = append(current.text, value)
				case "op":
					current.op = value
				default:
					current.quickstart = true
				}
			}
			continue
		}
		if current != nil {
			current.started = true
			current.code = append(current.code, line)
		} else {
			header = append(header, line)
		}
	}
	if rec.id != id {
		fail("%s: @recipe id must be the directory name (got %q)", id, rec.id)
	}
	if rec.title == "" || rec.summary == "" {
		fail("%s: @recipe needs a title and @summary", id)
	}
	if len(steps) == 0 {
		fail("%s: no @step", id)
	}
	ids := map[string]bool{}
	for _, s := range steps {
		if !stepID.MatchString(s.id) || s.title == "" {
			fail("%s: a @step needs a kebab-case id and a title", id)
		}
		if ids[s.id] {
			fail("%s: duplicate step id %s", id, s.id)
		}
		ids[s.id] = true
		if s.op != "" {
			if _, ok := ops[s.op]; !ok {
				fail("%s/%s: @op %s is not in the snapshot", id, s.id, s.op)
			}
		}
		if len(trimBlank(s.code)) == 0 {
			fail("%s/%s: the step has no code", id, s.id)
		}
	}
	return rec, strings.Join(trimBlank(header), "\n"), steps
}

var importLine = regexp.MustCompile(`^\s*(?:([A-Za-z_][A-Za-z0-9_]*)\s+)?"([^"]+)"\s*$`)

// pruneImports drops the imports a code string does not use (the header's own non-import lines count
// as code too), and collapses the blank lines that leaves inside the import block.
func pruneImports(header, code string) string {
	lines := strings.Split(header, "\n")
	var nonImport []string
	inBlock := false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "import ("):
			inBlock = true
		case inBlock && strings.TrimSpace(l) == ")":
			inBlock = false
		case inBlock, strings.HasPrefix(l, "import "):
		default:
			nonImport = append(nonImport, l)
		}
	}
	used := code + "\n" + strings.Join(nonImport, "\n")
	usesIdent := func(alias, path string) bool {
		ident := alias
		if ident == "" {
			ident = path[strings.LastIndex(path, "/")+1:]
			if i := strings.LastIndex(ident, "-"); i >= 0 {
				ident = ident[:i]
			}
		}
		return regexp.MustCompile(`\b` + regexp.QuoteMeta(ident) + `\.`).MatchString(used)
	}
	var out []string
	var block []string
	inBlock = false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "import ("):
			inBlock = true
			block = nil
		case inBlock && strings.TrimSpace(l) == ")":
			inBlock = false
			kept := trimBlank(block)
			// collapse runs of blank lines left by pruned groups
			var collapsed []string
			for _, b := range kept {
				if strings.TrimSpace(b) == "" && len(collapsed) > 0 && strings.TrimSpace(collapsed[len(collapsed)-1]) == "" {
					continue
				}
				collapsed = append(collapsed, b)
			}
			if len(collapsed) > 0 {
				out = append(out, "import (")
				out = append(out, collapsed...)
				out = append(out, ")")
			}
		case inBlock:
			if m := importLine.FindStringSubmatch(l); m != nil && !usesIdent(m[1], m[2]) {
				continue
			}
			block = append(block, l)
		case strings.HasPrefix(l, "import "):
			if m := importLine.FindStringSubmatch(strings.TrimPrefix(l, "import ")); m != nil && !usesIdent(m[1], m[2]) {
				continue
			}
			out = append(out, l)
		default:
			out = append(out, l)
		}
	}
	// A pruned import block may leave two blank lines in a row around it.
	var final []string
	for _, l := range out {
		if strings.TrimSpace(l) == "" && len(final) > 0 && strings.TrimSpace(final[len(final)-1]) == "" {
			continue
		}
		final = append(final, l)
	}
	return strings.Join(final, "\n")
}

// closeBraces appends the `}` the code leaves open (the quick start hoists `func main() {`).
func closeBraces(code string) string {
	open := strings.Count(code, "{") - strings.Count(code, "}")
	for i := 0; i < open; i++ {
		code += "\n}"
	}
	return code
}

func version() string {
	src, err := os.ReadFile(filepath.Join(root, "version.go"))
	if err != nil {
		fail("%v", err)
	}
	m := regexp.MustCompile(`Version = "([^"]+)"`).FindSubmatch(src)
	if m == nil {
		fail("version.go has no Version")
	}
	return string(m[1])
}

func buildGuides(sha string) guides {
	ids, err := examples.IDs(root)
	if err != nil {
		fail("%v", err)
	}
	var recipes []recipe
	type quick struct{ header, code string }
	var quicks []quick
	for _, id := range ids {
		src, err := os.ReadFile(filepath.Join(root, "examples", id, "main.go"))
		if err != nil {
			fail("%v", err)
		}
		rec, header, steps := parseExample(id, string(src))
		r := recipe{ID: rec.id, Title: rec.title, Summary: rec.summary}
		for i, s := range steps {
			code := strings.Join(trimBlank(s.code), "\n")
			if s.quickstart {
				quicks = append(quicks, quick{header, code})
			}
			st := step{ID: s.id, Title: s.title, SDK: code}
			if i == 0 && header != "" {
				st.SDK = header + "\n\n" + code
			}
			if len(s.text) > 0 {
				text := strings.Join(s.text, " ")
				st.Text = &text
			}
			if s.op != "" {
				curl := curlFor(s.op)
				op := s.op
				st.Curl, st.OperationID = &curl, &op
			}
			r.Steps = append(r.Steps, st)
		}
		recipes = append(recipes, r)
	}
	if len(quicks) == 0 {
		fail("no step is marked @quickstart")
	}
	for _, q := range quicks {
		if q.header != quicks[0].header {
			fail("@quickstart steps must come from one example")
		}
	}
	var codes []string
	for _, q := range quicks {
		codes = append(codes, q.code)
	}
	quickCode := strings.Join(codes, "\n\n")
	return guides{
		SDKVersion:    version(),
		GeneratedFrom: sha,
		Install:       install,
		QuickStart:    closeBraces(pruneImports(quicks[0].header, quickCode) + "\n\n" + quickCode),
		Recipes:       recipes,
	}
}

func serialise(g guides) string {
	b, err := marshal(g, "  ")
	if err != nil {
		fail("%v", err)
	}
	return string(b) + "\n"
}

func main() {
	var err error
	if root, err = examples.Root(); err != nil {
		fail("%v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "openapi-api-key-ops.json"))
	if err != nil {
		fail("%v", err)
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		fail("%v", err)
	}
	ops = map[string]snapshotOp{}
	for _, op := range snap.Operations {
		ops[op.OperationID] = op
	}
	out := filepath.Join(root, "guides", "guides.json")
	if len(os.Args) > 1 && os.Args[1] == "--check" {
		committedRaw, err := os.ReadFile(out)
		if err != nil {
			fail("guides/guides.json is missing: run `make guides` and commit it")
		}
		var committed guides
		if err := json.Unmarshal(committedRaw, &committed); err != nil {
			fail("guides/guides.json is not readable: %v", err)
		}
		fresh := buildGuides(committed.GeneratedFrom)
		if serialise(fresh) != serialise(committed) {
			fail("guides/guides.json is stale: run `make guides` and commit it")
		}
		fmt.Println("guides/guides.json is current")
		return
	}
	sha := "0000000"
	if shaOut, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output(); err == nil && len(bytes.TrimSpace(shaOut)) > 0 {
		sha = string(bytes.TrimSpace(shaOut))
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fail("%v", err)
	}
	g := buildGuides(sha)
	if err := os.WriteFile(out, []byte(serialise(g)), 0o644); err != nil {
		fail("%v", err)
	}
	steps := 0
	for _, r := range g.Recipes {
		steps += len(r.Steps)
	}
	fmt.Printf("guides/guides.json: %d recipes, %d steps (from %s)\n", len(g.Recipes), steps, sha)
}
