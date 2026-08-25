// Command deskdispatch is the DISPATCH-seam adapter verb for the desk drain loops.
//
// WHAT IT IS. Handing one queue item to one agent is a ceremony, not a call: take a
// durable claim so no second desk on no second machine dispatches the same item, cut a
// worktree in the item's OWN repo, register the work, make sure a human-gated item has a
// decision issue in front of the human, compute the dispatcher's model-attestation
// labels, and assemble the agent's prompt from the shared kits. Carried as prose, that
// ceremony ran to ~150 lines and was re-interpreted at every dispatch. deskdispatch is
// the one implementation of it, so two sessions dispatching the same item hand their
// agents byte-identical instructions.
//
// THE ENGINE SEAM. The drain engine's contract has a per-item Dispatch phase between
// selecting the queue and landing the result. deskdispatch is the ADAPTER a loop's
// Dispatch implementation calls — not an engine hook. Loops bind to this CLI, never to
// the engine's Go API, which is what lets the engine's internals change without a prose
// rewrite anywhere.
//
// WRAP, NEVER RE-IMPLEMENT. The durable claim and the decision-issue gate are consumer
// scripts that live in the repo being worked (`tools/dispatch-claim.sh`,
// `tools/decision-issue.sh`). deskdispatch INVOKES them; it does not carry a copy of
// either. A second implementation of a claim protocol is two claim protocols, and two
// claim protocols dispatch the same item twice — which is the exact failure the durable
// claim exists to prevent. Both scripts already speak the deskkit exit-code contract, so
// their verdicts pass straight through.
//
// ON CONTENTION, NAME THE HOLDER — NEVER STEAL. A claim held by someone else exits 5 with
// the existing holder printed. There is no inline steal: breaking a live claim is a
// deliberate, auditable act with a stated reason, and it belongs to the human or to the
// claim tool's own steal verb, never to a dispatcher in a hurry.
//
// WHAT IT DOES NOT DO. It does not launch the agent — the loop does that with the prompt
// this verb prints. It never opens a PR, never posts a review, never flips a status.
//
// Exit codes (deskkit contract): 0 dispatch prepared · 3 disabled · 5 refused (claim held
// by a live holder, or a caller precondition) · 6 unverifiable (a claim, worktree, or
// gate whose state could not be established — never "assume free"). See
// deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskdispatch — the per-item dispatch ceremony (engine seam: DISPATCH).

USAGE:
  deskdispatch <item-key> [--tier strong|any] [--kit worker|review|verifier]
               [--repo OWNER/NAME] [--root DIR] [--model SLUG] [--branch NAME]
               [--brief PATH] [--gate-human] [--pr N]
               [--prompt-file FILE] [--quiet] [--dry-run]
  deskdispatch --kits
  deskdispatch --version

<item-key> is the claim key, passed through UNCHANGED to the repo's claim tool. This verb
never invents, rewrites, or normalises a key: a key it reshaped would not collide with the
one another desk holds, and a claim that does not collide is not a claim.

STEPS, in order. Each prints one line; the first red one stops the dispatch and NAMES itself.

  1 claim-acquire     runs the target repo's tools/dispatch-claim.sh acquire <key>.
                      Exit 5 there = a LIVE holder owns it: this verb prints the holder
                      and exits 5. It never steals.
  2 worktree-create   ` + "`deskwt add`" + ` in the item's OWN repo root, off
                      refs/remotes/origin/main. Cross-repo is the default case, not the
                      exception: an item belongs to a repo, and a worker handed the wrong
                      one recreates the work where nobody asked for it.
  3 roster-register   ` + "`deskroster set`" + ` for the work entry when --pr is known; without
                      it the registration is the AGENT's first act after its PR opens, and
                      the exact command is emitted into the prompt.
  4 decision-gate     with --gate-human (or a --brief whose own metadata gates on a
                      human), runs the repo's tools/decision-issue.sh ensure so the human
                      has something concrete to decide. Idempotent by the script's own
                      marker dedupe.
  5 model-stamp       computes and validates the dispatcher's attestation labels
                      (dispatched-model:<slug>, dispatched-tier:<tier>) and applies them
                      when --pr is known. The stamp attests what the DISPATCHER launched;
                      a self-applied stamp is worthless by design.
  6 prompt-emit       writes the assembled agent prompt to stdout, or to --prompt-file.

--kits lists the prompt kits this binary carries and exits 0.
--dry-run runs no step: it prints the plan and the prompt that WOULD be emitted.
--quiet suppresses the per-step OK lines; failures and the prompt always print.

Exit: 0 dispatch prepared · 3 disabled · 5 refused (live claim holder / caller
precondition) · 6 unverifiable (a claim or gate whose state could not be established).`

func main() {
	// Explicit roster class: deskdispatch ACTS (it claims, it stamps, it registers), so
	// ciEligible=false — it reads the config-home file and never the environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskdispatch sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 1 && args[0] == "--kits" {
		for _, k := range kitNames() {
			fmt.Println(k)
		}
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		fmt.Fprintln(os.Stderr, "\nPROMPT KITS (references/, embedded in this binary): "+
			joinKits())
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill-switch check is the FIRST gated action.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	err := cmdDispatch(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
