// Command deskdisable is the `disable` verb of the component model (see
// component-model.md §4/§7): given a component id, it replays that
// component's own apply steps in LIFO (reverse) order.
//
//   - An INSIDE step (component-model.md §5) is reversed for real, by a
//     registered executor (executors.go) — never by re-parsing the manifest's
//     free-text `inverse:` prose, which is documentation for a human, not an
//     instruction a tool can execute safely.
//   - An OUTSIDE step is never auto-compensated by this build: it is printed
//     as a human checklist line, enriched with whatever the ledger
//     (`.assay/ledger.jsonl`, deskkit/ledger.go) recorded for it. No
//     compensation in this tree is marked `unattended: true`, so this is the
//     only path an outside step ever takes here.
//
// deskdisable refuses — and changes nothing — when the named component has no
// manifest, when a --cascade name has no manifest, when disabling would strand
// an active dependent that --cascade did not list first, or when an inside
// step has no registered automatic reverse (it never guesses one).
//
// USAGE:
//
//	deskdisable <component> [--root <dir>] [--dry-run] [--yes] [--cascade a,b,c]
//	deskdisable --version
//
// --dry-run computes the exact same plan (and any refusal) but writes
// nothing. A run that is neither --dry-run nor --yes refuses, so a bare
// invocation can never mutate a tree by accident.
//
// Exit codes (deskkit contract): 0 ok · 5 refused · 6 could-not-check.
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

// helpLong is the root help's prose. The flags are generated from the command tree
// (desktools-v2/47); this is what the tree cannot say on its own.
const helpLong = `deskdisable — replay a component's reverses (component-model.md §4/§7).

  deskdisable <component> [--root <dir>] [--dry-run] [--yes] [--cascade a,b,c]
  deskdisable --version

Reads component.yaml manifests under --root (default "."). Reverses the named
component's apply steps in LIFO order: inside steps are reversed for real by a
registered executor; outside steps are printed as a human checklist line
(never auto-compensated in this build), enriched from .assay/ledger.jsonl.

Refuses (and touches nothing) when:
  - the component has no manifest under --root;
  - a --cascade name has no manifest under --root;
  - another still-present component requires a key it provides, unless
    --cascade lists that dependent (disabled first, in the given order);
  - an inside step has no registered automatic reverse.

--dry-run computes and prints the exact same plan or refusal but writes
nothing. A bare invocation (neither --dry-run nor --yes) refuses.

Exit codes: 0 ok · 5 refused (also a command line deskdisable cannot parse) ·
6 could-not-check (deskkit contract).`

func main() {
	os.Exit(deskkit.PerfRun("deskdisable", nil, os.Args[1:], func(a []string) int {
		// The roster class is an explicit declaration, not a config read: deskdisable acts on
		// a tree, so it is ciEligible=false.
		deskkit.SetToolClass(deskkit.ClassForTool(false))
		return run(a, os.Stdout, os.Stderr)
	}))
}

// run is one invocation: deskdisable has no process startup (it reads no roster), so the
// command tree is the whole of it.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr)
}

// disableFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var disableFn = disableAndPrint

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

// versionLine is what --version, -version and the legacy `version` word print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskdisable sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskdisable declares: flags and defaults only, no environment
// variable and no configuration file.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "root", Kind: cli.String, Flag: "root", Default: ".", Usage: "tree root to discover component.yaml under"},
		{Key: "dry-run", Kind: cli.Bool, Flag: "dry-run", Default: false, Usage: "compute and print the plan; write nothing"},
		{Key: "yes", Kind: cli.Bool, Flag: "yes", Default: false, Usage: "actually run (required unless --dry-run)"},
		{Key: "cascade", Kind: cli.String, Flag: "cascade", Default: "", Usage: "comma-separated dependent component ids to disable first, in order"},
	}
}

// splitCascade is the --cascade value as the ordered list of non-empty, trimmed ids.
func splitCascade(v string) []string {
	var out []string
	if strings.TrimSpace(v) == "" {
		return out
	}
	for _, c := range strings.Split(v, ",") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// newTree builds deskdisable's command tree on the given streams.
func newTree(stdout, stderr io.Writer) *cobra.Command {
	root := cli.NewRoot("deskdisable", "replay a component's reverses")
	root.Use = "deskdisable <component>"
	root.Long = helpLong
	set := cli.Declare(root, bindings()...)
	// One <component>; a second word is refused before anything runs.
	root.Args = func(_ *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("unexpected extra argument(s) %v", args[1:])
		}
		return nil
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if cmd.Flags().NFlag() > 0 {
				fmt.Fprintln(stderr, "deskdisable: expected a component id")
			}
			// A bare invocation prints the usage to stderr and refuses, as before.
			cmd.SetOut(stderr)
			_ = cmd.Help()
			return exitErr(deskkit.ExitRefused)
		}
		v, err := set.Resolve(cli.Inputs{})
		if err != nil {
			fmt.Fprintf(stderr, "deskdisable: refused: %v\n", err)
			return exitErr(deskkit.ExitRefused)
		}
		if onResolve != nil {
			onResolve(v)
		}
		if !v.Bool("dry-run") && !v.Bool("yes") {
			fmt.Fprintln(stderr, "deskdisable: refusing to run without --dry-run or --yes")
			return exitErr(deskkit.ExitRefused)
		}
		return codeErr(disableFn(Options{Root: v.String("root"), Target: args[0],
			Cascade: splitCascade(v.String("cascade")), DryRun: v.Bool("dry-run")}, stdout, stderr))
	}
	// `deskdisable version` printed the build stamp before the migration; it still does.
	root.AddCommand(&cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			fmt.Fprintln(stdout, versionLine())
			return nil
		}})
	return root
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer) int {
	return cli.Run(func() *cobra.Command { return newTree(stdout, stderr) }, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A command line deskdisable cannot parse is refused (5), as the flag set refused it.
		UsageExit: deskkit.ExitRefused,
		// The Go flag package accepted -root, -dry-run, -yes and -cascade after the component;
		// they stay accepted.
		GoFlagCompat: true,
	})
}

// disableAndPrint is the disable itself: the plan (or refusal) printed, its code returned.
func disableAndPrint(o Options, stdout, stderr io.Writer) int {
	plan, err := Disable(o)
	printPlan(stdout, plan)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	return deskkit.ExitOK
}

func printPlan(w io.Writer, plan *Plan) {
	if plan == nil {
		return
	}
	if len(plan.Cascade) > 0 {
		fmt.Fprintf(w, "disable plan for %s (cascade first: %s):\n", plan.Target, strings.Join(plan.Cascade, ", "))
	} else {
		fmt.Fprintf(w, "disable plan for %s:\n", plan.Target)
	}
	if len(plan.Steps) == 0 {
		fmt.Fprintln(w, "  (no apply steps)")
	}
	for _, s := range plan.Steps {
		fmt.Fprintf(w, "  [%s/%s] %s: %s\n", s.Component, s.StepID, s.Kind, s.Detail)
	}
}
