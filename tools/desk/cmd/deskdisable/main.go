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
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskdisable — replay a component's reverses (component-model.md §4/§7).

USAGE:
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

Exit codes: 0 ok · 5 refused · 6 could-not-check (deskkit contract).`

func main() {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return deskkit.ExitRefused
	}
	switch args[0] {
	case "--version", "version":
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskdisable sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, usage)
		return deskkit.ExitOK
	}
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "deskdisable: expected a component id first, got %q\n\n%s\n", args[0], usage)
		return deskkit.ExitRefused
	}

	target := args[0]
	fs := flag.NewFlagSet("deskdisable", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		root    = fs.String("root", ".", "tree root to discover component.yaml under")
		dryRun  = fs.Bool("dry-run", false, "compute and print the plan; write nothing")
		yes     = fs.Bool("yes", false, "actually run (required unless --dry-run)")
		cascade = fs.String("cascade", "", "comma-separated dependent component ids to disable first, in order")
	)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args[1:]); err != nil {
		return deskkit.ExitRefused
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "deskdisable: unexpected extra argument(s) %v\n\n%s\n", fs.Args(), usage)
		return deskkit.ExitRefused
	}
	if !*dryRun && !*yes {
		fmt.Fprintln(stderr, "deskdisable: refusing to run without --dry-run or --yes")
		return deskkit.ExitRefused
	}

	var cascadeList []string
	if strings.TrimSpace(*cascade) != "" {
		for _, c := range strings.Split(*cascade, ",") {
			if c = strings.TrimSpace(c); c != "" {
				cascadeList = append(cascadeList, c)
			}
		}
	}

	plan, err := Disable(Options{Root: *root, Target: target, Cascade: cascadeList, DryRun: *dryRun})
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
