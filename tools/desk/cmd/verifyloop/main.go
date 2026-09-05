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

	// `verifyloop --dry-run [--root .]` is shorthand for `verifyloop verdict --dry-run …`:
	// the deterministic runner's CI-testable surface (compose + sign + print, no filing).
	if args[0] == "--dry-run" || args[0] == "-dry-run" {
		return cmdVerdict(args)
	}

	switch args[0] {
	case "verdict":
		// The deterministic runner: run check/check:ci rows, batch ~5 min, sign, print the
		// would-be verify-verdict issue body. Filing is BLOCKED-ON-HUMAN (autonomous cutover).
		return cmdVerdict(args[1:])
	case "plan":
		// The OPERATING-ENVELOPE preflight runs at BOOT, before the queue is even
		// read. A red envelope is could-not-run for the WHOLE pass: one summary
		// line and exit — no queue read, no claim, no dispatch, and no issue filed
		// about the desk's own envelope.
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

	// bucketed collects the non-dispatchable dispositions (deferred + the three buckets) so
	// the dispatchable list stays the genuinely-actionable set and the rest are surfaced with a
	// count and a one-line "why it waits", never silently listed as DISPATCH (queueclass.go).
	bucketed := map[disposition][]bucketMember{}
	dispatchable := 0
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
		if disp, reason := classifyItem(it, tier); disp != dispDispatch {
			bucketed[disp] = append(bucketed[disp], bucketMember{ID: it.ID, Reason: reason})
			continue
		}
		prompt := renderDispatchPrompt(it, tier)
		if err := assertNoSharedCheckout(prompt); err != nil {
			return deskkit.Refused(err.Error())
		}
		dispatchable++
		fmt.Printf("\n=== DISPATCH %s (tier=%s) ===\n%s\n", it.ID, tier, prompt)
	}

	printBuckets(dispatchable, bucketed)
	return nil
}

// bucketMember is one non-dispatchable queue item: its ID plus the per-item reason detail
// (the blocked-until condition, the lane name, or the repair pipeline reference; empty for
// awaiting-human, whose why is identical for every member).
type bucketMember struct {
	ID     string
	Reason string
}

// printBuckets renders the deferred section and each non-empty bucket, each headed by a count
// and its one-line "why it waits" note. The order is deterministic (deferred, then the three
// buckets) so the output is stable across runs.
func printBuckets(dispatchable int, bucketed map[disposition][]bucketMember) {
	total := 0
	for _, m := range bucketed {
		total += len(m)
	}
	fmt.Printf("\nverify-desk plan: %d dispatchable, %d deferred/bucketed (not offline-convertible this run)\n",
		dispatchable, total)
	for _, disp := range []disposition{dispDeferred, dispAwaitingHuman, dispAwaitingOnlineLane, dispInRepair} {
		members := bucketed[disp]
		if len(members) == 0 {
			continue
		}
		fmt.Printf("\n-- %s (%d): %s\n", disp, len(members), disp.whyItWaits())
		for _, m := range members {
			if m.Reason != "" {
				fmt.Printf("   %s — %s\n", m.ID, m.Reason)
			} else {
				fmt.Printf("   %s\n", m.ID)
			}
		}
	}
}

const usage = `verifyloop — verify-desk reference consumer of the drain engine.

USAGE:
  verifyloop plan    --root <repo> [--sha <targetSHA>] [--runner <id>]
  verifyloop verdict --root <repo> [--dry-run] [--window 5m] [--runner <id>] [--pem <path>]
  verifyloop --dry-run [--root <repo>]        # shorthand for 'verdict --dry-run'
  verifyloop --version

'plan' prints the deterministic scheduler output: the Awaiting queue, each item's tier, and
the exact dispatch instruction (or the human-route note). It spawns nothing and writes nothing.
The item keys it prints (<stream>/<NN>) resolve to their file, frontmatter and board row with
'statusgen brief <key>'.

'verdict' is the DETERMINISTIC runner: it runs each brief's check/check:ci Verify rows locally
(exit code = verdict), batches results over the flush window into ONE signed verdict-v1
payload, and prints the would-be verifier-App issue body. --dry-run composes + signs + prints
without filing (the CI-testable surface). A missing verifier PEM is a loud envelope error and
nothing is signed. Filing the issue is the autonomous cutover — gate: human, BLOCKED-ON-HUMAN.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
`
