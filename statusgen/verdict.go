package main

import "fmt"

// finalVerdict ends stdout for the gate modes with exactly one machine-parseable
// line, so callers (and the agents driving them) never have to interrogate
// `echo $?` in a fresh shell to learn how a --lint/--check run ended:
//
//	--lint:  "LINT: PASS"  or "LINT: FAIL <n> problem(s)"
//	--check: "CHECK: PASS" or "CHECK: DRIFT <n>"
//
// n counts the failure items already reported on stderr — PROBLEM lines for
// lint (an internal error that stops the run before problems can be counted
// reports as 1), drifted/stale generated files for check. Pure output contract:
// the verdict NEVER changes what passes or fails, and write/record modes are
// untouched.
// verdictAccum makes the one-verdict-line contract survive multi-root runs
// (assay-selfcontain/01). A multi-root run executes the per-root pipeline once
// per root, and each pass calls finalVerdict; without this, N roots would print
// N verdict lines and a caller reading "the last line" would see only the last
// root's outcome. When non-nil, finalVerdict ACCUMULATES instead of printing and
// the driver prints exactly one line for the whole run.
//
// nil in single-root runs, so 1-arg output stays byte-identical.
var verdictAccum *int

func finalVerdict(mode string, n int) {
	if verdictAccum != nil {
		*verdictAccum += n
		return
	}
	switch mode {
	case "lint":
		if n == 0 {
			fmt.Println("LINT: PASS")
		} else {
			fmt.Printf("LINT: FAIL %d problem(s)\n", n)
		}
	case "check":
		if n == 0 {
			fmt.Println("CHECK: PASS")
		} else {
			fmt.Printf("CHECK: DRIFT %d\n", n)
		}
	}
}
