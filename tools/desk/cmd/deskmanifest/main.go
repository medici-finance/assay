// Command deskmanifest is the component-manifest lint for the component model
// defined by the component-manifest spec (§2). It discovers every
// `component.yaml` in a tree, parses it against §2, resolves every `inject`
// key to a `provides`, checks each `range` against the provider's `version`,
// and reports cycles among `inject.required` — all from the declarations alone,
// so an undeclared dependency is a CI failure instead of an outage.
//
// It is a READ-ONLY reader. It reads `component.yaml` files under --root and
// writes nothing; it contacts no cluster, no forge, and no roster.
//
// THREE STATES, ONE EXIT CODE EACH (brief-v1 §8, this tool's own contract — it
// deliberately does NOT use deskkit's canonical 0/3/4/5/6 map, because a lint's
// three states are clean / problems / could-not-check, not the write-tool set):
//
//	checked-clean       → exit 0  — every required inject resolves, in range, acyclic.
//	checked-failed      → exit 1  — one or more problems; each is named.
//	could-not-check     → exit 2  — the lint could not run (root missing/unreadable).
//	                                Never rounded up to clean, never down to a problem
//	                                it did not observe.
//
// Providers MAY carry attributes (a `flavour`, an `evidence` marker path) and
// inject entries MAY constrain a required key's flavour; a key the catalogue
// (components/KEYS.md) marks `exclusive: true` MUST have at most one ACTIVE
// provider — a second is a lint PROBLEM (component-model.md §9). `--activation`
// additionally reports every component's computed ACTIVE/INACTIVE state.
//
// USAGE:
//
//	deskmanifest lint [--root <dir>] [--activation]
//	deskmanifest --version
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	os.Exit(deskkit.PerfRun("deskmanifest", nil, os.Args[1:], func(a []string) int {
		return run(a, os.Stdout, os.Stderr)
	}))
}

// helpLong is the root help's prose. The verbs and flags are generated from the command tree
// (desktools-v2/46); this is what the tree cannot say on its own.
const helpLong = `deskmanifest — the component-manifest lint.

  deskmanifest lint [--root <dir>] [--activation]

lint discovers every component.yaml under --root (default "."), resolves every
inject key to a provides, checks version ranges (and flavour constraints), and
reports cycles and exclusive-key violations (a key components/KEYS.md marks
exclusive: true with more than one ACTIVE provider). --activation additionally
prints every component's computed ACTIVE/INACTIVE state and, when INACTIVE,
why.

Exit codes:
  0  checked-clean    — every required inject resolves, in range, acyclic
  1  checked-failed   — problems found (each is named)
  2  could-not-check  — the lint could not run (root missing/unreadable), or the
                        command line could not be parsed`

// versionText is what --version, -version and the legacy `version` word print.
const versionText = "deskmanifest — component-manifest lint"

// run is one invocation: deskmanifest has no process startup (it reads no roster), so the
// command tree is the whole of it.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr)
}

// lintFn is the typed handler the lint verb calls; the contract tests replace it to observe the
// values that reach it.
var lintFn = lintReport

// onResolve, when set, sees every invocation's resolved settings.
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == exitClean {
		return nil
	}
	return exitErr(code)
}

// lintBindings are every setting the lint verb declares: flags and defaults only, no
// environment variable and no configuration file.
func lintBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "root", Kind: cli.String, Flag: "root", Default: ".", Usage: "tree root `dir` to discover component.yaml under"},
		{Key: "activation", Kind: cli.Bool, Flag: "activation", Default: false,
			Usage: "include an ACTIVATION report (§6.1): every component's ACTIVE/INACTIVE state and, when INACTIVE, why"},
	}
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("deskmanifest", "the component-manifest lint")
		root.Long = helpLong
		// A bare deskmanifest still prints its usage to stderr and exits could-not-check (2): it
		// was asked to check nothing.
		root.RunE = func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(stderr)
			_ = cmd.Help()
			return exitErr(exitCouldNotCheck)
		}
		lint := &cobra.Command{
			Use:   "lint",
			Short: "lint every component.yaml under --root",
			Args:  cobra.NoArgs,
		}
		set := cli.Declare(lint, lintBindings()...)
		lint.RunE = func(*cobra.Command, []string) error {
			return codeErr(lintCommand(set, stdout, stderr))
		}
		root.AddCommand(lint)
		// `deskmanifest version` printed the version line before the migration; it still does.
		root.AddCommand(&cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				fmt.Fprintln(stdout, versionText)
				return nil
			}})
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionText,
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy code: could-not-check (2), never a lint verdict.
		UsageExit: exitCouldNotCheck,
		// lint's flags were Go flag-package flags; the single-dash spellings stay accepted.
		GoFlagCompat: true,
	})
}

// lintCommand resolves the lint verb's settings and runs the lint.
func lintCommand(set *cli.Set, stdout, stderr io.Writer) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintf(stderr, "deskmanifest: %v\n", err)
		return exitCouldNotCheck
	}
	if onResolve != nil {
		onResolve(v)
	}
	report, code := lintFn(v.String("root"), v.Bool("activation"))
	fmt.Fprint(stdout, report)
	return code
}
