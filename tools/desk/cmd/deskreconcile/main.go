// Command deskreconcile is the desk-side board-reconcile writer.
//
// It runs `statusgen reconcile --backfill --apply` — the command that flips a stream README's
// Status cell to implemented — from a verb the desk/worker App can run, and carries the
// result as exactly ONE draft PR on the fixed branch board/reconcile. That removes the workflow dependency
// the scheduled-reconcile job #1175 is blocked on: no App may push
// the `.github/workflows/assay-statusgen.yml` change that would schedule the reconcile, so
// this does the same job outside CI.
//
// It writes ONLY stream README Status cells (todo|in-progress -> implemented, real
// merged-PR witness only — the narrow, safe transition statusgen enforces), always in an
// isolated worktree cut from the fetched remote head, and never opens a second PR. It never
// passes statusgen's `--regenerate` opt-in (under which `--apply` also re-renders a drive
// plan page's snapshot region), so a row whose move would re-render one is held, not written.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
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
` + "`statusgen reconcile --backfill --apply`" + ` (the command that flips a stream README Status
cell todo|in-progress -> implemented, real merged-PR witness only), and — when a stream README
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
	os.Exit(deskkit.PerfRun("deskreconcile", nil, os.Args[1:], func(a []string) int {
		return execute(a, os.Stdout, os.Stderr, startup)
	}))
}

// startup is what every real run does first, after its command line parsed, and never for
// help or version (help reads nothing).
//
// This tool ACTS on the roster (it writes through deskpr): ciEligible=false — the
// config-home file only, never the environment, in CI as well as locally.
func startup(stderr io.Writer) int {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(stderr)
	if !deskkit.CheckVerbActivation(stderr) {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// run is execute without the process startup.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, nil)
}

// guard is the kill switch and loopIdentity the per-loop stop-flag precondition, variables so
// the contract tests can count them.
var (
	guard        = deskkit.Guard
	loopIdentity = deskkit.RequireLoopIdentity
)

// reconcileFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var reconcileFn = Run

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// versionLine is what --version and -version print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskreconcile sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskreconcile declares: flags and defaults only. The tool reads
// no configuration file or environment variable for them (statusgen's own GITHUB_TOKEN
// fallback is statusgen's).
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "root", Kind: cli.String, Flag: "root", Default: ".", Usage: "the target repo checkout to reconcile"},
		{Key: "worktree", Kind: cli.String, Flag: "worktree", Usage: "the isolated linked worktree to run in (absolute, outside --root)"},
		{Key: "branch", Kind: cli.String, Flag: "branch", Default: defaultBranch, Usage: "the fixed carry branch"},
		{Key: "issue", Kind: cli.Int, Flag: "issue", Default: 0, Usage: "the tracking issue for the fresh-create PR body trailer"},
		{Key: "repo", Kind: cli.String, Flag: "repo", Usage: "owner/name statusgen reads PRs from to witness a flip (default: the checkout's origin)"},
		{Key: "token-file", Kind: cli.String, Flag: "token-file", Usage: "file holding the GitHub API token for the PR read (else GITHUB_TOKEN)"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run", Default: false, Usage: "reconcile against origin/main and report the rows it would flip; write nothing"},
	}
}

// execute runs one invocation against a fresh command tree. pre is the process startup (nil
// in the in-package tests).
func execute(args []string, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("deskreconcile", "flip merged briefs' board Status cells and carry them as one draft PR")
		root.Long = usage
		set := cli.Declare(root, bindings()...)
		// A leading `help` word is the adapter's help command; any other word (including
		// `help` after --) is refused before anything runs, as before.
		root.Args = func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("refused: deskreconcile takes no positional arguments")
			}
			return nil
		}
		root.RunE = func(_ *cobra.Command, _ []string) error {
			if code := reconcile(set, stdout, stderr, pre); code != deskkit.ExitOK {
				return exitErr(code)
			}
			return nil
		}
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy refusal code (5), not the adapter's default.
		UsageExit:    deskkit.ExitRefused,
		GoFlagCompat: true,
	})
}

// reconcile resolves the settings, then runs the process startup, the kill switch, the loop
// identity check and the reconcile, in that order.
func reconcile(set *cli.Set, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintln(stderr, "refused:", err)
		return deskkit.ExitRefused
	}
	if onResolve != nil {
		onResolve(v)
	}
	if pre != nil {
		if code := pre(stderr); code != deskkit.ExitOK {
			return code
		}
	}
	// Kill switch first, before any surface is read.
	if err := guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	// The per-loop stop flag is matched against $DESK_LOOP; refuse with it unset so a stop
	// flag a human is holding fires rather than silently never matching. deskpr, which this
	// verb shells out to, refuses on the same condition.
	if err := loopIdentity("deskreconcile"); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)

	res, err := reconcileFn(Options{
		Root:      v.String("root"),
		Worktree:  v.String("worktree"),
		Branch:    v.String("branch"),
		Now:       time.Now(),
		DryRun:    v.Bool("dry-run"),
		Issue:     v.Int("issue"),
		Repo:      v.String("repo"),
		TokenFile: v.String("token-file"),
	})
	// The step audit surface prints regardless of outcome: a run that reports without the
	// steps that produced it is a claim, not evidence.
	for _, s := range res.Steps {
		fmt.Fprintln(stderr, s)
	}
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	printResult(res, stdout)
	return deskkit.ExitOK
}

// printResult writes the human-facing summary: what flipped, or that nothing did.
func printResult(res Result, stdout io.Writer) {
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
