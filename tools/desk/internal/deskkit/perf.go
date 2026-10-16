package deskkit

// perf.go — the desk kit's side of the opt-in, LOCAL-ONLY per-invocation performance record.
//
// The record's schema, writer, day-file layout, retention, install id and CPU reading live in
// the shared perfrec module (github.com/medici-finance/assay/perfrec), the one writer that
// statusgen also calls; this file keeps what is specific to the desk tools — the tool, loop and
// verb rosters and their mappers, the child/forge counters, PerfRun and PerfStep. The contract
// below is perfrec's, restated here because every clause still binds this side.
//
// The audit ledger (audit.go) records WHAT a verb did; nothing recorded how long it took or
// what it cost. This file adds that as a SEPARATE dataset rather than as a field on the
// ledger's Entry: the ledger is load-bearing state (ratelimit.go counts its rows,
// idempotent.go reads them), so its schema stays exactly as it is, and the perf history is
// the opposite in every respect — opt-in, pruned to 7 UTC days on write, and read by no
// control at all.
//
// The contract, each clause enforced by a test rather than promised:
//
//   - OFF BY DEFAULT. PerfEnabled is ASSAY_TELEMETRY == "1" exactly. Off, PerfRun calls the
//     verb and returns, and nothing is created: no directory, no day file, no install id.
//   - NO FREE TEXT. PerfRecord has six string fields and each is a constant, a compiled-in
//     roster member, a build stamp or random hex. tool, verb, role and result each pass
//     through a mapper whose return is one of its own constants, never any part of its input.
//   - LOCAL ONLY. Nothing here opens a socket; there is no sender and no endpoint. The forge
//     round-trip COUNTER lives beside the transport seam (forgetransport.go) so this file
//     needs no network import at all.
//   - NEVER LOAD-BEARING. perfAppend returns nothing. A failure to resolve, create, open or
//     write is swallowed: the record never changes a verb's exit code, never prints, never
//     blocks, and takes no lock (one O_APPEND write of one short line).
//   - NEVER THE LEDGER. Every path joins perfDir() — a perf/ SUBDIRECTORY of the state dir —
//     so no writer, pruner or reader here can name audit.jsonl or one of its day segments.

import (
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// PerfSchema is the on-disk record version (perfrec's).
const PerfSchema = perfrec.Schema

// perfEnvVar is the one opt-in. It is the SAME variable statusgen/telemetry.go reads.
const perfEnvVar = perfrec.EnvVar

// perfRetentionDays is the history window: today plus the six prior UTC days.
const perfRetentionDays = perfrec.RetentionDays

// perfOther / perfRoleNone are the mappers' fallbacks — constants, never the input.
const (
	perfOther    = perfrec.Other
	perfRoleNone = perfrec.RoleNone
	perfStepPfx  = "step:"
	perfUnpinned = perfrec.Unpinned
)

// PerfRecord is one perf-v1 line — perfrec's record type, so reader and writer cannot drift.
type PerfRecord = perfrec.Record

// PerfEnabled reports whether local perf logging is armed: ASSAY_TELEMETRY compared to "1"
// EXACTLY (perfrec.Enabled).
func PerfEnabled() bool { return perfrec.Enabled() }

// The in-process counters PerfRun reads. They are incremented unconditionally (an atomic add
// is cheaper than the branch that would skip it) and reset at PerfRun entry; with the switch
// off they are never read and never leave memory.
var (
	perfChildProcs  atomic.Int64
	perfChildWallNs atomic.Int64
	perfForgeCalls  atomic.Int64
)

// perfCountChild records one child started through deskkit.Run and its wall time.
func perfCountChild(d time.Duration) {
	perfChildProcs.Add(1)
	perfChildWallNs.Add(int64(d))
}

// perfCountForgeCall records one HTTP round-trip through the counting forge transport.
func perfCountForgeCall() { perfForgeCalls.Add(1) }

// perfResetCounters zeroes all three counters (PerfRun entry).
func perfResetCounters() {
	perfChildProcs.Store(0)
	perfChildWallNs.Store(0)
	perfForgeCalls.Store(0)
}

// PerfCounters returns the current child count, summed child wall time and forge round-trips.
func PerfCounters() (children int64, childWall time.Duration, forgeCalls int64) {
	return perfChildProcs.Load(), time.Duration(perfChildWallNs.Load()), perfForgeCalls.Load()
}

// perfNow is the clock (a test hook; production is time.Now).
var perfNow = time.Now

// perfWriter is the shared writer, on this package's clock hook.
var perfWriter = &perfrec.Writer{Now: func() time.Time { return perfNow() }}

// PerfRun is the ONE wrapper a verb's main() puts around its run function:
//
//	os.Exit(deskkit.PerfRun("<tool>", verbs, os.Args[1:], run))
//
// It must wrap run rather than defer inside it, because os.Exit runs no deferred function.
// Off, it calls run and returns. Armed, it times run, builds the record (verb resolved by
// matching args[0] against verbs — the verb's own registered subcommand set — and "other"
// for anything else), appends it best-effort, and returns run's exit code UNCHANGED.
func PerfRun(tool string, verbs []string, args []string, run func([]string) int) int {
	if !PerfEnabled() {
		return run(args)
	}
	perfResetCounters()
	began := time.Now()
	code := run(args)
	wall := time.Since(began)

	rec := newPerfRecord(tool, perfVerb(verbs, args), wall, code)
	children, childWall, forge := PerfCounters()
	rec.ChildProcs = children
	rec.ChildWallMs = childWall.Milliseconds()
	rec.ForgeCalls = forge
	if ms, ok := perfrec.CPUMillis(); ok {
		rec.CPUMs = &ms
	}
	perfAppend(rec)
	return code
}

// perfBootSteps is the compiled roster of boot-step names PerfStep accepts — deskboot's own
// step constants, pinned equal to its step list by that package's tests. A step outside it is
// recorded as "step:other", so a caller can never route text into the verb field.
var perfBootSteps = []string{
	"loop-identity",
	"worktree-prune",
	"worktree-lock",
	"roster-set",
	"roster-preflight",
	"token-mint",
	"board-fetch",
	"worktree-current",
}

// PerfBootSteps returns a copy of the step-name roster, in boot order.
func PerfBootSteps() []string { return append([]string(nil), perfBootSteps...) }

// PerfStep writes one boot-step record: verb "step:<name>", wallMs the step's duration,
// exit the step's exit code. Child and forge counts are not attributed per step (they stay
// 0) and cpuMs is omitted — a process-wide CPU reading is not a per-step measurement.
func PerfStep(tool, step string, d time.Duration, exit int) {
	if !PerfEnabled() {
		return
	}
	perfAppend(newPerfRecord(tool, perfStepVerb(step), d, exit))
}

// perfStepVerb returns "step:" plus the ROSTER's spelling of step, or "step:other".
func perfStepVerb(step string) string {
	for _, s := range perfBootSteps {
		if s == step {
			return perfStepPfx + s
		}
	}
	return perfStepPfx + perfOther
}

// newPerfRecord builds a record through the four mappers. verb must already be a roster
// value (perfVerb / the step roster); every other string field is mapped here.
func newPerfRecord(tool, verb string, wall time.Duration, exit int) PerfRecord {
	sha, _ := Version()
	return perfrec.New(perfTool(tool), verb, perfRole(os.Getenv("DESK_LOOP")), wall, exit, sha, perfNow())
}

// perfTool maps a tool name onto the canonical tool roster, or "other".
func perfTool(raw string) string {
	if c, ok := CanonicalToolKey(raw); ok {
		if _, member := canonicalToolKeys[c]; member {
			return c
		}
	}
	return perfOther
}

// perfVerb returns the element of verbs that args[0] names, or "other".
func perfVerb(verbs, args []string) string { return perfrec.Verb(verbs, args) }

// perfRole maps $DESK_LOOP onto the loop roster, or "none" when unset or unknown.
func perfRole(raw string) string {
	if c, ok := CanonicalLoopName(strings.TrimSpace(raw)); ok {
		if _, member := canonicalLoopNames[c]; member {
			return c
		}
	}
	return perfRoleNone
}

// resultForExit is the fixed exit→result map; perfrec owns it, and a test pins that it agrees
// with this package's Exit* and Result* constants.
func resultForExit(exit int) string { return perfrec.ResultForExit(exit) }

// shaPattern is what a build stamp looks like; anything else records as "unpinned".
var shaPattern = perfrec.ShaPattern

// perfSourceSHA is Version()'s build stamp, constrained to hex.
func perfSourceSHA() string {
	s, _ := Version()
	return perfrec.SourceSHA(s)
}

// perfDir is <state dir>/perf. Every perf path joins it, so none can name audit.jsonl.
func perfDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return perfrec.Dir(dir), nil
}

// perfDayOf returns the UTC day a day-file name records, ok=false for any other name.
func perfDayOf(name string) (time.Time, bool) { return perfrec.DayOf(name) }

// perfAppend writes rec as one line to today's day file, best-effort. It returns nothing:
// the perf log is never allowed to change what the verb does.
func perfAppend(rec PerfRecord) { _ = perfWrite(rec) }

// perfWrite is perfAppend's body with the error kept, for the tests that need to see WHY a
// write did not land.
func perfWrite(rec PerfRecord) error {
	dir, err := StateDir()
	if err != nil {
		return err
	}
	return perfWriter.Write(dir, rec)
}

// perfPrune removes day files older than the retention window (perfrec.Prune on this clock).
func perfPrune(dir string) { perfrec.Prune(dir, perfNow()) }

// installIDPattern is the only shape an install id may have: 32 lowercase hex characters.
var installIDPattern = perfrec.InstallIDPattern

// perfInstallID reads or mints <dir>/install-id.
func perfInstallID(dir string) (string, error) { return perfrec.InstallID(dir) }
