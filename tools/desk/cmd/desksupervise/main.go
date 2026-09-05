// Command desksupervise is the liveness OBSERVER: the read-mostly loop that turns
// loopengine's fully-coded, fully-inert liveness taxonomy (ObservableProbe, LivenessPolicy —
// see internal/loopengine/liveness.go and probes.go) into a logged, minutes-scale reclaim of
// a wedged dispatch, with no human in the loop.
//
// Verbs:
//
//	desksupervise tick [--root DIR] [--repo OWNER/NAME] [--dry-run] [--now RFC3339]
//	                    [--claims-fixture FILE] [--observations-fixture FILE]
//	desksupervise run --interval DUR [--root DIR] [--repo OWNER/NAME] [--dry-run]
//	desksupervise --version
//
// tick enumerates every `state=dispatched` dispatch claim, runs the three house probes
// (or the fixtures, which bypass the forge and audit file so the Verify rows run offline),
// evaluates loopengine.DefaultLivenessPolicy() (or `<StateDir>/liveness.json` when
// present) against each, and prints one classification line per claim. With --dry-run
// (the only mode the Verify table exercises) it never mutates anything — no claim is
// released, no issue is filed, no journal line is written — it only prints what a live tick
// WOULD do. run --interval loops tick forever, honouring the kill switch between ticks and
// exiting 0 on SIGTERM, mirroring `deskwt prune --interval`.
//
// This is a READ-MOSTLY tool: the only writes are a claim release
// (deskkit.Forge.DeleteRef on "dispatch/<key>" — see actions.go for why this, not
// deskkit.ReleaseMatching, is the release path a DISPATCH claim actually needs), a journal
// line, and — at most once per key, deskfile's own dedupe makes the repeat a no-op — a
// `help wanted` issue naming a blocked-timeout run. None of the three run under --dry-run.
//
// Exit codes (deskkit contract): 0 ok (every claim classified, none blind) · 3 disabled ·
// 5 refused · 6 unverifiable — including a tick that classified every claim but found at
// least one BLIND (could-not-check): the RUN succeeded, but the READING is incomplete, and
// exit 0 must never claim otherwise (the three-state-instrument rule). See
// internal/deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `desksupervise — the liveness observer: reclaim a wedged dispatch in minutes, not hours.

USAGE:
  desksupervise tick [--root DIR] [--repo OWNER/NAME] [--dry-run] [--now RFC3339]
                      [--claims-fixture FILE] [--observations-fixture FILE]
  desksupervise status [--json] [--stops] [--root DIR] [--repo OWNER/NAME] [--now RFC3339]
                      [--claims-fixture FILE] [--observations-fixture FILE] [--stops-fixture FILE]
  desksupervise run --interval DUR [--root DIR] [--repo OWNER/NAME] [--dry-run]
  desksupervise stop <key> --reason "..."
  desksupervise status --stops
  desksupervise --version

tick enumerates every state=dispatched dispatch claim (live: via the claim tool's own
records under --root/--repo; offline: via --claims-fixture, a JSON array of claim
records), runs the three house liveness probes (offline: via --observations-fixture, a
JSON object keyed by claim key, which BYPASSES the forge and audit file entirely — this
is what makes the Verify table run with no network and no filesystem audit dependency),
classifies each against loopengine.DefaultLivenessPolicy(), and prints one line per claim:

  <key>  <ALIVE|NEVER-STARTED|HEARTBEAT-EXPIRED|OVER-WALL-CAP|COULD-NOT-CHECK>
  last=<ts|none> via=<source|-> action=<none|RECLAIM-ELIGIBLE|BLOCKED-TIMEOUT|BLIND>

--dry-run (the mode the Verify table exercises) never mutates anything; without it,
RECLAIM-ELIGIBLE releases the claim and BLOCKED-TIMEOUT files a help-wanted issue naming
the run (idempotent — deskfile's own dedupe makes a repeat filing a no-op) — both also
write a journal line. A COULD-NOT-CHECK claim (action=BLIND) is never acted on either way,
and a tick that saw one exits 6 — the reading is incomplete even though the tick itself
ran cleanly (see the three-state-instrument rule: could-not-check is never a pass).

status renders the observer's per-claim runtime state as ONE structured record — a human
table, or (--json) a document validating against schemas/desksupervise-status-v1.json: per
claim its liveness, the three remaining-time timers, its armed stop (if any), and token
accounting (could-not-check BY DESIGN — the harness holds usage, no desk read path exists,
so it is NEVER rendered as zero). --stops filters to claims carrying an armed stop. status
is a PURE READ (no reclaim, no filing, no journal) and, like tick, exits 6 when any claim
was COULD-NOT-CHECK this tick. run --interval additionally writes this JSON to
<StateDir>/supervise/status.json atomically each tick for a local reader.

run --interval DUR loops tick forever (sweep, sleep, sweep), honouring the kill switch /
STOP flags between ticks and exiting 0 on SIGTERM — mirrors ` + "`deskwt prune --interval`" + `.
--now is for the Verify fixtures only; live runs always use the real clock.

stop <key> --reason "..." arms the PER-RUN stop flag (STOP.run.<key>) so deskkit.Guard
refuses that one run's next desk verb — a halt for a single wedged or superseded run that
never touches the loop-wide kill switch. tick arms the same flag automatically before it
releases a reclaimed claim. status --stops lists the armed per-run stops (key, armed_at,
reason) — the desk window's cadence read.

Exit: 0 ok (every claim ALIVE/reclaimed/landed, none blind) · 3 disabled · 5 refused ·
6 unverifiable (a claim could not be checked, or a real precondition failed).`

func main() {
	// Explicit roster class (the desk-tools contract every tool declares): desksupervise
	// ACTS on the roster (it reads config-home files, mints tokens for its production probes),
	// so ciEligible=false — it never reads the environment for its roster in CI either.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("desksupervise sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Kill-switch check is the FIRST action. Guard writes its own result=disabled audit line.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "tick":
		err = cmdTick(rest)
	case "run":
		err = cmdRun(rest)
	case "stop":
		err = cmdStop(rest)
	case "status":
		err = cmdStatus(rest)
	default:
		fmt.Fprintf(os.Stderr, "desksupervise: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}

// auditCtx accumulates the fields for the ONE audit line every invocation emits (the
// deskwt/deskclaim pattern). desksupervise has no PR/head of its own — it observes OTHER
// dispatches' PRs — so those stay null in the schema.
type auditCtx struct {
	verb          string
	repo          string
	detail        string
	successResult string // ResultOK unless a noop set it to ResultNoop
}

func (a *auditCtx) log(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "desksupervise",
		Verb:       a.verb,
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	})
}

func (a *auditCtx) finalize(err error) {
	if err == nil {
		result := a.successResult
		if result == "" {
			result = deskkit.ResultOK
		}
		a.log(result, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRateLimited:
		result = deskkit.ResultRateLimited
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}
