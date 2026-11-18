package einvoice

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// SPELLING (ruling R230, 18 Nov 2026): "organization" is written with a z in this repository, in
// words people read, identifiers, comments, tests, file names, example names and guide ids. The rule
// covers that one word family and every cased or joined form of it (sub-organization,
// SubOrganizations, ORGANIZATION); every other British spelling (authorise, colour, licence) stays.
//
// The s spelling may appear only where spellingAllowed says, each entry with its reason. An entry
// that no longer matches anything fails as well, so the list can only shrink: the generated files
// leave it when `make sync` regenerates them from an API document that serves the z spelling.

// sSpelling is the s spelling of the word family, in any case and inside any identifier.
var sSpelling = regexp.MustCompile(`(?i)organi[s](?:e|ing|ation)`)

var spellingAllowed = []struct{ file, reason string }{
	{"scripts/openapi-api-key-ops.json", "copied from einvoice-js by make sync, which takes it from the services' API document"},
	{"types_gen.go", "generated from that snapshot by make sync"},
}

// CHANGELOG.md is history: every released entry stays as it was written. Above the first released
// version, the s spelling may appear only inside a code span, where an entry quotes an old value or
// name in backticks; never in its prose.
var (
	spellingReleased = regexp.MustCompile(`^#{1,6} \[?\d+\.\d+\.\d+`)
	spellingCodeSpan = regexp.MustCompile("`[^`\n]*`")
)

func spellingFiles(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	seen := map[string]bool{}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f == "" || seen[f] {
			continue
		}
		if _, err := os.Stat(f); err != nil {
			continue
		}
		seen[f] = true
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

func spellingIsAllowed(file string) bool {
	for _, a := range spellingAllowed {
		if a.file == file {
			return true
		}
	}
	return false
}

// spellingOffences is the file:line of every s spelling the rule does not allow in this file's text.
func spellingOffences(file, text string) []string {
	if spellingIsAllowed(file) {
		return nil
	}
	lines := strings.Split(text, "\n")
	released := -1
	if file == "CHANGELOG.md" {
		for i, line := range lines {
			if spellingReleased.MatchString(line) {
				released = i
				break
			}
		}
	}
	var found []string
	for i, line := range lines {
		if released != -1 && i >= released {
			break
		}
		checked := line
		if file == "CHANGELOG.md" {
			checked = spellingCodeSpan.ReplaceAllString(line, "")
		}
		if sSpelling.MatchString(checked) {
			shown := strings.TrimSpace(line)
			if len(shown) > 120 {
				shown = shown[:120]
			}
			found = append(found, fmt.Sprintf("%s:%d: %s", file, i+1, shown))
		}
	}
	return found
}

// spellingWord assembles a word, so that this file carries no s spelling itself.
func spellingWord(head, tail string) string { return head + tail }

func TestSpellingReadsTheRepository(t *testing.T) {
	files := spellingFiles(t)
	for _, want := range []string{"go.mod", "spelling_test.go"} {
		if !slices.Contains(files, want) {
			t.Fatalf("git ls-files does not list %s", want)
		}
	}
}

func TestSpellingNoFileNameOrPathCarriesTheSSpelling(t *testing.T) {
	for _, f := range spellingFiles(t) {
		if sSpelling.MatchString(f) {
			t.Errorf("%s: the path carries the s spelling", f)
		}
	}
}

func TestSpellingNoFileCarriesItOutsideTheAllowedPlaces(t *testing.T) {
	var found []string
	for _, f := range spellingFiles(t) {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.IndexByte(raw, 0) != -1 {
			continue
		}
		found = append(found, spellingOffences(f, string(raw))...)
	}
	if len(found) > 0 {
		t.Errorf("the s spelling of the word, outside the allowed places:\n%s", strings.Join(found, "\n"))
	}
}

func TestSpellingAnAllowedFileStillCarriesTheSSpelling(t *testing.T) {
	files := spellingFiles(t)
	for _, a := range spellingAllowed {
		t.Run(a.file, func(t *testing.T) {
			if !slices.Contains(files, a.file) {
				t.Fatalf("%s is allowed but not in the repository: remove it from spellingAllowed", a.file)
			}
			raw, err := os.ReadFile(a.file)
			if err != nil {
				t.Fatal(err)
			}
			if !sSpelling.Match(raw) {
				t.Errorf("%s no longer carries the s spelling: remove it from spellingAllowed (%s)", a.file, a.reason)
			}
		})
	}
}

func TestSpellingTheCheckCatchesEveryForm(t *testing.T) {
	words := []string{
		spellingWord("organi", "sation"), spellingWord("Organi", "sations"), spellingWord("ORGANI", "SATION"),
		spellingWord("organi", "sational"), spellingWord("organi", "se"), spellingWord("organi", "sed"),
		spellingWord("organi", "ses"), spellingWord("organi", "sing"), spellingWord("organi", "ser"),
		spellingWord("sub-organi", "sation"), spellingWord("SubOrgani", "sations"), spellingWord("sub_organi", "sations"),
		spellingWord("organi", "sationID"), spellingWord("IssuesAsOrgani", "sation"),
	}
	for _, word := range words {
		if !sSpelling.MatchString(word) {
			t.Errorf("the check misses %s", word)
		}
		if got := spellingOffences("README.md", "a line with "+word+" in it"); len(got) != 1 {
			t.Errorf("%s: %d offences, want 1", word, len(got))
		}
	}
}

func TestSpellingTheCheckLeavesOtherWordsAlone(t *testing.T) {
	for _, word := range []string{"organization", "Organizations", "ORGANIZATION", "organizational", "organize", "sub-organization", "organism", "authorise", "colour", "licence", "centre"} {
		if sSpelling.MatchString(word) {
			t.Errorf("the check catches %s", word)
		}
	}
}

func TestSpellingInTheChangelogHistoryAndQuotedOldValuesAreAllowed(t *testing.T) {
	old := spellingWord("organi", "sation")
	changelog := strings.Join([]string{
		"# Changelog",
		"## Unreleased",
		"- `issuedAs.kind` answers `organization` where it answered `" + old + "`.",
		"- Every " + old + " reads with a z.",
		"## 0.2.2 (2026-11-12)",
		"- The " + old + "'s connection.",
	}, "\n")
	want := []string{"CHANGELOG.md:4: - Every " + old + " reads with a z."}
	if got := spellingOffences("CHANGELOG.md", changelog); !reflect.DeepEqual(got, want) {
		t.Errorf("CHANGELOG.md offences = %q, want %q", got, want)
	}
	if got := spellingOffences("README.md", changelog); len(got) != 3 {
		t.Errorf("README.md offences = %d, want 3", len(got))
	}
}
