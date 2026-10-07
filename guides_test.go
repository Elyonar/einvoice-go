package einvoice

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// GUIDES: guides/guides.json is what the portal's Developers Overview renders (ruling R78), with the
// same schema as einvoice-js's. This pins the schema, ties every step to a snapshot operation, and
// fails when the committed file is not what `make guides` produces from examples/ today.

type guideStep struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Text        *string `json:"text"`
	SDK         string  `json:"sdk"`
	Curl        *string `json:"curl"`
	OperationID *string `json:"operationId"`
}

type guideRecipe struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Summary string      `json:"summary"`
	Steps   []guideStep `json:"steps"`
}

type guidesFile struct {
	SDKVersion    string        `json:"sdkVersion"`
	GeneratedFrom string        `json:"generatedFrom"`
	Install       string        `json:"install"`
	QuickStart    string        `json:"quickStart"`
	Recipes       []guideRecipe `json:"recipes"`
}

func loadGuides(t *testing.T) (guidesFile, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile("guides/guides.json")
	if err != nil {
		t.Fatal(err)
	}
	var g guidesFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	return g, generic
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func exampleDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("examples")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join("examples", e.Name(), "main.go")); err == nil {
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out
}

func nonEmpty(s string) bool { return strings.TrimSpace(s) != "" }

func TestGuidesHaveExactlyTheTopLevelSchemaThePortalReads(t *testing.T) {
	g, generic := loadGuides(t)
	if got := keysOf(generic); strings.Join(got, ",") != "generatedFrom,install,quickStart,recipes,sdkVersion" {
		t.Fatal(got)
	}
	if g.SDKVersion != Version {
		t.Fatalf("sdkVersion %q, Version %q", g.SDKVersion, Version)
	}
	if !regexp.MustCompile(`^[0-9a-f]{7,40}$`).MatchString(g.GeneratedFrom) {
		t.Fatal(g.GeneratedFrom)
	}
	if g.Install != "go get github.com/elyonar/einvoice-go" {
		t.Fatal(g.Install)
	}
	if !nonEmpty(g.QuickStart) || !strings.Contains(g.QuickStart, `"github.com/elyonar/einvoice-go"`) {
		t.Fatal(g.QuickStart)
	}
	if strings.Count(g.QuickStart, "{") != strings.Count(g.QuickStart, "}") {
		t.Fatal("the quick start does not balance its braces")
	}
	if len(g.Recipes) == 0 {
		t.Fatal("no recipes")
	}
}

func TestGuidesHaveOneRecipePerExampleEachWithTheExactRecipeAndStepSchema(t *testing.T) {
	g, generic := loadGuides(t)
	var ids []string
	for _, r := range g.Recipes {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	if strings.Join(ids, ",") != strings.Join(exampleDirs(t), ",") {
		t.Fatal(ids, exampleDirs(t))
	}
	rawRecipes := generic["recipes"].([]any)
	directives := regexp.MustCompile(`@(step|text|op|recipe|summary|quickstart)\b`)
	for i, r := range g.Recipes {
		rr := rawRecipes[i].(map[string]any)
		if strings.Join(keysOf(rr), ",") != "id,steps,summary,title" {
			t.Fatal(keysOf(rr))
		}
		if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(r.ID) || !nonEmpty(r.Title) || !nonEmpty(r.Summary) || len(r.Steps) == 0 {
			t.Fatalf("%+v", r)
		}
		seen := map[string]bool{}
		rawSteps := rr["steps"].([]any)
		for j, s := range r.Steps {
			rs := rawSteps[j].(map[string]any)
			if strings.Join(keysOf(rs), ",") != "curl,id,operationId,sdk,text,title" {
				t.Fatal(keysOf(rs))
			}
			if seen[s.ID] {
				t.Fatalf("%s: duplicate step %s", r.ID, s.ID)
			}
			seen[s.ID] = true
			if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(s.ID) || !nonEmpty(s.Title) || !nonEmpty(s.SDK) {
				t.Fatalf("%s/%s", r.ID, s.ID)
			}
			if s.Text != nil && !nonEmpty(*s.Text) {
				t.Fatalf("%s/%s: empty text", r.ID, s.ID)
			}
			if s.Curl != nil && !nonEmpty(*s.Curl) {
				t.Fatalf("%s/%s: empty curl", r.ID, s.ID)
			}
			if (s.Curl == nil) != (s.OperationID == nil) {
				t.Fatalf("%s/%s: curl and operationId must come together", r.ID, s.ID)
			}
			if directives.MatchString(s.SDK) {
				t.Fatalf("%s/%s: a directive leaked into the code", r.ID, s.ID)
			}
		}
		// imports are hoisted into the first step only
		if !strings.Contains(r.Steps[0].SDK, `"github.com/elyonar/einvoice-go"`) {
			t.Fatalf("%s: the first step has no import", r.ID)
		}
		for _, s := range r.Steps[1:] {
			if regexp.MustCompile(`(?m)^import `).MatchString(s.SDK) {
				t.Fatalf("%s/%s: imports outside the first step", r.ID, s.ID)
			}
		}
	}
}

func TestGuidesNameOnlyOperationsOfTheAPIKeySnapshotAndCurlCallsThatOperation(t *testing.T) {
	g, _ := loadGuides(t)
	ops := map[string]snapshotOp{}
	for _, op := range loadSnapshot(t).Operations {
		ops[op.OperationID] = op
	}
	for _, r := range g.Recipes {
		for _, s := range r.Steps {
			if s.OperationID == nil {
				continue
			}
			op, ok := ops[*s.OperationID]
			if !ok {
				t.Fatalf("%s/%s: %s is not in the snapshot", r.ID, s.ID, *s.OperationID)
			}
			first := strings.Split(*s.Curl, "\n")[0]
			if !strings.Contains(first, "https://gp.useyona.com"+op.Path) {
				t.Fatal(first)
			}
			if op.Method != "GET" && !strings.Contains(first, "-X "+op.Method+" ") {
				t.Fatal(first)
			}
		}
	}
}

func TestGuidesNeverCarryAKey(t *testing.T) {
	g, _ := loadGuides(t)
	raw, _ := os.ReadFile("guides/guides.json")
	if regexp.MustCompile(`sk_(test|live)_[a-z2-7]{16}_`).Match(raw) {
		t.Fatal("a key in the guides")
	}
	for _, r := range g.Recipes {
		for _, s := range r.Steps {
			if s.Curl != nil && !strings.Contains(*s.Curl, "Authorization: Bearer $YONA_API_KEY") {
				t.Fatal(*s.Curl)
			}
		}
	}
	newCall := regexp.MustCompile(`einvoice\.New\(\s*((?:[^(),]|\([^)]*\))+)`)
	for _, id := range exampleDirs(t) {
		src, err := os.ReadFile(filepath.Join("examples", id, "main.go"))
		if err != nil {
			t.Fatal(err)
		}
		matches := newCall.FindAllStringSubmatch(string(src), -1)
		if len(matches) == 0 {
			t.Fatalf("%s never creates a client", id)
		}
		for _, m := range matches {
			if strings.TrimSpace(m[1]) != `os.Getenv("YONA_API_KEY")` {
				t.Fatalf("%s: the key must come from os.Getenv(\"YONA_API_KEY\"), got %s", id, m[1])
			}
		}
	}
}

func TestGuidesAreCurrentAFreshExportOfExamplesEqualsTheCommittedFile(t *testing.T) {
	out, err := exec.Command("go", "run", "./scripts/export_guides", "--check").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
