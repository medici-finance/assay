// Command deskperf reads the opt-in, local-only perf history the desk verbs write when
// ASSAY_TELEMETRY=1 (deskkit/perf.go) and turns it into per-tool percentiles
// and a boot-cost breakdown.
//
//	deskperf report [--days N] [--tool X] [--role R] [--top N] [--json]
//	deskperf boot   [--days N] [--role R] [--json]
//	deskperf --version
//	deskperf -h | --help
//
// It is a separate binary rather than a deskaudit subcommand because the two datasets make
// opposite promises: the audit ledger never loses a row and is read by the budget and
// idempotency paths; the perf history is opt-in, pruned to 7 UTC days on write, and read by
// nobody but a human or a desk. deskperf's own code reads only <state dir>/perf/ and never
// audit.jsonl. The one exception is the shared kill-switch guard (deskkit.Guard) every desk
// verb runs under: it writes its audit row, and reads the ledger tail when disarmed. Apart
// from that row deskperf writes nothing, and it opens no network connection.
//
// An empty history is a REFUSAL that names its reason, never an all-zeroes report: "nothing
// was slow" and "nobody was looking" are different facts.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskperf — read the local perf history (opt-in: ASSAY_TELEMETRY=1).

usage:
  deskperf report [--days N] [--tool X] [--role R] [--top N] [--json]
      per tool: count, p50 / p90 / max wall ms, total wall ms, share of total wall,
      refusal ratio (refused + unwritten + unverifiable over count); then the top-N
      slowest invocations (default 10) as ts / tool / verb / wallMs. Legacy
      records (the original schema) and perf-v2 records are reported as SEPARATE
      groups, never summed: the perf-v2 group adds its boundary and CPU coverage,
      the measured-CPU distribution, per-counter measured/unavailable coverage and
      per-phase distributions
  deskperf boot [--days N] [--role R] [--json]
      deskboot's per-step p50 / p90 / max and the summed boot cost, per role
  deskperf --version      print the build stamp
  deskperf -h | --help    this text

--days defaults to 7 and is capped at 7: the history keeps 7 UTC days and no more, so a
longer window is refused rather than silently answered with 7.

The history lives in <state dir>/perf/YYYY-MM-DD.jsonl (state dir: ~/.config/assay). It is
written only when ASSAY_TELEMETRY is exactly 1, holds no repo, PR, branch, path, argv or
free text, and never leaves the machine. deskperf only reads it.

exit: 0 ok · 3 disabled (kill switch/stop flag) · 5 refused (bad flags, or no record
matches the filter) · 6 unverifiable (no perf history to read)`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskperf sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Mandatory first call, like every desk tool: a read is refused while the kill switch or
	// a stop flag is armed.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, "deskperf:", err.Error())
		return deskkit.ExitCodeOf(err)
	}

	var err error
	switch args[0] {
	case "report":
		err = cmdReport(args[1:], stdout, stderr)
	case "boot":
		err = cmdBoot(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "deskperf: unknown subcommand %q\n\n%s\n", args[0], usage)
		return deskkit.ExitRefused
	}
	if deskkit.IsHelpRequest(err) {
		fmt.Fprintln(stderr, usage)
		return deskkit.ExitOK
	}
	if err != nil {
		fmt.Fprintln(stderr, "deskperf:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}

// window parses --days: default 7, at least 1, and REFUSED above 7.
func window(days int) error {
	if days < 1 {
		return deskkit.Refused(fmt.Sprintf("--days %d: must be at least 1", days))
	}
	if days > deskkit.PerfMaxDays {
		return deskkit.Refused(fmt.Sprintf("--days %d: the perf history keeps %d UTC days and no more, "+
			"so there is no day %d to read — ask for --days %d or fewer", days, deskkit.PerfMaxDays, deskkit.PerfMaxDays+1, deskkit.PerfMaxDays))
	}
	return nil
}

// parse runs a FlagSet, handing a help request back as flag.ErrHelp (tier two).
func parse(fs *flag.FlagSet, args []string) error {
	fs.SetOutput(new(strings.Builder))
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return deskkit.Refused("bad flags: " + err.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("unexpected arguments: " + strings.Join(fs.Args(), " "))
	}
	return nil
}

// load reads the window and turns an EMPTY history into a reason-named refusal. The reason
// distinguishes logging that is off from logging that is on but has nothing in the window,
// so the refusal never reads as an all-zeroes report.
func load(days int) (deskkit.PerfHistory, error) {
	h, err := deskkit.LoadPerfHistory(days)
	if err != nil {
		return h, deskkit.Unverifiable("cannot read the perf history directory", err)
	}
	if len(h.Records) == 0 {
		if !deskkit.PerfEnabled() {
			return h, deskkit.Unverifiable(fmt.Sprintf(
				"no perf history: local perf logging is off (ASSAY_TELEMETRY is not 1) — "+
					"%d day file(s) and no record in the last %d day(s). Export ASSAY_TELEMETRY=1 "+
					"in the desk's environment to start recording.", h.DayFiles, days), nil)
		}
		return h, deskkit.Unverifiable(fmt.Sprintf(
			"no perf history: local perf logging is on (ASSAY_TELEMETRY=1) but no wrapped verb "+
				"has recorded in the last %d day(s) (%d day file(s), %d unparseable line(s) skipped)",
			days, h.DayFiles, h.Skipped), nil)
	}
	return h, nil
}

// noMatch is the refusal for a filter that matched nothing in a NON-empty history.
func noMatch(what string) error {
	return deskkit.Refused("perf history present but no record matches " + what)
}

// skippedLine reports unparseable lines rather than dropping them silently.
func skippedLine(w io.Writer, skipped int) {
	if skipped > 0 {
		fmt.Fprintf(w, "note: %d unparseable line(s) skipped (a truncated final line or a foreign schema)\n", skipped)
	}
}
