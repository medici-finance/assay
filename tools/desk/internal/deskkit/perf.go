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
//   - NO FREE TEXT. Every PerfRecord string field is a constant, a compiled-in roster member,
//     a build stamp or random hex. tool, verb, role and result each pass through a mapper
//     whose return is one of its own constants, never any part of its input (and perfrec's
//     Label gate behind it for perf-v2).
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

// PerfSchema is the LEGACY record version (perfrec's perf-v1): boot and launch steps are still
// written in it, and every older day file holds it.
const PerfSchema = perfrec.Schema

// PerfSchemaV2 is the record version PerfRun writes (perfrec's perf-v2): wall and CPU over the
// process-init boundary, counters with explicit coverage.
const PerfSchemaV2 = perfrec.SchemaV2

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

// PerfRecord is one perf-history line in either schema — perfrec's record type, so reader and
// writer cannot drift. Its Schema decides what WallMs and CPUMs mean; readers group by it.
type PerfRecord = perfrec.Record

// PerfEnabled reports whether local perf logging is armed: ASSAY_TELEMETRY compared to "1"
// EXACTLY (perfrec.Enabled).
func PerfEnabled() bool {
	if perfActive.Load() {
		if selected := perfSelectedConsent.Load(); selected != nil {
			return *selected
		}
	}
	return perfrec.Enabled()
}

// The in-process counters PerfRun reads. They are incremented unconditionally (an atomic add
// is cheaper than the branch that would skip it) and reset at PerfRun entry; with the switch
// off they are never read and never leave memory.
var (
	perfChildProcs      atomic.Int64
	perfChildWallNs     atomic.Int64
	perfForgeCalls      atomic.Int64
	perfActive          atomic.Bool
	perfSelectedVerb    atomic.Pointer[string]
	perfSelectedConsent atomic.Pointer[bool]
)

// perfCountChild records one attempt through the shared process observers and
// its wall time, including an attempt that could not start.
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

// perfEntry is the measurement start PerfRun uses: perfrec's process-init mark, taken before
// this package or any main initialises, so a verb's configuration echo and activation checks
// are inside the measurement. A test hook.
var perfEntry = perfrec.ProcessEntry

// perfProbe is the clock and CPU source: the real monotonic clock and getrusage, with the
// record's ts on this package's clock hook.
func perfProbe() perfrec.Probe {
	p := perfrec.SystemProbe()
	p.Stamp = func() time.Time { return perfNow() }
	return p
}

// PerfRun is the ONE wrapper a verb's main() puts around its run function:
//
//	os.Exit(deskkit.PerfRun("<tool>", verbs, os.Args[1:], run))
//
// It must wrap run rather than defer inside it, because os.Exit runs no deferred function.
// Off, it calls run and returns. Armed, it writes one perf-v2 record: wall and CPU both from
// the process-init mark (perfEntry) to the end of run, so the time main spent before PerfRun
// is included and the two deltas share one boundary; the child-process and forge counters as
// MEASURED (lower-bound) subprocess, subprocessMs and http counters; every cache and
// coalesced-waiter counter UNAVAILABLE (no seam here observes them). The verb is resolved by
// matching args[0] against verbs — the verb's own registered subcommand set — or a
// PerfCommand selection, and "other" for anything else. The record is appended best-effort
// after the measurement ends, and run's exit code is returned UNCHANGED.
func PerfRun(tool string, verbs []string, args []string, run func([]string) int) int {
	ctx := perfrec.ResolveContext(perfrec.ContextInput{})
	return PerfRunSelected(tool, verbs, args, ctx, run)
}

// PerfRunSelected measures one invocation using an already frozen parsed context.
func PerfRunSelected(tool string, verbs []string, args []string, ctx perfrec.Context, run func([]string) int) int {
	return PerfRunResolved(tool, verbs, args, func(a []string) (int, perfrec.Context) { return run(a), ctx })
}

// PerfRunResolved retains the process-init boundary while the real parser supplies context.
// The callback owns a local selection value; no process-global attribution is mutated.
func PerfRunResolved(tool string, verbs []string, args []string, run func([]string) (int, perfrec.Context)) int {
	return PerfRunMeasured(tool, verbs, args, func(a []string, _ *perfrec.Session) (int, perfrec.Context) { return run(a) })
}

// PerfRunMeasured supplies the invocation's optional session to the actual parser callback.
// Select its context before effects to admit typed provider observations. No ambient sink.
// Optional phases are a fixed tool-owned roster, bounded by perfrec; existing callers keep
// their empty roster and unchanged measurement boundary.
func PerfRunMeasured(tool string, verbs []string, args []string, run func([]string, *perfrec.Session) (int, perfrec.Context), phases ...string) int {
	return perfRunMeasured(tool, verbs, args, run, false, phases...)
}

// PerfRunMeasuredConfig retains process-entry measurement until the application
// resolves its existing config opt-in. Other tools keep PerfRunMeasured's off
// fast path. No environment or process-wide consent is modified.
func PerfRunMeasuredConfig(tool string, verbs []string, args []string, run func([]string, *perfrec.Session) (int, perfrec.Context), phases ...string) int {
	return perfRunMeasured(tool, verbs, args, run, true, phases...)
}

// PerfSelectConsent is scoped to the active invocation, like PerfCommand. The
// application calls it only with its validated effective configuration.
func PerfSelectConsent(enabled bool) {
	if perfActive.Load() {
		value := enabled
		perfSelectedConsent.Store(&value)
	}
}

func perfRunMeasured(tool string, verbs []string, args []string, run func([]string, *perfrec.Session) (int, perfrec.Context), late bool, phases ...string) int {
	enabled := PerfEnabled()
	if !enabled && !late {
		code, _ := run(args, nil)
		return code
	}
	perfResetCounters()
	perfSelectedVerb.Store(nil)
	perfSelectedConsent.Store(nil)
	perfActive.Store(true)
	defer perfActive.Store(false)
	sess := perfrec.Begin(perfProbe(), perfEntry(), phases)
	code, ctx := run(args, sess)
	if consent := perfSelectedConsent.Load(); consent != nil {
		enabled = *consent
	}
	if !enabled {
		return code
	}
	sess.SelectContext(ctx)
	children, childWall, forge := PerfCounters()
	sess.Add(perfrec.CounterSubprocess, children)
	sess.Add(perfrec.CounterSubprocessMs, childWall.Milliseconds())
	sess.Add(perfrec.CounterHTTP, forge)

	verb := perfVerb(verbs, args)
	if selected := perfSelectedVerb.Load(); selected != nil {
		verb = *selected
	}
	sha, _ := Version()
	rec := sess.Finish(perfTool(tool), verb, perfRole(os.Getenv("DESK_LOOP")), code, sha)
	perfAppendEnabled(enabled, rec)
	return code
}

// PerfCommand selects an attribution from a command's compiled vocabulary after its
// real parser resolves the operation. It changes no execution or consent behavior.
// Unknown values become "other"; raw argv and kit values never enter a record.
func PerfCommand(roster []string, selected string) {
	if !perfActive.Load() {
		return
	}
	verb := perfVerb(roster, []string{selected})
	perfSelectedVerb.Store(&verb)
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
	"first-sweep-prereqs",
}

// PerfBootSteps returns a copy of the step-name roster, in boot order.
func PerfBootSteps() []string { return append([]string(nil), perfBootSteps...) }

// perfLaunchSteps is the compiled roster of cellctl's pre-handover launch phases, in launch
// order: the work a `cellctl desk` launch does before the session process takes over. It is
// kept apart from perfBootSteps (deskboot's list, pinned equal to that package's own) and
// every name carries the "launch-" prefix so a launch step can never read as a boot step. A
// step outside both rosters is still "step:other".
var perfLaunchSteps = []string{
	"launch-role-lease",
	"launch-storage-admission",
	"launch-plugin-enable",
	"launch-fetch-main",
	"launch-worktree",
	"launch-preparation",
	"launch-scratch-lock-wait",
	"launch-scratch-admission",
	"launch-cache-enrollment",
	"launch-child-start",
	"launch-first-start",
	"launch-all-starts",
}

// PerfLaunchSteps returns a copy of the launch-step roster, in launch order.
func PerfLaunchSteps() []string { return append([]string(nil), perfLaunchSteps...) }

// PerfStep writes one step record (a boot step or a launch step) in the LEGACY perf-v1 schema,
// unchanged: verb "step:<name>", wallMs the step's duration, exit the step's exit code. Child
// and forge counts are not attributed per step (they stay 0) and cpuMs is omitted — a
// process-wide CPU reading is not a per-step measurement. A step subdivides an invocation
// that has its own record, so readers keep steps out of invocation totals.
func PerfStep(tool, step string, d time.Duration, exit int) {
	PerfStepEnabled(PerfEnabled(), tool, step, d, exit)
}

// PerfStepEnabled uses the launch owner's resolved opt-in. The fixed rosters,
// existing writer, state directory and retention contract remain unchanged.
func PerfStepEnabled(enabled bool, tool, step string, d time.Duration, exit int) {
	if enabled {
		perfAppendEnabled(true, newPerfRecord(tool, perfStepVerb(step), d, exit))
	}
}

// perfStepVerb returns "step:" plus the ROSTER's spelling of step (boot or launch), or
// "step:other".
func perfStepVerb(step string) string {
	for _, roster := range [][]string{perfBootSteps, perfLaunchSteps} {
		for _, s := range roster {
			if s == step {
				return perfStepPfx + s
			}
		}
	}
	return perfStepPfx + perfOther
}

// newPerfRecord builds a perf-v1 (step) record through the four mappers. verb must already be a roster
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
func perfAppendEnabled(enabled bool, rec PerfRecord) {
	dir, err := StateDir()
	if err == nil {
		_ = perfWriter.WriteEnabled(enabled, dir, rec)
	}
}

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
