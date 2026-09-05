package main

// liveness.go — the branch-liveness probe that lets `deskclaim acquire`/`stale` answer the
// fourth arm of deskkit.isStale ("is this recorded branch still doing live work?").
//
// THE DEFECT IT CLOSES. deskkit.isStale never reclaims a claim that records a --branch when
// no BranchActive probe is wired: "cannot prove inactive => conservative, do not steal". The
// CLI wired no probe, so every --branch claim was un-reclaimable through the tool at ANY age,
// and the only exit left was a hand-delete that bypasses the directory-wide flock — the very
// guard that closes double-dispatch. This probe supplies the missing signal so the reclaim
// can happen through the tool, under the flock, with an audit trail.
//
// FAIL-CLOSED COMPOSITION (the load-bearing rule). A branch is INACTIVE only when EVERY
// signal the probe can actually read says so; a signal it could not look at means ACTIVE. So
// the ONLY path to "inactive" is: the repo is a readable git repo AND the branch is checked
// out in none of its worktrees AND the beacon dir is readable AND the owner session's beacon
// says gone. Anything else — no repo, not a git repo, an unreadable beacon dir — is ACTIVE
// ("cannot prove"), because stealing a claim whose worktree lives on ANOTHER machine (a
// branch invisible to a local `git worktree list`) is exactly the failure the beacon second
// signal exists to prevent. The probe never contacts the forge.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// beaconFreshWindow mirrors deskwt/lockreclaim.go's window: a session whose roster beacon was
// re-stamped within the last hour is LIVE. One definition of "the session is still there"
// governs both the worktree-lock reclaim and this claim reclaim.
const beaconFreshWindow = 60 * time.Minute

// livenessProbe answers "does this branch still have live work?" for one claim. It is built
// with the repo to inspect, the roster beacon dir, and the claim's OWNER session (the beacon
// is keyed on the session, not the branch, so the owner must be supplied out of band — the
// deskkit.BranchActive callback receives only the branch). `because` records the reason for
// the LAST verdict, for the audit/report line; it uses the fixed vocabulary the README lists.
type livenessProbe struct {
	repo      string // git repo to read `git worktree list` from; "" means "no repo"
	beaconDir string // roster beacon dir; "" means "cannot read the beacon signal"
	owner     string // the claim owner's session id, for the beacon lookup

	because string // set on every active() call: the fixed-vocabulary reason
}

// because vocabulary — the fixed set the stale-verb report and the README share.
const (
	becauseAgeUnderTTL     = "age-under-ttl"        // claim younger than its TTL (age floor; probe not consulted)
	becauseBranchCheckout  = "branch-checked-out:"  // + the worktree path holding the branch
	becauseBeaconLive      = "beacon-live"          // the owner session's roster beacon is fresh
	becauseCannotProve     = "no-repo-cannot-prove" // a required liveness signal could not be read (no/again git repo, or unreadable beacon dir)
	becauseOldNoLiveSignal = "old-no-live-signal"   // old, and every readable signal says inactive (reclaimable)
)

// active is the deskkit.ClaimConfig.BranchActive callback. It reports whether `branch` is
// still doing live work, fail-closed per the composition rule above, and records the reason
// in p.because. It is only ever called by isStale for a claim already past its TTL, so a
// false here (INACTIVE) is what licenses a reclaim.
func (p *livenessProbe) active(branch string) bool {
	// Signal 1 — the branch is checked out in a registered worktree of --repo.
	path, checkedOut, couldLook := branchCheckedOut(p.repo, branch)
	if !couldLook {
		// No repo, or not a readable git repo: signal 1 is unreadable, so we cannot prove
		// the branch is inactive. ACTIVE, cannot prove — do NOT reclaim.
		p.because = becauseCannotProve
		return true
	}
	if checkedOut {
		p.because = becauseBranchCheckout + path
		return true
	}

	// Signal 2 — the owner session has a live roster beacon.
	switch beaconLiveness(p.beaconDir, p.owner) {
	case beaconLive:
		p.because = becauseBeaconLive
		return true
	case beaconGone:
		// Both signals were readable and neither says active: the ONLY inactive path.
		p.because = becauseOldNoLiveSignal
		return false
	default: // beaconCannotLook — the beacon dir is unreadable; cannot prove inactive.
		p.because = becauseCannotProve
		return true
	}
}

// branchCheckedOut reports whether `branch` is checked out in a registered worktree of the
// repo rooted at `repo`, and whether the question could be answered at all. It reads
// `git worktree list --porcelain` (a purely local read — no network, offline-safe).
//
//	couldLook == false → repo is "" or not a readable git repo (signal unreadable → ACTIVE)
//	couldLook == true, checkedOut == true  → held; path names the worktree
//	couldLook == true, checkedOut == false → the repo has no worktree on this branch
//
// A git error is a "could not look", never "nobody holds it" — reading an unreadable
// worktree list as "free" is the reading that steals a live claim.
func branchCheckedOut(repo, branch string) (path string, checkedOut bool, couldLook bool) {
	if repo == "" || branch == "" {
		return "", false, false
	}
	out, err := gitWorktreeList(repo)
	if err != nil {
		return "", false, false
	}
	want := "refs/heads/" + branch
	var cur string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if ref == want && cur != "" {
				return cur, true, true
			}
		}
	}
	return "", false, true
}

// gitWorktreeList runs `git -C repo worktree list --porcelain` and returns trimmed stdout.
// The argv is an explicit literal slice — no shell, no caller-supplied flags — so nothing a
// claim records can turn into a git option. A non-zero exit (not a git repo, git absent) is
// an error the caller reads as "could not look".
func gitWorktreeList(repo string) (string, error) {
	cmd := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// beaconEvidence is what the roster can say about one session's liveness.
type beaconEvidence int

const (
	// beaconCannotLook — the beacon dir or the beacon file could not be read, or its
	// timestamp could not be parsed. Proves NOTHING → the caller treats it as ACTIVE.
	beaconCannotLook beaconEvidence = iota
	// beaconGone — the beacon dir is readable and either holds no beacon for this session,
	// or holds one that stopped being re-stamped past the freshness window: session gone.
	beaconGone
	// beaconLive — the session re-stamped its beacon inside the freshness window: alive.
	beaconLive
)

// sessionBeacon is the tolerant read shape of a roster beacon; only `updated` is consulted.
type sessionBeacon struct {
	Updated string `json:"updated"`
}

// beaconLiveness reports what the roster knows about the owner session. It mirrors deskwt's
// readSessionBeacon: an absent beacon in a readable dir is positive evidence of death; an
// unreadable dir/file or an unparseable timestamp proves nothing (beaconCannotLook).
func beaconLiveness(beaconDir, owner string) beaconEvidence {
	if beaconDir == "" || owner == "" {
		return beaconCannotLook
	}
	raw, err := os.ReadFile(filepath.Join(beaconDir, owner+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return beaconGone // readable dir, no beacon for this session
		}
		return beaconCannotLook // unreadable — prove nothing
	}
	var b sessionBeacon
	if json.Unmarshal(raw, &b) != nil {
		return beaconCannotLook
	}
	upd, terr := time.Parse(time.RFC3339, strings.TrimSpace(b.Updated))
	if terr != nil {
		return beaconCannotLook // an unparseable stamp is neither fresh nor stale
	}
	if time.Since(upd) > beaconFreshWindow {
		return beaconGone
	}
	return beaconLive
}

// resolveRepo turns the --repo flag into the repo the probe reads. An explicit value is used
// verbatim; an empty flag defaults to the current directory ONLY when it is itself a git repo
// (so a probe run from inside a worktree Just Works), else "" (no repo → the probe answers
// ACTIVE / cannot-prove).
func resolveRepo(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if _, lerr := gitWorktreeList(cwd); lerr != nil {
		return "" // cwd is not a git repo — no repo signal
	}
	return cwd
}

// resolveBeaconDir returns the roster beacon dir (<StateDir>/roster) when it exists as a
// directory, else "" ("the roster can say nothing"). A missing dir is NOT an error here — it
// is simply the absence of the beacon signal, which the probe folds into cannot-prove/ACTIVE.
func resolveBeaconDir() string {
	base, err := deskkit.StateDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, "roster")
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return ""
	}
	return dir
}
