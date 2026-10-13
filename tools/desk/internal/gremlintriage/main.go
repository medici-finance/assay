// Command gremlintriage is the offline half of the daily Gremlins sweep
// (tools/gremlins/run.sh). It never runs Gremlins itself; it reads the files the
// sweep wrote and does two jobs:
//
//	plan    — given one package's dry-run report, choose today's slice of its
//	          files so the run fits its time budget, rotating through every file
//	          over successive days.
//	triage  — given every package's report, count killed / lived / not covered /
//	          timed out, and cross-reference each LIVED mutant against the
//	          committed muhar specs (mutations*.json). A survivor in a function no
//	          spec touches is reported as a candidate guard, with file:line.
//
// WHY a separate tool and not more shell: the cross-reference needs Go's parser
// (which function encloses a line) and the counts need to be tested offline.
//
// The sweep is DISCOVERY, never a gate. Survivors, candidates and a package that
// ran out of budget all exit 0. Exit 2 means the sweep itself broke: a report
// that is missing or unreadable, a Gremlins failure, a package whose tests are
// red in the mutant layout, or no package producing any evidence at all.
//
// It lives under internal/ as a main package (like examples/workpad-render) so
// the desk module's ordinary test run covers it, without shipping as a desk verb.
package main

import (
	"fmt"
	"io"
	"os"
)

// exitBroke is the only failing exit: the sweep produced no trustworthy evidence.
const exitBroke = 2

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gremlintriage plan|triage [flags]")
		return exitBroke
	}
	switch args[0] {
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	case "triage":
		return runTriage(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "gremlintriage: unknown subcommand %q (want plan or triage)\n", args[0])
		return exitBroke
	}
}
