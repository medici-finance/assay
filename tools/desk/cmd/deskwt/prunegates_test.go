package main

// Tests for the prune sweep's PERFORMANCE restructure (issue #1037).
//
// The restructure is a performance change whose only possible failure is a wrong REMOVAL,
// so nothing here asserts a duration. Every row asserts BEHAVIOUR: which paths came out of
// a sweep, which gate held a worktree, how many times a gate ran, how many git invocations
// a batch made. A timing assertion would pass on a fast machine whatever the code did.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// hashTree returns a stable digest of every file under root (relative path + bytes), so a
// test can assert a directory is BYTE-IDENTICAL before and after an operation.
func hashTree(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	var names []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		names = append(names, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(names)
	for _, rel := range names {
		b, rerr := os.ReadFile(filepath.Join(root, rel))
		if rerr != nil {
			t.Fatalf("read %s: %v", rel, rerr)
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// advanceMain moves the fixture's origin/main one commit forward, so worktrees created
// before it are BELOW the tip and the fresh-worktree guard does not hold them.
func advanceMain(t *testing.T, work, marker string) {
	t.Helper()
	writeFile(t, filepath.Join(work, marker+".txt"), "mainline "+marker+"\n")
	mustGit(t, work, "add", marker+".txt")
	mustGit(t, work, "commit", "-m", "advance mainline "+marker)
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mustGit(t, work, "rev-parse", "HEAD"))
}

// --- row 2: the removal SET is exactly what it was ---------------------------------
//
// The single-point-of-failure row. A fixture carrying one worktree of every class is
// swept, and the set of paths that came out is compared against an explicit literal — the
// set the gate order produced before the restructure. It compares SETS, never counts: a
// count matches when one worktree was wrongly spared and another wrongly deleted.

func TestPruneRemovalSetUnchanged(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	// One worktree per class the gates distinguish.
	mergedClean := addWorktree(t, "cls-merged-clean")
	dirty := addWorktree(t, "cls-dirty")
	unmerged := addWorktree(t, "cls-unmerged")
	locked := addWorktree(t, "cls-locked")
	freshAtTip := addWorktree(t, "cls-fresh-at-tip")

	// dirty: a tracked modification, and its branch lands on the mainline so ONLY the
	// tracked-clean gate can hold it (see the gate-order note in prune.go).
	writeFile(t, filepath.Join(dirty, "README.md"), "locally modified\n")

	// unmerged: a real commit that never reaches origin/main — an open PR in flight.
	writeFile(t, filepath.Join(unmerged, "feature.txt"), "active work\n")
	mustGit(t, unmerged, "add", "feature.txt")
	mustGit(t, unmerged, "commit", "-m", "unmerged feature work")

	// locked: byte-identical to the fully removable state, so ONLY the lock can hold it.
	mustGit(t, work, "worktree", "lock", "--reason", "claude agent pid 1234", locked)

	// Move the mainline past every worktree created so far, EXCEPT freshAtTip which is
	// re-cut afterwards so its HEAD sits exactly at the new tip.
	advanceMain(t, work, "first")
	mustGit(t, work, "worktree", "remove", "--force", freshAtTip)
	freshAtTip = addWorktree(t, "cls-fresh-at-tip2")
	writeFile(t, filepath.Join(freshAtTip, "untracked-new-source.go"), "package x\n")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}

	// The expected removal set, as an explicit literal: exactly the merged+clean
	// worktree below the tip, and nothing else.
	wantRemoved := map[string]bool{mergedClean: true}
	for _, p := range []string{mergedClean, dirty, unmerged, locked, freshAtTip} {
		_, statErr := os.Stat(p)
		gone := os.IsNotExist(statErr)
		if gone != wantRemoved[p] {
			t.Fatalf("removal set changed: %s gone=%v, want gone=%v; stderr:\n%s", p, gone, wantRemoved[p], errout)
		}
	}
	if !strings.Contains(errout, "removed 1 merged+clean") {
		t.Fatalf("expected exactly one removal in the summary; stderr:\n%s", errout)
	}
}

// --- row 3: the ancestor set fails CLOSED ------------------------------------------
//
// With refs/remotes/origin/main unreadable the sweep can prove NOTHING merged. It must
// hold every candidate and remove nothing — never proceed on an empty set, which would
// read every worktree as unmerged today and, one refactor later, as merged.

func TestPruneHoldsAllNoAncestorSet(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "would-be-removable")
	advanceMain(t, work, "first")

	// Delete the mainline ref the gate is computed against.
	mustGit(t, work, "update-ref", "-d", "refs/remotes/origin/main")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "removed 0 merged+clean") {
		t.Fatalf("a sweep that cannot read the mainline removed something; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "could-not-check") {
		t.Fatalf("expected a sweep-level could-not-check warning; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "unverifiable") {
		t.Fatalf("expected every candidate held as unverifiable; stderr:\n%s", errout)
	}
}

// --- row 4: the merge gate runs BEFORE the tracked-clean gate -----------------------
//
// Asserted on behaviour, not timing: the tracked-clean gate is COUNTED, and a worktree the
// merge gate held must never have had its Status() computed. That is the whole saving the
// reorder buys (~25% of the sweep on the measured repo), and it is invisible to a clock on
// a small fixture.

func TestMergeGateBeforeStatus(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	unmerged := addWorktree(t, "gate-unmerged")
	merged := addWorktree(t, "gate-merged")
	writeFile(t, filepath.Join(unmerged, "feature.txt"), "active work\n")
	mustGit(t, unmerged, "add", "feature.txt")
	mustGit(t, unmerged, "commit", "-m", "unmerged feature work")
	advanceMain(t, work, "first")

	// The sweep iterates RESOLVED roots (/private/var/... on darwin), so the test must key
	// on the resolved form or its counters silently read zero and the row passes blind.
	unmergedRT, mergedRT := resolvePath(unmerged), resolvePath(merged)

	cleanCalls := map[string]int{}
	orig := dirtyTrackedIn
	dirtyTrackedIn = func(sc *sweepCtx, rt string) (string, error) {
		cleanCalls[rt]++
		return orig(sc, rt)
	}
	t.Cleanup(func() { dirtyTrackedIn = orig })

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if cleanCalls[unmergedRT] != 0 {
		t.Fatalf("the tracked-clean gate ran %d time(s) for a worktree the merge gate held — the reorder did not take effect; stderr:\n%s",
			cleanCalls[unmergedRT], errout)
	}
	if cleanCalls[mergedRT] != 1 {
		t.Fatalf("the tracked-clean gate ran %d time(s) for the merged candidate, want exactly 1; stderr:\n%s",
			cleanCalls[mergedRT], errout)
	}
	assertExists(t, unmerged)
	if _, err := os.Stat(merged); !os.IsNotExist(err) {
		t.Fatalf("the merged+clean worktree survived the sweep (err=%v); stderr:\n%s", err, errout)
	}
	if !strings.Contains(errout, "unmerged") {
		t.Fatalf("expected the held worktree to be reported unmerged; stderr:\n%s", errout)
	}
}

// --- row 6: --dry-run writes NOTHING ------------------------------------------------
//
// The fixture carries both things the old dry run mutated: a DANGLING admin entry (which
// Step A's unconditional `git worktree prune` dropped) and a STALE lock (which the opt-in
// reclaim pass unlocked). After a dry run, .git/worktrees/ must be byte-identical and both
// must still be REPORTED.

func TestPruneDryRunIsReadOnly(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	removable := addWorktree(t, "dry-removable")
	dangling := addWorktree(t, "dry-dangling")
	stale := addWorktree(t, "dry-stale-lock")
	advanceMain(t, work, "first")

	// A dangling admin entry: the directory is gone, the registration is not.
	if err := os.RemoveAll(dangling); err != nil {
		t.Fatalf("remove %s: %v", dangling, err)
	}
	// A lock old enough for --lock-ttl to judge stale, naming no session.
	mustGit(t, work, "worktree", "lock", stale)

	adminDir := filepath.Join(work, ".git", "worktrees")
	before := hashTree(t, adminDir)

	rc, errout := runCapErr(t, []string{"prune", "--dry-run", "--reclaim-stale-locks", "--lock-ttl", "1ns"})
	if rc != 0 {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, errout)
	}

	if after := hashTree(t, adminDir); after != before {
		t.Fatalf("--dry-run mutated .git/worktrees/ (%s -> %s); stderr:\n%s", before[:12], after[:12], errout)
	}
	assertExists(t, removable)
	assertExists(t, stale)
	// Still locked: the reclaim pass reported, it did not unlock.
	if list := mustGit(t, work, "worktree", "list", "--porcelain"); !strings.Contains(list, "locked") {
		t.Fatalf("--dry-run retired a lock:\n%s", list)
	}
	// And it must still SAY what it would have done — a report that declined to look
	// would be worse than one that mutated.
	if !strings.Contains(errout, "removed 1") {
		t.Fatalf("--dry-run did not report the removal it would make; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "reclaimed lock on") {
		t.Fatalf("--dry-run did not report the lock it would reclaim; stderr:\n%s", errout)
	}
	// No singleton stamp is written by a dry run either.
	home, _ := os.UserHomeDir()
	if entries, err := os.ReadDir(filepath.Join(home, ".config", "assay", "prune")); err == nil && len(entries) != 0 {
		t.Fatalf("--dry-run wrote %d singleton stamp(s)", len(entries))
	}
}

// --- row 7: removal is BATCHED, and still verified ---------------------------------
//
// `git worktree prune` and `git worktree list --porcelain` are both repo-wide. Running
// them per removal is quadratic in the worktree count. The assertion is on the recorded
// git argv: one prune for the whole sweep, however many trees came out.

func TestPruneBatchesRemoval(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	var targets []string
	for _, n := range []string{"batch-a", "batch-b", "batch-c", "batch-d", "batch-e"} {
		targets = append(targets, addWorktree(t, n))
	}
	advanceMain(t, work, "first")

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	for _, p := range targets {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s survived the sweep (err=%v); stderr:\n%s", p, err, errout)
		}
	}
	if !strings.Contains(errout, "removed 5 merged+clean") {
		t.Fatalf("expected 5 removals; stderr:\n%s", errout)
	}

	prunes, lists := 0, 0
	for _, argv := range *calls {
		if len(argv) < 3 || argv[0] != "git" {
			continue
		}
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, "worktree prune"):
			prunes++
		case strings.Contains(joined, "worktree list --porcelain"):
			lists++
		}
	}
	// One bookkeeping prune at Step A, one deregistration prune after the loop. Five
	// removals must not add five more.
	if prunes > 2 {
		t.Fatalf("`git worktree prune` ran %d times for 5 removals — the removal is not batched; argv:\n%v", prunes, gitCalls(*calls))
	}
	// Enumeration + lock state + the one post-loop verification. The count must not grow
	// with the number of removals.
	if lists > 4 {
		t.Fatalf("`git worktree list --porcelain` ran %d times for 5 removals — the verification is not batched; argv:\n%v", lists, gitCalls(*calls))
	}
}

// A path that is still registered after the batched deregistration is DEMOTED: not counted
// as removed, and named. Batching must not turn a per-path verdict into a blanket one.
func TestPruneBatchDemotesStrandedPath(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	good := addWorktree(t, "demote-good")
	stranded := addWorktree(t, "demote-stranded")
	advanceMain(t, work, "first")

	// Make the deregistration listing report `stranded` as still registered after the
	// prune, by replaying the pre-removal listing for the verification read only.
	strandedRT := resolvePath(stranded)
	origPaths := worktreePathsFn
	worktreePathsFn = func(g *pathGuard, dir string) (map[string]bool, error) {
		set, err := origPaths(g, dir)
		if err != nil {
			return nil, err
		}
		set[strandedRT] = true
		return set, nil
	}
	t.Cleanup(func() { worktreePathsFn = origPaths })

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "removed 1 merged+clean") {
		t.Fatalf("a path that was still registered was counted as removed; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, strandedRT) || !strings.Contains(errout, "still registered after prune") {
		t.Fatalf("the stranded path was not named; stderr:\n%s", errout)
	}
	if _, err := os.Stat(good); !os.IsNotExist(err) {
		t.Fatalf("the verified removal did not happen for %s (err=%v); stderr:\n%s", good, err, errout)
	}
}

// The fail-closed row above deletes refs/remotes/origin/main, which is caught EARLIER — by
// the fresh-worktree tip guard, which resolves the same ref. So it does not actually
// exercise the sweep-level ancestor-set guard, and a mutation that removed that guard
// survived it (recorded rather than papered over: the escape is why this second row exists).
//
// This row isolates it. The ref RESOLVES, so every gate ahead of the merge gate passes
// normally; only the WALK fails. That is the state in which an unguarded sweep would read
// an empty ancestor set as an answer — today that marks everything unmerged, which is
// accidentally safe, and one refactor later marks everything merged, which is not.
func TestPruneHoldsAllWhenTheMainlineWalkFails(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "walk-fails-candidate")
	advanceMain(t, work, "first")

	orig := logOriginMain
	logOriginMain = func(sc *sweepCtx, dir string) ([]string, error) {
		return nil, errWalkFixture
	}
	t.Cleanup(func() { logOriginMain = orig })

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != 0 {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "removed 0 merged+clean") {
		t.Fatalf("a sweep whose mainline walk failed removed something; stderr:\n%s", errout)
	}
	// The SWEEP-LEVEL reason, not a per-worktree one: this must never be reported as
	// "unmerged", which would read as a finding about the worktree rather than about the
	// instrument.
	if !strings.Contains(errout, "for this sweep") {
		t.Fatalf("expected the sweep-level could-not-check reason on every candidate; stderr:\n%s", errout)
	}
	if strings.Contains(errout, "unmerged (branch") {
		t.Fatalf("a candidate was reported UNMERGED when the sweep could not read the mainline at all; stderr:\n%s", errout)
	}
}

var errWalkFixture = fixtureError("synthetic: the mainline walk failed")

type fixtureError string

func (e fixtureError) Error() string { return string(e) }
