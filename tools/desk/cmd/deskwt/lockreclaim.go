package main

// lockreclaim.go — the LOCK LIFECYCLE half of `deskwt prune`.
//
// THE DEFECT. A worktree lock is a promise ("a live session is using this"), and nothing
// ever retired that promise. A session locks its worktree at boot (role-init, and the role
// skills' cooperative `git worktree lock --reason "<role> live session"`) and then simply
// ends; no code path unlocks it. prune consults the lock FIRST and LEAVES every locked
// worktree — correctly, because deleting a locked worktree's directory strands its
// registration (issue #264) — so the locked population only ever GROWS. Worse, a lock also
// blocks the safe bookkeeping step: `git worktree prune` refuses to drop the dangling admin
// entry of a LOCKED worktree even when its directory is already gone, so those accumulate
// too. The measured end state is a repo with hundreds of registered worktrees, dozens of them
// locked and prune-immune, some pointing at paths that no longer exist.
//
// THE FIX. Give the lock a lifecycle: an OPT-IN reclaim pass that unlocks a lock it can
// PROVE is stale, and then lets the ordinary prune rules decide the worktree's fate. The
// reclaim is deliberately weak:
//
//   - It is OFF by default (--reclaim-stale-locks). A sweep that has not been asked to
//     reclaim behaves exactly as before.
//   - It only ever UNLOCKS. It never removes anything. Every existing safety gate
//     (shared-checkout identity, sanctioned prefix, current worktree, tracked-clean,
//     fresh-at-tip, merged-into-origin/main) runs afterwards, unchanged, on the now-unlocked
//     worktree — so an unlocked worktree that is dirty, unpushed or unmerged is still LEFT.
//     Reclaiming a lock can therefore never lose work; the worst case is a worktree that
//     stays registered but is no longer marked as belonging to a live session.
//   - It reclaims only worktrees under a sanctioned prefix, never the shared checkout and
//     never the current worktree.
//   - It acts on EVIDENCE, never on a guess, and prints the evidence for every unlock.
//
// THE STALENESS EVIDENCE, in preference order:
//
//  1. Session death (preferred). The lock reason carries a `session=<id>` stamp (role-init
//     writes it; the role skills' cooperative lock incantation writes it too). That id is
//     looked up in the roster the desk already keeps — one JSON beacon per session under
//     <StateDir()>/roster/<session>.json, re-stamped with an `updated` timestamp while the
//     session lives. No beacon, or a beacon that stopped being updated, is positive evidence
//     the session is gone; a beacon updated inside the freshness window is positive evidence
//     it is alive, and a live session's lock is held even past its TTL.
//  2. Age (--lock-ttl, default 0 = disabled). Git keeps exactly one timestamp for a lock:
//     the mtime of the `locked` file in the worktree's admin directory. A lock older than
//     the TTL is treated as stale. This is the fallback for locks that name no session —
//     every lock taken before the stamp existed, and any lock taken by hand.
//
// Anything else is HELD. An unreadable roster, an unparseable beacon timestamp, a missing
// admin file: none of them are evidence of death, so none of them reclaim a lock. The pass
// fails CLOSED the same way the rest of the tool does.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	// lockSessionKey is the stamp a lock reason carries so a later sweep can attribute the
	// lock to a session: `worker-desk live session (deskwt role-init session=<id>)`. A lock
	// reason without it names no session and is judged by --lock-ttl alone.
	lockSessionKey = "session="

	// beaconFreshWindow is how recently a session's roster beacon must have been re-stamped
	// for its lock to count as LIVE. It matches the roster's own zombie threshold (a session
	// that registered work and then stopped reporting for an hour is treated as gone
	// elsewhere in the desk tools), so one definition of "a session is still there" governs
	// both the roster report and this reclaim.
	beaconFreshWindow = 60 * time.Minute
)

// reclaimEntry records ONE lock this sweep retired: which worktree, the lock reason it
// carried, and the evidence that judged it stale. All three are printed — a reclaim that
// could not be explained afterwards would be indistinguishable from a bug.
type reclaimEntry struct{ path, reason, why string }

// lockVerdict is the staleness judgement for one lock. `why` is filled in BOTH cases: the
// held case's reason is what makes a "nothing was reclaimed" run diagnosable.
type lockVerdict struct {
	stale bool
	why   string
}

// sessionFromLockReason extracts the `session=<id>` stamp from a lock reason, or "" when the
// reason names no session. Parsing is whitespace-token based and tolerant of the surrounding
// punctuation a human-written reason carries, because the reason is free text written by
// several writers (this tool, the role skills, a person at a prompt).
func sessionFromLockReason(reason string) string {
	const punct = "()[]{},;:\"'"
	for _, f := range strings.Fields(reason) {
		// Strip the punctuation a free-text reason wraps the token in — `(session=x)` is
		// as common a spelling as `session=x`, and a parser that only understood the bare
		// form would silently attribute nothing and hold every such lock forever.
		f = strings.Trim(f, punct)
		if !strings.HasPrefix(f, lockSessionKey) {
			continue
		}
		if s := strings.Trim(strings.TrimPrefix(f, lockSessionKey), punct); s != "" {
			// The id becomes a path segment under the roster dir, so it must pass the
			// same shape gate every other worktree/session name in this package passes
			// (nameRe + no ".."): a reason like `session=../../x` must never traverse.
			// An id that fails the gate attributes nothing — the lock is then judged by
			// the age test alone, exactly like a reason with no session stamp.
			if !nameRe.MatchString(s) || strings.Contains(s, "..") {
				return ""
			}
			return s
		}
	}
	return ""
}

// beaconEvidence is what the roster can say about the session named in a lock reason.
type beaconEvidence int

const (
	// beaconUnreadable — the roster, the beacon file or its timestamp could not be read.
	// This proves NOTHING and must never reclaim a lock.
	beaconUnreadable beaconEvidence = iota
	// beaconAbsent — the roster is readable and holds no beacon for this session: the
	// session is gone (its beacon was cleaned up, or it never lived long enough to write one).
	beaconAbsent
	// beaconFresh — the session re-stamped its beacon inside the freshness window: alive.
	beaconFresh
	// beaconStale — the session has a beacon but stopped re-stamping it: gone.
	beaconStale
)

// sessionBeacon is the tolerant read shape of a roster beacon. Only `updated` is consulted
// here; the beacon carries more (role, open work) that this pass has no business judging.
// The read is deliberately tolerant for the same reason the desk's other beacon readers are:
// more than one writer has written this file over the project's life, and a reader that
// understood only one of them would report a confident, wrong answer.
type sessionBeacon struct {
	Updated string `json:"updated"`
}

// rosterBeaconDir returns the directory holding the session beacons, or "" when it cannot be
// resolved or does not exist. "" means "the roster can say nothing", which the caller treats
// as no evidence at all — never as "no session is alive".
func rosterBeaconDir() string {
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

// readSessionBeacon reports what the roster knows about one session.
func readSessionBeacon(rosterDir, session string, now time.Time) (beaconEvidence, time.Time) {
	if rosterDir == "" || session == "" {
		return beaconUnreadable, time.Time{}
	}
	raw, err := os.ReadFile(filepath.Join(rosterDir, session+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return beaconAbsent, time.Time{}
		}
		return beaconUnreadable, time.Time{}
	}
	var b sessionBeacon
	if jerr := json.Unmarshal(raw, &b); jerr != nil {
		return beaconUnreadable, time.Time{}
	}
	upd, terr := time.Parse(time.RFC3339, strings.TrimSpace(b.Updated))
	if terr != nil {
		// An unparseable timestamp cannot be proven stale OR fresh. Inventing a death here
		// would be exactly as wrong as missing one.
		return beaconUnreadable, time.Time{}
	}
	if now.Sub(upd) > beaconFreshWindow {
		return beaconStale, upd
	}
	return beaconFresh, upd
}

// judgeLock is the whole staleness decision, kept pure (no git, no clock) so the heuristics
// can be tested directly. haveMtime=false means git kept no readable timestamp for this lock,
// so the TTL test cannot fire — a missing timestamp is not an old one.
func judgeLock(reason string, lockMtime time.Time, haveMtime bool, ttl time.Duration, rosterDir string, now time.Time) lockVerdict {
	if sess := sessionFromLockReason(reason); sess != "" {
		switch ev, upd := readSessionBeacon(rosterDir, sess, now); ev {
		case beaconFresh:
			// A live session outranks every age test: a long-running session's lock is not
			// stale merely because the session is long-running.
			return lockVerdict{false, fmt.Sprintf("session %s is live (roster beacon updated %s ago)", sess, roundAge(now.Sub(upd)))}
		case beaconStale:
			return lockVerdict{true, fmt.Sprintf("session %s is gone: roster beacon last updated %s ago, past the %s freshness window",
				sess, roundAge(now.Sub(upd)), beaconFreshWindow)}
		case beaconAbsent:
			// This branch trusts an invariant written elsewhere: a LIVE session keeps its
			// roster beacon present and fresh (deskroster's heartbeat). A live session that
			// never wrote a beacon is judged dead here; its safety net is the unchanged
			// removal gates (dirty / fresh-at-tip / unmerged), which the reclaim only ever
			// hands an UNLOCKED worktree to — never a removal decision.
			return lockVerdict{true, fmt.Sprintf("session %s is gone: no roster beacon for it", sess)}
		}
		// beaconUnreadable — the roster proves nothing; fall through to the age test.
	}
	if ttl > 0 && haveMtime && now.Sub(lockMtime) > ttl {
		return lockVerdict{true, fmt.Sprintf("lock taken %s ago, past the --lock-ttl %s", roundAge(now.Sub(lockMtime)), ttl)}
	}
	if ttl > 0 {
		return lockVerdict{false, "no evidence the locking session is gone, and the lock is inside --lock-ttl " + ttl.String()}
	}
	return lockVerdict{false, "no evidence the locking session is gone (--lock-ttl is disabled)"}
}

// roundAge renders a duration at minute resolution — the precision the evidence actually has.
func roundAge(d time.Duration) string {
	if d < time.Minute {
		return "under 1m"
	}
	return d.Round(time.Minute).String()
}

// lockFileMtimes returns resolved-worktree-path → mtime of that worktree's `locked` admin
// file. That mtime is the ONLY timestamp git keeps for a lock, so it is the only age evidence
// available. Best effort by construction: a worktree whose admin files cannot be read is
// simply ABSENT from the map (no age evidence → the TTL test cannot fire for it), never given
// an invented timestamp.
func (g *pathGuard) lockFileMtimes() map[string]time.Time {
	out := make(map[string]time.Time)
	if g.commonDir == "" {
		return out
	}
	adminRoot := filepath.Join(g.commonDir, "worktrees")
	entries, err := os.ReadDir(adminRoot)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		admin := filepath.Join(adminRoot, e.Name())
		fi, serr := os.Stat(filepath.Join(admin, "locked"))
		if serr != nil {
			continue // not locked, or unreadable — either way, no age evidence
		}
		// `gitdir` holds the absolute path of the worktree's own .git file; its parent is
		// the worktree root. Resolved the same way worktreeRoots resolves, so the keys
		// compare equal to the paths the caller iterates.
		gd, rerr := os.ReadFile(filepath.Join(admin, "gitdir"))
		if rerr != nil {
			continue
		}
		dotGit := strings.TrimSpace(string(gd))
		if dotGit == "" {
			continue
		}
		out[resolvePath(filepath.Dir(dotGit))] = fi.ModTime()
	}
	return out
}

// reclaimStaleLocks is the opt-in reclaim pass. It unlocks every lock it can PROVE stale on a
// worktree prune is otherwise entitled to consider, and returns one entry per lock actually
// retired plus any warnings worth printing. It removes nothing: the caller's ordinary
// eligibility rules run afterwards on the unlocked worktrees, unchanged.
//
// Each unlock is POSITIVELY VERIFIED — the lock state is re-read afterwards and only locks
// that are genuinely gone are counted, so the reported count can never overstate what
// happened (git reporting success is not the same as the lock being gone).
func reclaimStaleLocks(guard *pathGuard, dir, cwd string, ttl time.Duration) ([]reclaimEntry, []string, error) {
	locked, err := guard.lockedWorktrees(dir)
	if err != nil {
		return nil, nil, err
	}
	if len(locked) == 0 {
		return nil, nil, nil
	}

	mtimes := guard.lockFileMtimes()
	rosterDir := rosterBeaconDir()
	var warns []string
	if rosterDir == "" {
		warns = append(warns, "roster beacons are unreadable — no session-death evidence this sweep"+
			ttlHint(ttl))
	}

	// Deterministic order: the same repo state must produce the same log every time.
	paths := make([]string, 0, len(locked))
	for p := range locked {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	now := time.Now()
	var candidates []reclaimEntry
	for _, rt := range paths {
		// Never unlock something prune would refuse to touch anyway. The lock is the
		// LAST thing protecting the shared checkout and the current worktree from a future
		// change to the rules below it; leave it in place.
		if rt == guard.sharedCheckout || rt == cwd || !guard.allowed(rt) {
			continue
		}
		reason := locked[rt]
		mt, haveMtime := mtimes[rt]
		v := judgeLock(reason, mt, haveMtime, ttl, rosterDir, now)
		if !v.stale {
			continue
		}
		if _, uerr := runGit(dir, "worktree", "unlock", rt); uerr != nil {
			warns = append(warns, "could not unlock "+rt+" ("+v.why+"): "+uerr.Error())
			continue
		}
		candidates = append(candidates, reclaimEntry{path: rt, reason: reason, why: v.why})
	}
	if len(candidates) == 0 {
		return nil, warns, nil
	}

	after, aerr := guard.lockedWorktrees(dir)
	if aerr != nil {
		return nil, warns, aerr
	}
	var done []reclaimEntry
	for _, c := range candidates {
		if _, still := after[c.path]; still {
			warns = append(warns, "unlock reported success but "+c.path+" is still locked — left alone")
			continue
		}
		done = append(done, c)
	}
	return done, warns, nil
}

// ttlHint tells the reader whether the age fallback is even armed, so a sweep that reclaimed
// nothing says WHY it could reclaim nothing.
func ttlHint(ttl time.Duration) string {
	if ttl > 0 {
		return " (falling back to --lock-ttl " + ttl.String() + ")"
	}
	return " (and --lock-ttl is disabled, so no lock can be judged stale)"
}

// lockReasonText renders a lock reason for a log line, naming the empty reason explicitly
// rather than printing nothing where a reason should be.
func lockReasonText(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "(no reason given)"
	}
	return `"` + reason + `"`
}
