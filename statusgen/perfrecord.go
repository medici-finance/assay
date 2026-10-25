package main

// perfrecord.go — statusgen's side of the opt-in, LOCAL-ONLY perf record (#3101), written in
// perf-v2: the same session format the desk tools write (perfrec.Begin/Phase/Finish). An
// explicitly selected perf profile writes one perf-v3 summary instead (statusgen/26, see the
// section at the end of this file).
//
// With ASSAY_TELEMETRY exactly "1", every statusgen process that exits through exitProcess
// appends one perf-v2 line to the same <state dir>/perf/ day files the desk tools write, so
// `deskperf report --tool statusgen` reads them:
//
//   - wall and CPU from perfrec's process-init mark to the exit, one boundary;
//   - per-stage phase timings (perfPhases, written in roster order), so most of the wall time
//     has a breakdown: board parse, source lint, link lint, the forge claim read, the view
//     build, render, the STATUS.md write or compare, and --corroborate's forge corroboration.
//     The phases do not sum to the wall time: process start before the first stage and the
//     exit after the last one fall inside the wall time but outside every phase;
//   - the `http` counter: every direct HTTP attempt at statusgen's forge seams (the GitHub
//     REST and GraphQL clients, the GitLab client, and the telemetry sender), counted at the
//     call, so it is MEASURED even when it is zero. The other counters are listed
//     unavailable: statusgen's many `gh`/`git` child processes have no shared seam to count.
//
// Unset or any other value, nothing is created and nothing changes: the session is nil and
// every phase call is a no-op. The writer — schema, day-file layout, 7-day retention, install
// id, CPU reading — is the shared perfrec module; this file owns only what is statusgen's: the
// verb and phase rosters, the forge counter and the exit wrapper. It is unrelated to
// telemetry.go's network payload, which keeps its own double opt-in.
//
// NO FREE TEXT: the verb and every phase name are members of the rosters below, never any part
// of argv. NEVER LOAD-BEARING: measuring adds no forge call, no output and no exit path; a
// record that cannot be written is dropped silently; the exit code and output are exactly what
// they were without the switch.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// perfToolName is the record's tool field. It is a constant, not derived from argv[0].
const perfToolName = "statusgen"

// perfDefaultVerb is the mode of a run that names none: the plain board regeneration.
const perfDefaultVerb = "regen"

// perfSubcommands are the positional sub-commands main() dispatches before flag parsing
// ("version" also answers to --version). perfModeFlags are the flags that select a mode of
// the default command. Both are compiled-in rosters; TestPerfModeRosterIsRegistered pins the
// flags against the flag set main() really registers.
var (
	perfSubcommands = []string{
		"version", "init", "verifyrun", "verifyclosure", "verifyflip", "mergecheck",
		"shardcheck", "phantoms", "enforcement-status", "backfill", "reconcile", "outcomes",
		"regen", "conform", "patterns", "instance", "runtime-resource", "migrate", "brief",
		"newbrief", "lint", "verify-gate-close", "context",
	}
	perfModeFlags = []string{
		"check", "lint", "lint-audit", "instrument-audit", "record", "verify-issues",
		"decision-issues", "owed-issues", "drive-issues", "watchdog", "drive-snapshot",
		"signoff-digest", "scan-issues", "transcribe-scan", "transcribe-verdict",
		"transcribe-scan-delta", "close-verify", "auto-flip-model", "register-links", "alarms",
		"verif-backlog", "autonomy", "ladder", "issues", "self-improvement", "cynefin", "dora",
		"dora-timing", "dora-json", "trend", "roadmap", "consumers", "corroborate",
		"bottleneck", "intake-exits", "intake-debt", "throughput", "leadtime",
		"flow-efficiency", "flow", "first-pass-yield", "review-rework", "decision-latency",
		"net-flow", "assayscore", "launch", "requirements-rollup", "export-evidence",
		"export-audit-pack", "graph", "gate-scores", "next-up", "eligibility", "coverage",
		"cluster-pending-queue", "gate-telemetry", "telemetry", "telemetry-dry-run",
	}
)

// perfVerb maps argv (without the program name) onto a roster member. A positional
// sub-command wins; else the first argument that is a mode flag; else the default regen.
// It returns the ROSTER's spelling, never the input.
func perfVerb(args []string) string {
	if len(args) == 0 {
		return perfDefaultVerb
	}
	if args[0] == "--version" {
		return "version"
	}
	if v := perfrec.Verb(perfSubcommands, args); v != perfrec.Other {
		return v
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		if v := perfrec.Verb(perfModeFlags, []string{name}); v != perfrec.Other {
			return v
		}
	}
	if !strings.HasPrefix(args[0], "-") {
		return perfrec.Other // a positional token that is no sub-command: unknown, not regen
	}
	return perfDefaultVerb
}

// perfPhases is statusgen's fixed phase roster. perfrec writes phases in roster order, which
// is the order a run first enters them except for one pair: the view build is entered before
// the claim read and resumed after it, yet "claims" is listed before "view". A record's phase
// order is therefore the roster's, never a trace of the run. A stage entered more than once
// (one run() per root, the source lint resumed around its named sub-stages, the view build
// either side of the claim read) accumulates into one entry whose n counts the entries. The
// sub-stages split out of "lint" are the checks that each took more than ~5% of a full
// --lint/--check of this repository; every other check stays inside "lint".
var perfPhases = []string{
	"parse",       // budget specs, stream and finding load, placeholders, archived streams
	"lint",        // the per-stream/per-brief source checks not named below
	"briefs",      // checkBriefFiles: every brief file's frontmatter and sections
	"ordering",    // orderingGateNotices: README/brief prose that reads as an ordering gate
	"evidence",    // evidenceActorGate: Evidence-row actors against git history
	"links",       // the doc-tree block: fixture corpora, doc walk and the lints that read it
	"claims",      // the claim read (a forge round-trip with --forge or on the write path)
	"view",        // findings, next-up, history ages, drives: the board's model
	"render",      // emit: the STATUS.md bytes
	"write",       // record / check-compare / write of STATUS.md and the register views
	"corroborate", // --corroborate: forge corroboration of human stamps
}

// perfSession measures this process: started at package initialisation from perfrec's
// process-init mark when the switch is on, nil (every call a no-op) when it is off.
var perfSession = perfBegin()

func perfBegin() *perfrec.Session {
	if !perfrec.Enabled() {
		return nil
	}
	return perfrec.Begin(perfrec.SystemProbe(), perfrec.ProcessEntry(), perfPhases)
}

// perfPhase starts a timed stage and returns its stop (idempotent; a no-op when off).
func perfPhase(name string) func() { return perfSession.Phase(name) }

// perfStage times a sequence of non-overlapping stages: enter closes the open stage and opens
// the next, done closes whichever is open. A function with many returns defers done once.
type perfStage struct{ stop func() }

func (p *perfStage) enter(name string) {
	p.done()
	p.stop = perfPhase(name)
}

func (p *perfStage) done() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
}

// perfHTTPCalls counts direct HTTP attempts at the forge seams, process-wide. It is
// incremented unconditionally (an atomic add is cheaper than the branch that would skip it)
// and read only when a record is built.
var perfHTTPCalls atomic.Int64

// perfCountHTTP records one direct HTTP attempt. Each forge seam calls it once, immediately
// before its client's Do, so a retry, a 304 and a transport failure each count; a redirect
// the client follows inside that Do is part of the same attempt and does not count again.
func perfCountHTTP() { perfHTTPCalls.Add(1) }

// perfSourceStamp is the ONE place the record's build stamp is read: the commit the binary was
// built from (perfrec.Revision: the toolchain's vcs.revision). statusgen has no link-time commit
// stamp, hence the empty one. It is never statusgenVersion: that is a release TAG, which
// perfrec.SourceSHA's hex-only gate records as "unpinned".
func perfSourceStamp() string { return perfrec.Revision("") }

// perfBuildRecord ends the session and builds this process's record; ok is false when the
// switch is off.
func perfBuildRecord(exit int) (perfrec.Record, bool) {
	if perfSession == nil {
		return perfrec.Record{}, false
	}
	perfSession.Add(perfrec.CounterHTTP, perfHTTPCalls.Load())
	return perfSession.Finish(perfToolName, perfVerb(os.Args[1:]), perfrec.RoleNone, exit, perfSourceStamp()), true
}

// recordPerf appends this process's perf line when ASSAY_TELEMETRY=1: the perf-v3 summary when
// a profile is selected (recordV3), else the unchanged perf-v2 line. It returns nothing and
// never prints: the record is not allowed to change what statusgen does.
func recordPerf(exit int) {
	if !perfrec.Enabled() {
		return
	}
	if recordV3(exit) {
		return
	}
	if rec, ok := perfBuildRecord(exit); ok {
		perfrec.Append(rec)
	}
}

// exitProcess is the ONE way main() ends the process: it records the perf line (a no-op when
// the switch is off) and then exits with code. main.go calls no os.Exit directly, so no exit
// path escapes the record (TestMainExitsOnlyThroughRecorder).
func exitProcess(code int) {
	recordPerf(code)
	os.Exit(code)
}

// parseMainFlags is flag.Parse() with the exit routed through exitProcess: the default
// command line's ExitOnError would call os.Exit inside the flag package on a usage error,
// an exit path no record could see. Behaviour is the flag package's: -h/--help exits 0, any
// other parse error exits 2, and the usage text is printed by the flag set as before.
//
// The shared --perf-* context flags are bound here, on the same flag set, so they are parsed by
// the real parser (a malformed value is perf coverage, never a usage error). After a clean
// parse the invocation's context is selected once and the residual "evaluate" phase opens.
func parseMainFlags() {
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
	perfBindFlags(flag.CommandLine)
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			exitProcess(0)
		}
		exitProcess(2)
	}
	perfSelect()
	perfEnter("evaluate") // closed by recordPerf at exit
}

// --- perf-v3 (statusgen/26) ------------------------------------------------------------------
//
// An explicitly SELECTED profile (--perf-profile / ASSAY_PERF_PROFILE = tool | workflow, with
// ASSAY_TELEMETRY=1) replaces the perf-v2 line with one perf-v3 process-summary record. No
// profile — unset, "off", or any invalid value — keeps the perf-v2 line above unchanged: its
// phase roster, its http counter and its boundary do not move.
//
// The two sessions are separate and never mixed: the perf-v2 session (perfSession) is begun at
// package initialisation, before any flag is parsed, and only its record is dropped when a
// profile is selected; the perf-v3 session (perfV3Session) is begun once the context resolves.
// Both start from the same mark, perfrec.ProcessEntry(), and both end at exitProcess.

// perfV3Phases is statusgen's fixed perf-v3 phase roster. Phases are EXCLUSIVE: entering one
// pauses the phase it nests in, so phase ms never double count (phaseOverlap stays false for
// sequential code) and n counts timed segments — a paused phase resumes as a new segment.
//
//	discover  enumerating stream, archive and findings sources (load.go)
//	parse     reading + parsing a stream README, a findings file or a brief file
//	evaluate  everything else after the command line parsed: checks, eligibility, coverage
//	render    building the board text or a JSON view
//	write     writing STATUS.md and the register views
//	deskread  one deskread child, start to exit, as seen from statusgen (n = child starts)
var perfV3Phases = []string{"discover", "parse", "evaluate", "render", "write", "deskread"}

// perfNames are the shared --perf-* context flags (the desk tools' adapter registers the same
// eight). statusgen binds them on its own flag set: it may not import the desk adapter.
var perfNames = []string{"perf-profile", "perf-session-id", "perf-agent-id", "perf-parent-agent-id",
	"perf-parent-invocation-id", "perf-loop-role", "perf-role", "perf-work"}

// perfInput collects the parsed --perf-* values. Only the flag package writes it (Set below),
// so values reach the resolver exactly as the real parser accepted them; it is never re-read
// from argv.
var perfInput perfrec.ContextInput

type perfFlag struct{ name string }

func (perfFlag) String() string { return "" }
func (v perfFlag) Set(s string) error {
	val := perfrec.Value{Value: s, Present: true}
	switch v.name {
	case "perf-profile":
		perfInput.Profile = val
	case "perf-session-id":
		perfInput.SessionID = val
	case "perf-agent-id":
		perfInput.AgentID = val
	case "perf-parent-agent-id":
		perfInput.ParentAgentID = val
	case "perf-parent-invocation-id":
		perfInput.ParentInvocationID = val
	case "perf-loop-role":
		perfInput.DeskRole = val
	case "perf-role":
		perfInput.Role = val
	case "perf-work":
		perfInput.Work = append(perfInput.Work, s)
		perfInput.WorkPresent = true
	}
	return nil // optional metadata never fails the parse; perfrec classifies it as coverage
}

// perfBindFlags registers the eight context flags on fs. It never rejects a value.
func perfBindFlags(fs *flag.FlagSet) {
	for _, name := range perfNames {
		if fs.Lookup(name) == nil {
			fs.Var(perfFlag{name}, name, "optional local performance context (never operational authority)")
		}
	}
}

var (
	perfOnce      sync.Once
	perfCtx       perfrec.Context
	perfV3Session atomic.Pointer[perfrec.Session]
)

// perfSelect resolves this invocation's context ONCE (fresh invocation id per process) from
// the parsed flags, then the documented environment keys. A selected profile starts the v3
// session at the process-init mark. Sub-commands with their own parsers, help, version and
// usage errors reach it first from recordPerf, so they resolve from the environment alone
// (plus anything parsed before the error) and record the whole interval with no phases.
func perfSelect() {
	perfOnce.Do(func() {
		perfCtx = perfrec.ResolveContext(perfInput)
		if perfCtx.Profile() == "" {
			return
		}
		perfV3Session.Store(perfrec.BeginSelected(perfrec.SystemProbe(), perfrec.ProcessEntry(), perfV3Phases, perfCtx))
	})
}

type perfFrame struct {
	name string
	stop func() // nil while paused by a nested phase
}

var (
	perfMu    sync.Mutex
	perfStack []*perfFrame
)

// perfEnter opens phase name and returns its idempotent leave. With no v3 session it is a
// no-op. Re-entering the phase already on top is a no-op too (the outer call times it).
func perfEnter(name string) (leave func()) {
	s := perfV3Session.Load()
	if s == nil {
		return func() {}
	}
	perfMu.Lock()
	defer perfMu.Unlock()
	if n := len(perfStack); n > 0 {
		top := perfStack[n-1]
		if top.name == name {
			return func() {}
		}
		if top.stop != nil {
			top.stop()
			top.stop = nil
		}
	}
	f := &perfFrame{name: name, stop: s.Phase(name)}
	perfStack = append(perfStack, f)
	var once sync.Once
	return func() { once.Do(func() { perfLeave(s, f) }) }
}

func perfLeave(s *perfrec.Session, f *perfFrame) {
	perfMu.Lock()
	defer perfMu.Unlock()
	i := len(perfStack) - 1
	for ; i >= 0 && perfStack[i] != f; i-- {
	}
	if i < 0 {
		return // already closed by recordPerf
	}
	if f.stop != nil {
		f.stop()
	}
	if i != len(perfStack)-1 {
		// Left out of order (a concurrent caller): drop this frame only; the top keeps timing.
		perfStack = append(perfStack[:i], perfStack[i+1:]...)
		return
	}
	perfStack = perfStack[:i]
	if i > 0 && perfStack[i-1].stop == nil {
		perfStack[i-1].stop = s.Phase(perfStack[i-1].name)
	}
}

// perfCloseAll stops every open phase, innermost first, so Finish records them.
func perfCloseAll() {
	perfMu.Lock()
	defer perfMu.Unlock()
	for i := len(perfStack) - 1; i >= 0; i-- {
		if perfStack[i].stop != nil {
			perfStack[i].stop()
		}
	}
	perfStack = nil
}

// recordV3 writes the perf-v3 process summary for a selected profile. It reports whether it
// did, so recordPerf falls back to the unchanged perf-v2 line when no profile is selected.
// Request coverage stays "missing": statusgen observes no forge request itself, and the
// deskread child reports its own (a child that does not is a coverage gap, never a zero).
func recordV3(exit int) bool {
	perfSelect()
	s := perfV3Session.Load()
	if s == nil {
		return false
	}
	perfCloseAll()
	role := perfrec.RoleNone
	if r := perfCtx.Data().Role; r != "" {
		role = r
	}
	perfrec.Append(s.Finish(perfToolName, perfVerb(os.Args[1:]), role, exit, perfSourceStamp()))
	return true
}

// perfChildFlags are the context flags handed to a deskread child. The child's desk role is
// not among them: it resolves from the operational DESK_LOOP the child inherits unchanged.
var perfChildFlags = []string{"perf-profile", "perf-session-id", "perf-agent-id", "perf-parent-agent-id",
	"perf-parent-invocation-id", "perf-role", "perf-work"}

var perfProbed sync.Map // deskread binary -> whether its help lists every perfChildFlags entry

// deskreadTakesPerf is the version gate for the context hand-off: it asks the resolved deskread
// binary, once per process, whether its help lists every flag perfChildArgs would pass. A build
// that predates them (it would refuse an unknown flag, changing statusgen's result) or that
// cannot answer gets no flags. The probe runs with an empty environment and a bound (plus a
// WaitDelay, so an inherited pipe cannot stretch it), so it can reach no credential, record
// nothing and never hang a run. Callers run it before entering the deskread phase.
func deskreadTakesPerf(bin string) bool {
	if v, ok := perfProbed.Load(bin); ok {
		return v.(bool)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--help")
	cmd.Env = []string{}
	cmd.WaitDelay = time.Second // a grandchild holding the pipe cannot outlast the bound
	out, _ := cmd.CombinedOutput()
	ok := len(out) > 0
	for _, name := range perfChildFlags {
		ok = ok && bytes.Contains(out, []byte("--"+name))
	}
	perfProbed.Store(bin, ok)
	return ok
}

// perfChildArgs returns one deskread child's argv (args[0] is the read kind) with this
// invocation's context inserted as --perf-* flags after the kind. perfrec.ContextEnv decides
// what is forwarded (fed no inherited environment, it composes only the context): the tool
// profile alone, or under the workflow profile the session, agent, role and work with this
// invocation as the child's parent. Empty values are passed as empty flags, which clear what
// the child would otherwise inherit; with no profile selected the child is told "off", so an
// overridden or cleared parent never hands its child a context it did not use. Switch off, or a
// deskread without the flags, the argv is unchanged and the child inherits as before.
func perfChildArgs(bin string, args []string) []string {
	if !perfrec.Enabled() || len(args) == 0 {
		return args
	}
	perfSelect()
	if !deskreadTakesPerf(bin) {
		return args
	}
	flags := []string{"--perf-profile=off"}
	if perfCtx.Profile() != "" {
		flags = nil
	}
	for _, kv := range perfrec.ContextEnv(nil, perfCtx, true) {
		k, v, _ := strings.Cut(kv, "=")
		name := "perf-" + strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(k, "ASSAY_PERF_")), "_", "-")
		if name != "perf-work" {
			flags = append(flags, "--"+name+"="+v)
			continue
		}
		var refs []json.RawMessage
		if json.Unmarshal([]byte(v), &refs) != nil || len(refs) == 0 {
			flags = append(flags, "--perf-work=")
			continue
		}
		for _, r := range refs {
			flags = append(flags, "--perf-work="+string(r))
		}
	}
	out := append([]string{args[0]}, flags...)
	return append(out, args[1:]...)
}
