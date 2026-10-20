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
// THE CLAIM TOOL, AND THE LEGACY-SCRIPT FALLBACK (issues 708, 1151). The claim script is a
// shebang bash file, so a freshly adopted tree that never received it — and a native-Windows
// adopter, where CreateProcess will not run a `.sh` — cannot dispatch through it. This verb
// therefore PREFERS `deskclaim-ref` (cmd/deskclaim-ref) whenever it resolves on PATH: a
// pure-Go port of the SAME wire protocol (the refs/dispatch/<id> claim ref, the same holder
// encoding, the same 0/5/6 exit codes), invoked by bare name with no shell. The resolved
// root's `tools/dispatch-claim.sh` is the fallback for a tree that predates the binary.
// Because both speak the same protocol and land the claim in the same ref, a Go dispatcher
// and a still-running bash dispatcher collide on one claim and never double-dispatch.
//
// THE CLAIM CHILD RUNS AS THE DISPATCHING ROLE, NEVER ON AMBIENT AUTH (issue 1151). Both
// claim tools read only the credential they are handed — `--token-file` for the binary,
// GH_TOKEN for the script — and until 1151 this verb handed them nothing, so a sandboxed
// desk window (no ambient `gh` login) failed closed on every fresh claim while its other
// steps minted their own role token and succeeded. Now the claim step mints (or reuses)
// the dispatching role's token through the same seam the model stamp uses and passes it in
// the tool's own shape; an explicit GH_TOKEN already in the environment wins only once it is
// verified to BE the dispatching role's App (issue 1631 — otherwise it is ignored with a NOTICE,
// and one whose identity cannot be read refuses); a mint
// failure is the refusal, never a fall-back to whatever `gh` is logged in as. The decision
// gate's script shells out to the forge CLI too, so it is handed the SAME credential in
// environment shape from that one resolution (issue 1146) — keyed on what the child does, not
// on whether its name is literally `gh`.
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
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// perfVerbs is deskdispatch's registered first-argument set for the opt-in local perf record
// (deskkit.PerfRun): args[0] is recorded only when it is one of these, and as
// "other" otherwise, so no argv text can reach the record.
var perfVerbs = []string{"--stamp-only", "--check-verifier", "--attest-verifier", "--kits", "--version", "--help", "-h", "help", "-version", "dispatch:worker", "dispatch:worker-objective", "dispatch:review", "dispatch:verifier", "stamp-only:worker", "stamp-only:worker-objective", "stamp-only:review", "stamp-only:verifier"}

func main() {
	// Wire the phantom check's PR-list transport LIVE for the shipped binary (represented_live.go).
	// Assigned here, in main(), not at package init: tests call run()/cmdDispatch directly and keep
	// the nil default (the check stays inert unless a test wires its own recorded transport), while
	// the real binary reads the repo's open+merged changes through the typed Forge seam.
	listRepresentedPRs = liveRepresentedPRs
	// ...and the per-PR file read that keeps a briefs-AUTHORING PR from counting as the brief's
	// delivery (authoring.go).
	listPRFiles = livePRFiles
	// ...and the review round's reads: the pre-dispatch gate and the lane's scope and round
	// number (reviewroundread.go). Unwired, a review dispatch holds nothing and states a full pass.
	reviewRoundForgeFn = liveReviewRoundForge
	// ...and the brief-admission decision hold's read (briefadmission.go). Unwired, a
	// human-gated brief is never held on its decision issue.
	decisionWaitForgeFn = liveDecisionWaitForge
	args := os.Args[1:]
	// main owns the declared class, echo and activation discovery. Cobra invokes
	// these callbacks only for execution, after help/version/kits and parsing.
	env := dispatchCLI{
		before: func() error {
			deskkit.SetToolClass(deskkit.ClassForTool(false))
			deskkit.EchoEffectiveConfig(os.Stderr)
			if !deskkit.CheckVerbActivation(os.Stderr) {
				return deskkit.Unverifiable("deskdispatch activation could not be established", nil)
			}
			return nil
		},
		handle: func(mode string, o dispatchOpts, override string) error {
			if err := dispatchGuard(); err != nil {
				return err
			}
			return executeMode(mode, o, override)
		},
	}
	// The one measurement finalizer wraps the whole command, parsing included, so
	// the timer never restarts after Cobra. Consent is the environment opt-in only;
	// help/version/kits classify themselves without reading operational config.
	os.Exit(deskkit.PerfRun("deskdispatch", perfVerbs, args, func(a []string) int {
		return parseDispatchCLI(a, env, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
	}))
}

func dispatchGuard() error {
	if err := deskkit.Guard(); err != nil {
		return err
	}
	deskkit.WarnIfUnpinned(os.Stderr)
	return nil
}

func run(args []string) int {
	return parseDispatchCLI(args, dispatchCLI{before: dispatchGuard, handle: executeMode}, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
}
