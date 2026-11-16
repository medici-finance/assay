package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskDir delegates to the exported canonical resolver deskkit.StateDir() — the state
// directory (~/.config/assay) where the outward-write flock file is placed. Tests redirect
// it (and deskkit's audit log) by setting $HOME.
//
// The lock FILE is the same path across deskevidence/deskpost/deskrelease, so they
// serialise against each other — which is what matters, because they share one audit.jsonl.
// This helper (and its two siblings in cmd/deskpost/writeflow.go, cmd/deskrelease/writeflow.go)
// now delegates to deskkit.StateDir() so the path can never drift from the canonical one.
func deskDir() (string, error) {
	return deskkit.StateDir()
}

// auditLock is the flock held across AllowWrite→remote-call→audit-append (enterWriteWindow),
// so two parallel desk sessions cannot double-commit or double-count. A lock held longer
// than lockWait is treated as stale → Unverifiable (exit 6).
type auditLock struct{ l *deskkit.AuditLock }

// lockWait is how long ONE run may wait for its locks, in total, before refusing. The
// preparation lock, the audit lock of the write window and the audit lock of a refusal's
// append all draw on the same budget (auditCtx.waitLeft), so a run waits at most lockWait
// however many of them are busy; an acquisition left with no budget makes one attempt.
// Only time spent waiting counts: preparation work between the locks does not.
// It is a package var for ONE reason: so a test can shorten it and prove the deadline
// exists (removing it survived every behavioural test — #406). There is
// deliberately no flag and no env override. Shortening it in production would not make
// the tool faster, it would make it give up on a legitimately busy lock, and giving up
// early is one step from "assume free" — the exact failure a fail-closed tool forbids. 60s stays.
var lockWait = 60 * time.Second

func acquireAuditLock(wait time.Duration) (*auditLock, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	// The one choke point for the audit lock: it records the holder's pid and start beside the lock,
	// and a lock still busy after the wait names the lock file, that holder and the clearing action.
	l, err := deskkit.AcquireAuditLock(dir, wait)
	if err != nil {
		return nil, err
	}
	return &auditLock{l: l}, nil
}

func (l *auditLock) release() { l.l.Release() }

// enterWriteWindow is the ONE place deskevidence takes the shared audit lock and spends its
// write budget: every landing calls it immediately before its conditional forge write, and the
// lock it takes is then held through that write, the attribution read and the deferred audit
// append (finalize), so the read-check-write-append window stays one critical section exactly
// as #227 made it.
//
// The lock is NOT held across preparation. The admission reads, the remote snapshot, the
// closure and decision-gate reads and the two statusgen lint runs used to run inside it, so one
// landing kept the desk-wide lock for as long as its lint took and starved every other desk
// writer that shares it — the intake drain and reviewer posts (#5023). Nothing preparation
// reads is protected by this lock: the budget ledger is read here, under it, and the remote
// target is protected by the write itself, which is conditional on the blob preparation read
// (ExpectedSHA) and refuses when it moved. Preparation's own exclusion — two landings staging
// into the same landing root — is the per-root preparation lock runOutward holds instead.
//
// A second call is a no-op on the lock (a GitLab side-branch fallback spends no budget).
// The wait draws on the run's one lockWait budget (waitLeft).
func (a *auditCtx) enterWriteWindow(tool, repo string, pr int) error {
	if a.lock == nil {
		start := time.Now()
		l, err := acquireAuditLock(a.waitLeft())
		a.lockWaited += time.Since(start)
		if err != nil {
			return err
		}
		a.lock = l
	}
	// Re-validate what preparation read, once, under the lock and before the budget is spent:
	// a refusal here spends nothing and writes nothing.
	if !a.rechecked {
		a.rechecked = true
		for _, check := range a.rechecks {
			if err := check(); err != nil {
				return err
			}
		}
	}
	return evidenceWriteBudget(tool, repo, pr)
}

// lockForAppend takes the audit lock for the single audit append when the run ended before
// its write window (a refusal, a noop, a dry-run), so that append is serialised against the
// other writers' read-then-append exactly as it was when the lock spanned the whole run. A
// lock it cannot get is not a reason to drop the line: the append is O_APPEND-atomic and the
// line records an invocation that wrote nothing, so it is appended without the lock, the
// same rule the lock-failure path has always followed.
//
// It waits only for what is left of the run's lockWait budget. A run that already spent the
// budget on a busy lock (the write window's, or the preparation lock's) makes one attempt and
// does not wait a second lockWait for its refusal row.
func (a *auditCtx) lockForAppend() {
	if a.lock != nil {
		return
	}
	start := time.Now()
	l, err := acquireAuditLock(a.waitLeft())
	a.lockWaited += time.Since(start)
	if err == nil {
		a.lock = l
	}
}

// waitLeft is what remains of this run's lockWait budget; zero means one attempt, no wait.
func (a *auditCtx) waitLeft() time.Duration {
	if left := lockWait - a.lockWaited; left > 0 {
		return left
	}
	return 0
}

// releaseAuditLock drops the audit lock if this run took it. Safe when it never did.
func (a *auditCtx) releaseAuditLock() {
	if a.lock != nil {
		a.lock.release()
		a.lock = nil
	}
}

// prepLockName is the per-landing-root preparation lock's file name in the desk state dir:
// one file per canonical root key (prepLockKey), so two landings on DIFFERENT roots never
// wait on each other and two on the SAME root still cannot stage into it at once.
func prepLockName(key string) string {
	return "evidence-prep-" + deskkit.Sha256Hex([]byte(key))[:16] + ".lock"
}

// prepLockKey is the canonical identity of a landing root: the absolute path with symlinks
// resolved, then the directory's own identity (rootIdentity: device and inode where the
// platform has them, a case-folded path on Windows). Two spellings of one directory — a
// relative path, a symlink, a re-cased path on a case-insensitive filesystem — therefore
// take ONE lock. shown is the resolved path, for messages.
func prepLockKey(root string) (key, shown string, err error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", "", deskkit.Unverifiable("cannot resolve the landing root for the preparation lock", err)
	}
	if real, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = real
	}
	return rootIdentity(abs), abs, nil
}

// enterPrep takes the preparation lock for one landing root, drawing on the run's one lockWait
// budget. cmdEvidence calls it with the parsed --root before admission and again with the
// root the landing stages into once admission has named it (they differ for a cross-repo
// run); a root whose canonical key this run already holds is not taken twice.
func (a *auditCtx) enterPrep(root string) error {
	key, shown, err := prepLockKey(root)
	if err != nil {
		return err
	}
	if a.prepKeys[key] {
		return nil
	}
	start := time.Now()
	p, err := acquirePrepLock(key, shown, a.waitLeft())
	a.lockWaited += time.Since(start)
	if err != nil {
		return err
	}
	if a.prepKeys == nil {
		a.prepKeys = map[string]bool{}
	}
	a.prepKeys[key] = true
	a.prep = append(a.prep, p)
	return nil
}

// releasePrep drops every preparation lock this run took, newest first.
func (a *auditCtx) releasePrep() {
	for i := len(a.prep) - 1; i >= 0; i-- {
		a.prep[i].release()
	}
	a.prep, a.prepKeys = nil, nil
}

// addRecheck registers a check that enterWriteWindow re-runs under the audit lock, immediately
// before the write: preparation's reads that the write relies on (admission, the local input,
// visibility, the board and the decision-gate verdict) are re-read there and the landing
// refuses when one changed. Preparation itself runs without the audit lock (#5023), so nothing
// it read may be carried into the write unchecked.
func (a *auditCtx) addRecheck(fn func() error) {
	a.rechecks = append(a.rechecks, fn)
}

// changedSincePrep is the refusal for an input that no longer matches what preparation read.
// A re-check that could not be made at all keeps its could-not-check class.
func changedSincePrep(what string, err error) error {
	msg := "refused: " + what + " changed between preparation and the write window; nothing was written — re-run the landing"
	if err == nil {
		return deskkit.Refused(msg)
	}
	if deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
		return deskkit.Refused(msg + ": " + err.Error())
	}
	return deskkit.Unverifiable(what+" could not be re-checked in the write window; nothing was written", err)
}

// prepLock is a held per-root preparation lock.
type prepLock struct {
	held *deskkit.HeldLock
	dir  *os.Root
}

func (p *prepLock) release() {
	if p == nil {
		return
	}
	_ = p.held.Release()
	_ = p.dir.Close()
}

// acquirePrepLock serialises preparation on one landing root. The PROBLEM-diff guard, the
// decision-gate judge and the verified-sidecar gate stage the landing's content INTO that
// root and restore it afterwards (lintdiff.go, decisiongate.go), so two landings on the same
// root would read each other's staged bytes. The audit lock used to serialise them as a side
// effect of spanning the whole run; this lock keeps that exclusion, scoped to the root it
// protects, with the same bounded wait and the same fail-closed refusal. A lock still busy
// after the wait is contention, classed exactly as the audit lock's (#6227): the audit line
// says contended, which neither the write budget nor the circuit breaker counts.
func acquirePrepLock(key, abs string, wait time.Duration) (*prepLock, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, deskkit.Unverifiable("cannot create the desk state dir", err)
	}
	d, err := os.OpenRoot(dir)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot open the desk state dir", err)
	}
	name := prepLockName(key)
	f, err := d.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		d.Close()
		return nil, deskkit.Unverifiable("cannot open the preparation lock", err)
	}
	held, err := deskkit.AcquireHeld(d, name, f, deskkit.LockSpec{Name: "deskevidence preparation lock for " + abs, Wait: wait})
	if err != nil {
		d.Close()
		var busy *deskkit.BusyError
		if errors.As(err, &busy) {
			return nil, deskkit.AuditLockContended(fmt.Sprintf(
				"another deskevidence landing is preparing on %s (preparation lock %s busy for %s; retryable)",
				abs, busy.Path, busy.Waited.Round(time.Second)))
		}
		return nil, deskkit.Unverifiable("cannot acquire the preparation lock", err)
	}
	return &prepLock{held: held, dir: d}, nil
}

// runOutward is the single orchestration path for deskevidence's one mutating verb.
// Guard() has already run in main. It takes two locks with two different scopes:
//
//	per-root preparation lock: validation → remote reads → staged lint/gates → … → audit append
//	shared audit lock:                    enterWriteWindow: AllowWrite → forge write → audit append
//
// The shared audit lock covers ONLY the write window (enterWriteWindow), so the rate-limit
// read, the remote write and the audit append are one critical section — two concurrent
// invocations cannot each read a budget below the cap and both commit (#227) — while the
// long preparation no longer holds the lock every other desk writer needs (#5023).
//
// Because preparation runs without the audit lock, the write window does not trust what it
// read: every input the write relies on (admission and its attestation, the local input
// pinned by digest, visibility, the on-behalf-of principal, the board and the decision-gate
// verdict; on the outcome path the record bytes, join facts and redaction) is re-read there
// through the uncached forge, and a change refuses before the budget is spent (addRecheck).
// Only the expensive staged gates stay in preparation, their inputs pinned by those re-reads.
// The preparation lock is keyed on the parsed --root (and the root admission names), made
// canonical by rootIdentity; a busy one is contention (#6227), never a could-not-check.
//
// This is a serialisation fix, not a retry: the second caller does not race and lose, it
// waits for the lock and then re-reads the budget it was going to spend. If it cannot
// acquire its locks inside 60s of waiting in total (one lockWait budget shared by the
// preparation lock and the audit lock) it is Unverifiable (exit 6) — never "assume free".
//
// Defer ORDER is load-bearing. Go runs defers LIFO, so `ac.releaseAuditLock()` (registered
// FIRST) runs LAST — i.e. after `ac.finalize`. Registering them the other way round
// would release the lock before the audit line is appended and leave the append outside
// the critical section, which is the specific window #227 is about. Do not reorder.
func runOutward(args []string, options childOptions) (err error) {
	ac := &auditCtx{verb: "commit", options: options}
	if len(args) > 0 {
		ac.repo = args[0]
	}

	// The preparation locks are taken inside cmdEvidence (ac.enterPrep), from the PARSED --root
	// and the root admission names, so the lock is keyed on the root the landing actually uses.
	// They are released last, after the audit append.
	defer ac.releasePrep()
	defer ac.releaseAuditLock()
	defer func() { ac.lockForAppend(); ac.finalize(err) }()

	// Capability preflight (RefuseIfTreeV2BelowV1) runs inside cmdEvidence, under the
	// preparation lock on the parsed --root it reads.
	return cmdEvidence(args, ac)
}

// onceString is a string flag that counts how often it was set, so a repeated flag is refused
// instead of silently taking its last value.
type onceString struct {
	value string
	count int
}

func (s *onceString) String() string { return s.value }

func (s *onceString) Set(v string) error {
	s.value = v
	s.count++
	return nil
}

// sameBoard reports whether two board reads carry the same edits in the same order.
func sameBoard(a, b []boardEdit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].Delete != b[i].Delete || !bytes.Equal(a[i].Content, b[i].Content) {
			return false
		}
	}
	return true
}

// addVisibilityRecheck re-runs the public-repo write gate in the write window through the
// forge without the publication snapshot, whose visibility answer was cached at preparation.
func addVisibilityRecheck(ac *auditCtx, owner, name string) {
	ac.addRecheck(func() error {
		if err := publicRepoGateFn(deskkit.ForgeRepoInfoFetcher{Forge: ac.freshForge}, owner, name); err != nil {
			return changedSincePrep("the repository's visibility", err)
		}
		return nil
	})
}
