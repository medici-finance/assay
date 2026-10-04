package main

// usagedoc_test.go — class guards for "the desk names a deskpr flag the help does not
// document, or one the verb does not have".
//
// Defect class: a flag that exists in a verb's flag set but is missing from its usage text
// (so an author reading the help cannot find it), and its mirror, a remedy string elsewhere
// in the desk that tells an author to run `deskpr <verb> --flag` for a flag the verb never
// registered. Both leave an author following a refusal with no verb to run.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// flagDefRe matches a flag registration on a flag set: fs.String("name", ...), fs.Bool, fs.Int.
var flagDefRe = regexp.MustCompile(`\bfs\.(?:String|Bool|Int)\("([a-z][a-z-]*)"`)

// scanOverrideDef matches the one registration that names its flag through a constant.
var scanOverrideDef = regexp.MustCompile(`\bfs\.String\(deskkit\.ScanOverrideFlag\b`)

// registeredFlags returns every flag name the deskpr verbs register, read from the
// non-test sources of this package.
func registeredFlags(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range flagDefRe.FindAllStringSubmatch(string(b), -1) {
			out[m[1]] = true
		}
		if scanOverrideDef.Match(b) {
			out[deskkit.ScanOverrideFlag] = true
		}
	}
	if len(out) < 8 {
		t.Fatalf("flag scanner found only %d registrations (%v) — its matcher no longer matches the source", len(out), out)
	}
	return out
}

// undocumentedFlags returns the registered flags whose `--name` never appears in text.
func undocumentedFlags(flags map[string]bool, text string) []string {
	var missing []string
	for name := range flags {
		if !strings.Contains(text, "--"+name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// TestUsageDocumentsEveryFlag: every flag a deskpr verb registers appears in the usage text
// `deskpr --help` and `deskpr <verb> --help` print.
func TestUsageDocumentsEveryFlag(t *testing.T) {
	if missing := undocumentedFlags(registeredFlags(t), usage); len(missing) > 0 {
		t.Errorf("deskpr usage does not mention registered flag(s) %v — an author reading --help cannot find them", missing)
	}
	// The desk-decided surface is the one the defect was filed on: pin its pieces by name.
	for _, want := range []string{"--decided", deskkit.DeskDecidedHeading, deskkit.DeskDecidedLabel, "decision:", "alternative:", "cost:"} {
		if !strings.Contains(usage, want) {
			t.Errorf("deskpr usage does not document %q", want)
		}
	}
}

// TestUsageGuardPositiveControl: the scanner must flag a planted undocumented flag, so a
// matcher that silently stopped matching fails instead of reporting clean.
func TestUsageGuardPositiveControl(t *testing.T) {
	planted := map[string]bool{"decided": true, "no-such-documented-flag": true}
	got := undocumentedFlags(planted, usage)
	if len(got) != 1 || got[0] != "no-such-documented-flag" {
		t.Errorf("planted undocumented flag not isolated: got %v", got)
	}
}

// remedyRe matches a remedy that names a deskpr verb with a flag: `deskpr edit --decided`.
var remedyRe = regexp.MustCompile("deskpr (?:create|edit|update)\\b[^`\"\\n]*?(--[a-z][a-z-]*)")

// remedyFlagsIn returns every --flag a `deskpr <verb> ... --flag` remedy in text names.
func remedyFlagsIn(text string) []string {
	var out []string
	for _, m := range remedyRe.FindAllStringSubmatch(text, -1) {
		out = append(out, strings.TrimPrefix(m[1], "--"))
	}
	return out
}

// TestNamedRemediesAreRealFlags: every `deskpr <verb> --flag` remedy named anywhere in the
// desk's non-test Go sources or the shipped skill/prompt text is a flag deskpr registers.
func TestNamedRemediesAreRealFlags(t *testing.T) {
	flags := registeredFlags(t)
	// help/version are handled before flag parsing and are real spellings.
	flags["help"], flags["version"] = true, true

	var roots = []string{"../../", "../../../../plugins/assay/skills"}
	checked := 0
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			isGo := strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
			isDoc := strings.HasSuffix(p, ".md")
			if !isGo && !isDoc {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			for _, f := range remedyFlagsIn(string(b)) {
				checked++
				if !flags[f] {
					t.Errorf("%s names remedy `deskpr ... --%s`, but deskpr registers no such flag", p, f)
				}
			}
			return nil
		})
	}
	if checked == 0 {
		t.Fatal("remedy scanner matched nothing — it must at least see the deskflip refusal's `deskpr edit --decided`")
	}
}

// TestRemedyGuardPositiveControl: a planted remedy naming a flag deskpr lacks must be seen.
func TestRemedyGuardPositiveControl(t *testing.T) {
	got := remedyFlagsIn("re-run `deskpr edit --no-such-flag` to fix it; or `deskpr create --decided F`")
	if len(got) != 2 || got[0] != "no-such-flag" || got[1] != "decided" {
		t.Errorf("planted remedy not extracted: got %v", got)
	}
	if registeredFlags(t)["no-such-flag"] {
		t.Error("the planted flag must not be a registered flag")
	}
}
