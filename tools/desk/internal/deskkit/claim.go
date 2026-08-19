package deskkit

// claim.go — the canonical claimable-action LOCK primitive.
//
// Three disjoint on-disk claim schemas used to coexist:
//
//   - loopengine's dispatch claim:  {itemID, runner, branch, claimed}
//   - deskroster's reader:          {brief, session, ts}
//   - the desk skills' bash idiom:  {brief, repo, session, ts}
//
// This file unifies them behind ONE exported type (Claim) whose WRITE shape is
// {kind, item, owner, branch, ts} and whose READ is TOLERANT — a custom
// UnmarshalJSON maps every legacy field name onto Item/Owner/Branch/TS, so List and
// the stale check read every existing on-disk claim during rollout.
//
// THE DOUBLE-DISPATCH LESSON (load-bearing). The atomic-claim idiom used to live in a
// shell `(set -C; … > "$f")`. The writeguard hook blocks the `>` redirect, so an
// agent falls back to the Write tool — a BLIND overwrite with no O_EXCL — and every
// racer "succeeds": double-dispatch. Moving atomicity into this binary is the fix.
// The HARD INVARIANT that follows: no code path here may degrade to "proceed
// unclaimed / assume free." A lock this process cannot HOLD, or a claim file it
// cannot READ, is deskkit.Unverifiable (exit 6) — NEVER a silent success. See the
// fail-closed returns in Acquire and isStale, and TestAcquireNeverAssumesFree_Source.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Claim kinds — the handoff a claim guards (coverage table). Kind is
// attribution/observability, not a correctness axis: two claims on the SAME Item collide
// regardless of Kind, because the on-disk path is keyed on Item alone.
const (
	KindDispatch = "dispatch" // Board/issue Next-up → worker (rows 1-3)
	KindRoute    = "route"    // inbound event → routing action (row 4, next adopter)
	KindFile     = "file"     // review pass → new issue filing (row 5, next adopter)
	KindClose    = "close"    // merged PR → issue closure (row 9, next adopter)
	KindVerify   = "verify"   // merged/awaiting brief → verify dispatch (row 8)
)

// DefaultStaleClaim is the age past which a claim with no provably-live branch may be
// reclaimed when a caller does not state its own StaleClaim. It matches loopengine's
// production value of ">120 min, no branch".
const DefaultStaleClaim = 120 * time.Minute

// claimLockWait is how long Acquire waits for the claims flock before failing closed.
// It is a package var for ONE reason: so a test can shorten it and prove the deadline
// exists. There is deliberately no flag and no env override — shortening it in
// production would not make claiming faster, it would make it give up on a legitimately
// busy lock, and giving up early is one step from "assume free" (the double-dispatch failure).
var claimLockWait = 60 * time.Second

// Claim is the canonical claim record. The WRITE shape is exactly {kind,item,owner,
// branch,ts}; Branch is omitted when empty. Reading is tolerant (see UnmarshalJSON).
type Claim struct {
	Kind   string `json:"kind"`
	Item   string `json:"item"`
	Owner  string `json:"owner"`
	Branch string `json:"branch,omitempty"`
	TS     string `json:"ts"`
}

// UnmarshalJSON is the TOLERANT read that heals the schema fork: it accepts
// the canonical field names AND the two legacy shapes, mapping every alias onto the
// canonical fields. Canonical names win when both are present.
//
//	item  ← item | itemID | brief
//	owner ← owner | runner | session
//	ts    ← ts | claimed
//	branch ← branch
//
// (The bash idiom's `repo` field carries no canonical slot and is dropped on read; it
// was never load-bearing for the lock.)
func (c *Claim) UnmarshalJSON(data []byte) error {
	var a struct {
		// canonical
		Kind   string `json:"kind"`
		Item   string `json:"item"`
		Owner  string `json:"owner"`
		Branch string `json:"branch"`
		TS     string `json:"ts"`
		// legacy loopengine
		ItemID  string `json:"itemID"`
		Runner  string `json:"runner"`
		Claimed string `json:"claimed"`
		// legacy roster / bash
		Brief   string `json:"brief"`
		Session string `json:"session"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	c.Kind = a.Kind
	c.Item = firstNonEmpty(a.Item, a.ItemID, a.Brief)
	c.Owner = firstNonEmpty(a.Owner, a.Runner, a.Session)
	c.Branch = a.Branch
	c.TS = firstNonEmpty(a.TS, a.Claimed)
	return nil
}

// ClaimConfig carries the claim-directory and stale-policy semantics, reused verbatim
// from loopengine's Config (ClaimsDir / StaleClaim / BranchActive).
type ClaimConfig struct {
	// ClaimsDir is the directory the claim files live in (production:
	// ~/.config/assay/claims). Required.
	ClaimsDir string
	// StaleClaim is the age past which a claim with no provably-live branch may be
	// reclaimed. Zero (the default) is treated as DefaultStaleClaim rather than "instantly
	// stale" — a zero-value footgun would steal live claims.
	StaleClaim time.Duration
	// BranchActive optionally reports whether a stale claim's recorded branch still has
	// live work; nil means a recorded branch cannot be proven live, so a claim with a
	// recorded branch is conservatively NOT reclaimed (it is reclaimed only when it has no
	// branch and is older than StaleClaim). See isStale.
	BranchActive func(branch string) bool
}

func (cfg ClaimConfig) withDefaults() ClaimConfig {
	if cfg.StaleClaim <= 0 {
		cfg.StaleClaim = DefaultStaleClaim
	}
	return cfg
}

// claimPath returns the claim file path for an item in cfg.ClaimsDir. The item ID is
// sanitized to a single path segment so "feature/01" cannot escape the claims dir.
// This MUST stay byte-identical to loopengine.claimPath so a claim written through one
// is found through the other during rollout.
func claimPath(cfg ClaimConfig, item string) string {
	safe := strings.NewReplacer("/", "_", string(os.PathSeparator), "_", "..", "_").Replace(item)
	return filepath.Join(cfg.ClaimsDir, safe+".claim")
}

// claimLock is a held flock over the whole claims directory.
type claimLock struct{ f *os.File }

// acquireClaimLock takes the directory-wide flock that serialises the ENTIRE
// create-or-reclaim decision, mirroring cmd/deskevidence/writeflow.go's acquireAuditLock:
// open the lock file O_CREATE|O_RDWR, loop LOCK_EX|LOCK_NB with a bounded deadline,
// sleep 50ms between attempts.
//
// Every non-success return is deskkit.Unverifiable (exit 6) — a lock this process cannot
// hold is NEVER permission to proceed. It NEVER returns (nil, nil).
func acquireClaimLock(dir string) (*claimLock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, Unverifiable("claim: cannot create claims dir", err)
	}
	// The lock file is directory-scoped (".claims.lock") and does NOT end in .claim, so it
	// is never itself listed as a claim. A directory-wide lock serialises fresh claimants
	// AND reclaimers against one another, which is what closes the double-dispatch:
	// no O_EXCL create can race a stale reclaim's in-place rewrite.
	f, err := os.OpenFile(filepath.Join(dir, ".claims.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, Unverifiable("claim: cannot open claims lock", err)
	}
	deadline := time.Now().Add(claimLockWait)
	for {
		lerr := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if lerr == nil {
			return &claimLock{f: f}, nil
		}
		if lerr != syscall.EWOULDBLOCK {
			f.Close()
			return nil, Unverifiable("claim: cannot acquire claims lock", lerr)
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, Unverifiable(fmt.Sprintf(
				"claim: claims lock held >%s (stale lock?) — another claim op may be stuck; retry, or a human clears it",
				claimLockWait), nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *claimLock) release() {
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}

// Acquire flock-serialises the whole create-or-reclaim decision, then:
//
//   - fresh item          → O_CREATE|O_EXCL create the claim → (true, nil);
//   - existing + live      → (false, nil)  collision, do NOT dispatch;
//   - existing + stale      → reclaim IN PLACE (O_WRONLY|O_TRUNC, never Remove) → (true, nil);
//   - lock unheld / unreadable / unwritable → (false, Unverifiable)  fail closed.
//
// The reclaim rewrites the file in place — it is NEVER Remove-then-create. A Remove would
// leave a window in which the claim file does not exist and an unrelated fresh O_EXCL
// create would succeed alongside the reclaim: two winners, one item, double-dispatch. The
// directory-wide flock already excludes concurrent reclaimers; the in-place rewrite closes
// the fresh-claimant gap as well.
func Acquire(cfg ClaimConfig, c Claim) (acquired bool, err error) {
	if cfg.ClaimsDir == "" {
		return false, Unverifiable("claim: Acquire needs a ClaimsDir", nil)
	}
	if c.Item == "" {
		return false, Refused("claim: Acquire needs a non-empty Item")
	}
	cfg = cfg.withDefaults()
	if c.TS == "" {
		c.TS = time.Now().UTC().Format(time.RFC3339)
	}

	lock, lerr := acquireClaimLock(cfg.ClaimsDir)
	if lerr != nil {
		// A lock we could not hold is NOT permission to write. Fail closed (exit 6),
		// never "assume free".
		return false, lerr
	}
	defer lock.release()

	payload, merr := json.Marshal(c)
	if merr != nil {
		return false, Unverifiable("claim: cannot marshal claim", merr)
	}
	path := claimPath(cfg, c.Item)

	f, oerr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if oerr == nil {
		_, werr := f.Write(payload)
		cerr := f.Close()
		if werr != nil {
			return false, Unverifiable("claim: cannot write claim", werr)
		}
		if cerr != nil {
			return false, Unverifiable("claim: cannot close claim", cerr)
		}
		return true, nil
	}
	if !os.IsExist(oerr) {
		return false, Unverifiable("claim: cannot create claim", oerr)
	}

	// A claim already exists — collision unless provably stale.
	stale, serr := isStale(cfg, path)
	if serr != nil {
		return false, serr // unreadable existing claim => fail closed, do not steal
	}
	if !stale {
		return false, nil // live claim held elsewhere
	}

	rf, rerr := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return false, nil // released underneath us; let the next cycle claim it fresh
		}
		return false, Unverifiable("claim: cannot rewrite stale claim", rerr)
	}
	_, werr := rf.Write(payload)
	cerr := rf.Close()
	if werr != nil {
		return false, Unverifiable("claim: cannot write reclaimed claim", werr)
	}
	if cerr != nil {
		return false, Unverifiable("claim: cannot close reclaimed claim", cerr)
	}
	return true, nil
}

// isStale reports whether the existing claim file is older than cfg.StaleClaim AND its
// recorded branch cannot be proven live. A file that cannot be read is an error (fail
// closed) — an unreadable existing claim is never treated as free to steal.
func isStale(cfg ClaimConfig, path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, Unverifiable("claim: cannot stat existing claim", err)
	}
	if time.Since(fi.ModTime()) <= cfg.StaleClaim {
		return false, nil // recent — not stale
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false, Unverifiable("claim: cannot read existing claim", err)
	}
	var rec Claim
	if json.Unmarshal(b, &rec) == nil && rec.Branch != "" {
		// A branch is recorded. Reclaim only if we can PROVE it is not live.
		if cfg.BranchActive != nil && cfg.BranchActive(rec.Branch) {
			return false, nil // branch still has live work — not stale
		}
		if cfg.BranchActive == nil {
			return false, nil // cannot prove inactive => conservative, do not steal
		}
	}
	return true, nil // old, and no live branch to protect
}

// IsStale reports whether the claim for item is reclaimable (old, no live branch). A
// missing claim is not stale (there is nothing to reclaim). An unreadable one is
// Unverifiable (fail closed).
func IsStale(cfg ClaimConfig, item string) (bool, error) {
	if cfg.ClaimsDir == "" {
		return false, Unverifiable("claim: IsStale needs a ClaimsDir", nil)
	}
	cfg = cfg.withDefaults()
	path := claimPath(cfg, item)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, Unverifiable("claim: cannot stat claim", err)
	}
	return isStale(cfg, path)
}

// Release removes an item's claim file (used when a dispatch fails after claiming, so the
// slot is handed back rather than parked behind a stale claim). A missing file is a
// no-op; a genuine removal error is Unverifiable.
func Release(cfg ClaimConfig, item string) error {
	if cfg.ClaimsDir == "" {
		return Unverifiable("claim: Release needs a ClaimsDir", nil)
	}
	if err := os.Remove(claimPath(cfg, item)); err != nil && !os.IsNotExist(err) {
		return Unverifiable("claim: cannot remove claim", err)
	}
	return nil
}

// ReleaseMatching removes an item's claim ONLY IF the on-disk claim still matches the
// expected owner/branch/ts — a compare-and-delete run under the same directory-wide flock
// every reclaimer takes. It exists for crash recovery: recovery reads claims with List (no
// lock), classifies them against the replayed journal, then deletes the ones it owns. In the
// window between the List and the delete another consumer can legitimately reclaim the SAME
// item IN PLACE — Acquire rewrites owner/branch/ts under the flock — and an unconditional
// Release would then delete that fresh, live claim, reopening the very double-dispatch window
// Acquire's in-place-never-Remove rule closes. Comparing the ts (which every reclaim rewrites)
// under the lock closes it here too.
//
//   - (true, nil)  the claim still matched what was classified and was removed;
//   - (false, nil) the claim no longer matches (reclaimed/rewritten underneath us) or is
//     already gone — nothing removed, and neither is an error;
//   - (false, err) the lock could not be held, or the file could not be read/removed — fail
//     closed, never a silent success.
func ReleaseMatching(cfg ClaimConfig, want Claim) (bool, error) {
	if cfg.ClaimsDir == "" {
		return false, Unverifiable("claim: ReleaseMatching needs a ClaimsDir", nil)
	}
	if want.Item == "" {
		return false, Refused("claim: ReleaseMatching needs a non-empty Item")
	}
	cfg = cfg.withDefaults()

	lock, lerr := acquireClaimLock(cfg.ClaimsDir)
	if lerr != nil {
		return false, lerr // a lock we could not hold is NEVER permission to delete
	}
	defer lock.release()

	path := claimPath(cfg, want.Item)
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return false, nil // already gone — nothing to remove, not an error
		}
		return false, Unverifiable("claim: ReleaseMatching cannot read existing claim", rerr)
	}
	var have Claim
	if json.Unmarshal(b, &have) != nil {
		// A claim we cannot parse is one we cannot prove is the one we classified — leave it.
		return false, Unverifiable("claim: ReleaseMatching cannot parse existing claim", nil)
	}
	if have.Owner != want.Owner || have.Branch != want.Branch || have.TS != want.TS {
		return false, nil // reclaimed/rewritten between the List and now — do NOT remove it
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return false, Unverifiable("claim: ReleaseMatching cannot remove claim", err)
	}
	return true, nil
}

// List reads every claim file in cfg.ClaimsDir through the tolerant reader, so a display
// consumer (deskroster, deskclaim list) sees every claim regardless of which writer wrote
// it. A MISSING directory is empty history (nil, nil). Unlike Acquire's lock — which fails
// closed — List is a best-effort DISPLAY read: an individual unreadable or malformed file
// is skipped, because a single bad file must not blank the whole roster. A directory that
// cannot be read at all is Unverifiable.
func List(cfg ClaimConfig) ([]Claim, error) {
	if cfg.ClaimsDir == "" {
		return nil, Unverifiable("claim: List needs a ClaimsDir", nil)
	}
	entries, err := os.ReadDir(cfg.ClaimsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, Unverifiable("claim: cannot read claims dir", err)
	}
	var out []Claim
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".claim") {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(cfg.ClaimsDir, e.Name()))
		if rerr != nil {
			continue // best-effort display read — skip an unreadable file
		}
		var c Claim
		if json.Unmarshal(data, &c) != nil {
			continue // skip a malformed file
		}
		out = append(out, c)
	}
	return out, nil
}

// firstNonEmpty returns the first non-empty string, or "" if all are empty. It is the
// accessor fallback the tolerant read is built on.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
