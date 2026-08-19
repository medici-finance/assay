// Command fanoutloop is the worker-desk (batch-fanout) REFERENCE CONSUMER of the deterministic
// drain engine (tools/desk/internal/loopengine) — the SECOND consumer after verify-desk, whose
// entire job is to VALIDATE that the engine contract generalizes to a
// consumer that fits it hardest (Land a near-no-op, a standing N=8 pool with orphan-resume priority,
// tiering by effort × exec-tier). It exists to prove the contract end-to-end and to serve as the
// interim-mode debug surface — NOT (yet) to drive the standing fanout window. The autonomous cutover
// (the worker desk actually booting this as its driver) is gate:human — BLOCKED-ON-IAN.
//
// Subcommands:
//
//	fanoutloop plan --root <repo> [--sha <targetSHA>]
//	    Deterministic scheduler OUTPUT: read the Next-up board (orphan resumes first, then rows in
//	    board order, issue-* placeholders skipped), compute each item's tier, and print the
//	    EXACT dispatch instruction the operator would execute as an Agent call. No agents are spawned,
//	    nothing is written, and no network is touched — this is the "the Go engine owns all scheduler
//	    state and emits exact dispatches" surface (§9.1).
//
//	fanoutloop --version
//
// The `run` (autonomous drive) path is deliberately NOT exposed here: a Go conductor cannot call the
// harness Agent tool, and wiring the live window is the human-gated cutover.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func main() {
	// Declare the roster class EXPLICITLY (never ClassWrite-by-omission): this tool ACTS on the
	// dispatch queue, so ciEligible=false — config-home file only, never the environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective trust/authority configuration to stderr before doing anything (P3): a
	// control surface that lives in settings rather than a diff is visible only at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("fanoutloop sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Guard (kill switch + stop flags) is the FIRST action, before any board read.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	switch args[0] {
	case "plan":
		return loopengine.ExitCodeOf(cmdPlan(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "fanoutloop: unknown subcommand %q\n\n%s", args[0], usage)
		return deskkit.ExitRefused
	}
}

func cmdPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	root := fs.String("root", ".", "repo root to scan for the Next-up queue")
	sha := fs.String("sha", "", "merged-main target SHA workers branch from")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("bad flags: " + err.Error())
	}

	f := &FanoutLoop{Root: *root, TargetSHA: *sha}
	items, err := f.SelectQueue()
	if err != nil {
		return deskkit.Unverifiable("cannot read the Next-up queue", err)
	}
	fmt.Printf("worker-desk plan: %d item(s) to dispatch (orphan resumes first, then Next-up in board order)\n", len(items))
	for _, it := range items {
		tier, terr := f.TierPolicy(it)
		if terr != nil {
			fmt.Printf("\n-- %s: tier error: %v\n", it.ID, terr)
			continue
		}
		prompt := renderDispatchPrompt(it, tier)
		if err := assertNoSharedCheckout(prompt); err != nil {
			return deskkit.Refused(err.Error())
		}
		fmt.Printf("\n=== DISPATCH %s (tier=%s) ===\n%s\n", it.ID, tier, prompt)
	}
	return nil
}

const usage = `fanoutloop — worker-desk (batch-fanout) reference consumer of the drain engine.

USAGE:
  fanoutloop plan --root <repo> [--sha <targetSHA>]
  fanoutloop --version

'plan' prints the deterministic scheduler output: the dispatch queue (orphan resumes first, then the
Next-up board in board order, issue-* placeholders skipped), each item's tier, and the
exact dispatch instruction. It spawns nothing, writes nothing, and touches no network. The autonomous
drive / live-window cutover is gate:human — BLOCKED-ON-IAN.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
`
