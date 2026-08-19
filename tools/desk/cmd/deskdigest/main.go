// Command deskdigest builds ONE batched weekly decision queue for the human, and
// nothing else. It is a REPORT: it never closes an issue, never flips a PR, never
// applies a label, and never enacts a decision of its own.
//
// THE PROBLEM. Decision items reach the human as individual issues hoping to be
// noticed. Measured live on 2026-08-13 across the two tracked repos, 21 open
// `needs-decision` issues were queued on one person, the oldest open since
// 2026-07-15 (29 days). Each one is a separate interrupt with its own context, and
// several park real work behind them. Batching them into one weekly issue — every
// item carrying its age, its classification, the desk's recommendation and a
// default-if-no-answer date — converts N interrupts into one sitting.
//
// THREE INVARIANTS, and they are the reason the tool exists rather than niceties:
//
//  1. AN EMPTY WEEK IS A REPORT, NOT A SKIP. A digest that silently omits a week
//     with nothing in it is indistinguishable from a digest that failed to run. A
//     zero-item week renders — and posts — a digest that SAYS it is zero, names the
//     repos it read, and says how it knows.
//
//  2. NO SAFE DEFAULT. An item the classifier cannot place renders as
//     `unclassified` — needs a look — and NEVER silently as `reversible`. The
//     desk-decides lane is entered only by a POSITIVE signal. Nothing falls into it
//     by exhaustion of the alternatives.
//
//  3. COULD-NOT-READ IS ITS OWN STATE. A repo whose issues did
//     not come back, a label that does not exist yet, an item whose comments could
//     not be fetched — each renders as itself. None of them collapses into "nothing
//     to report", and none of them collapses into a classification. Every input
//     collected appears in exactly one section of the output, and the digest states
//     that count so a reader can check the arithmetic.
//
// WHAT IT COMPOSES RATHER THAN REDERIVES.
//
//   - CLOSURE belongs to deskclose. Superseding last week's digest is
//     a close, so deskdigest PRINTS the exact `deskclose superseded` command and
//     performs nothing. It constructs no closing argv on any path.
//   - FILING belongs to deskfile. deskdigest writes exactly one issue
//     — its own weekly digest — and files nothing else.
//   - The SLA / ESCALATE lane belongs to issueboard. deskdigest
//     renders plain item age and points at issueboard for the escalation clock; it
//     does not re-implement a second, drifting threshold.
//   - R-3's SIGN-OFF GATE belongs to deskclose's authority check, which FETCHES and
//     VERIFIES the artifact because it ACTS on it. deskdigest acts on nothing, so it
//     reads only the CLAIM on the Sign-off line and says so in those words. It never
//     reports a ruling as signed on the strength of a local file.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited ·
// 5 refused · 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskdigest"

const usage = `deskdigest — one batched weekly decision queue. A report: it decides nothing.

USAGE:
  deskdigest [--dry-run] [--post --digest-repo <owner/repo>] [flags]
  deskdigest --version

  (no flag)          same rendering as --dry-run: print the digest markdown, write nothing.
  --dry-run          print the digest markdown to stdout. No outward write, ever.
  --post             create-or-update THIS WEEK's digest issue in --digest-repo.
  --digest-repo R    where the weekly digest issue lives (required by --post; must be in
                     the desk-tools repo set — deskdigest introduces no repo list).
  --repos a,b        the repos to read. Required unless --all-repos is given: deskdigest
                     does not default its own read target.
  --all-repos        read the roster's whole scan scope. The explicit opt-in that replaces
                     a silent default — a bare run reads nothing and says why.
  --rulings <path>   where to read R-3 from (default docs/streams/issue-flow/rulings.md).
                     The file states a CLAIM; deskdigest never treats it as authority.
  --now <RFC3339>    freeze the clock (week id, ages, veto/default dates).
  --week <ISO>       override the ISO week id, e.g. 2026-W33.

INPUT. Open issues labelled ` + "`needs-decision`" + ` plus open issues labelled
` + "`human-only`" + ` (infra only the human can perform: App permissions, token rotation,
key custody), across the read scope. A label that does not EXIST in a repo is reported as
such — zero rows under a missing label is not an empty queue, and the digest never lets
those two read the same.

CLASSIFICATION (R-3), highest precedence first:
  1. a comment by the roster's blessing authority carrying ` + "`decision-class: …`" + `
  2. the ` + "`human-only`" + ` label                       → human-only
  3. a ` + "`decision-class: …`" + ` line in the issue body, honoured only for a
     TRUSTED author — an untrusted body must not move an item into the desk-decides lane
  4. R-3's own criteria, applied conservatively: any irreversible / mechanism / security /
     spend signal → human-only; a one-commit-reversal signal and no human-only signal →
     reversible; anything else → UNCLASSIFIED.
There is no fifth arm. Unclassified is where an unrecognised item lands, by design.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.
An exit 6 from --post can mean the digest WAS posted and says, in its own body, which
repos it could not read. A partial digest that admits it is partial beats no digest.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident.
	// deskdigest reads the roster for its trust decisions (who is the blessing
	// authority, which authors are trusted) and writes one issue, so ciEligible=false:
	// the roster comes from the config-home file and never from the environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run: the blessing authority whose comment
	// outranks every other classification input is a control surface that lives in
	// settings rather than in a diff, so it is visible only at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	// --version / help are pure introspection: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskdigest sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fmt.Fprintln(stderr, usage)
			return deskkit.ExitOK
		}
	}

	// The kill switch is the FIRST action, before any flag payload is parsed. Guard
	// writes its own result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(stderr)

	err := dispatch(args, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, toolName+":", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
