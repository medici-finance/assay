package main

// kittext_test.go — the PUBLIC-TREE guard on the prompt kits.
//
// WHY IT EXISTS. The kits under references/ were EXTRACTED from house operating prose that
// is dense with private repository names, issue and PR references, internal document
// paths, item identifiers, and named incidents. Every clause was rewritten generic on the
// way in. Neutralising the text once is only half the fix: without a guard, the next
// person to strengthen a clause will reach for the original wording and quietly
// re-introduce the class. This test is the durable half.
//
// WHAT A LEAK COSTS, and why it is worth a test rather than a review note. A kit that
// names a private artifact publishes a MAP to it: the identifier confirms the artifact
// exists, names the tracker it lives in, and gives an outside reader a search term. That
// is true even though the artifact itself stays private — which is precisely why "the
// content is not in the diff" is not the standard being enforced here.
//
// THREE STATES, NOT TWO. A guard that finds nothing because the guard itself is broken
// must never read as clean. So the matcher is exercised against a deliberate positive
// control (TestLeakMatcherIsLive) — if the SAME matcher, run over text that certainly
// contains a hit, comes back clean, the instrument is broken and this file fails rather
// than passing.
//
// SCOPE. This checks the kit TEXT and the source comments in this package. It is not a
// substitute for the tree-wide sweep, and it does not claim to be: it closes the one class
// it can close mechanically, at the place that class is generated.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// withheldTokensEnv names the environment variable carrying the deployment's own withheld
// tokens — private repository names and the like — as a comma-separated list.
//
// MECHANISM PUBLIC, VALUES PRIVATE. The names themselves must NEVER appear in this tree:
// a guard that spells a withheld token as a source literal in order to forbid it has
// published the token, which is the failure it exists to prevent. So the shape-based
// patterns below are compiled in — they name no value — and the value-based half is
// supplied at run time by whoever knows the values.
//
// It is deliberately NOT an `ASSAY_`-prefixed roster key. The roster loader fails closed on
// a key it does not know, fleet-wide, so introducing one is a change to that contract; this
// is a test-time input with no such reach.
const withheldTokensEnv = "DESK_KIT_WITHHELD_TOKENS"

// leakPatterns are the shapes a kit must never carry. Each names what it is guarding, so a
// future failure explains itself instead of just going red.
var leakPatterns = []struct {
	why string
	re  *regexp.Regexp
}{
	{
		why: "a tracker-style issue or PR reference (#NNNN) — the number is a search term that " +
			"confirms an artifact exists and names where it lives",
		re: regexp.MustCompile(`#\d{2,}`),
	},
	{
		why: "a cross-repo issue reference (repo#123)",
		re:  regexp.MustCompile(`[A-Za-z0-9_-]+#\d+`),
	},
	{
		why: "an internal stream/document path — a path is a map to withheld material",
		re:  regexp.MustCompile(`docs/streams|docs/research|docs/roadmap|docs/archive`),
	},
	{
		why: "an item identifier of the <slug>/NN form used by the house's internal boards",
		re:  regexp.MustCompile(`\b[a-z][a-z0-9-]{3,}/[0-9]{2}\b`),
	},
	{
		why: "a dated incident citation — the date plus the surrounding sentence identifies a " +
			"private postmortem",
		re: regexp.MustCompile(`\b(19|20)\d\d-\d\d-\d\d\b`),
	},
	{
		why: "an F-number / R-number finding reference from the house register",
		re:  regexp.MustCompile(`\b[FR]-\d+\b`),
	},
}

// evidenceRowExemption is the ONE literal the date pattern must not flag: the Evidence
// row's own FORMAT string, which is a template rather than a date. Exempting it by exact
// literal — not by loosening the pattern — keeps every real date caught.
const evidenceRowExemption = "YYYY-MM-DD"

// allKits returns every embedded kit — the three class kits AND the common one. The common
// kit is easy to forget in a guard precisely because it is not selectable as a class, which
// is exactly why it is enumerated here rather than left to kitNames().
func allKits(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range kitNames() {
		text, err := kitText(name)
		if err != nil {
			t.Fatalf("kit %q: %v", name, err)
		}
		out[name] = text
	}
	common, err := commonKitText()
	if err != nil {
		t.Fatalf("common kit: %v", err)
	}
	out["common"] = common
	return out
}

func TestKitsCarryNoPrivateReferences(t *testing.T) {
	kits := allKits(t)
	if len(kits) != 4 {
		t.Fatalf("the guard scanned %d kits — every embedded kit must be covered", len(kits))
	}
	for name, text := range kits {
		checkNoLeaks(t, "kit "+name, text)
	}
}

// withheldTokens reads the deployment's own withheld tokens from the environment. An UNSET
// variable is could-not-check for that half of the guard, and this test says so out loud —
// a value-based check nobody supplied values for must not report the kits clean, because a
// silent pass there is indistinguishable from a real one.
func withheldTokens(t *testing.T) []string {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(withheldTokensEnv))
	if raw == "" {
		return nil
	}
	var out []string
	for _, tok := range strings.Split(raw, ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

func TestKitsCarryNoWithheldToken(t *testing.T) {
	tokens := withheldTokens(t)
	if len(tokens) == 0 {
		t.Skipf("could-not-check: %s is unset, so the value-based half of the leak guard did not run. "+
			"The shape-based half (TestKitsCarryNoPrivateReferences) still did. Supply the deployment's "+
			"withheld tokens as a comma-separated list to exercise this.", withheldTokensEnv)
	}
	for name, text := range allKits(t) {
		lower := strings.ToLower(text)
		for _, tok := range tokens {
			if strings.Contains(lower, strings.ToLower(tok)) {
				// The token itself is NOT echoed into the failure message: a CI log is a
				// published surface too.
				t.Errorf("kit %q carries a withheld token (entry %d of %s) — a kit that names a private "+
					"artifact publishes a map to it", name, indexOfToken(tokens, tok)+1, withheldTokensEnv)
			}
		}
	}
}

func indexOfToken(tokens []string, want string) int {
	for i, tok := range tokens {
		if tok == want {
			return i
		}
	}
	return -1
}

func checkNoLeaks(t *testing.T, what, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		scan := strings.ReplaceAll(line, evidenceRowExemption, "<date-format>")
		for _, p := range leakPatterns {
			if m := p.re.FindString(scan); m != "" {
				t.Errorf("%s carries %q — %s\n  line: %s", what, m, p.why, strings.TrimSpace(line))
			}
		}
	}
}

// TestLeakMatcherIsLive is the third state. It runs the SAME matcher over text that
// certainly contains every shape, and requires each pattern to fire. A guard whose matcher
// silently stopped matching would otherwise report the kits clean forever.
func TestLeakMatcherIsLive(t *testing.T) {
	positives := []string{
		"see #4242 for the rationale",
		"tracked as somerepo#42",
		"lifted from docs/streams/example/README.md",
		"the example-stream/07 item",
		"the 1999-12-31 incident",
		"finding F-99 covers it",
	}
	if len(positives) != len(leakPatterns) {
		t.Fatalf("%d positive controls for %d patterns — every pattern needs one, or an unexercised "+
			"pattern could rot into a no-op unnoticed", len(positives), len(leakPatterns))
	}
	for i, p := range leakPatterns {
		if p.re.FindString(positives[i]) == "" {
			t.Errorf("pattern %d (%s) did not fire on its positive control %q — the instrument is broken, "+
				"so a clean result from it means nothing", i, p.why, positives[i])
		}
	}
}

// The kits ship in a PUBLIC tree, so the source comments in this package are under the
// same rule as the kit text itself. A comment explaining a clause by citing the private
// incident it came from would leak exactly what the rewrite removed.
func TestKitReaderSourceCarriesNoPrivateReferences(t *testing.T) {
	for _, f := range []struct{ name, text string }{
		{"usage text", usage},
		{"tier clause", tierClause},
	} {
		checkNoLeaks(t, f.name, f.text)
	}
}
