package main

// lockreclaim_test.go — the lock LIFECYCLE.
//
// The defect these cover: a lock is taken at session boot and NOTHING ever retires it, so a
// dead session's lock is permanent, prune holds every locked worktree forever, and a lock on
// a worktree whose directory is already gone even keeps its dangling admin entry alive. The
// tests below pin both halves of the fix — the unconditional bookkeeping clean, and the
// opt-in, evidence-based reclaim — AND the invariant that matters most: reclaiming a lock
// never relaxes a single removal gate.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- fixtures ------------------

// advanceOriginMain marches the remote mainline one commit past the worktrees created so
// far, so a landed worktree is merged BELOW the tip — the fully-removable state in which
// only a lock (or a content gate) can keep it.
func advanceOriginMain(t *testing.T, work string) {
	t.Helper()
	writeFile(t, filepath.Join(work, "mainline.txt"), "marched forward\n")
	mustGit(t, work, "add", "mainline.txt")
	mustGit(t, work, "commit", "-m", "advance mainline")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mustGit(t, work, "rev-parse", "HEAD"))
}

// plantBeacon writes a roster beacon for `session` stamped `updated`, creating the roster
// directory. A beacon is how a session says "I am still here"; its absence or its staleness
// is the evidence that a session is gone.
func plantBeacon(t *testing.T, session string, updated time.Time) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster")
	writeFile(t, filepath.Join(dir, session+".json"),
		`{"session":"`+session+`","updated":"`+updated.UTC().Format(time.RFC3339)+`"}`+"\n")
}

// lockAdminFile is the path of git's `locked` marker for a worktree — the file whose mtime
// is the only age evidence a lock has.
func lockAdminFile(work, target string) string {
	return filepath.Join(work, ".git", "worktrees", filepath.Base(target), "locked")
}

// assertLocked / assertUnlocked read the lock state straight from git, never from the tool's
// own report, so a test cannot pass on a summary line that disagrees with the repo.
func assertLocked(t *testing.T, work, target string, want bool) {
	t.Helper()
	out := mustGit(t, work, "worktree", "list", "--porcelain")
	inBlock := false
	got := false
	target = resolvePath(target)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			inBlock = resolvePath(strings.TrimSpace(strings.TrimPrefix(line, "worktree "))) == target
			continue
		}
		if inBlock && (line == "locked" || strings.HasPrefix(line, "locked ")) {
			got = true
		}
	}
	if got != want {
		t.Fatalf("worktree %s locked = %v, want %v; porcelain:\n%s", target, got, want, out)
	}
}

// --- Step A: path-missing admin entries are cleaned, always ------------------
//
// The `git worktree prune` class: a worktree whose directory is gone leaves an admin entry
// behind. It is safe to drop unconditionally (nothing on disk is touched), so it needs no
// flag — and it must happen on a plain `deskwt prune` with no arguments at all.
//
// The COUNT assertion is a regression in its own right: `git worktree prune --verbose`
// reports on STDERR, and the sweep read only stdout, so the bookkeeping figure in the summary
// line was structurally zero however many entries a sweep dropped — the one number an
// operator would use to tell a draining repo from a stuck one.
func TestPruneCleansMissingPathEntry(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "vanished")

	// The directory disappears out from under git (a manual rm, a wiped /tmp, a container
	// restart) — the entry survives in `git worktree list` until something prunes it.
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("removing worktree dir: %v", err)
	}
	if !strings.Contains(mustGit(t, work, "worktree", "list", "--porcelain"), target) {
		t.Fatalf("positive control void: the admin entry for %s is already gone before prune ran", target)
	}

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if strings.Contains(mustGit(t, work, "worktree", "list", "--porcelain"), target) {
		t.Fatalf("path-missing admin entry survived a plain prune; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "pruned 1 bookkeeping") {
		t.Fatalf("expected the summary to report 1 bookkeeping entry; got:\n%s", errout)
	}
}

// A LOCKED worktree whose directory is gone is the compounded failure: `git worktree prune`
// honours the lock and will NOT drop the entry, so it is immortal until the lock is retired.
// Without the opt-in it stays (that is the pre-existing, safe behaviour); with it, the lock
// is reclaimed and the entry finally drops.
func TestReclaimUnblocksMissingPath(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "vanished-locked")
	mustGit(t, work, "worktree", "lock", "--reason", "worker-desk live session (session=ghost-1)", target)
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("removing worktree dir: %v", err)
	}
	// The session is gone: the roster exists but holds no beacon for it.
	plantBeacon(t, "someone-else", time.Now())

	// Without the opt-in: the lock keeps the dangling entry alive.
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(mustGit(t, work, "worktree", "list", "--porcelain"), target) {
		t.Fatalf("positive control void: the locked dangling entry was dropped WITHOUT the opt-in; stderr:\n%s", errout)
	}

	// With it: the lock is reclaimed, and the second bookkeeping pass drops the entry.
	rc, errout = runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --reclaim-stale-locks rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if strings.Contains(mustGit(t, work, "worktree", "list", "--porcelain"), target) {
		t.Fatalf("dangling entry of a reclaimed lock survived; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("expected locks-reclaimed 1; got:\n%s", errout)
	}
	if !strings.Contains(errout, "pruned 1 bookkeeping") {
		t.Fatalf("expected the post-reclaim bookkeeping pass to drop 1 entry; got:\n%s", errout)
	}
}

// --- the opt-in gate: a stale lock is reclaimed ONLY when asked -------------------

// stalelockFixture builds the fully-removable worktree (tracked-clean, merged below the tip)
// locked by a session the roster shows as long gone. ONLY the lock keeps it.
func staleLockFixture(t *testing.T, name, session string) (work, target string, calls *[][]string) {
	t.Helper()
	work = newRepo(t)
	calls = withEnv(t, work)
	target = addWorktree(t, name)
	advanceOriginMain(t, work)
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session="+session+")", target)
	plantBeacon(t, session, time.Now().Add(-6*time.Hour)) // stopped reporting hours ago
	resetCalls(calls)
	return work, target, calls
}

func TestStaleLockHeldWithoutOptIn(t *testing.T) {
	work, target, _ := staleLockFixture(t, "held", "dead-1")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
	if !strings.Contains(errout, "locks-reclaimed 0") {
		t.Fatalf("a default sweep reclaimed a lock; got:\n%s", errout)
	}
	if !strings.Contains(errout, "locked-held 1") {
		t.Fatalf("expected the summary to count the lock-held worktree; got:\n%s", errout)
	}
}

func TestStaleLockReclaimedOnOptIn(t *testing.T) {
	work, target, calls := staleLockFixture(t, "reclaimed", "dead-2")
	_ = work

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --reclaim-stale-locks rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree %s survived after its stale lock was reclaimed (err=%v); stderr:\n%s", target, err, errout)
	}
	if !strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("expected locks-reclaimed 1; got:\n%s", errout)
	}
	if !strings.Contains(errout, "removed 1 merged+clean") {
		t.Fatalf("expected the unlocked worktree to then be removed by the ordinary rules; got:\n%s", errout)
	}
	// Every unlock must print WHAT it unlocked, the reason the lock carried, and WHY that
	// reason was judged stale — an unexplained unlock is indistinguishable from a bug.
	if !strings.Contains(errout, "reclaimed lock on "+resolvePath(target)) {
		t.Fatalf("reclaim line does not name the worktree; got:\n%s", errout)
	}
	if !strings.Contains(errout, "session=dead-2") {
		t.Fatalf("reclaim line does not surface the lock reason; got:\n%s", errout)
	}
	if !strings.Contains(errout, "session dead-2 is gone") {
		t.Fatalf("reclaim line does not state the staleness evidence; got:\n%s", errout)
	}
	if calls != nil && anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag during a reclaiming prune: %v", gitCalls(*calls))
	}
}

// --- a LIVE session's lock is held, always ------------------
//
// The failure that would make this whole verb unusable is unlocking a worktree somebody is
// working in. A beacon inside the freshness window is positive evidence the session is
// alive, and it outranks the age test: --lock-ttl 1ns here would reclaim ANY lock that fell
// through to it, so the hold is proof the live-session evidence won.
func TestPruneHoldsLockOfLiveSession(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "livesession")
	advanceOriginMain(t, work)
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session=alive-1)", target)
	plantBeacon(t, "alive-1", time.Now())

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks", "--lock-ttl", "1ns"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
	if !strings.Contains(errout, "locks-reclaimed 0") {
		t.Fatalf("a LIVE session's lock was reclaimed; got:\n%s", errout)
	}
	if !strings.Contains(errout, "locked-held 1") {
		t.Fatalf("expected the live lock to be reported as locked-held; got:\n%s", errout)
	}
}

// --- the age fallback: a lock naming no session ------------------

func TestUnattributedLockByTTL(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "ancient")
	advanceOriginMain(t, work)
	// A lock reason from before the session stamp existed: nothing to look up in the roster.
	mustGit(t, work, "worktree", "lock", "--reason", "worker-desk live session", target)
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(lockAdminFile(work, target), old, old); err != nil {
		t.Fatalf("aging the lock file: %v", err)
	}

	// The age test is OFF by default, so the opt-in alone must not reclaim it.
	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertLocked(t, work, target, true)
	if !strings.Contains(errout, "locks-reclaimed 0") {
		t.Fatalf("an unattributed lock was reclaimed with --lock-ttl disabled; got:\n%s", errout)
	}

	rc, errout = runCapErr(t, []string{"prune", "--reclaim-stale-locks", "--lock-ttl", "24h"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --lock-ttl rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("expected the aged lock to be reclaimed; got:\n%s", errout)
	}
	if !strings.Contains(errout, "past the --lock-ttl 24h") {
		t.Fatalf("reclaim line does not state the age evidence; got:\n%s", errout)
	}
}

// --- the invariant: unlocking relaxes NOTHING ------------------
//
// The reclaim only removes the lock. Every content gate then runs on the unlocked worktree
// exactly as before, so work that would have been protected without a lock is still
// protected with the lock gone. These two fixtures are the ones a careless implementation
// would delete.

func TestDirtyHeldAfterReclaim(t *testing.T) {
	work, target, _ := staleLockFixture(t, "dirty-after", "dead-3")
	writeFile(t, filepath.Join(target, "README.md"), "seed\nuncommitted tracked change\n")

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, false) // the lock WAS retired …
	if !strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("expected the stale lock to be reclaimed; got:\n%s", errout)
	}
	// … and the tracked-clean gate still held the worktree.
	if !strings.Contains(errout, "dirty") {
		t.Fatalf("expected a dirty skip after the reclaim; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a DIRTY worktree after reclaiming its lock; stderr:\n%s", errout)
	}
}

func TestUnmergedHeldAfterReclaim(t *testing.T) {
	work, target, _ := staleLockFixture(t, "unmerged-after", "dead-4")
	// Work committed on the branch and pushed to its OWN upstream, but never merged to
	// origin/main: an open PR in flight. The active-worker guard must still hold it.
	writeFile(t, filepath.Join(target, "feat.txt"), "feature work\n")
	mustGit(t, target, "add", "feat.txt")
	mustGit(t, target, "commit", "-m", "feature work (open PR, unmerged)")
	mustGit(t, work, "update-ref", "refs/remotes/origin/feature", mustGit(t, target, "rev-parse", "HEAD"))
	mustGit(t, target, "branch", "--set-upstream-to=origin/feature")

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, false)
	if !strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("expected the stale lock to be reclaimed; got:\n%s", errout)
	}
	if !strings.Contains(errout, "unmerged") {
		t.Fatalf("expected an unmerged skip after the reclaim; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed an UNMERGED worktree after reclaiming its lock; stderr:\n%s", errout)
	}
}

// A worktree OUTSIDE the sanctioned prefixes is none of this tool's business, and the reclaim
// is not the place to make it the exception: prune would never remove it, so unlocking it
// would be a pure loss of somebody else's state. The lock stays even with every staleness
// test armed (dead session AND a 1ns TTL).
func TestNoReclaimOutsidePrefix(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	outside := filepath.Join(t.TempDir(), "outside-wt")
	mustGit(t, work, "worktree", "add", "--track", "-b", "outside", outside, "origin/main")
	mustGit(t, work, "worktree", "lock", "--reason", "somebody else (session=dead-5)", outside)
	plantBeacon(t, "someone-else", time.Now()) // roster readable, no beacon for dead-5

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks", "--lock-ttl", "1ns"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, outside)
	assertLocked(t, work, outside, true)
	if !strings.Contains(errout, "locks-reclaimed 0") {
		t.Fatalf("a lock outside the sanctioned prefixes was reclaimed; got:\n%s", errout)
	}
}

// --- flags ------------------

func TestLockTTLRequiresOptIn(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	if rc := run([]string{"prune", "--lock-ttl", "24h"}); rc != deskkit.ExitRefused {
		t.Fatalf("prune --lock-ttl without --reclaim-stale-locks rc = %d, want 5 "+
			"(a knob that silently does nothing is refused)", rc)
	}
	for _, bad := range []string{"nope", "-5m"} {
		if rc := run([]string{"prune", "--reclaim-stale-locks", "--lock-ttl", bad}); rc != deskkit.ExitRefused {
			t.Fatalf("prune --lock-ttl %q rc = %d, want 5", bad, rc)
		}
	}
	// 0 is the documented "disabled" spelling and must be accepted, not refused.
	if rc := run([]string{"prune", "--reclaim-stale-locks", "--lock-ttl", "0"}); rc != deskkit.ExitOK {
		t.Fatalf("prune --lock-ttl 0 rc = %d, want 0", rc)
	}
}

// --- the heuristics, directly ------------------

func TestSessionFromLockReason(t *testing.T) {
	cases := []struct{ reason, want string }{
		{"", ""},
		{"worker-desk live session", ""},
		{"worker-desk live session (deskwt role-init session=abc-123)", "abc-123"},
		{"session=plain", "plain"},
		{"worker-desk live session (session=paren-wrapped)", "paren-wrapped"},
		{"intake-desk live session session=s1 (pid 4242)", "s1"},
		{"session=", ""},
		{"a-session=notthekey", ""}, // the key is a whole token, not a substring
	}
	for _, c := range cases {
		if got := sessionFromLockReason(c.reason); got != c.want {
			t.Errorf("sessionFromLockReason(%q) = %q, want %q", c.reason, got, c.want)
		}
	}
}

// TestJudgeLockEvidence pins the decision table, including the three "proves nothing" states
// that must NEVER reclaim: an unreadable roster, a lock naming no session with the age test
// disabled, and a lock with no readable timestamp.
func TestJudgeLockEvidence(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	rosterDir := t.TempDir()
	writeFile(t, filepath.Join(rosterDir, "live.json"),
		`{"session":"live","updated":"`+now.Add(-5*time.Minute).Format(time.RFC3339)+`"}`+"\n")
	writeFile(t, filepath.Join(rosterDir, "zombie.json"),
		`{"session":"zombie","updated":"`+now.Add(-6*time.Hour).Format(time.RFC3339)+`"}`+"\n")
	writeFile(t, filepath.Join(rosterDir, "garbled.json"), "{not json\n")

	old := now.Add(-48 * time.Hour)
	fresh := now.Add(-1 * time.Minute)

	cases := []struct {
		name      string
		reason    string
		mtime     time.Time
		haveMtime bool
		ttl       time.Duration
		roster    string
		wantStale bool
	}{
		{"live session holds even past the TTL", "r session=live", old, true, time.Hour, rosterDir, false},
		{"zombie session is stale", "r session=zombie", fresh, true, 0, rosterDir, true},
		{"unknown session is stale", "r session=never-was", fresh, true, 0, rosterDir, true},
		{"garbled beacon proves nothing", "r session=garbled", fresh, true, 0, rosterDir, false},
		{"garbled beacon still falls through to the TTL", "r session=garbled", old, true, time.Hour, rosterDir, true},
		{"unreadable roster proves nothing", "r session=zombie", fresh, true, 0, "", false},
		{"no session, TTL disabled, holds", "worker-desk live session", old, true, 0, rosterDir, false},
		{"no session, past the TTL, stale", "worker-desk live session", old, true, time.Hour, rosterDir, true},
		{"no session, inside the TTL, holds", "worker-desk live session", fresh, true, time.Hour, rosterDir, false},
		{"no timestamp is not an old timestamp", "worker-desk live session", time.Time{}, false, time.Hour, rosterDir, false},
	}
	for _, c := range cases {
		got := judgeLock(c.reason, c.mtime, c.haveMtime, c.ttl, c.roster, now)
		if got.stale != c.wantStale {
			t.Errorf("%s: judgeLock stale = %v, want %v (why: %s)", c.name, got.stale, c.wantStale, got.why)
		}
		if strings.TrimSpace(got.why) == "" {
			t.Errorf("%s: judgeLock returned an empty explanation", c.name)
		}
	}
}
