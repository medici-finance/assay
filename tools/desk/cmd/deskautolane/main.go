// Command deskautolane is the AUTO-APPROVE LANE's verb: it answers whether a PR is in the
// lane, recomputes the gate-time demotion score that can eject it, and evaluates the full
// condition chain a lane merge would have to pass.
//
// WHAT THE LANE IS. A narrow lane in which the reviewer App may merge a class of PRs with no
// per-merge human act. A PR is admitted by CATEGORY — every changed path inside an area a
// named human opted in (operator config, ASSAY_AUTOAPPROVE_AREAS), no tripwire firing — and
// it stays in the lane only while a SCORE recomputed at every gate finds no demotion signal.
// The score ejects; it never admits. An ejection is one-way. The decision logic lives in
// deskkit (autolane.go); this verb reads the forge, calls it, and performs the lane's writes.
//
// INERT AS SHIPPED — three independent reasons, any one of which is enough:
//
//  1. CONFIG. The four ASSAY_AUTOAPPROVE_* roster keys are absent by default, and absent is
//     CLOSED: every verb refuses at condition `config` before its first forge request.
//  2. ENACTMENT. Every WRITE this verb can make (the admission label, the ejection label
//     swap and its one comment) is gated on the rulings register's R-8 Sign-off line — read
//     through the forge at the register repo's default branch — naming a comment on the
//     ONE configured sign-off thread in that repo, an issue, whose author is a User and the
//     roster-pinned blessing authority, whose body opens with the bare line `Enact: R-8` and
//     carries no word from the rejection lexicon, which was created after the latest change
//     to R-8's text that the register's path history records, and which no later acceptance
//     by the same authority on that thread supersedes. The line ships EMPTY, so every write
//     refuses `ruling-unsigned`.
//  3. NO MERGE WRITE. This release carries no merge mutation at all. `merge` evaluates the
//     whole condition chain and, with --dry-run, reports what it would do; without
//     --dry-run it refuses at `merge-write` after every condition held. The forge operation
//     that performs a lane merge is a separate, later change.
//
// NEVER A FLAG ON deskflip. The ready flip has no merge verb and no override flag, and that
// contract stays whole: a defect here cannot widen the ready flip, and a closed lane leaves
// the ready flip untouched.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 4 rate-limited · 5 refused / ejected ·
// 6 could-not-check. There is no override flag.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = deskkit.AutoLaneToolName

const usage = `deskautolane — the auto-approve lane: category admits, a gate-time score ejects.

USAGE:
  deskautolane check     [<pr>] --repo <owner/repo> [--rulings-repo <owner/repo>] [--rulings <path>] [--root <dir>] [--fpy-file <path>]
  deskautolane recompute  <pr>  --repo <owner/repo> [--rulings-repo <owner/repo>] [--rulings <path>] [--root <dir>]
  deskautolane merge      <pr>  --repo <owner/repo> [--dry-run] [--rulings-repo <owner/repo>] [--rulings <path>] [--root <dir>] [--fpy-file <path>]
  deskautolane --version

check      READ-ONLY. Reports the lane's config, enactment and kill-signal state and, given a
           PR, its category admit, demotion score and ejection latch. Writes nothing.
           Exit 0 only when the lane is enacted and armed and the PR (if named) is admitted
           and unejected; 5 otherwise; 6 on could-not-check.
recompute  Recomputes the score at the current head. A PR in the lane whose category or score
           fails is EJECTED (auto-lane label removed, the human-queue label added, ONE marked
           comment, an autolane:eject audit line — the one-way latch). An admissible PR not
           yet in the lane is ADMITTED (the auto-lane label). Both writes require the R-8
           enactment gate; unenacted, recompute reports what it would do and writes nothing.
merge      Evaluates, in a pinned order, every condition a lane merge must pass. --dry-run
           writes NOTHING: it prints "dry-run: would merge ..." or "dry-run: would eject: ..."
           and stops. Without --dry-run a failing category or score EJECTS (as recompute
           does), and a PR that passes every condition is refused at merge-write: this
           release carries no merge mutation.

The lane is CLOSED unless the four required ASSAY_AUTOAPPROVE_* roster keys are set, and every write
is refused until the enactment gate holds: R-8's Sign-off line in the rulings register
(--rulings, default docs/streams/issue-flow/rulings.md), read through the forge from
--rulings-repo (default --repo) at its DEFAULT branch, names a comment on the one configured
sign-off thread (ASSAY_AUTOAPPROVE_SIGNOFF_THREAD, an issue; unset is could-not-check) in that repo,
whose author is a User and the blessing authority, whose FIRST non-empty line is the bare
line "Enact: R-8" with no word from the rejection/negation lexicon in the body, and which
was created after the latest change to R-8's text above its Sign-off line that the
register's path history records (the merging PR's merged_at; a Sign-off-only change does not
count; the forge's history is simplified and can omit a change a merge reverted), and which no
later acceptance by the authority on that thread supersedes. --root names only the local
tree whose .assay-surfaces the only-narrowing config check reads. --fpy-file names the
harvested per-class first-pass-yield file; absent or unreadable, the lane HOLDS.

Exit: 0 ok · 3 disabled · 4 rate-limited · 5 refused/ejected · 6 could-not-check.`

func main() {
	// The lane ACTS on other people's PRs, so its roster — including the lane keys — comes
	// from the config-home file and never from the environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errw io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(out, "%s sourceSHA=%s builtAt=%s\n", toolName, sha, built)
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(errw, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}
	// The kill switch first, before any payload is parsed — it gates the read verb too.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(errw, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(errw)

	o, err := parseArgs(args, out)
	if err != nil {
		fmt.Fprintln(errw, toolName+":", err.Error())
		return deskkit.ExitCodeOf(err)
	}
	verr := dispatch(o)
	audit(o, verr)
	if verr != nil {
		fmt.Fprintln(errw, toolName+":", verr.Error())
	}
	return deskkit.ExitCodeOf(verr)
}

// dispatch routes the verb. The verb set is CLOSED.
func dispatch(o *opts) error {
	switch o.verb {
	case verbCheck:
		return cmdCheck(o)
	case verbRecompute:
		return cmdRecompute(o)
	case verbMerge:
		return cmdMerge(o)
	}
	return deskkit.Refused(fmt.Sprintf("refused: unknown verb %q — the verb set is CLOSED: %s, %s, %s",
		deskkit.StripControl(o.verb), verbCheck, verbRecompute, verbMerge))
}
