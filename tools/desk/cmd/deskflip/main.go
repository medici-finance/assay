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
// TOCTOU. GitHub's ready mutation has no compare-and-swap, so the head is re-read
// immediately before the flip. A head that moved during the checks means the verified
// state is stale, and a stale verification is refused rather than acted on.
//
// RELATIONSHIP TO THE APP-IDENTITY WRITE PATH. This verb performs the mutation with the
// ambient forge CLI, as the operator running the loop. Where a flip must be recorded under
// the desk App's own identity, the App-identity write verb remains the path — it
// re-verifies the same conditions in-tool with its own credentials. deskflip is the
// role-gated LAND adapter, and it never widens what either path will accept.
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
  pr-open-draft     the PR is OPEN and still a draft. Anything else is nothing to flip.
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
  head-stable       the head is re-read immediately before the mutation; if it moved
                    during the checks, the verified state is stale — refuse.

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
	deskkit.WarnIfUnpinned(os.Stderr)

	err := cmdFlip(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
