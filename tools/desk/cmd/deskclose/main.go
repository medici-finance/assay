// Command deskclose closes issues and pull requests as the PROPAGATION OF A
// HUMAN-AUTHORIZED EVENT — never as a judgement of its own.
//
// The problem it exists for: the reject/duplicate exit is unused (2-4% of closes)
// because no agent may close an issue and the only sanctioned exit is a full fix-PR
// cycle. A signed ruling (R-1) grants a small number of narrow close lanes. This tool
// puts that grant in CODE, so a close outside the lanes is impossible rather than
// merely forbidden.
//
// THE AUTHORIZATION GATE IS THE POINT. deskclose closes other people's work in bulk.
// The single property that makes that safe is that every closure traces back to a
// human-authored artifact that deskclose FETCHED and VERIFIED — not to a flag the
// caller set, not to a login the caller claims, and not to a file the caller wrote.
// Two gates, both fail-closed, both applied before any write:
//
//  1. THE RULING GATE (every mode). R-1 in docs/streams/issue-flow/rulings.md must
//     carry a Sign-off URL, and that URL must resolve to a comment authored by the
//     CONFIGURED BLESSING AUTHORITY (deskkit.IsBlessAuthorityIDStrict — a human
//     login pinned to a numeric user id). Until R-1 is signed, deskclose closes
//     NOTHING, in any mode. The file states the CLAIM; the fetch is what makes it
//     authority. A caller who edits rulings.md in their own worktree only changes
//     which URL gets fetched — they cannot forge the author of the comment at it.
//
//  2. THE MANIFEST GATE (manifest mode). The manifest names its own authorizing
//     comment, and that comment's body must carry the manifest's content digest.
//     Authorization therefore binds to an EXACT row set: it cannot be replayed onto
//     a different manifest, and a row added after the human looked invalidates it.
//
// WHY LOGIN ALONE CANNOT GATE THIS. The desk, the workers and the human all reach
// GitHub through overlapping identities, and a shared automation account reports
// `"type": "User"` exactly as a person does. So type-checking is necessary and not
// sufficient: the authorizing author must be the roster's single pinned blessing
// authority. An App identity can never be the blessing authority (the roster loader
// refuses to accept one), and a shared automation login is not it either.
//
// THERE IS NO --force, NO --yes, AND NO ENVIRONMENT OVERRIDE. Every refusal below is
// reachable only by satisfying it. TestNoForceEscapeHatch pins that.
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

const usage = `deskclose — close issues/PRs only as propagation of a human-authorized event.

USAGE:
  deskclose duplicate      -R <owner/repo> <N> --of <M> --mined <summary>
  deskclose superseded     -R <owner/repo> <N> --by <ref> [--dispute <reason>]
  deskclose review-request -R <owner/repo> <N>
  deskclose manifest       -R <owner/repo> --file <manifest.yaml> [--resume-from <N>]
                                                                 [--max-wait <dur>]
  deskclose --version

Every mode accepts --dry-run (validate + read the remote, write nothing) and
--rulings <path> (where to read R-1 from; the SIGN-OFF URL in it is fetched and
verified either way, so this flag cannot manufacture authority).

duplicate      — REFUSED today. R-1 withdraws the duplicate lane from the desk: the
                 two-role duplicate procedure governs (a strong-tier worker folds the
                 loser's unique content into the FIRST-FILED survivor and marks the
                 second; a REVIEWER closes it, and only after agreeing the content
                 moved). The verb exists, validates its inputs — --mined is
                 mandatory — and then exits 5 naming both roles. It unlocks only if
                 R-1's sign-off EXPLICITLY supersedes the two-role ruling, which is what
                 R-1's own conflict disclosure requires.

superseded     — the item's substance is fully captured by --by <ref>. TWO-ROLE, keyed
                 on the TOKEN in use (its roster binding), never on a flag:
                   worker token   → PROPOSES: label ` + "`superseded?`" + ` + a marker comment
                                    naming the target. Never closes; --dispute refused.
                   reviewer token → CONFIRMS: needs a standing proposal by a different
                                    actor naming the same target; the target PR must be
                                    genuinely MERGED (closed-unmerged never satisfies);
                                    posts SUPERSEDED-CONFIRMED + a back-reference on the
                                    target, then closes not-planned.
                                  → --dispute <reason>: posts SUPERSEDED-DISPUTED and
                                    applies needs-decision, after which every close is
                                    refused — the item is a human's from there.
                 Any other token is refused; an unreadable one is could-not-check.

review-request — extracts the PR ref from the issue body; absent or ambiguous is a
                 refusal, never a guess. Verifies MERGED. Closes completed.

manifest       — a batch, authorized once by a human, applied ONE ROW AT A TIME with
                 a per-row audit line. Already-closed rows are idempotent no-ops. A
                 hard error stops the run and prints the exact --resume-from to
                 continue with; a rate-limit refusal WAITS and retries the SAME row.
                 No row is ever skipped.

DISPOSITION RECORDS. A pull-request target must already carry a machine-readable
disposition record (deskdisposition: the disposition:<verdict> label plus the
<!-- desk-disposition v1 --> marker comment). deskclose READS that record through
` + "`deskdisposition read --json`" + ` — it does not re-derive the verdict and does not
parse the marker itself. NEEDS-REBASE means live work and is refused; a record whose
Evidence names a different item than the caller declared is refused; an unreadable
record is could-not-check and is refused.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident.
	// deskclose ACTS on the roster in the strongest sense any desk tool does — it
	// closes other people's items — so ciEligible=false: the roster comes from the
	// config-home file and NEVER from the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run: the blessing authority this binary will
	// accept as the authorizing human is a control surface that lives in settings
	// rather than in a diff, so it is visible only at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(out, "deskclose sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill switch is the FIRST action, before any verb payload is parsed. Guard
	// writes its own result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	return deskkit.ExitCodeOf(dispatch(args, out))
}

// dispatch routes the verb. An unknown mode is REFUSED (exit 5) and the refusal names
// the closed set — Verify row 1's "any other mode string → exit 5". The default arm is
// the enumeration: there is no fall-through that could execute an unrecognised mode.
func dispatch(args []string, out io.Writer) error {
	verb, rest := args[0], args[1:]
	var err error
	switch verb {
	case modeDuplicate:
		err = cmdDuplicate(rest, out)
	case modeSuperseded:
		err = cmdSuperseded(rest, out)
	case modeReviewRequest:
		err = cmdReviewRequest(rest, out)
	case modeManifest:
		err = cmdManifest(rest, out)
	default:
		err = deskkit.Refused(fmt.Sprintf(
			"refused: unknown mode %q — the mode set is CLOSED: %s. "+
				"A close outside these lanes is not a missing feature; it is the thing R-1 declines to grant.",
			deskkit.StripControl(verb), modeList()))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskclose:", err.Error())
	}
	return err
}
