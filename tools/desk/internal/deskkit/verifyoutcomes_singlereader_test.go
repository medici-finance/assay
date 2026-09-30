package deskkit

// verifyoutcomes_singlereader_test.go — Verify row 5: the #882 choke-point guard for this
// module. One structural test: walk every non-test .go file under tools/desk and fail naming
// any file OTHER than the allow-listed reader/writer set that contains the literal
// "verify-outcomes" — the model clause 14 already establishes elsewhere in this codebase
// (corpusleak_test.go, principal_class_test.go's onBehalfOfCallSites): an ALLOW-LIST structural
// test, with a committed positive-control fixture so a broken matcher fails loud rather than
// reporting a false clean.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifyOutcomesChokePointAllowList is the ALLOW-LIST of every non-test .go file under
// tools/desk that may contain the literal "verify-outcomes" — the reader/name choke point
// itself, the writer (--outcome-record and its class guard), and every file that merely
// mentions the term in prose or an error-message string (never opens the path directly).
// Adding a new direct reader/writer means adding its line here, with a reason; a stale entry
// (a file that no longer contains the literal) is ALSO a failure, so this cannot drift into
// describing code that no longer exists — see the two-way check in the test below.
var verifyOutcomesChokePointAllowList = map[string]string{
	"internal/deskkit/verifyoutcomes.go":   "the reader/name choke point itself",
	"internal/deskkit/verifywake.go":       "the legacy receipt shape's field comments — no file I/O",
	"internal/deskkit/principal.go":        "prose describing a field's origin — no file I/O",
	"internal/deskkit/repairobligation.go": "prose analogy to a sibling sidecar — no file I/O",
	"cmd/deskevidence/deskevidence.go":     "the writer's class guard + isVerifyOutcomesSidecar gating predicate",
	"cmd/deskevidence/outcome.go":          "an error-message STRING, not a path — no file I/O",
	"cmd/deskevidence/outcomerecord.go":    "the writer — the --outcome-record landing shape",
	"cmd/deskevidence/verifiedgate.go":     "the verified-outcome closure gate — parses bytes cmdEvidence/cmdOutcomeRecordWrite already fetched, opens no path itself",
	"cmd/fanoutloop/repair.go":             "prose analogy to a sibling sidecar — no file I/O",
	"cmd/verifyloop/briefscan.go":          "the desk-side reader's one caller, itself calling only deskkit.ReadVerifyOutcomes",
}

// verifyOutcomesChokePointScan walks walkRoot for non-test .go files containing the literal
// "verify-outcomes", returning hits as paths relative to moduleRoot (kept separate so a scan
// over the testdata fixture, rooted elsewhere, still reports stable, comparable keys).
func verifyOutcomesChokePointScan(walkRoot, moduleRoot string) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(walkRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// testdata is scan input for the POSITIVE CONTROL, addressed by its own
			// dedicated call below — never part of the live module scan.
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(raw), "verify-outcomes") {
			rel, relerr := filepath.Rel(moduleRoot, p)
			if relerr != nil {
				return relerr
			}
			hits = append(hits, filepath.ToSlash(rel))
		}
		return nil
	})
	return hits, err
}

func TestVerifyOutcomesSingleReader(t *testing.T) {
	// internal/deskkit -> tools/desk (the module root).
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	hits, err := verifyOutcomesChokePointScan(moduleRoot, moduleRoot)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	hitSet := map[string]bool{}
	for _, h := range hits {
		hitSet[h] = true
		if _, ok := verifyOutcomesChokePointAllowList[h]; !ok {
			t.Fatalf("%s contains the literal \"verify-outcomes\" and is NOT on the allow-list — "+
				"a new direct reader/writer (or a missing allow-list entry)", h)
		}
	}
	for rel := range verifyOutcomesChokePointAllowList {
		if !hitSet[rel] {
			t.Fatalf("allow-list entry %s no longer contains the literal \"verify-outcomes\" — remove the stale entry", rel)
		}
	}

	// Positive control (clause 14): the committed fixture under testdata/ plants ONE file,
	// outside the allow-list, that contains the literal. A guard whose matcher went blind
	// would report the fixture clean too — this proves it does not.
	fixtureRoot, err := filepath.Abs("testdata/chokepoint_fixture")
	if err != nil {
		t.Fatal(err)
	}
	fixtureHits, err := verifyOutcomesChokePointScan(fixtureRoot, fixtureRoot)
	if err != nil {
		t.Fatalf("fixture scan: %v", err)
	}
	if len(fixtureHits) == 0 {
		t.Fatal("positive control: the committed fixture (testdata/chokepoint_fixture) was not flagged — the matcher is not live")
	}
	for _, h := range fixtureHits {
		if _, ok := verifyOutcomesChokePointAllowList[h]; ok {
			t.Fatalf("positive-control fixture path %s unexpectedly matches the allow-list", h)
		}
	}
}
