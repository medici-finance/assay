// Command verifyloop is the verify-desk REFERENCE CONSUMER of the deterministic drain engine
// (tools/desk/internal/loopengine). It exists to prove the engine
// contract end-to-end and to serve as the interim-mode debug surface — NOT (yet) to drive the
// standing verify window. The autonomous cutover (the verify desk actually booting this as its
// driver, writing Evidence + status straight to main) is gate: human — BLOCKED-ON-HUMAN.
//
// Subcommands:
//
//	verifyloop plan --root <repo> [--sha <targetSHA>]
//	    Deterministic scheduler OUTPUT: read the Awaiting queue, compute each item's tier, and
//	    print the EXACT dispatch instruction the operator would execute (or the human-route
//	    note for a risk-flagged item). No agents are spawned, nothing is written — this is the
//	    "the Go engine owns all scheduler state and emits exact dispatches" surface (§9.1).
//
//	verifyloop --version
//
// The `run` (autonomous drive) path is deliberately NOT exposed here: a Go conductor cannot
// call the harness Agent tool, and wiring the live window is the human-gated cutover.
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
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (correctness review finding: SetToolClass had no caller anywhere
	// in the tree, so every tool was ClassWrite only because nothing ever set it).
	// This tool ACTS on the roster, so ciEligible=false: config-home file only, never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Every run echoes the EFFECTIVE trust/authority
	// configuration to stderr before it does anything. The roster, the allowed-repo
	// set and the risk-path additions live in repository settings or a config-home
	// file, not in a diff — so the RUN is the only place a change to them becomes
	// visible, and a NARROWING has to be as visible as a widening. Logins and paths
	// only; never a token or a credential path.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("verifyloop sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
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
		// The OPERATING-ENVELOPE preflight runs at BOOT, before the queue is even
		// read. A red envelope is could-not-run for the WHOLE pass: one summary
		// line and exit — no queue read, no claim, no dispatch, and no issue filed
		// about the desk's own envelope (ground-truth/07; #794 #571 #823 #638 #679).
		if err := preflightBoot(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return deskkit.ExitCodeOf(err)
		}
		return loopengine.ExitCodeOf(cmdPlan(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "verifyloop: unknown subcommand %q\n\n%s", args[0], usage)
		return deskkit.ExitRefused
	}
}

func cmdPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	root := fs.String("root", ".", "repo root to scan for the Awaiting queue")
	sha := fs.String("sha", "", "merged-main target SHA verifiers run against")
	runner := fs.String("runner", "", "this session's runner identity (author!=runner guard)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("bad flags: " + err.Error())
	}

	v := &VerifyLoop{Root: *root, TargetSHA: *sha, RunnerID: *runner}
	items, err := v.SelectQueue()
	if err != nil {
		return deskkit.Unverifiable("cannot read the Awaiting queue", err)
	}
	fmt.Printf("verify-desk plan: %d brief(s) awaiting (tier-1 first, oldest-first within class)\n", len(items))
	for _, it := range items {
		// author != runner is a STRUCTURAL engine guard, shown here for transparency.
		if err := loopengine.CheckAuthorRunner(it, *runner); err != nil {
			fmt.Printf("\n-- %s: REFUSED (%v) — needs a different runner\n", it.ID, err)
			continue
		}
		tier, terr := v.TierPolicy(it)
		if terr != nil {
			fmt.Printf("\n-- %s: tier error: %v\n", it.ID, terr)
			continue
		}
		if tier == loopengine.TierHuman {
			fmt.Printf("\n-- %s: ROUTE-HUMAN (risk-flagged; checkpoint-PR / labeled-issue path, drain continues)\n", it.ID)
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

const usage = `verifyloop — verify-desk reference consumer of the drain engine.

USAGE:
  verifyloop plan --root <repo> [--sha <targetSHA>] [--runner <id>]
  verifyloop --version

'plan' prints the deterministic scheduler output: the Awaiting queue, each item's tier, and
the exact dispatch instruction (or the human-route note). It spawns nothing and writes nothing.
The autonomous drive / live-window cutover is gate: human — BLOCKED-ON-HUMAN.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
`
