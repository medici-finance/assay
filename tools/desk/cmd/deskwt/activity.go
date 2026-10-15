package main

// The prune ACTIVITY gate: a worktree something touched recently is held, whatever its
// content proves.
//
// Every other removal gate asks what a worktree HOLDS — is its HEAD on the mainline, is it
// clean, is it pushed. None asks whether anyone is still USING it, and a tree a session is
// working in can pass all of them: a worker whose branch was just fast-forwarded to a
// merged commit, or one that has not yet written anything, is content-identical to
// abandoned residue. Removing it pulls the floor out from under a live process. The lock
// is the cooperative half of that guard (a session that locks its tree is never pruned);
// this gate is the half that needs no cooperation: git itself writes the evidence.
//
// Activity is the NEWEST mtime among the files git rewrites as a session works in a
// worktree — its admin entry's HEAD (every checkout, commit, reset), index (every add,
// status refresh, checkout), logs/HEAD (every ref move), ORIG_HEAD and FETCH_HEAD (merges,
// rebases, fetches; absent until first written) — and the worktree root directory itself
// (a file created or deleted at the top level). A tree whose newest such mtime is inside
// --min-idle is HELD with that age named. Evidence that cannot be read is not evidence of
// idleness: an unreadable HEAD, root or admin registry HOLDS the tree.

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// defaultMinIdle is the activity gate's window when --min-idle is not given. One hour: the
// same window the roster uses for "a session could still be live" (beaconFreshWindow), and
// long enough to span a build or a test run during which nothing in the tree is written.
// A package var only so the fixture suite, whose worktrees are all seconds old, can run
// the gates it was written for; production never assigns it.
var defaultMinIdle = 60 * time.Minute

// activityFiles are the admin-entry files whose mtime is activity evidence. HEAD is
// REQUIRED (every worktree has one; its absence means the entry cannot be judged); the
// rest are optional because git creates each lazily.
var activityFiles = []struct {
	name     string
	required bool
}{
	{"HEAD", true},
	{"index", false},
	{filepath.Join("logs", "HEAD"), false},
	{"ORIG_HEAD", false},
	{"FETCH_HEAD", false},
}

// worktreeAdminDirs maps every resolved worktree root to its admin entry under
// <common-dir>/worktrees, read ONCE per sweep (the per-candidate adminDirFor re-reads the
// whole registry). An unreadable registry is an error: the caller then holds every
// candidate rather than judging none of them idle.
func (g *pathGuard) worktreeAdminDirs() (map[string]string, error) {
	if g.commonDir == "" {
		return nil, deskkit.Unverifiable("the shared git-common-dir is not resolved", nil)
	}
	adminRoot := filepath.Join(g.commonDir, "worktrees")
	entries, err := os.ReadDir(adminRoot)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read the worktree admin registry", err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		admin := filepath.Join(adminRoot, e.Name())
		gd, rerr := os.ReadFile(filepath.Join(admin, "gitdir"))
		if rerr != nil {
			continue
		}
		dotGit := strings.TrimSpace(string(gd))
		if dotGit == "" {
			continue
		}
		out[resolvePath(filepath.Dir(dotGit))] = admin
	}
	return out, nil
}

// lastActivity is the newest activity mtime for the worktree rooted at rt whose admin
// entry is admin. An unreadable required file, or any read error other than "not there"
// on an optional one, is an error — never a zero time that would read as "idle forever".
func lastActivity(admin, rt string) (time.Time, error) {
	fi, err := os.Stat(rt)
	if err != nil {
		return time.Time{}, err
	}
	newest := fi.ModTime()
	for _, f := range activityFiles {
		fi, err := os.Stat(filepath.Join(admin, f.name))
		if err != nil {
			if !f.required && os.IsNotExist(err) {
				continue
			}
			return time.Time{}, err
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest, nil
}

// activityHold is the gate's verdict for one candidate: "" when the tree has been idle for
// at least minIdle, else the hold reason. admins is the sweep's worktreeAdminDirs map, or
// nil when that read failed (adminsErr says why). minIdle <= 0 disables the gate.
func activityHold(admins map[string]string, adminsErr error, rt string, minIdle time.Duration, now time.Time) string {
	if minIdle <= 0 {
		return ""
	}
	if adminsErr != nil {
		return "unverifiable worktree activity: " + errText(adminsErr)
	}
	admin, ok := admins[rt]
	if !ok {
		return "unverifiable worktree activity: no admin entry points at this worktree"
	}
	last, err := lastActivity(admin, rt)
	if err != nil {
		return "unverifiable worktree activity: " + errText(err)
	}
	age := now.Sub(last)
	if age < 0 {
		// A future mtime (clock skew, a touch -d) is the most recent activity possible.
		age = 0
	}
	if age < minIdle {
		return "active " + roundAge(age) + " ago (inside the " + minIdle.String() + " --min-idle window)"
	}
	return ""
}
