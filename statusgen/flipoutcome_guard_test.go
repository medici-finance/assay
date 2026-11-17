package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// flipoutcome_guard_test.go — one test per fail-closed guard of verifyflip's
// verify-outcome cross-check that has no other test: each builds a tree the
// guard alone stops, so a refactor that drops the guard turns the run into a
// flip and the test goes red. flipoutcome-mutations.json carries one mutant
// per guard.

// TestVflipOutcomeFuture: a record dated beyond the clock-skew tolerance never
// wins the reduction, so the latest record is unknown: could-not-check.
// regression: #6734
func TestVflipOutcomeFuture(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	vfOutcome(t, root, "blocked", vfSHA, "2099-01-01T00:00:00Z")
	vfCommit(t, root, vfMailVerifier)
	vfCouldNotCheck(t, root, readme, "vf/01", "beyond the clock-skew tolerance")
}

// TestVflipOutcomeNoAdder: a deciding record that only a merge commit
// introduces has no adding commit, so its author cannot be judged:
// could-not-check.
// regression: #6734
func TestVflipOutcomeNoAdder(t *testing.T) {
	o := vfDefaults()
	o.outcome = "blocked"
	root, readme := vfFixture(t, o)
	runGitEnv(t, root, nil, "checkout", "-q", "-b", "side")
	vfOther(t, root, "side.txt", vfMailWorker)
	runGitEnv(t, root, nil, "checkout", "-q", "main")
	vfOther(t, root, "main.txt", vfMailVerifier)
	vfGit(t, root, vfMailWorker, "merge", "-q", "--no-ff", "--no-commit", "side")
	vfOutcome(t, root, "verified", vfSHA, "2026-08-14T12:00:00Z")
	vfGit(t, root, vfMailWorker, "add", "-A")
	vfGit(t, root, vfMailWorker, "commit", "-q", "--no-edit")
	vfCouldNotCheck(t, root, readme, "vf/01", "no commit adds")
}

// TestVflipOutcomeLegacyDecides: a deciding legacy log line has no adding
// commit of its own to judge: could-not-check.
// regression: #6734
func TestVflipOutcomeLegacyDecides(t *testing.T) {
	o := vfDefaults()
	o.outcome = "blocked"
	root, readme := vfFixture(t, o)
	vfWriteAt(t, root, "docs/streams/verify-outcomes.jsonl",
		[]byte(`{"brief":"vf/01","ts":"2026-08-14T12:00:00Z","outcome":"verified","sha":"0000000000aa","source_sha":"`+vfSHA+`"}`+"\n"))
	vfCommit(t, root, vfMailWorker)
	vfCouldNotCheck(t, root, readme, "vf/01", "is a legacy log line")
}

// TestVflipOutcomeDeclined: a repository the typed reader declines (here an
// include in the repo config) is could-not-check for the record checks, with
// no native retry.
// regression: #6734
func TestVflipOutcomeDeclined(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	vfWriteAt(t, root, "extra.cfg", []byte("[example]\n\tkey = value\n"))
	vfCommit(t, root, vfMailVerifier)
	runGitEnv(t, root, nil, "config", "include.path", "../extra.cfg")
	vfCouldNotCheck(t, root, readme, "vf/01", "typed reader declines")
}

// TestVflipOutcomeWalkError: a history the adding-commit walk cannot read in
// full (an older tree object missing) is could-not-check, never an empty or
// partial set of adders.
// regression: #6734
func TestVflipOutcomeWalkError(t *testing.T) {
	o := vfDefaults()
	o.outcome = "blocked"
	root, readme := vfFixture(t, o)
	vfOutcomeCommitted(t, root, "verified", vfSHA, "2026-08-14T12:00:00Z")
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD~1:docs/streams/verify-outcomes/vf").Output()
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(string(out))
	obj := filepath.Join(root, ".git", "objects", id[:2], id[2:])
	if err := os.Remove(obj); err != nil {
		t.Fatalf("remove loose tree %s: %v", id, err)
	}
	vfCouldNotCheck(t, root, readme, "vf/01", "outcome record: git log")
}

// TestVflipOutcomeHeadBytes: a tracked record file whose bytes differ from
// HEAD refuses even when the typed status read reports it clean.
// regression: #6734
func TestVflipOutcomeHeadBytes(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	rel := "docs/streams/verify-outcomes.jsonl"
	line := `{"brief":"vf/01","ts":"2026-08-14T12:00:00Z","outcome":"blocked","sha":"0000000000aa","source_sha":"` + vfSHA + `"}` + "\n"
	vfWriteAt(t, root, rel, []byte(line))
	p := filepath.Join(root, filepath.FromSlash(rel))
	past := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(p, past, past); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailVerifier)
	if err := os.WriteFile(p, []byte(strings.Replace(line, `"vf/01"`, `"vf/02"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, past, past); err != nil {
		t.Fatal(err)
	}
	vfRefusesAt(t, root, readme, "differs from HEAD")
}
