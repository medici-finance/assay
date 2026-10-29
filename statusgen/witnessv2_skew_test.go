package main

// witnessv2_skew_test.go — statusgen/36 Verify rows 5 and 6, both against a
// REAL prior release binary ($SG_OLD_BIN, built by the Verify command from a
// release tag; in CI fetch-baseline.sh supplies the same tag's published
// binary, checksum-verified, or on the release runner builds it from source at
// the tag's pinned commit), never the changed code and never a golden file the
// changed code produced.
//
// Row 5 (version skew): a v2 witness the new reader releases is REFUSED by the
// previous release, because a v2 cell's bracketed marker does not resolve as
// a revision for a reader that predates v2.
//
// Row 6 (v1 unchanged): over a frozen v1 corpus kept as a fixture repository
// with its history, every v1 claim's verdict and reason from the new binary
// equal the baseline's.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// oldStatusgenBin returns $SG_OLD_BIN. An unset variable FAILS the test in
// every environment, CI included: a skip would let the skew guard disappear
// silently. CI exports SG_OLD_BIN from the pinned baseline via
// statusgen/fetch-baseline.sh: the release binary checked against that
// release's checksums.txt, or (release runner) built from source at the tag's
// pinned commit.
func oldStatusgenBin(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("SG_OLD_BIN")
	if bin == "" {
		t.Fatal("SG_OLD_BIN is unset: point SG_OLD_BIN at the previous release binary (statusgen/fetch-baseline.sh fetches it; this test never skips)")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("SG_OLD_BIN %q: %v", bin, err)
	}
	return bin
}

// witnessRegisterREADME is the witnesses register's README, relative to the
// repository root.
const witnessRegisterREADME = "docs/streams/witnesses/README.md"

// skewClaim is the slice of a claim both binaries share.
type skewClaim struct {
	Claim    string `json:"claim"`
	Kind     string `json:"kind"`
	Result   string `json:"result"`
	Revision string `json:"revision"`
	Released bool   `json:"released"`
	Reason   string `json:"reason"`
}

type skewBrief struct {
	Brief    string      `json:"brief"`
	Released bool        `json:"released"`
	Claims   []skewClaim `json:"claims"`
}

// coverageVia runs `--coverage --json` and returns the rows; a non-zero exit
// returns the error and its stderr tail instead (a reader that cannot read the
// tree refuses it as a whole).
func coverageVia(t *testing.T, bin, root string) []skewBrief {
	t.Helper()
	rows, err := coverageViaErr(t, bin, root)
	if err != nil {
		t.Fatalf("%s --coverage --json: %v", bin, err)
	}
	return rows
}

func coverageViaErr(t *testing.T, bin, root string) ([]skewBrief, error) {
	t.Helper()
	cmd := exec.Command(bin, "--root", root, "--coverage", "--json")
	out, err := cmd.Output()
	if err != nil {
		tail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			lines := strings.Split(strings.TrimSpace(string(ee.Stderr)), "\n")
			tail = lines[len(lines)-1]
		}
		return nil, fmt.Errorf("%v: %s", err, tail)
	}
	var rows []skewBrief
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatalf("decode %s output: %v\n%s", bin, err, out)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Brief < rows[j].Brief })
	return rows, nil
}

// TestWitnessSkewOldReader: the default v2 fixture releases under the new
// binary and is refused under the previous release.
// regression: #2026
func TestWitnessSkewOldReader(t *testing.T) {
	old := oldStatusgenBin(t)
	cur := buildStatusgenBinary(t)
	f := newGitV2Fixture(t)
	// The register README this repository ships: its self-declaration is what
	// lets a reader that predates the reserved `witnesses` name read the tree.
	readme, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(witnessRegisterREADME)))
	if err != nil {
		t.Fatalf("the shipped register README: %v", err)
	}
	f.ExtraFiles = map[string]string{witnessRegisterREADME: string(readme)}
	f.build()

	newRows := coverageVia(t, cur, f.Root)
	oldRows, oldErr := coverageViaErr(t, old, f.Root)

	released := func(rows []skewBrief) (n, total int) {
		for _, b := range rows {
			if b.Brief != gv2BriefID {
				continue
			}
			for _, c := range b.Claims {
				total++
				if c.Released {
					n++
				}
			}
		}
		return
	}
	nNew, totalNew := released(newRows)
	if totalNew == 0 || nNew != totalNew {
		t.Fatalf("non-vacuity: the new binary must release every v2 claim of the fixture, released %d of %d: %+v", nNew, totalNew, newRows)
	}
	if oldErr != nil {
		// A reader that cannot read the tree at all proves nothing about a v2
		// cell: the per-claim refusal below is the guard, so this is a failure.
		t.Fatalf("the previous release must read the fixture tree and refuse each v2 claim, but it exits non-zero: %v", oldErr)
	}
	nOld, totalOld := released(oldRows)
	if totalOld == 0 {
		t.Fatalf("the previous release produced no claims for the fixture: %+v", oldRows)
	}
	t.Logf("the previous release read the tree and released %d of %d v2 claims", nOld, totalOld)
	if nOld != 0 {
		t.Errorf("version skew: the previous release released %d of %d v2 claims; it must refuse every one: %+v", nOld, totalOld, oldRows)
	}
	for _, b := range oldRows {
		if b.Brief == gv2BriefID && b.Released {
			t.Errorf("version skew: the previous release released the brief %s on a v2 witness", b.Brief)
		}
	}
}

// corpusDir is the frozen v1 corpus: ordered step directories, applied as
// commits (the base, a feature branch writing the Evidence, a no-ff merge,
// then edits after the run).
const corpusDir = "testdata/witnessv2/v1corpus/steps"

func copyCorpusStep(t *testing.T, step, root, base string) {
	t.Helper()
	dir := filepath.Join(corpusDir, step)
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		gv2Write(t, root, filepath.ToSlash(rel), strings.ReplaceAll(string(b), "{{BASE}}", base))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// materializeV1Corpus builds the corpus repository and returns its root.
func materializeV1Corpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gv2Git(t, root, "init", "-q", "-b", "main")
	gv2Git(t, root, "config", "commit.gpgsign", "false")
	copyCorpusStep(t, "01-base", root, "")
	gv2Git(t, root, "add", "-A")
	gv2Git(t, root, "commit", "-q", "-m", "corpus: base")
	base := gv2Git(t, root, "rev-parse", "HEAD")

	gv2Git(t, root, "checkout", "-q", "-b", "corpus/branch")
	copyCorpusStep(t, "02-evidence", root, base)
	gv2Git(t, root, "add", "-A")
	gv2GitAs(t, root, fixtureVerifierName, fixtureVerifierEmail, "commit", "-q", "-m", "Evidence: witness rows")
	gv2Git(t, root, "checkout", "-q", "main")
	gv2Git(t, root, "merge", "-q", "--no-ff", "-m", "Merge pull request #7 from example-org/corpus-branch", "corpus/branch")

	copyCorpusStep(t, "03-after", root, base)
	gv2Git(t, root, "add", "-A")
	gv2Git(t, root, "commit", "-q", "-m", "corpus: edits after the run")
	return root
}

// TestWitnessV1Unchanged: over the frozen v1 corpus the new binary's verdict
// and reason for every claim equal the baseline release's.
// regression: #2026
func TestWitnessV1Unchanged(t *testing.T) {
	old := oldStatusgenBin(t)
	cur := buildStatusgenBinary(t)
	root := materializeV1Corpus(t)

	oldRows := coverageVia(t, old, root)
	newRows := coverageVia(t, cur, root)

	nReleased, nRefused, nClaims := 0, 0, 0
	for _, b := range oldRows {
		for _, c := range b.Claims {
			nClaims++
			if c.Released {
				nReleased++
			} else if c.Result != "" {
				nRefused++
			}
		}
	}
	if nReleased < 1 || nRefused < 1 {
		t.Fatalf("non-vacuity: the baseline must release at least one claim and refuse at least one, released %d refused %d of %d: %+v", nReleased, nRefused, nClaims, oldRows)
	}
	if len(oldRows) != len(newRows) {
		t.Fatalf("brief count: baseline %d, new %d", len(oldRows), len(newRows))
	}
	for i := range oldRows {
		o, n := oldRows[i], newRows[i]
		if o.Brief != n.Brief || o.Released != n.Released || len(o.Claims) != len(n.Claims) {
			t.Errorf("brief %s: baseline released=%v claims=%d, new %s released=%v claims=%d", o.Brief, o.Released, len(o.Claims), n.Brief, n.Released, len(n.Claims))
			continue
		}
		for j := range o.Claims {
			if o.Claims[j] != n.Claims[j] {
				t.Errorf("%s claim %q changed:\n  baseline: %+v\n  new:      %+v", o.Brief, o.Claims[j].Claim, o.Claims[j], n.Claims[j])
			}
		}
	}
}
