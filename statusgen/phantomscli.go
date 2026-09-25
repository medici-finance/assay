package main

import (
	"flag"
	"fmt"
	"io"
)

// phantomscli.go — the `statusgen phantoms` CLI. Self-contained,
// STATUS.md-free, offline (same
// discipline as --eligibility, eligibilitycli.go): it reads the tree once and
// prints one line per finding.
//
// WHY THIS VERB EXISTS. `--lint` never changes exit code for ANY board-honesty
// phantom class — severity is NOTICE, deliberately, per boardhonesty.go's
// package comment: arming a hard exclusion against a corpus already carrying a
// drifted-row backlog would red every unrelated PR on day one. But a class
// that fires the day a sibling merges is exactly the kind of thing a CI row or
// a scheduled desk sweep DOES want to go red on, without waiting for a "later
// ruling" that promotes the whole board-honesty family. `phantoms` is that
// narrow door: a dedicated verb, its own exit-code contract, and it never
// touches the regen/--lint path it sits beside.
//
// Only `--class sibling-merge-unreconciled` is wired today — the six classes
// boardhonesty.go's classifyPhantom judges are tree-only and stay NOTICE-only
// until a later ruling promotes them (see that file's package comment); this
// verb does not pre-empt that ruling by giving them a side-door exit code.
func runPhantoms(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("phantoms", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	class := fs.String("class", "", "phantom class to check — only \"sibling-merge-unreconciled\" is a dedicated exit-code verb today")
	var siblingRoots siblingRootFlags
	fs.Var(&siblingRoots, "sibling-root", `sibling checkout "<owner>/<repo>=<path>" this verb may read (repeatable; also read from DESK_ROOTS); a sibling named by neither is could-not-check`)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *class != phantomSiblingMergeUnreconciled {
		fmt.Fprintf(stderr, "statusgen phantoms: --class %q is not a recognized dedicated-verb class; only %q is wired today\n",
			*class, phantomSiblingMergeUnreconciled)
		return 2
	}

	streams, _, err := loadHydratedStreams(*root)
	if err != nil {
		fmt.Fprintln(stderr, "could-not-check:", err)
		return 2
	}

	// Invoking this verb IS the opt-in, so it does not consult
	// siblingMergeEnabled. It still reads only siblings the operator's map
	// (DESK_ROOTS or --sibling-root) names.
	overrides := effectiveSiblingRootOverrides(siblingRoots)
	notices, checkedFailed, couldNotCheck := siblingMergeCheck(streams, *root, overrides)
	for _, n := range notices {
		fmt.Fprintln(stdout, n)
	}

	// Exit contract (item 5(ii)): 0 clean, 1 at least one checked-failed row
	// (checked-failed always outranks a could-not-check finding elsewhere —
	// a live, actionable finding must never be masked by an unrelated
	// could-not-check on a different sibling), 2 any could-not-check and no
	// checked-failed.
	switch {
	case checkedFailed > 0:
		return 1
	case couldNotCheck > 0:
		return 2
	default:
		return 0
	}
}
