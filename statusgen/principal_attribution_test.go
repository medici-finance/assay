package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadPrincipalFixture copies a multi-principal/01 testdata tree into a temp root and
// loads its streams, mirroring attrProblems' pattern (attribution_test.go).
func loadPrincipalFixture(t *testing.T, name string) []*Stream {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", name))); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

// TestPrincipalAttributionNoPrincipalIsProblem is multi-principal/01 Verify row 3: an
// App-authored Evidence row with no on-behalf-of annotation, dated AT the cutover, is a
// PROBLEM (the fixture's Date cell is principalAttributionCutoverDate itself).
func TestPrincipalAttributionNoPrincipalIsProblem(t *testing.T) {
	streams := loadPrincipalFixture(t, "evidence-no-principal")
	problems, _ := principalAttributionProblems(streams)
	if !hasProblem(problems, "mp/brief-01", "on-behalf-of principal") {
		t.Fatalf("want a no-principal problem on mp/brief-01; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestPrincipalAttributionNoPrincipalPreCutoverIsNotice: the identical missing-
// annotation shape, but dated BEFORE principalAttributionCutoverDate — the write path
// could not have stamped this row, so it is a NOTICE (grandfathered), never a PROBLEM.
// This is the cutover-boundary reviewers on both lanes asked for: retroactive history
// must not condemn a repo the moment the check ships.
func TestPrincipalAttributionNoPrincipalPreCutoverIsNotice(t *testing.T) {
	streams := loadPrincipalFixture(t, "evidence-no-principal")
	path := filepath.Join(streams[0].Dir, "brief-01-no-principal.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(b), "| 2026-09-17 | assay-verifier-app[bot] |",
		"| 2026-01-01 | assay-verifier-app[bot] |", 1)
	if patched == string(b) {
		t.Fatal("fixture text did not match the expected Date/Runner cells — fixture drifted")
	}
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	streams2, _, err := loadStreams(streams[0].Root)
	if err != nil {
		t.Fatal(err)
	}
	problems, notices := principalAttributionProblems(streams2)
	if hasProblem(problems, "mp/brief-01") {
		t.Fatalf("a pre-cutover missing-annotation row must NOT be a hard problem; got:\n%s", strings.Join(problems, "\n"))
	}
	if !hasProblem(notices, "mp/brief-01", "NOTICE") {
		t.Fatalf("a pre-cutover missing-annotation row must still surface as a NOTICE; got:\n%s", strings.Join(notices, "\n"))
	}
}

// TestPrincipalAttributionNoPrincipalClearsWithTrailer is row 3's perturbation: adding
// the on-behalf-of annotation (naming a login the roster's human map recognises) clears
// the problem.
func TestPrincipalAttributionNoPrincipalClearsWithTrailer(t *testing.T) {
	scanWithRoster(t, scanExampleRoster()) // blesses "ada" (scanExampleRoster)
	streams := loadPrincipalFixture(t, "evidence-no-principal")
	path := filepath.Join(streams[0].Dir, "brief-01-no-principal.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(b), "assay-verifier-app[bot] |",
		"assay-verifier-app[bot] @ abc1234 (on-behalf-of human:ada) |", 1)
	if patched == string(b) {
		t.Fatal("fixture text did not match the expected Runner cell — fixture drifted")
	}
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	streams2, _, err := loadStreams(streams[0].Root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := principalAttributionProblems(streams2)
	if hasProblem(problems, "mp/brief-01") {
		t.Fatalf("adding a recognised on-behalf-of trailer should clear the problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestPrincipalAttributionUnknownPrincipalIsProblem is multi-principal/01 Verify row 4:
// an on-behalf-of annotation naming a login NOT in the roster's human map is a PROBLEM.
func TestPrincipalAttributionUnknownPrincipalIsProblem(t *testing.T) {
	scanWithRoster(t, scanExampleRoster()) // human map has "ada", not "ghost"
	streams := loadPrincipalFixture(t, "evidence-unknown-principal")
	problems, _ := principalAttributionProblems(streams)
	if !hasProblem(problems, "mp/brief-01", "not in this repo's roster human map") {
		t.Fatalf("want an unknown-principal problem on mp/brief-01; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestPrincipalAttributionUnknownPrincipalSilentWithoutRoster: with NO roster configured
// at all, this check has no human map to validate against and says nothing (statusgen's
// fail-closed rule for THIS check is silence, not a manufactured problem — a repo that
// has not adopted the roster at all gets no signal from a check it cannot answer).
func TestPrincipalAttributionUnknownPrincipalSilentWithoutRoster(t *testing.T) {
	scanWithNoRoster(t)
	streams := loadPrincipalFixture(t, "evidence-unknown-principal")
	problems, _ := principalAttributionProblems(streams)
	if hasProblem(problems, "mp/brief-01") {
		t.Fatalf("an unconfigured roster should not itself manufacture a problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestPrincipalAttributionHumanRunnerNeverChecked: a human-run Evidence row needs no
// on-behalf-of annotation at all and is never flagged, regardless of roster state.
func TestPrincipalAttributionHumanRunnerNeverChecked(t *testing.T) {
	scanWithRoster(t, scanExampleRoster())
	streams := loadPrincipalFixture(t, "evidence-no-principal")
	path := filepath.Join(streams[0].Dir, "brief-01-no-principal.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(b), "assay-verifier-app[bot] |", "human:ada |", 1)
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	streams2, _, err := loadStreams(streams[0].Root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := principalAttributionProblems(streams2)
	if hasProblem(problems, "mp/brief-01") {
		t.Fatalf("a human runner should never be checked for an on-behalf-of annotation; got:\n%s", strings.Join(problems, "\n"))
	}
}
