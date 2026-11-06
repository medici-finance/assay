package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fail-first on a cross-repository (version 2 receipt) run. The rows run in
// the deliverable home; the brief lives in the tracking checkout, so no
// deliverable commit holds it. The base is a deliverable commit and the
// receipt binds the brief's bytes.

const crossFFBody = "# Fixture\n\n## Verify\n\n| # | Command | Expect | Class |\n|---|---|---|---|\n| 1 | `test -f delivered.txt` | risk-bearing exit 0 | check:ci |\n\n## Evidence\n\n## Review\n"

// crossFFFixture is a cross-repository fixture whose deliverable home has a
// base commit without delivered.txt and a head commit that adds it, with
// origin/main at head (the post-merge verify shape). It returns the fixture,
// the tracking root, the base and the head.
func crossFFFixture(t *testing.T) (containerFixture, string, string, string) {
	t.Helper()
	fx, tracking := newCrossRepoFixture(t)
	putFile(t, fx.brief, crossFFBody)
	base := gitOutT(t, fx.home, "rev-parse", "HEAD")
	putFile(t, filepath.Join(fx.home, "delivered.txt"), "delivered\n")
	runGit(t, fx.home, "add", "delivered.txt")
	runGit(t, fx.home, "commit", "-m", "delivery")
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	runGit(t, fx.home, "update-ref", remoteMainRef, head)
	return fx, tracking, base, head
}

// regression: #5544
// TestCrossFFBaseRev: the cross-repository base check keeps the ancestry rule
// (a branch commit and an unknown commit are refused) but not the brief-held
// rule, which no deliverable commit can meet. The same-repository check still
// refuses the same base for the same outside brief, so the brief-held rule is
// skipped only where the caller chose the cross-repository form.
func TestCrossFFBaseRev(t *testing.T) {
	fx, _, base, head := crossFFFixture(t)
	if got, err := failFirstCrossRepoBaseRev(fx.home, base); err != nil || got != base {
		t.Fatalf("cross-repository base refused: %q %v", got, err)
	}
	if _, err := failFirstBaseRev(fx.home, base, fx.brief); err == nil || !strings.Contains(err.Error(), "cannot be shown to hold the brief") {
		t.Fatalf("same-repository check accepted a base for an outside brief: %v", err)
	}
	putFile(t, filepath.Join(fx.home, "branch.txt"), "branch\n")
	runGit(t, fx.home, "add", "branch.txt")
	runGit(t, fx.home, "commit", "-m", "branch work")
	branch := gitOutT(t, fx.home, "rev-parse", "HEAD")
	if _, err := failFirstCrossRepoBaseRev(fx.home, branch); err == nil {
		t.Fatal("cross-repository check accepted a branch commit as base")
	}
	if _, err := failFirstCrossRepoBaseRev(fx.home, strings.Repeat("c", 40)); err == nil || !strings.Contains(err.Error(), "names no commit") {
		t.Fatalf("cross-repository check accepted an unknown commit: %v", err)
	}
	if got, err := failFirstCrossRepoBaseRev(fx.home, ""); err != nil || got != head {
		t.Fatalf("default cross-repository base: %q %v", got, err)
	}
}

// regression: #5544
// TestCrossFFHostLane: an admitted version 2 run on the host takes a
// deliverable base and records a red there and a pass at head.
func TestCrossFFHostLane(t *testing.T) {
	fx, tracking, base, head := crossFFFixture(t)
	t.Setenv("DESK_LOOP", "verify-desk")
	// No check:ci class: the host lane runs the row without a network sandbox.
	putFile(t, fx.brief, "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f delivered.txt` | risk-bearing exit 0 |\n\n## Evidence\n\n## Review\n")
	b := crossBinding(t, fx)
	b.Source = head
	putFile(t, fx.check, crossCheck(t, b, tracking, fx.home))
	res, stderr := captureVerifyrunOut(t, []string{"--brief", fx.brief, "--root", fx.home, "--fail-first", "--base", base})
	if res.code != 0 {
		t.Fatalf("host cross-repository fail-first: %d %s", res.code, stderr)
	}
	if got := getFile(t, fx.brief); !strings.Contains(got, "base="+base[:treeSHALen]+" red rc=1") {
		t.Fatalf("no baseline red at the deliverable base: %s", got)
	}
}

// regression: #5544
// TestCrossFFContainer: the container lane imports history for a brief bound
// by a version 2 receipt, runs the base rows at the deliverable base, and the
// host closes the bracket.
func TestCrossFFContainer(t *testing.T) {
	fx, tracking, base, head := crossFFFixture(t)
	t.Setenv("DESK_LOOP", "")
	clock := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	savedNow := nowFunc
	nowFunc = func() time.Time { return clock }
	t.Cleanup(func() { nowFunc = savedNow })
	b := crossBinding(t, fx)
	b.Source = head
	putFile(t, fx.check, crossCheck(t, b, tracking, fx.home))
	bundle := filepath.Join(t.TempDir(), "history.pack")
	res, stderr := captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt, "--history-out", bundle})
	if res.code != 0 {
		t.Fatalf("open: %d %s", res.code, stderr)
	}
	gitdir := filepath.Join(t.TempDir(), "host.git")
	if err := os.Rename(filepath.Join(fx.home, ".git"), gitdir); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(fx.home, ".git"), "gitdir: /unmounted-host/git/worktrees/verifier\n")
	res, stderr = captureVerifyrun(t, append(fx.runArgs(), "--fail-first", "--base", base, "--history-pack", bundle))
	if res.code != 0 {
		t.Fatalf("container cross-repository fail-first: %d %s", res.code, stderr)
	}
	if w := getFile(t, fx.witness); !strings.Contains(w, "base="+base[:treeSHALen]+" red rc=1") || !strings.Contains(w, "pass exit=0 sandbox=container-netns") {
		t.Fatalf("missing head pass / baseline red: %s", w)
	}
	if err := os.Remove(filepath.Join(fx.home, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(gitdir, filepath.Join(fx.home, ".git")); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	res, stderr = captureVerifyrun(t, []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness})
	if res.code != 0 {
		t.Fatalf("close: %d %s", res.code, stderr)
	}
}

// regression: #5544
// TestCrossHistoryRefusesInner: the cross-repository import refuses a brief
// inside the deliverable home, and the same-repository import refuses one
// outside it; neither form stands in for the other.
func TestCrossHistoryRefusesInner(t *testing.T) {
	fx, _, _, head := crossFFFixture(t)
	bundle := filepath.Join(t.TempDir(), "history.pack")
	if err := exportContainerHistory(fx.home, bundle, head); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(fx.home, "docs", "brief.md")
	if _, cleanup, err := importCrossRepoHistory(fx.home, inner, bundle, head); err == nil {
		cleanup()
		t.Fatal("cross-repository import accepted a brief inside the home")
	}
	if _, _, cleanup, err := importContainerHistory(fx.home, fx.brief, bundle, head); err == nil {
		cleanup()
		t.Fatal("same-repository import accepted a brief outside the home")
	}
	repo, cleanup, err := importCrossRepoHistory(fx.home, fx.brief, bundle, head)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got, err := failFirstCrossRepoBaseRev(repo, head); err != nil || got != head {
		t.Fatalf("imported history does not resolve the admitted source: %q %v", got, err)
	}
}
