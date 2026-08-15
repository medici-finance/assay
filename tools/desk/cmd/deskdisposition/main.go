// Command deskdisposition records and reads PR DISPOSITION RECORDS — a worker's terminal
// verdict on a pull request that no longer needs code work.
//
// The defect it closes (#728, #827): a worker that finds an orphaned PR superseded posts
// "recommend close" as prose. The orphan sweep reads PR-level signals only and cannot see
// it, so the same PR is dispatched again — 8 of 10 completed orphan dispatches in one
// 2026-08-12 cycle were re-derivations of an already-posted conclusion, and tracker#829 was
// re-derived four times across three weeks. Worse, the release comment stating the
// conclusion RESET the staleness clock, so the sweep cited a worker's own "this is dead"
// note as evidence of activity.
//
// This makes the verdict machine-readable: `set` writes a label (the index a sweep filters
// on) and a marker comment (the record, carrying the evidence link). `sweep` is the read
// the orphan scan runs BEFORE dispatching. `read` is the full per-PR record, for
// issue-flow/03 (deskclose).
//
// WHAT IT DOES NOT DO: close anything. Closing a PR is a human-authorized event; this tool
// only records the finding that makes the close decidable. That boundary is the point —
// the previous "fix" for this class was workers stating close intent and nobody executing,
// which is how #1439 sat stated-but-unclosed from 2026-08-09.
//
// IDENTITY: gh runs under the caller's AMBIENT credential — the worker App token a
// worker-desk session has already minted, or the human's gh login. This tool gates
// WHETHER and WHERE (allowed repos, closed vocabulary, evidence required), never WHO; it
// mints no token. Same discipline as deskfile.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskdisposition — record and read machine-readable PR disposition verdicts.

USAGE:
  deskdisposition set   -R <owner/repo> --pr <N> --verdict <V> --evidence <url|owner/repo#N>
                        [--by <who>] [--date YYYY-MM-DD] [--dry-run]
  deskdisposition read  -R <owner/repo> --pr <N>
  deskdisposition sweep -R <owner/repo> [--limit 100]
  deskdisposition --version

VERDICTS (closed vocabulary):
  SUPERSEDED          the work landed through a different branch/PR — evidence REQUIRED
  RESOLVED-ELSEWHERE  the outcome was reached another way (issue already closed, row
                      already advanced on main) — evidence REQUIRED
  NEEDS-REBASE        still live work, mechanically blocked — stays dispatch-eligible

set    — writes the label AND the marker comment, in that order. Idempotent: an identical
         record already present is a no-op (exit 0), so re-running on every pass is free.
         It never closes the PR — that is deskclose's (issue-flow/03) human-authorized act.

read   — the FULL record for one PR (label + marker comment), for deskclose.

sweep  — the CHEAP read the orphan sweep runs before dispatch: one API call per repo,
         labels only. Prints one line per open PR:
             <number>\t<state>\t<verdict>\t<dispatch-eligible>\t<title>
         State is three-state: checked-clean (no record — a real candidate) /
         checked-failed (a record exists) / could-not-check. A could-not-check PR is
         NOT dispatch-eligible: re-dispatching on an unread record is the #728 waste.
         Exits 6 when the repo's list read fails at all — a sweep that could not look
         must never be reported as an empty queue.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident.
	// This tool ACTS on the roster (it gates the repo it writes to), so it is
	// ciEligible=false: it reads the config-home file and never the environment, in
	// CI as well as locally — matching deskfile/deskpr.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run: a control surface that lives in
	// settings rather than in a diff is visible only at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskdisposition sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill switch is checked FIRST, before the verb's payload is parsed.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "set":
		err = cmdSet(rest, os.Stdout)
	case "read":
		err = cmdRead(rest, os.Stdout)
	case "sweep":
		err = cmdSweep(rest, os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "deskdisposition: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskdisposition:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
