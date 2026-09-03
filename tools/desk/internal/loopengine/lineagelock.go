package loopengine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lineagelock.go — the boot-time exclusive lock that makes the "one live conductor per
// (Owner, Branch)" premise TRUE instead of merely asserted.
//
// Recover's whole safety argument (journal.go, recover.go) is: a claim under MY lineage
// (Owner==RunnerID, Branch==Branch) whose newest journalled event carries a DIFFERENT runID
// than mine must belong to a PRIOR, crashed run of me — so freeing it is safe. That inference
// is only sound if no OTHER live conductor of the same lineage exists right now; a live
// sibling's in-flight claim is byte-identical to a crashed run's, and reclaiming it is a
// double-dispatch. Nothing in the record can tell the two apart.
//
// A per-lineage exclusive flock decides it. flock is released by the OS when the holding
// process exits, so a CRASHED conductor's lock is already gone at the restart — the restart
// acquires it and recovers. A LIVE sibling still holds it — the restart cannot acquire it and
// therefore does NOT recover (it drains normally, just without touching the sibling's claims).
// The lock is held for the whole Run() lifetime, not just the recovery window, so a sibling
// that is already PAST its own recovery still blocks a later boot.

// lineageLock is a held exclusive flock proving this process is the sole live conductor of a
// (RunnerID, Branch) lineage.
type lineageLock struct{ f *os.File }

// lineageLockPath is the per-lineage lock file inside ClaimsDir. It hashes (RunnerID, Branch)
// to one filesystem-safe segment and does NOT end in .claim, so List never mistakes it for a
// claim. The NUL separator keeps ("ab","c") distinct from ("a","bc").
func lineageLockPath(cfg Config) string {
	sum := sha256.Sum256([]byte(cfg.RunnerID + "\x00" + cfg.Branch))
	return filepath.Join(cfg.ClaimsDir, ".lineage-"+hex.EncodeToString(sum[:8])+".lock")
}

// acquireLineageLock takes a NON-BLOCKING exclusive flock for this run's lineage.
//
//   - (lock, true)  this process is the sole live conductor of the lineage — recovery is safe;
//   - (nil, false)  another live conductor of the SAME lineage holds it (its in-flight claims
//     are live, not crashed — recovery must NOT run), OR the lineage cannot be anchored /
//     locked at all. Either way the answer is the conservative one: do not recover. A lock we
//     cannot take is never read as "no sibling is live".
//
// The lock is non-blocking on purpose: waiting would mean waiting out a live sibling's entire
// drain, and the only safe action while a sibling is live is to skip recovery, not to queue
// behind it.
func acquireLineageLock(cfg Config) (*lineageLock, bool) {
	// No lineage to anchor => classify can never classify anything anyway; nothing to lock.
	if cfg.ClaimsDir == "" || cfg.RunnerID == "" || cfg.Branch == "" {
		return nil, false
	}
	if err := os.MkdirAll(cfg.ClaimsDir, 0o700); err != nil {
		return nil, false
	}
	f, err := os.OpenFile(lineageLockPath(cfg), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false
	}
	if lerr := deskkit.TryLockExclusive(f); lerr != nil {
		// ErrLockBusy: a live sibling holds it. Any other error: cannot prove sole-liveness.
		// Both fail closed — do not recover.
		_ = f.Close()
		return nil, false
	}
	return &lineageLock{f: f}, true
}

// release drops the flock and closes the fd. A nil receiver is a no-op so callers can defer it
// unconditionally.
func (l *lineageLock) release() {
	if l == nil {
		return
	}
	_ = deskkit.UnlockFile(l.f)
	_ = l.f.Close()
}
