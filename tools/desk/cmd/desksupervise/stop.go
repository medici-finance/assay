package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stop.go — the two operator-facing verbs of the per-run stop signal:
//
//	desksupervise stop <key> --reason "..."   arm the STOP.run.<key> flag (audited)
//	desksupervise status --stops              list the armed per-run stops
//
// `stop` is the manual arm the observer (or a human) uses to halt ONE wedged or superseded
// run without touching the loop-wide kill switch; `tick` arms the same flag automatically
// for its two reclaim classes (see actions.go). `status --stops` is the desk window's read:
// the cadence sweep lists armed stops and issues the harness-side stop for each.

const stopUsage = `desksupervise stop — arm the per-run stop flag for ONE dispatched run.

USAGE:
  desksupervise stop <key> --reason "why this run is stopped"

<key> is the run's claim key (<repo>--<stream>--<NN> or <repo>--issue-<NN>) — the same key
deskdispatch records worktree-locally as assay.runKey. Arming writes <StateDir>/STOP.run.<key>;
deskkit.Guard then refuses every desk verb that run tries next (exit 3), naming --reason. The
per-run stop sits strictly BELOW the loop-wide DISABLED/STOP flags: it halts one run and
nothing else, and can never mask a loop-wide halt.

--reason is REQUIRED: a stop with no stated cause is one nobody can later act on.`

// cmdStop implements `desksupervise stop <key> --reason R`.
func cmdStop(args []string) (err error) {
	// --help is a pure read: print usage (naming --reason) and exit 0, no state touched.
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fmt.Fprintln(os.Stdout, stopUsage)
			return nil
		}
	}

	ac := &auditCtx{verb: "stop"}
	defer func() { ac.finalize(err) }()

	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return deskkit.Refused("refused: stop needs a <key> as its first argument (the run's claim key)")
	}
	key := args[0]

	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	reason := fs.String("reason", "", "why this run is stopped (required)")
	if perr := fs.Parse(args[1:]); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: stop takes exactly one <key> and --reason: unexpected " + strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*reason) == "" {
		return deskkit.Refused("refused: stop needs --reason \"...\" — a per-run stop with no stated cause is one nobody can act on")
	}

	if aerr := deskkit.ArmRunStop(key, *reason); aerr != nil {
		return deskkit.Unverifiable("could not arm the per-run stop for "+key, aerr)
	}
	ac.detail = fmt.Sprintf("armed STOP.run.%s: %s", key, firstLineOf(*reason))
	fmt.Fprintf(os.Stdout, "armed per-run stop for %s (%s) — %s refuses this run's next verb\n",
		key, deskkit.RunStopFlagName(key), "deskkit.Guard")
	return nil
}

const statusUsage = `desksupervise status — read the observer's own state.

USAGE:
  desksupervise status --stops     list the armed per-run stops (key  armed_at  reason)

--stops prints one line per armed STOP.run.<key> flag. It is the desk window's cadence read:
the sweep lists armed stops and, per harness, stops the dispatched worker for each.`

// cmdStatus implements `desksupervise status --stops`.
func cmdStatus(args []string) (err error) {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fmt.Fprintln(os.Stdout, statusUsage)
			return nil
		}
	}

	ac := &auditCtx{verb: "status"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	stops := fs.Bool("stops", false, "list the armed per-run stops")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: status takes no positional arguments")
	}
	if !*stops {
		return deskkit.Refused("refused: status needs a read to perform (e.g. --stops)")
	}

	list, lerr := deskkit.ListRunStops()
	if lerr != nil {
		return deskkit.Unverifiable("could not list per-run stops", lerr)
	}
	for _, s := range list {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", s.Key, s.ArmedAt.UTC().Format("2006-01-02T15:04:05Z"), s.Reason)
	}
	ac.detail = fmt.Sprintf("%d armed per-run stop(s)", len(list))
	if len(list) == 0 {
		ac.successResult = deskkit.ResultNoop
	}
	return nil
}

// firstLineOf collapses a reason to one audit-safe line.
func firstLineOf(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
