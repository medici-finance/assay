// Command regsuite gates a repository's regression-test suite: the tests
// named TestRegression_<repo>_<issue>[_Desc], per docs/test-policy.md
// "## Regression suite". It fails when a regression test fails, when the
// HEAD tree lists fewer such tests than the BASE tree it is compared
// against (a quiet deletion), or when a module's own selector executes
// fewer top-level regression tests than it lists (a `go test -run` that
// silently matches nothing — the vacuous pass docs/test-policy.md and
// statusgen/14 both name).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const usage = "usage: regsuite gate --head <dir> --base <dir> [--module <relpath>]... [--selector <regex>]\n"

func main() {
	os.Exit(mainRun(os.Args[1:], os.Stdout, os.Stderr))
}

func mainRun(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "gate":
		return runGateCmd(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n%s", args[0], usage)
		return 2
	}
}

// stringList is a repeatable flag.Value, used for --module.
type stringList []string

func (s *stringList) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprint([]string(*s))
}

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runGateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var head, base, selector string
	var modules stringList
	fs.StringVar(&head, "head", "", "head tree directory (required)")
	fs.StringVar(&base, "base", "", "base tree directory (required)")
	fs.StringVar(&selector, "selector", "", "execution -run selector override (default: "+regressionListPattern+")")
	fs.Var(&modules, "module", "restrict discovery to this repo-relative module path (repeatable)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 2
		}
		return 2
	}
	if head == "" || base == "" {
		fmt.Fprint(stderr, "--head and --base are required\n"+usage)
		return 2
	}

	r := runGate(gateOptions{
		Head:     head,
		Base:     base,
		Modules:  modules,
		Selector: selector,
	})
	fmt.Fprint(stdout, r.Report)
	return r.ExitCode
}
