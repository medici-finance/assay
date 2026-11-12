package main

import (
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

// auditLock is the flock held across AllowWrite→remote-call→audit-append,
// so two parallel desk sessions cannot double-commit or double-count. A lock held longer
// than lockWait is treated as stale → Unverifiable (exit 6).
type auditLock struct{ l *deskkit.AuditLock }

// lockWait is how long acquireAuditLock will wait for a contended lock before refusing.
// It is a package var for ONE reason: so a test can shorten it and prove the deadline
// exists (removing it survived every behavioural test — #406). There is
// deliberately no flag and no env override. Shortening it in production would not make
// the tool faster, it would make it give up on a legitimately busy lock, and giving up
// early is one step from "assume free" — the exact failure a fail-closed tool forbids. 60s stays.
var lockWait = 60 * time.Second

func acquireAuditLock() (*auditLock, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	// The one choke point for the audit lock: it records the holder's pid and start beside the lock,
	// and a lock still busy after the wait names the lock file, that holder and the clearing action.
	l, err := deskkit.AcquireAuditLock(dir, lockWait)
	if err != nil {
		return nil, err
	}
	return &auditLock{l: l}, nil
}

func (l *auditLock) release() { l.l.Release() }

// runOutward is the single orchestration path for deskevidence's one mutating verb.
// Guard() has already run in main. It holds the flock across the WHOLE of cmdEvidence:
//
//	validation → remote fetch → AllowWrite → commitFile → audit append
//
// so the rate-limit read, the remote call and the audit write are one critical section.
// Two concurrent invocations therefore cannot each read a budget below the cap and both
// commit — which is exactly what happened before this landed (#227), because
// deskevidence's write window was the only outward-write window in the desk-tools set with
// no lock over it.
//
// This is a serialisation fix, not a retry: the second caller does not race and lose, it
// waits for the lock and then re-reads the budget it was going to spend. If it cannot
// acquire the lock inside 60s it is Unverifiable (exit 6) — never "assume free".
//
// Defer ORDER is load-bearing. Go runs defers LIFO, so `lock.release()` (registered
// FIRST) runs LAST — i.e. after `ac.finalize`. Registering them the other way round
// would release the lock before the audit line is appended and leave the append outside
// the critical section, which is the specific window #227 is about. Do not reorder.
func runOutward(args []string) (err error) {
	ac := &auditCtx{verb: "commit"}
	if len(args) > 0 {
		ac.repo = args[0]
	}

	lock, lerr := acquireAuditLock()
	if lerr != nil {
		// Still exactly one audit line for this invocation. It is written WITHOUT
		// the lock — which is sound, because nothing outward happened and an append to
		// audit.jsonl is O_APPEND-atomic; the lock exists to serialise the read-then-write
		// sequence, not any single append.
		ac.finalize(lerr)
		return lerr
	}
	defer lock.release()
	defer func() { ac.finalize(err) }()

	// Capability admission is before landing reads, credentials and effects, but
	// inside the existing audit window: a refused attempt still leaves one row.
	var diagnostic strings.Builder
	if code := deskkit.RefuseIfTreeV2BelowV1(deskkit.RootsFromArgs(args), deskkit.EffectiveToolVersion(version), "deskevidence", &diagnostic); code != 0 {
		return capabilityError(code, diagnostic.String())
	}
	return cmdEvidence(args, ac)
}
