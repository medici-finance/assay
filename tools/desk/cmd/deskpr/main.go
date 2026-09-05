// Command deskpr is the worker-side desk tool.
// It encodes ONE workflow verb — "open the draft PR for the branch I'm on" — plus its
// hot-path siblings "push a follow-up to that same open PR" and "correct that PR's own
// body/title text". create is draft-only BY CONSTRUCTION: no code path can omit --draft.
// update pushes to an EXISTING open PR on the branch — draft or ready-flipped. edit
// replaces the body (and optionally the title) of that same open PR through the gates
// create runs, and pushes nothing. No verb can emit a git --force, and there is no verb
// for close/merge/ready (ready lives in deskpost with its preconditions).
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled,
// 4 rate-limited, 5 refused, 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskpr — push a feature branch and open (or update) its pull request.

USAGE:
  deskpr create --title T (--body-file F | --body-min B) [--base main] [--as-app=false]
  deskpr update [--as-app=false]
  deskpr edit --body-file F [--title T] [--as-app=false]
  deskpr --version

deskpr create is draft-only by construction: it can only open a DRAFT PR on a
non-default branch. deskpr update pushes a follow-up to an EXISTING open PR on the
branch — draft or ready-flipped. deskpr edit replaces that same open PR's body, and
optionally its title, and pushes nothing: it refuses when the branch has no OPEN PR
(which is also how a merged or closed one is refused), and it runs the trailer,
secret-scan, self-containment, rate-limit and public-repo gates create runs. There is
no ready/close/merge verb, and no verb can pass --force to git. Preconditions are
re-verified in-tool; on any state it cannot positively verify it refuses.

deskpr edit cannot change the body's link trailer. "Brief: <stream>/<NN>" / "Issue: #<N>"
is the derived board's edge from the PR to its work item: the replacement body must carry
exactly one, and when the PR's current body already has one, the replacement's must match
it. A PR whose current body has NO trailer may gain one — that is the pre-trailer
migration deskpr update tells you to perform. Because a body edit moves no head SHA,
edit also posts one short comment naming what changed, so a head-keyed review monitor
has an event to see.

By default, --as-app is true: gh calls authenticate as this session's App role via
desktoken, resolved from the loop identity ($DESK_LOOP). That is the worker App by
default, and the VERIFIER App under DESK_LOOP=verify-desk — so an Evidence PR is filed
under the same App that authored its branch commits, not misattributed to the worker
(#396). Pass --as-app=false for the example-org fallback (transition period). When no
loop carries an App role the worker App is the default. The branch push (committed
code) carries the role App's git authorship; the PR is filed under that same App.

PUBLIC-REPO SELF-CONTAINMENT (#203). When the target repo is not known-private, the
PR body and title are scanned for spans that only resolve inside the authoring house.
This is the ONE place the categories are enumerated; the skill text points here rather
than restating them.

  REFUSED (exit 5) — the span is unambiguous:
    * an absolute machine path (/Users/…, /home/…, /private/tmp/…, /tmp/tracker-…)
    * a scratch worktree name (tracker-…)
    * a session id (a hex UUID) or an agent id (agent-…)
    * an owner/name slug, with or without #N, naming a repo the roster marks PRIVATE
    * alias#N where the alias resolves to such a repo
    * a withheld register identifier (a stream slug, or a <slug>/<NN> brief id) from
      ASSAY_WITHHELD_IDENTIFIERS, read from roster.env or from the environment

  NOTICE on stderr, never a refusal — the check could not decide:
    * a bare #N above a number known to exist here (probably another repo's)
    * a bare #N with no reference number available: not checked at all
    * a word that is a PRIVATE repo's short name, 4+ characters (a shorter alias is
      ordinary English and is not noticed at all; its full slug still REFUSES)
    * ASSAY_WITHHELD_IDENTIFIERS not configured in roster.env or the environment:
      that category was not checked

A refusal takes the same audited --force-scan-override as any other scan refusal —
there is no second bypass and no flag that turns the check off. A known-private target
repo, and any repo when the roster is unconfigured, are unaffected.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (an earlier correctness review found SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskpr sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill-switch check is the FIRST action of the tool (before flag parsing of
	// the verb's payload). Guard writes its own result=disabled audit line.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Outward verbs present a LOOP IDENTITY. The kill switch's per-loop halt is
	// `STOP.<loop>`, matched against $DESK_LOOP; with the variable unset nothing matches,
	// so a stop flag a human is holding never fires and this verb keeps writing while the
	// operator believes it has been halted. The boot verb has checked this since it was
	// written — an outward verb run OUTSIDE a booted window did not, which is the gap.
	if err := deskkit.RequireLoopIdentity("deskpr"); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "create":
		err = cmdCreate(rest)
	case "update":
		err = cmdUpdate(rest)
	case "edit":
		err = cmdEdit(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskpr: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
