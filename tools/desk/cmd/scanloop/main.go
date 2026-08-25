package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident. This tool
	// ACTS on the roster — it applies the trust gate and writes through the sanctioned verbs — so
	// ciEligible=false: the config-home file only, never the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective trust/authority configuration before doing anything. The roster, the scan
	// scope and the write boundary live in settings rather than in a diff, so the RUN is the only
	// place a change to them becomes visible — and a NARROWING has to be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("scanloop sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Declare the loop identity BEFORE Guard so the per-loop stop flag is scoped to this drain.
	// An unrecognised name is could-not-check inside Guard, never "no flag held".
	if os.Getenv("DESK_LOOP") == "" {
		_ = os.Setenv("DESK_LOOP", LoopName)
	}
	// Guard (kill switch + stop flags) is the FIRST action, before any surface is read.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	switch args[0] {
	case "plan":
		err := cmdPlan(args[1:], os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return deskkit.ExitCodeOf(err)
	case "run":
		err := cmdRun(args[1:], os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return deskkit.ExitCodeOf(err)
	default:
		fmt.Fprintf(os.Stderr, "scanloop: unknown subcommand %q\n\n%s", args[0], usage)
		return deskkit.ExitRefused
	}
}

const usage = `scanloop — the intake desk's drain consumer of the deterministic drain engine.

USAGE:
  scanloop plan --root <repo> [--scan-target <owner/name>] [--inbound <file|->] [--state-dir <dir>]
                [--monitor <path>] [--coalesce-window 20m]
                [--scan-pr <N> --scan-branch <b> --scan-pr-created <ts>] [--now <RFC3339>]
  scanloop run  --root <repo> [--worktree-base <abs dir>] [--offline --inbound <file|->]
                [--dry-run] [everything 'plan' takes]
  scanloop --version

'plan' is READ-ONLY. It prints the inbound queue — surface, item, lane, age, claim state — the
monitor's arming coverage, the trust gate's three-state tally, the coalesce decision, and the
surfaces this drain does not read. It spawns nothing and writes nothing, and it deliberately does
NOT run the poller: a poll ADVANCES the per-repo baselines and would consume the events it reports.
Pass the standing window's captured poll with --inbound.

'run' is the drain. It arms the poller if it is not armed (arming and draining are the same act:
the seeding pass reports no inbound rather than replaying the backlog), applies the trust gate
BEFORE anything is queued, executes the dispatch lanes and records exactly ONE tracked exit per
item. --dry-run prints every lane step without running it; --offline takes the pass's events from
--inbound and opens no network read at all.

  the five tracked exits   placeholder · bug · finding · needs-decision · rejected-watching
                           An item that lands with none of them, or with two, is a refusal.
  the trust gate           Inbound not authored by a configured trusted identity is ignored unless
                           the blessing authority has commented on it. Quarantined items stay
                           VISIBLE and counted; they are never routed.
  the coalesce window      An open scan PR younger than the window absorbs the batch; at or past it
                           the PR stays sealed at a stable head and a fresh one is cut. An age that
                           cannot be read never coalesces.
  body regeneration        The scan PR's title and body are REGENERATED — re-derived from the
                           branch's own diff — on EVERY push, never carried over, never hand-edited.
  the judgment half        Which exit an item takes, and the ownership routing test, are EMITTED for
                           a model tier. This drain never computes them.
  one scan per pass        The scan is WHOLE-SCOPE: one run derives the delta for every issue in the
                           scan scope. Every mechanical item a pass admits therefore shares ONE scan
                           dispatch against one branch and one PR — and each inbound item still
                           leaves by its own tracked exit.
  the scan target          The ONE repo the delta is committed to and the PR opened against, resolved
                           from --scan-target or the --root checkout's origin. An issue on a repo in
                           the READ scope but outside the write boundary is ordinary work: its
                           placeholder lands here under a repo-stemmed name.
  blind never exits 0      A degraded repo, a suppressed burst, an unarmed poller, or a trust read
                           that could not be taken all make the pass unverifiable.

Stop flags are honoured on every cycle boundary, precedence DISABLED > STOP > STOP.` + LoopName + `.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
`
