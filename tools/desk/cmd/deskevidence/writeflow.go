package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// one file per resolved root, so two landings on DIFFERENT roots never wait on each other and
// two on the SAME root still cannot stage into it at once.
func prepLockName(root string) string {
	return "evidence-prep-" + deskkit.Sha256Hex([]byte(root))[:16] + ".lock"
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
// protects, with the same bounded wait and the same fail-closed refusal.
func acquirePrepLock(args []string, wait time.Duration) (*prepLock, error) {
	root := deskkit.RootsFromArgs(args)[0]
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve the landing root for the preparation lock", err)
	}
	if real, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = real
	}
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
	name := prepLockName(abs)
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
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"another deskevidence landing is preparing on %s (preparation lock %s busy for %s; retryable)",
				abs, busy.Path, busy.Waited.Round(time.Second)), nil)
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

	start := time.Now()
	prep, perr := acquirePrepLock(args, ac.waitLeft())
	ac.lockWaited += time.Since(start)
	if perr != nil {
		// Still exactly one audit line for this invocation; nothing outward happened.
		ac.lockForAppend()
		ac.finalize(perr)
		ac.releaseAuditLock()
		return perr
	}
	defer prep.release()
	defer ac.releaseAuditLock()
	defer func() { ac.lockForAppend(); ac.finalize(err) }()

	// Capability admission is before landing reads, credentials and effects; a refused
	// attempt still leaves one row.
	var diagnostic strings.Builder
	if code := deskkit.RefuseIfTreeV2BelowV1(deskkit.RootsFromArgs(args), deskkit.EffectiveToolVersion(version), "deskevidence", &diagnostic); code != 0 {
		return capabilityError(code, diagnostic.String())
	}
	return cmdEvidence(args, ac)
}
