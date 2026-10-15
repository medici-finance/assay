//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantMarkedCell creates a cell directory under cellsRoot whose managed Go
// cache root is marked and recorded — the state a sweep can discover — and
// returns its policy. cellName may nest (team/demo). ownerDir is the directory
// the root's trust domain binds to: ordinarily the cell directory itself; the
// orphan-reap tests bind the root to ANOTHER directory (one the sweep can
// remove) instead.
func plantMarkedCell(t *testing.T, cellsRoot, cellName, ownerDir string, budget int64) cellcache.Policy {
	t.Helper()
	canonCells, err := filepath.EvalSymlinks(cellsRoot)
	if err != nil {
		t.Fatalf("cells root: %v", err)
	}
	cell := filepath.Join(canonCells, cellName)
	if err := os.MkdirAll(cell, 0700); err != nil {
		t.Fatal(err)
	}
	// Discovery defines a cell the way cellctl does: a directory holding a
	// cell.env. Its content is the launcher's concern, not the sweep's.
	if err := os.WriteFile(filepath.Join(cell, "cell.env"), []byte("# fixture cell\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if ownerDir == "" {
		ownerDir = cell
	}
	canonOwner, err := filepath.EvalSymlinks(ownerDir)
	if err != nil {
		t.Fatalf("owner dir: %v", err)
	}
	p := cellcache.Policy{
		Root:   filepath.Join(cell, "go-cache"),
		Domain: cellcache.DomainFor(canonOwner),
		Budget: budget,
		Floor:  0,
	}
	if _, err := cellcache.Check(p, false); err != nil {
		t.Fatalf("initialize marked cache root: %v", err)
	}
	return p
}

func cacheMaterial(t *testing.T, p cellcache.Policy, name string, n int) {
	t.Helper()
	path := filepath.Join(p.Root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0600); err != nil {
		t.Fatal(err)
	}
}

func withCellsRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "cells")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CELLS_ROOT", root)
	return root
}

// A sweep over a cache within its policy is SILENT: the summary carries no
// cache segment at all (the noise-floor contract), and the cache is untouched.
func TestPruneCachePassSilentWhenClean(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "house", "", 1<<30)
	cacheMaterial(t, p, "build/warm", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if strings.Contains(errout, "cache-") {
		t.Fatalf("a clean cache pass printed a segment:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "warm")); err != nil {
		t.Fatal("a within-policy cache was touched", err)
	}
}

// Over the byte budget with no active consumer, the sweep reclaims — and SAYS
// so, with the count.
func TestPruneCachePassReclaimsUnderPressure(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "house", "", 1)
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "cache-reclaimed 1") {
		t.Fatalf("summary did not report the reclamation:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("over-budget cache was not reclaimed", err)
	}
	// The root itself stays: only the caches under it are disposable.
	if _, err := os.Stat(filepath.Join(p.Root, "owner")); err != nil {
		t.Fatal("reclaim removed the root's ownership marker", err)
	}
}

// A consumer lease holds the cache under pressure: the sweep reports the
// deferral and removes nothing. Active records protect mod/ and gopath/; build/
// entries older than the policy's trim age would be trimmed regardless, but this
// fixture's material is fresh, so nothing qualifies.
func TestPruneCachePassDefersToActiveLease(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "house", "", 1)
	// The lease comes FIRST: enrollment itself runs a pressure check, which
	// would reclaim any material already present.
	l, err := cellcache.Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Finish(true) })
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "cache-deferred 1") {
		t.Fatalf("summary did not report the deferral:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); err != nil {
		t.Fatal("a cache with an active consumer was reclaimed", err)
	}
}

// --no-cache skips the pass entirely: no reclaim under pressure, no segment.
func TestPruneCachePassNoCacheFlag(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "house", "", 1)
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune", "--no-cache"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if strings.Contains(errout, "cache-") {
		t.Fatalf("--no-cache printed a cache segment:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); err != nil {
		t.Fatal("--no-cache still reclaimed", err)
	}
}

// A dry run prints the would-act row and changes nothing — no removal, and
// the package's own dry mode does not even rewrite the root's report.
func TestPruneCachePassDryRun(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "house", "", 1)
	cacheMaterial(t, p, "build/result", 100)
	before, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}

	rc, errout := runCapErr(t, []string{"prune", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "RECLAIM cache root "+p.Root) {
		t.Fatalf("dry run did not plan the reclamation:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); err != nil {
		t.Fatal("dry run removed cache data", err)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry run rewrote the root's report")
	}
}

// A marked root whose recorded policy is unreadable is a could-not-check
// warning, never a silently invented default policy.
func TestPruneCachePassMarkedRootWithoutReport(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "house", "", 1<<30)
	if err := os.Remove(p.ReportPath()); err != nil {
		t.Fatal(err)
	}

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "could-not-check") || !strings.Contains(errout, "house") {
		t.Fatalf("the uncheckable root was not surfaced:\n%s", errout)
	}
}

// The orphan reap: a sweep that removes a worktree which IS a marked root's
// owning directory reaps the root too — the sweep's own gates are the proof
// no consumer can launch again, and the root holds no lease.
func TestPruneCachePassReapsOrphanedRoot(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "cellbox")

	// Advance origin/main so the fresh worktree is merged-but-not-at-tip: the
	// removable state.
	writeFile(t, filepath.Join(work, "mainline.txt"), "marched forward\n")
	mustGit(t, work, "add", "mainline.txt")
	mustGit(t, work, "commit", "-m", "advance mainline")
	newMain := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", newMain)

	// The marked root lives under the cells root (a custom
	// CELL_GO_CACHE_ROOT placement), but its trust domain binds it to the
	// worktree about to be removed.
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "cellbox", target, 1<<30)
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("the worktree itself was not removed", err)
	}
	if !strings.Contains(errout, "cache-orphans-reaped 1") {
		t.Fatalf("summary did not report the orphan reap:\n%s", errout)
	}
	if _, err := os.Stat(p.Root); !os.IsNotExist(err) {
		t.Fatal("the orphaned cache root survived its owner's removal", err)
	}
}

// The same root with an ACTIVE lease is refused even when its owner is gone:
// the lease set, read at act time, outranks the sweep's judgement.
func TestPruneCachePassOrphanReapHonoursLease(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "cellbox")

	writeFile(t, filepath.Join(work, "mainline.txt"), "marched forward\n")
	mustGit(t, work, "add", "mainline.txt")
	mustGit(t, work, "commit", "-m", "advance mainline")
	newMain := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", newMain)

	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, "cellbox", target, 1<<30)
	l, err := cellcache.Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Finish(true) })
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("the worktree itself was not removed", err)
	}
	if strings.Contains(errout, "cache-orphans-reaped 1") {
		t.Fatalf("a root with an active lease was reaped:\n%s", errout)
	}
	if !strings.Contains(errout, "could-not-check") {
		t.Fatalf("the refused orphan reap was not surfaced:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); err != nil {
		t.Fatal("a leased root's cache was touched", err)
	}
}

// The cache knobs refuse the pairings in which they would be silently inert —
// the same discipline as --lock-ttl's.
func TestPruneCacheFlagRefusals(t *testing.T) {
	for _, args := range [][]string{
		{"prune", "--no-cache", "--cells-root", "/tmp/x"},
		{"prune", "--branches-only", "--no-cache"},
		{"prune", "--branches-only", "--cells-root", "/tmp/x"},
	} {
		withEnv(t, t.TempDir())
		rc, errout := runCapErr(t, args)
		if rc != deskkit.ExitRefused {
			t.Fatalf("%v rc = %d, want 5; stderr:\n%s", args, rc, errout)
		}
		if !strings.Contains(errout, "has no effect") && !strings.Contains(errout, "which --no-cache skips") {
			t.Fatalf("%v refused without naming the inert knob:\n%s", args, errout)
		}
	}
}

// A nested cell (team/demo) is a supported cell name; its default marked root
// lives at <cells-root>/team/demo/go-cache and the sweep must reach it.
func TestPruneCachePassNestedCell(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, filepath.Join("team", "demo"), "", 1)
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "cache-reclaimed 1") {
		t.Fatalf("the nested cell's pressure was not reclaimed:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("the nested cell's over-budget cache survived the sweep", err)
	}
}

// An invalid candidate is could-not-check for THAT cell and nothing else: the
// sweep names it and still reaches every later candidate.
func TestPruneCachePassContinuesPastInvalidCandidate(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	bad := plantMarkedCell(t, cells, "bad", "", 1<<30)
	// A linked report is outside the root's custody: the read must refuse it.
	if err := os.Remove(bad.ReportPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/null", bad.ReportPath()); err != nil {
		t.Fatal(err)
	}
	good := plantMarkedCell(t, cells, "good", "", 1)
	cacheMaterial(t, good, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "could-not-check") || !strings.Contains(errout, "bad") {
		t.Fatalf("the invalid candidate was not surfaced:\n%s", errout)
	}
	if !strings.Contains(errout, "cache-reclaimed 1") {
		t.Fatalf("the valid cell behind the invalid one was not checked:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(good.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("the valid cell's over-budget cache survived the sweep", err)
	}
}

// A symlink that sorts BEFORE a nested cell must not hide the cell from the
// sweep: a link is not a directory to the walk, and treating it like one
// skips its remaining siblings. The symlink itself is probed (it may BE a
// cell) but never followed.
func TestPruneCachePassSymlinkDoesNotHideLaterCells(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	// "a-link" sorts before "team": a walk that skips a non-dir entry's
	// siblings never reaches team/demo.
	if err := os.Symlink(filepath.Join(cells, "nowhere"), filepath.Join(cells, "a-link")); err != nil {
		t.Fatal(err)
	}
	p := plantMarkedCell(t, cells, filepath.Join("team", "demo"), "", 1)
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "cache-reclaimed 1") {
		t.Fatalf("the nested cell behind the symlink was not swept:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("the nested cell's over-budget cache survived the sweep", err)
	}
}

// A symlinked cells root is a form cellctl's loader accepts (its cell check
// follows links). WalkDir would walk the link alone and find no cells, so the
// sweep canonicalizes the root first; the descendant no-follow boundary is
// unchanged.
func TestPruneCachePassSymlinkedCellsRoot(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cells := withCellsRoot(t)
	p := plantMarkedCell(t, cells, filepath.Join("team", "demo"), "", 1)
	cacheMaterial(t, p, "build/result", 100)
	link := filepath.Join(t.TempDir(), "cells-link")
	if err := os.Symlink(cells, link); err != nil {
		t.Fatal(err)
	}

	rc, errout := runCapErr(t, []string{"prune", "--cells-root", link})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "cache-reclaimed 1") {
		t.Fatalf("the linked cells root was not swept:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("the linked root's over-budget cache survived the sweep", err)
	}
}

// A relative CELLS_ROOT is another accepted form: the sweep absolutizes it
// before discovery, or the recorded-policy root comparison could never hold
// and no cache would be checked.
func TestPruneCachePassRelativeCellsRoot(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	base := t.TempDir()
	t.Chdir(base)
	t.Setenv("CELLS_ROOT", "cells")
	cells := filepath.Join(base, "cells")
	if err := os.MkdirAll(cells, 0700); err != nil {
		t.Fatal(err)
	}
	p := plantMarkedCell(t, cells, "demo", "", 1)
	cacheMaterial(t, p, "build/result", 100)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "cache-reclaimed 1") {
		t.Fatalf("the relative cells root was not swept:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "result")); !os.IsNotExist(err) {
		t.Fatal("the relative root's over-budget cache survived the sweep", err)
	}
}
