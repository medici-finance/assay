// Command deskpr is the worker-side desk tool.
// It encodes ONE workflow verb — "open the draft PR for the branch I'm on" — plus its
// hot-path sibling "push a follow-up to that same open PR". create is draft-only BY
// CONSTRUCTION: no code path can omit --draft. update pushes to an EXISTING open PR on
// the branch — draft or ready-flipped — but neither verb can emit a git --force, and
// there is no verb for edit/close/merge/ready (ready lives in deskpost with its
// preconditions).
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
  deskpr --version

deskpr create is draft-only by construction: it can only open a DRAFT PR on a
non-default branch. deskpr update pushes a follow-up to an EXISTING open PR on the
branch — draft or ready-flipped. There is no ready/edit/close/merge verb, and neither
verb can pass --force to git. Preconditions are re-verified in-tool;
on any state it cannot positively verify it refuses.

By default, --as-app is true: gh calls authenticate as
the worker App via desktoken worker. Pass --as-app=false for the example-org
fallback (transition period). The branch push (committed code) is the worker's git
authorship; the PR is filed under the worker App identity.

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
      ASSAY_WITHHELD_IDENTIFIERS

  NOTICE on stderr, never a refusal — the check could not decide:
    * a bare #N above a number known to exist here (probably another repo's)
    * a bare #N with no reference number available: not checked at all
    * a word that is a PRIVATE repo's short name (indistinguishable from prose)
    * ASSAY_WITHHELD_IDENTIFIERS unset: that category was not checked

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

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "create":
		err = cmdCreate(rest)
	case "update":
		err = cmdUpdate(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskpr: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
