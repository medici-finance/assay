// Command deskmerge answers, and (once a human grants it) fixes, ONE question about an
// open PR: is it current with `main`?
//
// WHY IT EXISTS. A large share of the stalled draft fleet is stalled only on
// merge-currency, and today bringing a PR current costs a full worker dispatch for a
// deterministic git operation. A register rule (R-5) would let the desk perform that one
// operation, on the theory that a clean two-parent merge of main introduces no
// reviewable content. deskmerge puts that theory in CODE, so the zero-authorship
// boundary is a compiled-in property rather than the desk's judgement.
//
// TWO VERBS, TWO DIFFERENT AUTHORITY QUESTIONS.
//
//	deskmerge check  — READ-ONLY. Reports currency in three states and writes nothing,
//	                   anywhere. Reading is not authorship, so `check` runs today,
//	                   ungated. This is the half that is useful while R-5 is unsigned.
//	deskmerge merge  — WRITES. Gated on R-5's Sign-off URL resolving to a comment
//	                   authored by the roster-pinned blessing authority. R-5 is UNSIGNED
//	                   as of 2026-08-13, so `merge` refuses (exit 5) and merges nothing.
//
// WHAT THIS INSTRUMENT CAN AND CANNOT DETECT. Stated up front because the failure this
// tool sits next to is precisely a checker that implied coverage it did not have.
//
//	CAN detect  — the branch is behind main; a `git merge main` would conflict, and on
//	              which paths; the conflicts are confined to the regenerable list; main's
//	              CI CONTRACT moved under the branch (new .github/workflows or
//	              .github/scripts since the merge-base), which is why a branch predating
//	              such a change fails a check that has nothing to do with its content —
//	              #898 added .github/scripts/verify-inscope.sh and any older branch fails
//	              it with exit 127, a merge-currency failure wearing a defect's clothes.
//	CANNOT detect on its own — SEMANTIC collisions between two individually-green
//	              branches that produce NO textual conflict. #912/#913:
//	              statusgen/emit.go's emit() grew an 8th parameter on main while another
//	              green PR still called it with 7 args; both PRs were green apart, main
//	              went red on merge. Nothing in a textual-conflict check sees that.
//	              `--probe` is the only answer offered: it BUILDS the merged tree in the
//	              temp worktree and reports the compiler's verdict. It catches exactly
//	              what the compiled-in probe catches and nothing else — a semantic
//	              collision in prose (docs/brief-rules.md currently carries DUPLICATE
//	              rule numbers 25 and 26, authored in parallel by two briefs,
//	              both merged green) is invisible to it.
//
// So `check` reports MERGE-CLEANLINESS. It never reports that the merged tree is
// VALID, and it never prints a word that could be read that way.
//
// THE ZERO-AUTHORSHIP BOUNDARY (merge verb), all compiled in:
//   - merge `main` INTO the PR branch, never the reverse. deskmerge never pushes to a
//     default branch and never calls `gh pr merge`. MERGE IS THE HUMAN'S. Pinned by
//     TestNeverMergesToMainOrCallsGhMerge.
//   - the result must be a TRUE two-parent merge commit whose parent 2 is the fetched
//     main head, verified by inspecting the COMMIT — not the intent. Anything else
//     (rebase, squash, fast-forward, the #72 single-parent masquerade) is
//     rolled back and refused. Never rebases.
//   - zero conflict hunks, OR every conflicted path on the compiled-in regenerable list
//     (regen.go) and resolved by re-running its generator. A MIXED conflict is refused
//     outright — there is no partial resolution.
//   - R-5's flip bound: a PR already flipped ready is refused, because a desk merge
//     would supply the very commit the desk's own at-head approval check is evaluated
//     against.
//   - the merge runs in a DETACHED temp worktree, removed on every exit path.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited ·
// 5 refused / checked-failed · 6 unverifiable / could-not-check.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskmerge"

const usage = `deskmerge — desk-side merge-currency: report it, or (once R-5 is signed) fix it.

USAGE:
  deskmerge check -R <owner/repo> <pr> [--probe] [--json] [--repo-root <dir>]
  deskmerge merge -R <owner/repo> <pr> [--dry-run] [--probe] [--repo-root <dir>]
                                       [--rulings <path>]
  deskmerge --version

check   READ-ONLY. Fetches, trials the merge in a detached temp worktree, and reports
        every dimension in three states — checked-clean / checked-failed /
        could-not-check. Writes nothing anywhere; needs no ruling. Exit 0 when the
        branch is mergeable by deskmerge (clean or regenerable-only, current or not),
        5 when it is CONFLICTED or a --probe build failed (that is worker work),
        6 when any dimension could not be examined.

merge   Merges origin's default branch INTO the PR branch and pushes the PR branch.
        Refuses unless R-5 in docs/streams/issue-flow/rulings.md carries a Sign-off URL
        that resolves to a comment authored by the roster-pinned blessing authority.
        R-5 is UNSIGNED today: this verb exits 5 and merges nothing. --dry-run runs the
        whole determination and stops before the write (clean/regenerable-only -> 0,
        conflicted -> 5) — and, being a read, is NOT ruling-gated.

--probe  Additionally BUILD the merged tree with the repo's compiled-in probe and report
         the result. This is the only detector deskmerge has for a semantic collision
         that produces no textual conflict (#912/#913). Without it,
         semantic validity is reported as NOT CHECKED, never as clean.

WHAT DESKMERGE NEVER DOES: push to a default branch, call ` + "`gh pr merge`" + `, rebase, squash,
produce a single-parent "merge", resolve a conflict outside the compiled-in regenerable
list, or run on a PR already flipped ready-for-review.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused/checked-failed ·
      6 unverifiable/could-not-check.`

func main() {
	// deskclose's reasoning applies unchanged: deskmerge ACTS on other people's
	// branches, so the roster comes from the config-home file and NEVER from the
	// environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(out, "deskmerge sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill switch is the FIRST action, before any verb payload is parsed — it
	// gates the read verb too. A desk that has been switched off must not be spending
	// someone's API budget on probes either.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	return deskkit.ExitCodeOf(dispatch(args, out))
}

// dispatch routes the verb. The verb set is CLOSED: the default arm is the enumeration,
// so there is no fall-through that could execute an unrecognised mode.
func dispatch(args []string, out io.Writer) error {
	verb, rest := args[0], args[1:]
	var err error
	switch verb {
	case verbCheck:
		err = cmdCheck(rest, out)
	case verbMerge:
		err = cmdMerge(rest, out)
	default:
		err = deskkit.Refused(fmt.Sprintf(
			"refused: unknown verb %q — the verb set is CLOSED: %s, %s",
			deskkit.StripControl(verb), verbCheck, verbMerge))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, toolName+":", err.Error())
	}
	return err
}

const (
	verbCheck = "check"
	verbMerge = "merge"
)
