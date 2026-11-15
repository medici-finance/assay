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

// crossFFFixture is a cross-repository fixture whose deliverable home has an
// older commit, a base commit without delivered.txt, and a head commit that
// adds it, with origin/main at base: the delivering change is on the branch,
// so the deliverable merge-base of HEAD and origin/main is base. It returns
// the fixture, the tracking root, the older commit, the base and the head.
func crossFFFixture(t *testing.T) (containerFixture, string, string, string, string) {
	t.Helper()
	fx, tracking := newCrossRepoFixture(t)
	putFile(t, fx.brief, crossFFBody)
	older := gitOutT(t, fx.home, "rev-parse", "HEAD")
	putFile(t, filepath.Join(fx.home, "pre.txt"), "pre\n")
	runGit(t, fx.home, "add", "pre.txt")
	runGit(t, fx.home, "commit", "-m", "pre-delivery")
	base := gitOutT(t, fx.home, "rev-parse", "HEAD")
	putFile(t, filepath.Join(fx.home, "delivered.txt"), "delivered\n")
	runGit(t, fx.home, "add", "delivered.txt")
	runGit(t, fx.home, "commit", "-m", "delivery")
	head := gitOutT(t, fx.home, "rev-parse", "HEAD")
	runGit(t, fx.home, "update-ref", remoteMainRef, base)
	return fx, tracking, older, base, head
}

// regression: #5544
// TestCrossFFBaseRev: a cross-repository base must be the deliverable
// merge-base exactly (ruling A on #5544). An older ancestor, a later
// (delivering) commit and an unknown commit are refused, and an unresolvable
// merge-base refuses with or without --base. The same-repository check still
// applies its own brief-held rule to an outside brief (an ancestor of the
// merge-base is refused there for that reason), so the cross-repository rule
// applies only where the caller chose the cross-repository form.
func TestCrossFFBaseRev(t *testing.T) {
	fx, _, older, base, head := crossFFFixture(t)
	if got, err := failFirstCrossRepoBaseRev(fx.home, base); err != nil || got != base {
		t.Fatalf("cross-repository merge-base refused: %q %v", got, err)
	}
	if got, err := failFirstCrossRepoBaseRev(fx.home, ""); err != nil || got != base {
		t.Fatalf("default cross-repository base: %q %v", got, err)
	}
	if _, err := failFirstBaseRev(fx.home, older, fx.brief); err == nil || !strings.Contains(err.Error(), "cannot be shown to hold the brief") {
		t.Fatalf("same-repository check accepted a base for an outside brief: %v", err)
	}
	for name, rev := range map[string]string{"older ancestor": older, "delivering commit": head} {
		if _, err := failFirstCrossRepoBaseRev(fx.home, rev); err == nil || !strings.Contains(err.Error(), "merge-base "+base+" exactly") {
			t.Fatalf("cross-repository check accepted the %s as base: %v", name, err)
		}
	}
	if _, err := failFirstCrossRepoBaseRev(fx.home, strings.Repeat("c", 40)); err == nil || !strings.Contains(err.Error(), "names no commit") {
		t.Fatalf("cross-repository check accepted an unknown commit: %v", err)
	}
	if _, err := failFirstCrossRepoBaseRev(fx.home, "-x"); err == nil {
		t.Fatal("cross-repository check accepted an option as base")
	}
	runGit(t, fx.home, "update-ref", remoteMainRef, head)
	for _, rev := range []string{"", head} {
		if _, err := failFirstCrossRepoBaseRev(fx.home, rev); err == nil || !strings.Contains(err.Error(), "HEAD itself") {
			t.Fatalf("cross-repository check accepted --base %q when the merge-base is HEAD: %v", rev, err)
		}
	}
	runGit(t, fx.home, "update-ref", "-d", remoteMainRef)
	for _, rev := range []string{"", base} {
		if _, err := failFirstCrossRepoBaseRev(fx.home, rev); err == nil || !strings.Contains(err.Error(), "cannot be resolved") {
			t.Fatalf("cross-repository check with no merge-base accepted --base %q: %v", rev, err)
		}
	}
	// The same-repository check, reached by an unadmitted run with this
	// outside brief, fails closed too: no --base is accepted unchecked.
	for _, rev := range []string{older, base, head} {
		if _, err := failFirstBaseRev(fx.home, rev, fx.brief); err == nil || !strings.Contains(err.Error(), "cannot be resolved") {
			t.Fatalf("same-repository check with no merge-base accepted --base %s: %v", rev, err)
		}
	}
}

// regression: #5544
// TestCrossFFPostMerge: once origin/main holds the change the deliverable
// merge-base is HEAD, and the run refuses (exit 2) before any row runs, with
// or without --base. Before, a delivering row green on the changed tree was
// reported as a defect (exit 1), and a red made only by an ignored home file
// was credited (exit 0).
func TestCrossFFPostMerge(t *testing.T) {
	for _, row := range []string{"delivered", "ignored"} {
		for _, flag := range []string{"", "head"} {
			fx, tracking, _, _, head := crossFFFixture(t)
			t.Setenv("DESK_LOOP", "verify-desk")
			body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f " + row + ".txt` | risk-bearing exit 0 |\n\n## Evidence\n\n## Review\n"
			putFile(t, fx.brief, body)
			putFile(t, filepath.Join(fx.home, ".git", "info", "exclude"), "ignored.txt\n")
			putFile(t, filepath.Join(fx.home, "ignored.txt"), "home only\n")
			runGit(t, fx.home, "update-ref", remoteMainRef, head)
			b := crossBinding(t, fx)
			b.Source = head
			putFile(t, fx.check, crossCheck(t, b, tracking, fx.home))
			args := []string{"--brief", fx.brief, "--root", fx.home, "--fail-first"}
			if flag != "" {
				args = append(args, "--base", head)
			}
			res, stderr := captureVerifyrunOut(t, args)
			if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "HEAD itself") {
				t.Fatalf("%s row, --base %q, post-merge: %d %s", row, flag, res.code, stderr)
			}
			if got := getFile(t, fx.brief); got != body {
				t.Fatalf("%s row, --base %q: a row ran before the refusal:\n%s", row, flag, got)
			}
		}
	}
}

// regression: #5544
// TestCrossFFHostLane: an admitted version 2 run on the host takes the
// deliverable merge-base as its base and records a red there and a pass at
// head.
func TestCrossFFHostLane(t *testing.T) {
	fx, tracking, _, base, head := crossFFFixture(t)
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
// by a version 2 receipt (pre-fix: refused as not home-relative). The imported
// history anchors origin/main to the admitted source, so the deliverable
// merge-base there is the source itself, which is HEAD, so the run refuses
// before any row with or without --base (the commit before the delivery
// included).
func TestCrossFFContainer(t *testing.T) {
	fx, tracking, _, base, head := crossFFFixture(t)
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
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(fx.home, ".git"))
		_ = os.Rename(gitdir, filepath.Join(fx.home, ".git"))
	})
	putFile(t, filepath.Join(fx.home, ".git"), "gitdir: /unmounted-host/git/worktrees/verifier\n")
	for _, extra := range [][]string{{"--base", base}, nil} {
		args := append(append(fx.runArgs(), "--fail-first", "--history-pack", bundle), extra...)
		res, stderr = captureVerifyrun(t, args)
		if res.code != verifyrunExitCouldNot || strings.Contains(stderr, "home-relative") || !strings.Contains(stderr, "HEAD itself ("+head+")") {
			t.Fatalf("container cross-repository run %v with the merge-base at HEAD: %d %s", extra, res.code, stderr)
		}
	}
}

// regression: #5544
// TestCrossHistoryRefusesInner: the cross-repository import refuses a brief
// inside the deliverable home, and the same-repository import refuses one
// outside it; neither form stands in for the other.
func TestCrossHistoryRefusesInner(t *testing.T) {
	fx, _, _, _, head := crossFFFixture(t)
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
	// The import anchors origin/main to the admitted source, so the merge-base
	// it resolves is that source, which is HEAD: refused, naming it.
	if _, err := failFirstCrossRepoBaseRev(repo, ""); err == nil || !strings.Contains(err.Error(), "HEAD itself ("+head+")") {
		t.Fatalf("imported history: want a refusal naming the admitted source %s: %v", head, err)
	}
}
