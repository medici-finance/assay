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
// desk must be able to tell them apart.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskpins — validate an .assay-versions pin file against its published contract.

usage:
  deskpins --check [--root <dir>]   validate <dir>/.assay-versions (default <dir> = ".")
  deskpins --version                print the build stamp
  deskpins -h | --help             this text

exit: 0 checked-clean · 1 checked-failed · 2 could-not-check · 3 malformed

The contract is docs/distribution.md § The ` + "`.assay-versions`" + ` pin file.`

// Validator exit codes — the three-state verdict plus the malformed split.
const (
	exitCheckedClean  = 0
	exitCheckedFailed = 1
	exitCouldNotCheck = 2
	exitMalformed     = 3
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskpins sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	root := "."
	check := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			check = true
		case "--root":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskpins: --root needs a directory")
				return deskkit.ExitRefused
			}
			i++
			root = args[i]
		default:
			fmt.Fprintf(stderr, "deskpins: unknown argument %q\n%s\n", args[i], usage)
			return deskkit.ExitRefused
		}
	}
	if !check {
		fmt.Fprintln(stderr, "deskpins: nothing to do — pass --check\n"+usage)
		return deskkit.ExitRefused
	}

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
