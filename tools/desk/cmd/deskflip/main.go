// Command deskflip is the LAND-seam adapter verb for the desk review loop: the
// ready-flip gate, made mechanical.
//
// WHAT IT IS. A draft PR becomes ready-for-human when — and only when — the reviewer App
// has APPROVED it AT THE CURRENT HEAD, every check is green, the PR is mergeable, a
// risk-classed PR additionally carries a security verdict at that same head, and the
// caller is the role that owns the flip. Those conditions were narrated in prose, which
// means they were re-derived by hand at every flip and, predictably, sometimes not all of
// them. deskflip is the ruling as a program: it re-reads each condition itself and, on a
// refusal, prints WHICH one failed.
//
// THE ENGINE SEAM. The drain engine's contract lands a completed item. deskflip is the
// ADAPTER the review loop's Land implementation calls — not an engine hook. The loop binds
// to this CLI, never to the engine's Go API.
//
// THE FLIP IS NOT AN APPROVAL AND NOT A MERGE. It moves a PR out of draft and swaps the
// queue-legibility labels so the PR now reads as waiting on the human rather than on the
// review lane. The merge remains the human's, always. deskflip has no un-ready verb, no
// merge verb, and no override flag — a gate a caller can wave past is not a gate.
//
// ROLE-GATED ON PURPOSE. The flip belongs to the role that WATCHED the review, so
// deskflip refuses unless the session presents that role's loop identity. That is not
// bureaucracy: a flip issued by a session that did not watch the review is a flip whose
// preconditions nobody was tracking as they changed.
//
// TOCTOU. The ready mutation has no compare-and-swap, so the state is re-read immediately
// before the flip — the head AND the reviewer verdicts. A head that moved means the
// verified state is stale. An unchanged head is NOT sufficient on its own: a
// `Security-Review: fail` is a retraction posted at the SAME head, so a head-only re-read
// would report "still current" and flip over a live withdrawal.
//
// ALREADY-READY PRs. The flip is idempotent, but the LABEL write is not bookkeeping — it
// asserts to everyone reading the queue that the review lane is done with this PR. So a PR
// already out of draft gets a pure no-op when its label is already correct, and a FULL
// re-gate before the label is written when it is not.
//
// IDENTITY. Every forge call this verb makes — the reads AND the mutation — runs under the
// review role's App installation token, resolved by the app-token condition before the
// first call. It does NOT fall back to the ambient forge CLI when that token is missing:
// it refuses. A flip and its queue labels written under the operator's own login read, in
// the timeline and to everyone afterwards, as a human decision, and unlike a failed read
// that cannot be taken back. The App-identity write verb remains a separate path — it
// re-verifies the same conditions in-tool with its own credentials — and deskflip never
// widens what either path will accept.
//
// Exit codes (deskkit contract): 0 flipped (or already flipped — idempotent) · 3 disabled ·
// 5 refused (a condition failed, and it is NAMED) · 6 unverifiable (a condition could not
// be READ — never rounded to green). See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskflip — the ready-flip gate (engine seam: LAND).

USAGE:
  deskflip <N> [--repo OWNER/NAME] [--root DIR] [--quiet] [--dry-run]
  deskflip --version

CONDITIONS. Every one is re-read here, in order; the first failure NAMES itself and
nothing is mutated:

  caller-role       $DESK_LOOP presents the review role's loop identity. The flip belongs
                    to the role that watched the review.
  app-token         the review role's App installation token is minted and readable, and
                    every forge call runs under it. There is NO ambient-credential
                    fallback: an unavailable token REFUSES (exit 5) naming the role and the
                    token path, because a flip written under an operator's own login reads
                    as a human decision and cannot be taken back.
  pr-open-draft     the PR is OPEN and still a draft. Anything else is nothing to flip.
  model-floor       the ready-flip is an authority-bearing write, so it requires a
                    strong-tier dispatch. The tier is read from the PR's DISPATCHER-ATTESTED
                    stamp (not a self-report): an attested below-tier dispatch, or a stamp
                    present-but-unreadable, is REFUSED with remediation. An UNATTESTED PR
                    (human-driven or pre-attestation) is not bricked — it proceeds with a
                    NOTICE. This is a floor, not a proof-of-model: it attests what the
                    dispatcher LAUNCHED, delegation downward stays legal, and escalating an
                    authority-bearing write upward does not. For incident recovery the env
                    toggle DESK_MODEL_FLOOR_OVERRIDE=1 bypasses the floor, and every bypass
                    is logged loudly (a silent one would nullify the layer). It is an env
                    toggle, never a wave-past-me flag on the verb.
  reviewer-approved the reviewer App's latest CORRECTNESS verdict is APPROVED and was
                    submitted AT THE CURRENT HEAD. A verdict at an earlier head is STALE,
                    which is a distinct answer from "no verdict".
  checks-green      every check at the head has completed successfully. A pending or
                    unreadable rollup is could-not-verify, never green.
  mergeable         the PR is mergeable. A conflicting PR is not flippable, and its
                    resolution is authored work that invalidates the approval anyway.
  security-verdict  on a RISK-CLASSED PR (a public repo is always one), an App review at
                    the CURRENT head carries the literal Security-Review: pass line. An
                    explicit fail at head blocks the flip whether risk-classed or not.
  head-stable       the head AND the reviewer verdicts are re-read immediately before the
                    mutation. A moved head means the verified state is stale; an unchanged
                    head is not enough on its own, because a security verdict can be
                    RETRACTED at the same head while the checks above were running.

ON PASS it performs the ready mutation and swaps the queue-legibility labels: the PR stops
reading as waiting on the review lane and starts reading as waiting on the human.

There is NO override flag, no un-ready verb, and no merge verb. The merge is the human's.

Exit: 0 flipped or already-flipped · 3 disabled · 5 refused (a condition failed, named) ·
6 unverifiable (a condition could not be read).`

func main() {
	// Explicit roster class: deskflip ACTS (it mutates a PR), and it reads the roster for
	// the reviewer App binding and the repo set — ciEligible=false.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskflip sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Outward verbs present a LOOP IDENTITY. The kill switch's per-loop halt is
	// `STOP.<loop>`, matched against $DESK_LOOP; with the variable unset nothing matches,
	// so a stop flag a human is holding never fires and this verb keeps writing while the
	// operator believes it has been halted. The boot verb has checked this since it was
	// written — an outward verb run OUTSIDE a booted window did not, which is the gap.
	if err := deskkit.RequireLoopIdentity("deskflip"); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	err := cmdFlip(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
