package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// toolName is the fixed audit/rate-limit key for this binary. A constant (rather than
// os.Args[0]) keeps AllowWrite's per-tool counting and every audit line aligned to the
// same key regardless of how the binary was invoked.
const toolName = "deskrelease"

// stdout/stderr are package vars so tests can capture them.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// writeResult is what planCut returns: the outcome to audit and the exit status.
// err is nil for ok/noop and a *deskkit.DeskError otherwise (its code drives the exit).
type writeResult struct {
	verb   string // discriminated verb for the audit ("cut" before the tag is known, then "cut:<tag>")
	sha    string // the commit involved — "" until known (early refusals)
	result string // deskkit.Result* value
	detail string
	err    error
}

func fromErr(verb, sha string, err error) writeResult {
	return writeResult{verb: verb, sha: sha, result: resultForErr(err), detail: err.Error(), err: err}
}

func noop(verb, sha, detail string) writeResult {
	return writeResult{verb: verb, sha: sha, result: deskkit.ResultNoop, detail: detail}
}

// dryNoop is the outcome of a `--dry-run`: exit 0, one audit line, and a result class
// that feeds NEITHER rate-limit meter.
//
// It is separate from noop because the two are different facts. A `noop` means the act
// was attempted and found already done — non-progress, which is exactly what the circuit
// breaker is meant to count. A `dryrun` means the act was never attempted: nothing was
// written, so five rehearsals must not open a 15-minute breaker against the real cut,
// which is what they did while a dry run audited `noop`.
//
// The ONLY caller is the a.dryRun branch of planCut, which returns before createTagRef.
// That is the property that makes the exemption safe rather than a laundering route: the
// flag that selects this result is the same flag that suppresses the write, so there is
// no argv for which a caller gets both the exemption and a ref. TestDryRunResultIsOnlyReachableWithoutAWrite
// pins it.
func dryNoop(verb, sha, detail string) writeResult {
	return writeResult{verb: verb, sha: sha, result: deskkit.ResultDryRun, detail: detail}
}

func done(verb, sha, detail string) writeResult {
	return writeResult{verb: verb, sha: sha, result: deskkit.ResultOK, detail: detail}
}

func resultForErr(err error) string {
	switch {
	case err == nil:
		return deskkit.ResultOK
	case deskkit.IsRateLimited(err):
		return deskkit.ResultRateLimited
	case deskkit.IsRefused(err):
		return deskkit.ResultRefused
	case deskkit.IsDisabled(err):
		return deskkit.ResultDisabled
	default:
		return deskkit.ResultUnverifiable
	}
}

// short renders a SHA prefix for human-readable messages; the FULL sha still goes to the
// audit HeadSHA field.
func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// deskDir delegates to the exported canonical resolver deskkit.StateDir() — the state
// directory (~/.config/assay) where the outward-write flock file is placed. Tests redirect
// it by setting $HOME.
func deskDir() (string, error) {
	return deskkit.StateDir()
}

// auditLock is the flock held across LoadEntries→remote-call→audit-append, so two
// parallel desk sessions cannot double-cut or double-count. A lock held >60s is treated
// as stale → Unverifiable (exit 6).
type auditLock struct{ f *os.File }

func acquireAuditLock() (*auditLock, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, deskkit.Unverifiable("cannot create desk-tools dir", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot open audit lock", err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		lerr := deskkit.TryLockExclusive(f)
		if lerr == nil {
			return &auditLock{f: f}, nil
		}
		if !errors.Is(lerr, deskkit.ErrLockBusy) {
			f.Close()
			return nil, deskkit.Unverifiable("cannot acquire audit lock", lerr)
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, deskkit.Unverifiable(
				"audit lock held >60s (stale lock?) — another desk write may be stuck; retry, or a human clears it", nil)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *auditLock) release() {
	_ = deskkit.UnlockFile(l.f)
	_ = l.f.Close()
}

// runOutward is the single orchestration path for the mutating verb. Guard() has already
// run in main. It holds the flock across:
//
//	LoadEntries → AllowWrite → plan(validation + remote calls) → audit append
//
// so the rate-limit read, the remote calls, and the audit write are one critical
// section. Every exit path writes exactly one audit line — including refusals,
// which is what lets the gates see a loop at all.
//
// WHICH gate a refusal feeds: a refusal is audited
// `refused`, which does NOT charge the outward-write budget (no ref was created, so
// there was no remote amplification to cap) and DOES count toward the non-progress
// circuit breaker. "A refusal loop must be stopped" is therefore enforced by the
// breaker, not by the shared hourly counter. A `--dry-run` is audited `dryrun` and feeds
// NEITHER meter — see cut.go's dryNoop and deskkit.ResultDryRun.
func runOutward(args []string, plan func(entries []deskkit.Entry) writeResult) int {
	lock, err := acquireAuditLock()
	if err != nil {
		return finishAudit(args, writeResult{verb: toolName, result: deskkit.ResultUnverifiable, detail: err.Error(), err: err})
	}
	defer lock.release()

	entries, lerr := deskkit.LoadEntries()
	if lerr != nil {
		return finishAudit(args, writeResult{verb: toolName, result: deskkit.ResultUnverifiable, detail: lerr.Error(), err: lerr})
	}
	// Scope the gate on the compiled-in repo — the same slug finishAudit records on
	// every line, so the meter reads back exactly what this tool writes. Passing "" here
	// skipped BOTH tiers and left release cuts bounded only by the breaker, which counts
	// consecutive NON-progress and so never trips on a run of successful releases
	// (surfaced in review). A release has no PR, so pr=0 puts it in the repo's
	// unnumbered bucket at the per-PR cap — the same bound this tool has always had
	// (current value and provenance: RateLimitPerPRPerHour in ratelimit.go).
	if aerr := deskkit.AllowWrite(toolName, repoSlug, 0); aerr != nil {
		return finishAudit(args, writeResult{verb: toolName, result: resultForErr(aerr), detail: aerr.Error(), err: aerr})
	}

	return finishAudit(args, plan(entries))
}

// finishAudit writes the audit line and returns the process exit code. No token material
// ever reaches this record — only the discriminated verb, an args digest, and the sha.
func finishAudit(args []string, wr writeResult) int {
	var shaPtr *string
	if wr.sha != "" {
		s := wr.sha
		shaPtr = &s
	}
	e := deskkit.Entry{
		Tool:       toolName,
		Verb:       wr.verb,
		ArgsDigest: deskkit.ArgsDigest(args),
		Repo:       repoSlug,
		PR:         nil, // a release cut has no PR
		HeadSHA:    shaPtr,
		Result:     wr.result,
		Detail:     wr.detail,
	}
	if err := deskkit.Log(e); err != nil {
		fmt.Fprintf(stderr, "deskrelease: WARNING: could not write audit line: %v\n", err)
	}

	if wr.err != nil {
		fmt.Fprintln(stderr, "deskrelease: "+wr.detail)
		return deskkit.ExitCodeOf(wr.err)
	}
	if wr.detail != "" {
		fmt.Fprintln(stdout, "deskrelease: "+wr.detail)
	}
	return deskkit.ExitOK
}
