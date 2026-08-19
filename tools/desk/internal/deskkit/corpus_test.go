package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The GOLDEN CORPUS tests.
//
// bodycheck shipped on asserted, unmeasured accuracy. The first real measurement found
// that 100% of one diff's flags were legitimate go.sum lines and that 3% of runs in
// the one credential layout the path rule names were admitted. Both numbers exist
// because somebody finally ran the scanner over the house's own artifacts instead of
// reasoning about it. These two tests are that measurement, checked in, in BOTH
// directions, release-blocking for desk-tools (docs/desk-tools-gate-bar.md).
//
// The directions are not symmetric in cost and the corpus says so: a false positive
// strands a correct PR (a fix sat blocked across two sessions and two
// abandoned worktrees) while a false negative posts a credential. Neither is acceptable,
// and a change that trades one for the other fails the bar rather than passing half of it.

const corpusSeam = "{{+}}"

// corpusArtifact is one loaded corpus file.
type corpusArtifact struct {
	file    string
	name    string // `# corpus:` header
	expect  string // `# expect:` header — "clean" or "refused"
	refs    string // `# refs:` header
	why     string // `# why:` header
	payload string // everything after the header, seams removed
}

// loadCorpus reads testdata/corpus/*.txt, strips each file's `#` header and its `{{+}}`
// seams, and cross-checks the declared expectation against the filename prefix.
//
// The cross-check is not ceremony. A corpus whose expectation lives in ONE place is a
// corpus in which a typo silently flips an artifact from "must refuse" to "must pass" and
// the suite stays green — which is the same defect class the corpus exists to catch one
// level up. Two independent statements of the expectation, checked against each other, is
// the cheapest available version of derive-or-diff here.
func loadCorpus(t *testing.T) []corpusArtifact {
	t.Helper()
	dir := filepath.Join("testdata", "corpus")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read the corpus directory %s: %v — a scan whose corpus cannot be "+
			"read has NOT been measured; this is could-not-check, never green", dir, err)
	}
	var out []corpusArtifact
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("cannot read corpus artifact %s: %v", e.Name(), err)
		}
		a := parseCorpusArtifact(t, e.Name(), string(raw))
		var wantPrefix string
		switch a.expect {
		case "clean":
			wantPrefix = "neg-"
		case "refused":
			wantPrefix = "pos-"
		default:
			t.Fatalf("%s: `# expect:` is %q; want \"clean\" or \"refused\"", e.Name(), a.expect)
		}
		if !strings.HasPrefix(e.Name(), wantPrefix) {
			t.Fatalf("%s: declares `# expect: %s` but its filename prefix says otherwise — "+
				"an artifact whose two statements of its own expectation disagree cannot be "+
				"scored; rename it %s… or fix the header", e.Name(), a.expect, wantPrefix)
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		t.Fatalf("no corpus artifacts found under %s", dir)
	}
	return out
}

// parseCorpusArtifact splits a corpus file into its `#` header and its payload. The header
// is the leading run of `#` lines; everything after is content, seams removed.
func parseCorpusArtifact(t *testing.T, file, raw string) corpusArtifact {
	t.Helper()
	a := corpusArtifact{file: file}
	lines := strings.Split(raw, "\n")
	i := 0
	for ; i < len(lines) && strings.HasPrefix(lines[i], "#"); i++ {
		k, v, ok := strings.Cut(strings.TrimPrefix(lines[i], "#"), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "corpus":
			a.name = strings.TrimSpace(v)
		case "expect":
			a.expect = strings.TrimSpace(v)
		case "refs":
			a.refs = strings.TrimSpace(v)
		case "why":
			a.why = strings.TrimSpace(v)
		}
	}
	if a.name == "" || a.expect == "" || a.refs == "" {
		t.Fatalf("%s: header must carry `# corpus:`, `# expect:` and `# refs:` — an "+
			"unattributed corpus artifact cannot be re-judged when the rule it pins changes", file)
	}
	a.payload = strings.ReplaceAll(strings.Join(lines[i:], "\n"), corpusSeam, "")
	if strings.TrimSpace(a.payload) == "" {
		t.Fatalf("%s: empty payload", file)
	}
	return a
}

// TestBodycheckCorpus is the FALSE-POSITIVE direction: every sanctioned house artifact in
// the corpus must scan CLEAN. Each `neg-` file is a shape a real PR was blocked on.
//
// Release-blocking. A red run here means the scanner is refusing the house's own
// artifacts, which is how a fix ended up sitting in two abandoned
// worktrees, how the operator ended up with a manual human step,
// and how a lockfile ended up being DROPPED from a PR rather than the scanner fixed.
func TestBodycheckCorpus(t *testing.T) {
	for _, a := range loadCorpus(t) {
		if a.expect != "clean" {
			continue
		}
		t.Run(a.name, func(t *testing.T) {
			if err := BodyCheck([]byte(a.payload)); err != nil {
				t.Fatalf("FALSE POSITIVE on a sanctioned artifact.\n"+
					"  artifact: %s (%s)\n  refs:     %s\n  why:      %s\n  refusal:  %v",
					a.name, a.file, a.refs, a.why, err)
			}
		})
	}
}

// TestBodycheckPositives is the FALSE-NEGATIVE direction: every credential-shaped artifact
// must still be REFUSED, at exit 5.
//
// Release-blocking, and it is the half that stops this brief from being a licence to
// loosen. The path-rule differential is the standing proof both directions matter: a comment
// asserted a residual was impossible, a measurement found it at 3% under the published
// example key's own layout, and the fixture-only lock could not see it because the
// headline value was still refused. Every artifact here is synthetic or a published
// example; none is a live credential.
func TestBodycheckPositives(t *testing.T) {
	for _, a := range loadCorpus(t) {
		if a.expect != "refused" {
			continue
		}
		t.Run(a.name, func(t *testing.T) {
			err := BodyCheck([]byte(a.payload))
			if err == nil {
				t.Fatalf("FALSE NEGATIVE — credential-shaped artifact ADMITTED.\n"+
					"  artifact: %s (%s)\n  refs:     %s\n  why:      %s",
					a.name, a.file, a.refs, a.why)
			}
			if ExitCodeOf(err) != ExitRefused {
				t.Fatalf("%s: exit code = %d, want %d (a refusal that is not exit 5 is not a stop)",
					a.name, ExitCodeOf(err), ExitRefused)
			}
		})
	}
}

// TestBodycheckCorpusCoversEveryCataloguedShape pins the corpus COMPOSITION against the
// issues it was built from, so the set cannot quietly shrink to whatever currently passes.
//
// This is the shape of failure the stream's conventions call out by name: a check that
// still runs but no longer looks at the thing it was built to look at reports green
// forever. Dropping the go.sum artifact because it is inconvenient would leave both other
// tests passing and the go.sum class unmeasured.
func TestBodycheckCorpusCoversEveryCataloguedShape(t *testing.T) {
	required := map[string]string{
		"neg-go-sum-checksums.txt":                 "shape 4 — go.sum h1: digests",
		"neg-package-lock-integrity.txt":           "shape 4 — SRI integrity fields",
		"neg-key-path-shell-assignment.txt":        "key=path Evidence idiom",
		"neg-sops-encrypted-manifest.txt":          "sanctioned SOPS manifest",
		"neg-camelcase-test-identifiers.txt":       "shape 3 — CamelCase identifiers",
		"neg-bodycheck-own-marker-literals.txt":    "the detector's own source",
		"pos-aws-secret-access-key.txt":            "the slash-layout AWS class",
		"pos-aws-key-slash-layout-synthetic.txt":   "the measured residual population",
		"pos-pem-private-key.txt":                  "private-key armor must stay refused",
		"pos-secret-hidden-in-a-sops-document.txt": "the sops exemption must stay scoped",
	}
	present := map[string]bool{}
	for _, a := range loadCorpus(t) {
		present[a.file] = true
	}
	for file, why := range required {
		if !present[file] {
			t.Errorf("corpus artifact %s is missing — it pins %s", file, why)
		}
	}
}

// TestBodycheckCorpusCanFail is the PROOF THE CHECK CAN FAIL, which this stream requires
// of every check it ships (the unfailable-check lesson: eight unfailable checks landed in
// one day, and that is an authoring-loop defect rather than carelessness).
//
// It runs the two scoring rules above against POSITIVE CONTROLS — an artifact declared
// clean that is in fact a credential, and one declared refused that is in fact prose — and
// asserts each is scored WRONG. A corpus harness that cannot fail is not evidence about
// the scanner; it is evidence about nothing. The controls are constructed here rather than
// checked in, so no control file can drift into the real corpus.
func TestBodycheckCorpusCanFail(t *testing.T) {
	controls := []struct {
		name    string
		expect  string
		payload string
	}{
		{
			name:    "a credential declared clean must be scored as a false positive",
			expect:  "clean",
			payload: "token " + scanGHToken,
		},
		{
			name:    "prose declared refused must be scored as a false negative",
			expect:  "refused",
			payload: "LGTM — approving this PR, the logic reads correctly.",
		},
	}
	for _, c := range controls {
		t.Run(c.name, func(t *testing.T) {
			err := BodyCheck([]byte(c.payload))
			scoredOK := (c.expect == "clean" && err == nil) || (c.expect == "refused" && err != nil)
			if scoredOK {
				t.Fatalf("positive control was scored PASSING: expect=%q payload=%q err=%v — "+
					"the corpus scoring rule cannot distinguish a correct verdict from an "+
					"incorrect one, so a green TestBodycheckCorpus/TestBodycheckPositives "+
					"proves nothing", c.expect, c.payload, err)
			}
		})
	}
}
