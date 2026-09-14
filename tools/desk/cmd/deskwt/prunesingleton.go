package main

// The prune singleton.
//
// `deskwt prune` is a BOOT STEP: every desk window runs it as it starts. Nothing
// coordinated those runs, so N windows booting together ran N identical full sweeps over
// the same repository at once — measured at five concurrent sweeps, 1.64x per-sweep
// contention and no serialisation, for one sweep's worth of useful work (issue #1037).
//
// Two mechanisms, answering two different questions, sharing one stamp file:
//
//   - CONCURRENCY — a non-blocking exclusive advisory lock (deskkit.TryLockExclusive) on
//     the stamp. A sweep that cannot take it does not sweep. The KERNEL releases an
//     advisory lock when its holder exits, however it exits — normally, on a panic, on
//     SIGKILL — so there is no liveness question to answer, no pid to probe, no process
//     start time to compare and nothing to time out. This is deliberately NOT a
//     pid-plus-TTL stamp: that design has to decide whether a recorded pid is still the
//     process that wrote it, and every wrong answer either wedges the boot step of every
//     window on the machine or lets two sweeps delete concurrently.
//
//   - RECENCY — a TTL debounce over the stamp's recorded finish time. The five windows in
//     the report did not all overlap: some arrived seconds after the first sweep finished,
//     found nothing holding them, and swept the same repository again. A sweep that takes
//     the lock and finds a COMPLETED sweep newer than the TTL skips.
//
// The TTL half FAILS OPEN and the lock half FAILS CLOSED, and the asymmetry is the whole
// point. A stamp that is missing, empty, truncated, not JSON, of an unknown schema, or
// carrying a future or unparseable timestamp is treated as NO STAMP and the sweep
// PROCEEDS. So the worst a corrupt or abandoned stamp can cost is one delayed sweep, for
// at most the TTL — it can never wedge prune, which is exactly the stale-lock class this
// file exists not to recreate. The lock, by contrast, is never assumed free: ErrLockBusy
// means another sweep is live, and deskkit's own documentation says to read it that way.
//
// The singleton is an EFFICIENCY mechanism, never a safety control. Every removal gate in
// prune.go runs unchanged inside whichever sweep does take the lock.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// pruneStampSchema is the stamp's format marker. A stamp carrying anything else is read
// as no stamp (fail open) rather than parsed on a guess.
const pruneStampSchema = "deskwt-prune-singleton-v1"

// defaultSingletonTTL is the debounce window: a sweep that completed less recently than
// this does not hold a new one. Ten minutes is chosen against the observed pattern — a
// burst of desk windows booting inside a minute or two — and is short enough that an
// operator who re-runs prune deliberately after a real change is not blocked for long.
const defaultSingletonTTL = 10 * time.Minute

// pruneStamp is the singleton's on-disk record. It carries no path, no branch, no ref and
// no identity in clear text: repoHash is a digest, and pid plus the two timestamps exist
// only so a held sweep can NAME its holder in the line it prints.
type pruneStamp struct {
	Schema     string `json:"schema"`
	RepoHash   string `json:"repoHash"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

// pruneSingleton is a held lock plus the stamp path it was taken on.
type pruneSingleton struct {
	file      *os.File
	path      string
	startedAt time.Time
}

// singletonOutcome says what the acquisition decided, so the caller can print the right
// line and pick the right exit path.
type singletonOutcome int

const (
	// singletonAcquired — this process holds the lock and should sweep.
	singletonAcquired singletonOutcome = iota
	// singletonHeld — another live sweep holds the lock; do not sweep.
	singletonHeld
	// singletonRecent — no live holder, but a sweep finished inside the TTL; do not sweep.
	singletonRecent
	// singletonUnavailable — the stamp directory could not be resolved or created. The
	// sweep PROCEEDS without a singleton: an efficiency mechanism that cannot be set up
	// must not stop the work it was meant to deduplicate.
	singletonUnavailable
)

// pruneStampDir is the directory the stamps live in. It is under the shared desk state
// directory (~/.config/assay), never inside the target repository — which is what keeps
// `--dry-run` able to promise it writes nothing at all inside the repo, and what keeps a
// read-only repository prunable.
func pruneStampDir() (string, error) {
	base, err := deskkit.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "prune"), nil
}

// pruneRepoKey digests the resolved repository path. A digest rather than the path itself
// for two independent reasons: no path character can escape into a filename, and the stamp
// (which is world-readable to the user's own tooling) names no directory layout.
func pruneRepoKey(repoPath string) string {
	sum := sha256.Sum256([]byte(repoPath))
	return hex.EncodeToString(sum[:])
}

// readPruneStamp reads and validates the stamp at path. EVERY failure — absent,
// unreadable, empty, not JSON, wrong schema — returns (nil, false): "no usable stamp", not
// an error. This is the fail-open half; see the file header.
func readPruneStamp(path string) (*pruneStamp, bool) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	var st pruneStamp
	if jerr := json.Unmarshal(b, &st); jerr != nil {
		return nil, false
	}
	if st.Schema != pruneStampSchema {
		return nil, false
	}
	return &st, true
}

// sweptWithin reports whether the stamp records a COMPLETED sweep that finished no longer
// than ttl ago, and how long ago that was. A missing, unparseable or FUTURE FinishedAt is
// "not within" — a clock that went backwards must not be able to hold prune indefinitely.
func (st *pruneStamp) sweptWithin(ttl time.Duration, now time.Time) (bool, time.Duration) {
	if st == nil || ttl <= 0 || st.FinishedAt == "" {
		return false, 0
	}
	fin, err := time.Parse(time.RFC3339, st.FinishedAt)
	if err != nil {
		return false, 0
	}
	age := now.Sub(fin)
	if age < 0 || age >= ttl {
		return false, 0
	}
	return true, age
}

// runningFor renders how long the stamp's recorded sweep has been running, for the
// held-by message only. An unparseable StartedAt yields "unknown" rather than a
// fabricated duration.
func (st *pruneStamp) runningFor(now time.Time) string {
	if st == nil || st.StartedAt == "" {
		return "unknown"
	}
	started, err := time.Parse(time.RFC3339, st.StartedAt)
	if err != nil {
		return "unknown"
	}
	d := now.Sub(started)
	if d < 0 {
		return "unknown"
	}
	return roundAge(d)
}

// acquirePruneSingleton takes the singleton for the repository rooted at repoPath.
//
// ttl <= 0 disables the RECENCY debounce only; the lock is always taken and can never be
// disabled — there is no --force in this verb and this adds none.
func acquirePruneSingleton(repoPath string, ttl time.Duration, now time.Time) (*pruneSingleton, singletonOutcome, string) {
	dir, err := pruneStampDir()
	if err != nil {
		return nil, singletonUnavailable, "prune singleton unavailable (cannot resolve the desk state directory): " + err.Error()
	}
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return nil, singletonUnavailable, "prune singleton unavailable (cannot create " + dir + "): " + mkErr.Error()
	}
	path := filepath.Join(dir, pruneRepoKey(repoPath)+".stamp")

	f, oerr := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if oerr != nil {
		return nil, singletonUnavailable, "prune singleton unavailable (cannot open the stamp): " + oerr.Error()
	}
	if lerr := deskkit.TryLockExclusive(f); lerr != nil {
		// FAIL CLOSED. ErrLockBusy is another live sweep; any other lock error is a state
		// this process cannot positively verify, and an unverifiable lock is never a grant.
		st, _ := readPruneStamp(path)
		_ = f.Close()
		if st != nil {
			return nil, singletonHeld, fmt.Sprintf("held by pid %d, running %s", st.PID, st.runningFor(now))
		}
		return nil, singletonHeld, "held by another sweep"
	}

	// The lock is ours. Only now is the recency question meaningful: a stamp whose holder
	// is still live would have failed the lock above.
	if st, ok := readPruneStamp(path); ok {
		// A sweep recorded by THIS process is never debounced. The recency check exists to
		// collapse a burst of SEPARATE desk windows into one sweep; a second sweep inside
		// one process is the same caller asking again — an operator re-running after a
		// change, or the --interval supervisor's next tick, whose cadence the operator
		// already stated and which this must not silently override.
		if st.PID != os.Getpid() {
			if within, age := st.sweptWithin(ttl, now); within {
				_ = deskkit.UnlockFile(f)
				_ = f.Close()
				return nil, singletonRecent, fmt.Sprintf("swept %s ago (under the %s singleton TTL)", roundAge(age), ttl)
			}
		}
	}

	s := &pruneSingleton{file: f, path: path, startedAt: now}
	s.write(pruneStamp{
		Schema:    pruneStampSchema,
		RepoHash:  pruneRepoKey(repoPath),
		PID:       os.Getpid(),
		StartedAt: now.UTC().Format(time.RFC3339),
	})
	return s, singletonAcquired, ""
}

// write replaces the stamp's contents. It is best effort: a stamp that cannot be written
// costs a future sweep its debounce, and must never cost this sweep its run.
func (s *pruneSingleton) write(st pruneStamp) {
	if s == nil || s.file == nil {
		return
	}
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	if _, serr := s.file.Seek(0, 0); serr != nil {
		return
	}
	if terr := s.file.Truncate(0); terr != nil {
		return
	}
	if _, werr := s.file.Write(append(b, '\n')); werr != nil {
		return
	}
	_ = s.file.Sync()
}

// release records the finish time and drops the lock. Called once per sweep; the interval
// supervisor calls it at the END OF EVERY TICK rather than holding the lock for its
// lifetime, so a long-lived supervisor can never lock out a boot-time or manual sweep.
func (s *pruneSingleton) release(repoPath string, now time.Time) {
	if s == nil || s.file == nil {
		return
	}
	s.write(pruneStamp{
		Schema:     pruneStampSchema,
		RepoHash:   pruneRepoKey(repoPath),
		PID:        os.Getpid(),
		StartedAt:  s.startedAt.UTC().Format(time.RFC3339),
		FinishedAt: now.UTC().Format(time.RFC3339),
	})
	_ = deskkit.UnlockFile(s.file)
	_ = s.file.Close()
	s.file = nil
}
