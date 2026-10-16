package main

// perfrecord.go — statusgen's side of the opt-in, LOCAL-ONLY perf-v1 record (#3101).
//
// With ASSAY_TELEMETRY exactly "1", every statusgen process that exits through exitProcess
// appends one perf-v1 line (tool "statusgen", verb = the mode that ran, wall and CPU
// milliseconds, exit and result) to the same <state dir>/perf/ day files the desk tools write,
// so `deskperf report --tool statusgen` reads them. Unset or any other value, nothing is
// created and nothing changes.
//
// The writer — schema, day-file layout, 7-day retention, install id, CPU reading — is the
// shared perfrec module, the one implementation the desk tools use too; this file owns only
// what is statusgen's: the verb roster and the exit wrapper. It is unrelated to
// telemetry.go's network payload, which keeps its own double opt-in and is untouched.
//
// NO FREE TEXT: the verb is always a member of the rosters below, never any part of argv.
// NEVER LOAD-BEARING: a record that cannot be written is dropped silently; the exit code and
// output are exactly what they were without the switch.

import (
	"errors"
	"flag"
	"os"
	"strings"
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
		"newbrief", "lint", "verify-gate-close",
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

// perfStart is the clock's zero: package initialisation, which precedes main().
var perfStart = time.Now()

// recordPerf appends this process's perf-v1 line when ASSAY_TELEMETRY=1. It returns nothing
// and never prints: the record is not allowed to change what statusgen does.
func recordPerf(exit int) {
	if !perfrec.Enabled() {
		return
	}
	rec := perfrec.New(perfToolName, perfVerb(os.Args[1:]), perfrec.RoleNone,
		time.Since(perfStart), exit, statusgenVersion, time.Now())
	if ms, ok := perfrec.CPUMillis(); ok {
		rec.CPUMs = &ms
	}
	perfrec.Append(rec)
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
func parseMainFlags() {
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			exitProcess(0)
		}
		exitProcess(2)
	}
}
