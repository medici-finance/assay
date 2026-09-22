// Command deskreconcile is the desk-side board-reconcile writer.
//
// It runs `statusgen reconcile --backfill --apply` — the ONLY writer of a stream README's
// Status cell — from a verb the desk/worker App can run, and carries the result as exactly
// ONE draft PR on the fixed branch board/reconcile. That removes the workflow dependency
// the scheduled-reconcile job #1175 is blocked on: no App may push
// the `.github/workflows/assay-statusgen.yml` change that would schedule the reconcile, so
// this does the same job outside CI.
//
// It writes ONLY stream README Status cells (todo|in-progress -> implemented, real
// merged-PR witness only — the narrow, safe transition statusgen enforces), always in an
// isolated worktree cut from the fetched remote head, and never opens a second PR.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// version is an optional bare-`vX.Y.Z` build stamp (`-ldflags -X main.version`); empty on
// a real release, where the namespaced ReleaseTag stamp supplies the version.
var version string

const usage = `deskreconcile — flip merged briefs' board Status cells and carry the result as one draft PR.

USAGE:
  deskreconcile [--root DIR] --worktree DIR [--branch board/reconcile] [--issue N] [--dry-run]
  deskreconcile --version

It fetches origin/main of the target checkout into an ISOLATED worktree, runs
` + "`statusgen reconcile --backfill --apply`" + ` (the only writer of a stream README Status cell:
todo|in-progress -> implemented, real merged-PR witness only), and — when a stream README
changed — commits ONLY those README files as ONE commit (chore(board): reconcile <date>)
on the fixed branch board/reconcile, then opens or UPDATES exactly one draft PR titled
"chore(board): reconcile". It never opens a second PR, and it changes nothing when there is
nothing to flip.

  --root DIR       the target repo checkout to reconcile (default: current directory).
  --worktree DIR   the isolated linked worktree to run in — an ABSOLUTE path OUTSIDE --root
                   (required).
  --branch NAME    the fixed carry branch (default: board/reconcile).
  --issue N        the tracking issue the fresh-create PR body carries as its Issue: #<N>
                   trailer (required for a real run; unused on --dry-run and on a
                   follow-up push to an already-open PR).
  --dry-run        fetch and reconcile against origin/main, print the rows it WOULD flip,
                   then discard the worktree. Commits nothing, pushes nothing, opens no PR.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// This tool ACTS on the roster (it writes through deskpr): ciEligible=false — the
	// config-home file only, never the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, stdout *os.File) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskreconcile sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) >= 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(os.Stderr, usage)
		return deskkit.ExitOK
	}

	// Kill switch first, before any surface is read.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	// The per-loop stop flag is matched against $DESK_LOOP; refuse with it unset so a stop
	// flag a human is holding fires rather than silently never matching. deskpr, which this
	// verb shells out to, refuses on the same condition.
	if err := deskkit.RequireLoopIdentity("deskreconcile"); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	fs := flag.NewFlagSet("deskreconcile", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	root := fs.String("root", ".", "the target repo checkout to reconcile")
	worktree := fs.String("worktree", "", "the isolated linked worktree to run in (absolute, outside --root)")
	branch := fs.String("branch", defaultBranch, "the fixed carry branch")
	issue := fs.Int("issue", 0, "the tracking issue for the fresh-create PR body trailer")
	repo := fs.String("repo", "", "owner/name statusgen reads PRs from to witness a flip (default: the checkout's origin)")
	tokenFile := fs.String("token-file", "", "file holding the GitHub API token for the PR read (else GITHUB_TOKEN)")
	dryRun := fs.Bool("dry-run", false, "reconcile against origin/main and report the rows it would flip; write nothing")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprintln(os.Stderr, usage)
			return deskkit.ExitOK
		}
		return deskkit.ExitRefused
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "refused: deskreconcile takes no positional arguments")
		return deskkit.ExitRefused
	}

	res, err := Run(Options{
		Root:      *root,
		Worktree:  *worktree,
		Branch:    *branch,
		Now:       time.Now(),
		DryRun:    *dryRun,
		Issue:     *issue,
		Repo:      *repo,
		TokenFile: *tokenFile,
	})
	// The step audit surface prints regardless of outcome: a run that reports without the
	// steps that produced it is a claim, not evidence.
	for _, s := range res.Steps {
		fmt.Fprintln(os.Stderr, s)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	printResult(res, stdout)
	return deskkit.ExitOK
}

// printResult writes the human-facing summary: what flipped, or that nothing did.
func printResult(res Result, stdout *os.File) {
	verb := "flipped"
	if res.DryRun {
		verb = "would flip"
	}
	if res.NoOp {
		fmt.Fprintln(stdout, "nothing to reconcile — no stream README Status cell changed; no commit, no PR")
		return
	}
	fmt.Fprintf(stdout, "%s %d row(s):\n", verb, len(res.Flipped))
	for _, f := range res.Flipped {
		fmt.Fprintf(stdout, "  %s: %s -> %s (witness %s)\n", f.ID, f.From, f.To, f.Witness)
	}
	if res.DryRun {
		fmt.Fprintf(stdout, "dry-run: %d README file(s) would be committed on %s; nothing was written\n", len(res.Changed), defaultBranch)
		return
	}
	fmt.Fprintf(stdout, "committed %d README file(s) on %s and opened/updated the draft PR\n", len(res.Changed), defaultBranch)
}
