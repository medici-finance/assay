// Command untrustcorpus loads and validates the positive-control corpus for the
// untrusted-read sample corpus, and assembles each entry's inert bytes from the
// codepoint/description table at run time.
//
// It is a READ-ONLY tool. It reads the table under a DERIVED root (never a hand-typed
// path: --root overrides, else ASSAY_UNTRUST_CORPUS_ROOT, else the shipped in-repo
// location) and writes nothing. It never executes a generated sample — it only decodes
// the codepoints the table names.
//
// USAGE:
//
//	untrustcorpus check [--root <dir>]   validate the corpus structurally
//	untrustcorpus list  [--root <dir>]   list entries (id, class, layer, scan verdict)
//	untrustcorpus --version
//
// check EXIT CODES (deskkit contract, exitcodes.go):
//
//	0  corpus-ok           — every entry names an existing detector layer, carries a
//	                         current sha256, and no entry or layer is orphaned
//	5  refused             — the corpus has structural problems (named on stderr)
//	6  could-not-determine — the table could not be read or parsed
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit/untrustcorpus"
)

const usage = `untrustcorpus — the positive-control corpus loader + validator.

USAGE:
  untrustcorpus check [--root <dir>]   validate the corpus structurally
  untrustcorpus list  [--root <dir>]   list entries (id, class, layer, scan verdict)
  untrustcorpus --version

The root is DERIVED, not typed: --root overrides; else ASSAY_UNTRUST_CORPUS_ROOT; else
the shipped in-repo location.

check exit codes:
  0  corpus-ok           — every entry names an existing detector layer, carries a
                           current sha256, and no entry or layer is orphaned
  5  refused             — structural problems (named on stderr)
  6  could-not-determine — the table could not be read or parsed`

func main() {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "untrustcorpus sourceSHA=%s builtAt=%s releaseTag=%s\n",
			sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return deskkit.ExitRefused
	}
	sub := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet("untrustcorpus "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "corpus table root (default: derived — see --help)")
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(rest); err != nil {
		return deskkit.ExitRefused
	}

	dir := *root
	if dir == "" {
		dir = untrustcorpus.DefaultRoot()
	}

	samples, err := untrustcorpus.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-determine: %v\n", err)
		return deskkit.ExitUnverifiable
	}

	switch sub {
	case "check":
		res := untrustcorpus.Check(samples)
		if !res.OK() {
			for _, p := range res.Problems {
				fmt.Fprintf(stderr, "corpus-problem: %s\n", p)
			}
			return deskkit.ExitRefused
		}
		fmt.Fprintf(stdout, "corpus-ok (%d entries)\n", len(samples))
		return deskkit.ExitOK
	case "list":
		for _, s := range samples {
			fmt.Fprintf(stdout, "%s\tclass=%s\tlayer=%s\tscan=%s\treason=%s\tbytes=%d\n",
				s.ID, s.Class, s.Layer, s.Expect.Scan, s.Expect.Reason, len(s.Bytes))
		}
		return deskkit.ExitOK
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", sub)
		fmt.Fprintln(stderr, usage)
		return deskkit.ExitRefused
	}
}
