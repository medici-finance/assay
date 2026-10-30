// Command deskrebaseline turns a PROVABLY-INTACT-BUT-STALE `## Verify` row into a one-row
// re-baseline PR, instead of the Nth duplicate "stale Verify" issue.
//
// A third of recorded verify runs fail on the same shape: a row pins a path, a count, or a
// tool idiom the tree moved out from under it, so the table fails AS WRITTEN while the work
// it checks is intact. The desk correctly refuses to flip and files another issue, and the
// brief re-fails every drain. deskrebaseline classifies the failing row against a NARROW
// safe set and, only when it can PROVE the work intact from git history, opens a draft PR
// that re-baselines that one row. Everything it cannot prove intact it REFUSES — the verb
// files nothing green on its own authority; the re-baseline is a PR, reviewed by the
// reviewer App and merged by a human (brief verify-integrity/05).
//
// The safe/refusal boundary is the single point of failure: a loose "safe" launders a real
// regression into a green. It therefore fails CLOSED (classify.go) — a row reaches the safe
// set only by a positive proof, and everything else is filed. In this first cut the ONE safe
// class the shipped verb produces is safe:rename (a single rename hop to an existing file).
// safe:count (a pure-additions count drift) and safe:idiom (a tool idiom retired by a
// recorded ruling) are NOT YET IMPLEMENTED: the classifier carries their arms, but
// gatherRowFacts (facts.go) does not populate their facts, so such a row falls through to
// refused:unclassified and is filed as today (see docs/rebaseline.md).
//
// USAGE:
//
//	deskrebaseline <brief> --row K [--root DIR] [--repo owner/name] [--open]
//	deskrebaseline --version
//
// <brief>   a brief file path, or a `<stream>/<NN>` id resolved under --root first, then
//
//	under the configured stream root for --repo.
//
// --row K   the 1-based row number in the brief's `## Verify` table to classify.
// --root    the checkout root the row's paths and commands resolve against
//
//	(default: `git rev-parse --show-toplevel` from the working directory).
//
// --repo    owner/name, used only as the fallback to resolve a `<stream>/<NN>` brief id under its
//
//	configured stream root (default: derived from --root's origin remote).
//
// --open    push the one-row `rebaseline/<stream>-<NN>-row-<K>` branch and open a DRAFT PR
//
//	via `deskpr create` (as the loop identity — the verifier App under
//	DESK_LOOP=verify-desk). WITHOUT --open the verb is DRY-RUN: it prints the
//	classification, the git evidence and the plan, creates no branch, and exits 0.
//	Before any mutation --open refuses unless the brief resolves inside --root,
//	HEAD is the fetched refs/remotes/origin/main, and the checkout is clean
//	(open.go openPreflight). The commit names only the brief's path.
//
// EXIT CODES (deskkit contract):
//
//	dry-run (no --open) — always exit 0: a classification report is not a refusal. Read the
//	    printed `verdict:` line; a refused:* verdict means "file the issue as today".
//	--open on a safe:* verdict — 0 when the PR opened (deskpr's own exit otherwise).
//	--open on a refused:* verdict — 5 (refused): no branch, no PR; file the issue.
//	other — 5 refused (bad flags / no such row), 6 unverifiable (brief unreadable).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stdout and stderr are the current invocation's streams; open.go writes through them too.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// helpLong is the root help's prose. The flags are generated from the command tree
// (desktools-v2/46); this is what the tree cannot say on its own.
const helpLong = `deskrebaseline — re-baseline a provably-intact-but-stale Verify row via a one-row PR.

  deskrebaseline <brief> --row K [--root DIR] [--repo owner/name] [--open]
  deskrebaseline --version

<brief> is a brief file path, or a <stream>/<NN> id (resolved under --root, then --repo's root).

Without --open the verb is a DRY-RUN: it prints the classification and git evidence, creates
no branch, and exits 0. --open pushes rebaseline/<stream>-<NN>-row-K and opens a DRAFT PR via
deskpr create (verifier App under DESK_LOOP=verify-desk); it refuses before any write unless
the brief is inside --root, HEAD is the fetched refs/remotes/origin/main, and the checkout is
clean.

The verb never merges and never lands on main; the re-baseline PR is reviewed and merged
by a human. It fails CLOSED: a row is re-baselined only when git PROVES the work intact.`

func main() {
	os.Exit(deskkit.PerfRun("deskrebaseline", nil, os.Args[1:], func(a []string) int { return run(a) }))
}

// run is one invocation on the process streams.
func run(args []string) int {
	return execute(args, os.Stdout, os.Stderr, func(w io.Writer) { deskkit.WarnIfUnpinned(w) })
}

// rebaselineFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var rebaselineFn = rebaseline

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version and -version print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskrebaseline sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskrebaseline declares: flags and defaults only, no environment
// variable and no configuration file.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "row", Kind: cli.Int, Flag: "row", Default: 0, Usage: "the 1-based row number `K` in the brief's ## Verify table (required)"},
		{Key: "root", Kind: cli.String, Flag: "root", Default: "", Usage: "checkout root the row resolves against (default: git toplevel of the working directory)"},
		{Key: "repo", Kind: cli.String, Flag: "repo", Default: "", Usage: "`owner/name` for <stream>/<NN> resolution (default: derived from origin)"},
		{Key: "open", Kind: cli.Bool, Flag: "open", Default: false, Usage: "push the one-row branch and open a DRAFT PR via deskpr create (default: dry-run)"},
	}
}

// execute runs one invocation against a fresh command tree. pre is the process startup (the
// unpinned-build warning); it runs after the command line parses and never for help or version.
func execute(args []string, out, errw io.Writer, pre func(io.Writer)) int {
	prevOut, prevErr := stdout, stderr
	stdout, stderr = out, errw
	defer func() { stdout, stderr = prevOut, prevErr }()

	build := func() *cobra.Command {
		root := cli.NewRoot("deskrebaseline", "re-baseline a provably-intact-but-stale Verify row via a one-row PR")
		root.Use = "deskrebaseline <brief> --row K"
		root.Long = helpLong
		set := cli.Declare(root, bindings()...)
		// Exactly one <brief>; a second word is refused, as the hand-rolled reader refused it.
		root.Args = func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("unexpected extra argument %q (one <brief> only)", args[1])
			}
			return nil
		}
		root.RunE = func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				// A bare invocation prints the usage and refuses, as before.
				cmd.SetOut(errw)
				_ = cmd.Help()
				return exitErr(deskkit.ExitRefused)
			}
			v, err := set.Resolve(cli.Inputs{})
			if err != nil {
				fmt.Fprintf(errw, "deskrebaseline: %v\n", err)
				return exitErr(deskkit.ExitRefused)
			}
			if onResolve != nil {
				onResolve(v)
			}
			o := options{root: v.String("root"), repo: v.String("repo"), open: v.Bool("open"), row: v.Int("row")}
			if len(args) == 1 {
				o.brief = args[0]
			}
			if err := checkOptions(o, v.Source("row") == cli.Flag); err != nil {
				fmt.Fprintf(errw, "deskrebaseline: %v\nRun 'deskrebaseline --help' for usage.\n", err)
				return exitErr(deskkit.ExitRefused)
			}
			if pre != nil {
				pre(errw)
			}
			return codeErr(rebaselineFn(o))
		}
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: out, Err: errw},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A command line deskrebaseline cannot parse is refused (5), as before.
		UsageExit: deskkit.ExitRefused,
		// -version was a legacy spelling; the single-dash long forms stay accepted.
		GoFlagCompat: true,
	})
}

// checkOptions holds the two rules the flag parser cannot: a <brief> is required, and --row
// must be given as a positive integer.
func checkOptions(o options, rowGiven bool) error {
	if o.brief == "" {
		return fmt.Errorf("a <brief> (path or <stream>/<NN>) is required")
	}
	if !rowGiven {
		return fmt.Errorf("--row K is required")
	}
	if o.row < 1 {
		return fmt.Errorf("--row must be a positive integer, got %q", strconv.Itoa(o.row))
	}
	return nil
}

// rebaseline is one run from parsed options: resolve the brief and row, classify, print the
// plan, and — only with --open on a safe verdict — open the one-row PR.
func rebaseline(opts options) int {
	root := opts.root
	if root == "" {
		if tl, ok := gitOutput(".", "rev-parse", "--show-toplevel"); ok {
			root = tl
		} else {
			fmt.Fprintln(stderr, "deskrebaseline: --root not given and the working directory is not a git checkout")
			return deskkit.ExitRefused
		}
	}
	repo := opts.repo
	if repo == "" {
		repo = deriveRepo(root)
	}

	lb, err := loadBrief(repo, root, opts.brief)
	if err != nil {
		fmt.Fprintf(stderr, "deskrebaseline: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	rowK, ok := lb.row(opts.row)
	if !ok {
		fmt.Fprintf(stderr, "deskrebaseline: brief %s has no Verify row %d\n", lb.Path, opts.row)
		return deskkit.ExitRefused
	}

	facts := gatherRowFacts(root, rowK, lb.RiskBearing, lb.RiskReason)
	cls := Classify(facts)

	printPlan(stdout, lb, rowK, facts, cls, opts)

	if !opts.open {
		// DRY-RUN: a classification report, never a refusal. Exit 0; no branch created.
		return deskkit.ExitOK
	}

	if !cls.Verdict.IsSafe() {
		fmt.Fprintf(stderr, "deskrebaseline: %s — not re-baselineable; file the issue as today (%s)\n", cls.Verdict, cls.Reason)
		return deskkit.ExitRefused
	}
	return openRebaselinePR(root, repo, lb, rowK, facts, cls)
}

type options struct {
	brief string
	row   int
	root  string
	repo  string
	open  bool
}

// deriveRepo reads owner/name from the checkout's origin remote, offline. "" when it cannot.
func deriveRepo(root string) string {
	url, ok := gitOutput(root, "-C", root, "remote", "get-url", "origin")
	if !ok || url == "" {
		return ""
	}
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	if i := strings.Index(url, "github.com"); i >= 0 {
		rest := url[i+len("github.com"):]
		rest = strings.TrimLeft(rest, ":/")
		if strings.Count(rest, "/") == 1 {
			return rest
		}
	}
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return ""
}

// printPlan prints the classification, the intactness evidence and the re-baseline plan —
// the dry-run output, and the record a --open run echoes before it acts.
func printPlan(w io.Writer, lb loadedBrief, row verifyRow, f RowFacts, cls Classification, opts options) {
	fmt.Fprintf(w, "brief: %s\n", lb.Path)
	fmt.Fprintf(w, "row: %d (class %s)\n", row.Num, row.Class)
	fmt.Fprintf(w, "command: %s\n", row.Command)
	if row.Expect != "" {
		fmt.Fprintf(w, "expect: %s\n", row.Expect)
	}
	fmt.Fprintf(w, "verdict: %s\n", cls.Verdict)
	fmt.Fprintf(w, "reason: %s\n", cls.Reason)
	if f.PathRef != "" {
		fmt.Fprintf(w, "evidence: pinned-path=%s exists=%t", f.PathRef, f.PathExists)
		if f.RenameHop != "" {
			fmt.Fprintf(w, " rename-hop=%s", f.RenameHop)
		}
		if f.RenameProbe != "" {
			fmt.Fprintf(w, " rename-rerun=%s", f.RenameProbe)
		}
		fmt.Fprintln(w)
	}
	if f.CommandProbed {
		fmt.Fprintf(w, "evidence: command-probed ran=%t fails-expect=%t\n", f.CommandRan, f.RCDiffers)
	}
	if cls.Verdict.IsSafe() {
		branch := rebaselineBranch(lb.Path, row.Num)
		if opts.open {
			fmt.Fprintf(w, "plan: open draft PR on branch %s (one row re-baselined)\n", branch)
		} else {
			fmt.Fprintf(w, "plan: --open would push branch %s and open a draft PR (dry-run: no branch created)\n", branch)
		}
	} else {
		fmt.Fprintf(w, "plan: file the issue as today with reason %q\n", cls.Verdict)
	}
}
