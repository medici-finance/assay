// Command deskpins validates a consumer's `.assay-versions` pin file against the
// published contract (docs/distribution.md § The `.assay-versions` pin file).
//
// It is a READ-ONLY instrument: no kill-switch gate, no audit line, no writes.
// Its whole job is to report, three-state, whether a pin file conforms:
//
//	deskpins --check --root <dir>     # checks <dir>/.assay-versions
//
// Exit codes ARE the verdict, and there are four because a validator must keep
// three failure kinds apart from success and from one another:
//
//	0  checked-clean     every artifact line parses and satisfies the grammar
//	1  checked-failed    a readable, parseable line VIOLATES a rule
//	                     (bad tag, bad/short digest, duplicate artifact name)
//	2  could-not-check   the pin file is absent or unreadable — fail-closed,
//	                     NEVER reported clean; stderr names the missing file
//	3  malformed         a data line has the wrong field count (missing sha256)
//
// `absent` (2) and `malformed` (3) are deliberately distinct: "the file is not
// there" and "a line in the file does not parse" are different facts, and the
// desk must be able to tell them apart. A command line deskpins cannot parse is
// refused with 5, outside the verdict codes.
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

// helpLong is the root help's prose. The flags are generated from the command tree
// (desktools-v2/46); this is what the tree cannot say on its own.
const helpLong = `deskpins — validate an .assay-versions pin file against its published contract.

  deskpins --check [--root <dir>]   validate <dir>/.assay-versions (default <dir> = ".")

exit: 0 checked-clean · 1 checked-failed · 2 could-not-check · 3 malformed · 5 refused (bad command line)

The contract is docs/distribution.md § The ` + "`.assay-versions`" + ` pin file.`

// Validator exit codes — the three-state verdict plus the malformed split.
const (
	exitCheckedClean  = 0
	exitCheckedFailed = 1
	exitCouldNotCheck = 2
	exitMalformed     = 3
)

func main() {
	os.Exit(deskkit.PerfRun("deskpins", nil, os.Args[1:], func(a []string) int { return run(a, os.Stdout, os.Stderr) }))
}

// run is one invocation: deskpins has no process startup (it reads no roster), so the
// command tree is the whole of it.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr)
}

// pinsFn is the typed handler the command tree calls; the contract tests replace it to observe
// the values that reach it.
var pinsFn = checkPins

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
	return fmt.Sprintf("deskpins sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// bindings are every setting deskpins declares: flags and defaults only, no environment
// variable and no configuration file.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "check", Kind: cli.Bool, Flag: "check", Default: false, Usage: "validate <root>/.assay-versions"},
		{Key: "root", Kind: cli.String, Flag: "root", Default: ".", Usage: "the `dir` whose .assay-versions is checked"},
	}
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("deskpins", "validate an .assay-versions pin file against its published contract")
		root.Long = helpLong
		set := cli.Declare(root, bindings()...)
		// deskpins takes no positional argument; a stray word is refused before anything runs.
		root.Args = func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown argument %q", args[0])
			}
			return nil
		}
		root.RunE = func(cmd *cobra.Command, _ []string) error {
			return codeErr(pins(cmd, set, stdout, stderr))
		}
		// `deskpins version` printed the build stamp before the migration; it still does.
		root.AddCommand(&cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				fmt.Fprintln(stdout, versionLine())
				return nil
			}})
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy refusal code (5), outside the verdict codes.
		UsageExit: deskkit.ExitRefused,
		// -version was a legacy spelling and stays accepted. The same rewrite newly accepts
		// -check and -root (and pflag accepts --check=V and --root=D), which the hand-rolled
		// loop refused with 5: a declared widening, not a preserved spelling.
		GoFlagCompat: true,
	})
}

// pins resolves the settings and runs the check. With nothing asked of it, deskpins still
// prints its usage to stderr and refuses (exit 5), as the hand-rolled reader did.
func pins(cmd *cobra.Command, set *cli.Set, stdout, stderr io.Writer) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintf(stderr, "deskpins: refused: %v\n", err)
		return deskkit.ExitRefused
	}
	if onResolve != nil {
		onResolve(v)
	}
	if !v.Bool("check") {
		if cmd.Flags().NFlag() > 0 {
			fmt.Fprintln(stderr, "deskpins: nothing to do — pass --check")
		}
		cmd.SetOut(stderr)
		_ = cmd.Help()
		return deskkit.ExitRefused
	}
	return pinsFn(v.String("root"), stdout, stderr)
}

// checkPins is the check itself: deskkit.CheckPins on <root>/.assay-versions, its verdict as the
// exit code.
func checkPins(root string, stdout, stderr io.Writer) int {
	res := deskkit.CheckPins(root)
	switch res.State {
	case deskkit.PinCheckedClean:
		fmt.Fprintln(stdout, res.Summary)
		return exitCheckedClean
	case deskkit.PinCheckedFailed:
		fmt.Fprintln(stderr, res.Summary)
		for _, r := range res.Reasons {
			fmt.Fprintln(stderr, "  "+r)
		}
		return exitCheckedFailed
	case deskkit.PinMalformed:
		fmt.Fprintln(stderr, res.Summary)
		for _, r := range res.Reasons {
			fmt.Fprintln(stderr, "  "+r)
		}
		return exitMalformed
	case deskkit.PinCouldNotCheck:
		// Fail-closed: name the missing file on stderr, never report clean.
		fmt.Fprintln(stderr, res.Summary)
		return exitCouldNotCheck
	default:
		fmt.Fprintln(stderr, "deskpins: unknown check state")
		return exitCouldNotCheck
	}
}
