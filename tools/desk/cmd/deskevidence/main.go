// Command deskevidence commits Evidence rows as the verifier App
// (the verifier App) — the unforgeable verify-desk identity
// It replaces hand-editing + `git commit` in the
// verify-desk loop:
//
//	deskevidence <owner/repo> <branch> --evidence-file <repo-path>
//
// Reads the local file at <repo-path>, BodyChecks it, fetches the
// current file from GitHub on <branch> to get the tree SHA, checks
// idempotency (same content at same head → noop), and commits via the
// GitHub Contents API AS the verifier App. The commit author is
// the verifier App because the API call uses the App's installation
// token — unforgeable (a PAT commit would be example-org, which is the
// whole point).
//
// The tool has NO push, no merge — commit only (weakest-verb). "Commit
// only" does NOT mean the write is local: a Contents-API PUT carries the
// branch, so the commit lands on the REMOTE branch as soon as the call
// returns. There is nothing left to push and nothing to stage — including
// on main. Because of that, main/master are refused unless VERIFIER_MAIN_OK
// is exactly "1" in the environment (the prefix "refs/heads/" is stripped
// before the comparison, so refs/heads/main does not slip past), the repo
// must be in the compiled-in set (deskkit.IsAllowedRepo), and a target
// named STATUS.md is refused on every branch because that file is generated
// and main's CI is its single writer. All three refusals are exit 5 and
// happen before any network call.
//
// Inherits deskkit: Guard, BodyCheck, AllowWrite,
// audit + idempotency, fail-closed.
//
// Exit codes: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// version is an optional bare-`vX.Y.Z` build stamp (`-ldflags -X main.version`)
// feeding the brief-reading version gate (derived-board/06); empty on a real
// release, where the namespaced ReleaseTag stamp supplies the version.
var version string

const usage = `deskevidence — commit Evidence rows as the verifier App (the verifier App).

USAGE:
  deskevidence <owner/repo> <branch> --evidence-file <repo-path> [--root <dir>]
               [--brief-path <repo-path>] [--append-only] [--allow-shrink]
  deskevidence --version

Commits the content of the local file at --evidence-file (a repo-relative path)
to the same path on <branch> via the GitHub Contents API, using the verifier App
installation token. Author = the verifier App.

--root <dir> resolves a repo-relative --evidence-file against <dir> (e.g. the verifier
worktree) instead of the current working directory, so a stale cwd cannot read the wrong
checkout's copy (#1709). The path committed to the branch stays the repo-relative one.

--append-only refuses a commit that would leave FEWER rows than the remote already holds
(a shrink is almost always a stale-base/wrong-file mistake). It is auto-enabled for .jsonl
sidecars; --allow-shrink overrides it when a row reduction is genuinely intended.

When --brief-path is given, the evidence content (from --evidence-file) is appended
to the brief file at --brief-path in the Evidence section, and THAT file is committed.
Otherwise the file at --evidence-file is committed as-is.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskevidence sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action of the tool. Guard writes its
	// own result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, "deskevidence: "+err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Brief-reading version gate (derived-board/06 §6): a stamped deskevidence
	// below v1.0.0 refuses a brief-v2 tree (exit 6).
	if code := deskkit.RefuseIfTreeV2BelowV1(deskkit.RootsFromArgs(args), deskkit.EffectiveToolVersion(version), "deskevidence", os.Stderr); code != 0 {
		return code
	}

	// Outward verbs present a LOOP IDENTITY. The kill switch's per-loop halt is
	// `STOP.<loop>`, matched against $DESK_LOOP; with the variable unset nothing matches,
	// so a stop flag a human is holding never fires and this verb keeps writing while the
	// operator believes it has been halted. The boot verb has checked this since it was
	// written — an outward verb run OUTSIDE a booted window did not, which is the gap.
	if err := deskkit.RequireLoopIdentity("deskevidence"); err != nil {
		fmt.Fprintln(os.Stderr, "deskevidence: "+err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(stderr)

	// runOutward holds the flock across the whole write window and writes the
	// single audit line (#227).
	err := runOutward(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskevidence: "+err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
