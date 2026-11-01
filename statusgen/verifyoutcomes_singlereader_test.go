package main

// verifyoutcomes_singlereader_test.go — Verify row 5: the #882 choke-point guard for THIS
// module (statusgen is a separate Go module from tools/desk, so it carries its own copy of
// this guard, exactly the way it carries its own copy of the record-reading rules). One
// structural test: walk every non-test .go file recursively under statusgen/ and fail naming any
// file OTHER than the allow-listed reader/writer set that contains the literal
// "verify-outcomes" — the ALLOW-LIST structural-test model clause 14 establishes elsewhere in
// this codebase, with a committed positive-control fixture so a broken matcher fails loud
// rather than reporting a false clean.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifyOutcomesChokePointAllowList is the ALLOW-LIST of every non-test .go file recursively under
// statusgen/ that may contain the literal "verify-outcomes" — the reader/name choke point
// itself, the migration writer (`statusgen outcomes split`), and every file that merely
// mentions the term in prose (never opens the path directly). Adding a new direct reader/writer
// means adding its line here, with a reason; a stale entry is ALSO a failure (see the two-way
// check in the test below).
var verifyOutcomesChokePointAllowList = map[string]string{
	"outcomessplit.go":           "the migration writer (statusgen outcomes split) — writes record files directly, by design",
	"verifyoutcomes.go":          "the reader/name choke point itself, plus the pre-existing legacy shard-glob union reader",
	"verifyclosure.go":           "prose describing what deskevidence's verified-outcome sidecar gate is for — no file I/O",
	"briefdoc/registers.go":      "the shared reserved-register name declaration in briefdoc (a string key), moved from load.go — no file I/O",
	"evaluation/coverage_api.go": "statusgen/evaluation: the bookkeeping-path pattern the pure coverage evaluator exempts from stamping — a string constant, no file I/O (the evaluator opens nothing)",
	"main.go":                    "prose describing the `outcomes` subcommand dispatch and the verifyclosure gate — no file I/O",
}

// verifyOutcomesChokePointScan walks walkRoot (non-recursive into "testdata") for non-test .go
// files containing the literal "verify-outcomes", returning module-relative slash paths. A basename exemption must never cover
// another file in the module with the same name.
func verifyOutcomesChokePointScan(walkRoot string) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(walkRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
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
			rel, err := filepath.Rel(walkRoot, p)
			if err != nil {
				return err
			}
			hits = append(hits, filepath.ToSlash(rel))
		}
		return nil
	})
	return hits, err
}

func TestVerifyOutcomesSingleReader(t *testing.T) {
	moduleRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	hits, err := verifyOutcomesChokePointScan(moduleRoot)
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
	for name := range verifyOutcomesChokePointAllowList {
		if !hitSet[name] {
			t.Fatalf("allow-list entry %s no longer contains the literal \"verify-outcomes\" — remove the stale entry", name)
		}
	}

	// Positive control (clause 14): the committed fixture under testdata/ plants ONE file,
	// outside the allow-list, that contains the literal.
	fixtureRoot, err := filepath.Abs("testdata/chokepoint_fixture")
	if err != nil {
		t.Fatal(err)
	}
	fixtureHits, err := verifyOutcomesChokePointScan(fixtureRoot)
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

// regression: #4947
func TestVerifyOutcomesSingleReaderPaths(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"briefdoc/registers.go", "registers.go", "other/registers.go"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("package probe\n// verify-outcomes\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	hits, err := verifyOutcomesChokePointScan(root)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, h := range hits {
		found[h] = true
	}
	for _, name := range []string{"briefdoc/registers.go", "registers.go", "other/registers.go"} {
		if !found[name] {
			t.Errorf("scanner lost qualified path %s: %v", name, hits)
		}
	}
	for _, name := range []string{"registers.go", "other/registers.go"} {
		if _, ok := verifyOutcomesChokePointAllowList[name]; ok {
			t.Errorf("unrelated %s inherits the moved declaration exemption", name)
		}
	}
}
