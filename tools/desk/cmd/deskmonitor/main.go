// Command deskmonitor is the desk roles' portable inbound surface: the stateful "did anything
// change?" pollers a desk window arms behind the harness Monitor tool, as a Go verb the release
// already ships instead of two bash scripts a native-Windows adopter cannot run.
//
//	deskmonitor inbound [--token-file OWNER=PATH ...] [owner/repo ...]   open issues (intake desk; scanloop arms it)
//	deskmonitor pr [owner/repo ...]                                      open PRs (review desk)
//
// Each is the port of a script that stays in the plugin tree as its PARITY ORACLE
// (plugins/assay/scripts/inbound-monitor.sh, pr-monitor.sh): same arguments, same environment
// knobs, same state files, same exit codes, and stdout byte-identical on the same forge answers —
// parity_test.go runs both against one recorded fixture and diffs them. What the port replaces is
// the machinery, not the contract: gh → the resolved forge client (deskkit.ForgeFor), jq →
// encoding/json, sort/comm → sort.Strings and a set difference, mktemp → os.CreateTemp,
// sleep → time.
//
// stdout carries ONLY the poll grammar scanloop's ParseMonitorOutput reads, which is why this main
// does not echo the effective config the way a desk verb normally does: the caller reads the
// child's combined output, and a config echo there would surface as unparsed monitor lines.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskmonitor — the desk roles' stateful inbound pollers.

usage:
  deskmonitor inbound [--token-file OWNER=PATH ...] [owner/repo ...]
      open-issue poll (the intake desk's inbound surface; scanloop arms it).
  deskmonitor pr [owner/repo ...]
      open-PR poll (the review desk's queue monitor).
  deskmonitor <inbound|pr> --help    the poller's own contract.
  deskmonitor --version              source SHA / build time.

Both print MONITOR-ARMED on a seed cycle, their event lines after it, and
MONITOR-DEGRADED for any repo whose read could not be trusted — a degraded repo
keeps its previous baseline, so an outage is absorbed rather than replayed.

Exit codes: 0 clean · 1 precondition failure · 2 at least one repo DEGRADED.
`

func main() {
	// Property A, first: whatever App token the launching shell exported, this poller is not it.
	dropInheritedTokens()
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	dropInheritedTokens()
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}
	switch args[0] {
	case "inbound":
		return runInbound(args[1:], stdout, stderr)
	case "pr":
		return runPR(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "--version", "version":
		s, b := deskkit.Version()
		fmt.Fprintf(stdout, "deskmonitor sourceSHA=%s builtAt=%s releaseTag=%s\n", s, b, deskkit.ReleaseTagOrDev())
		return 0
	default:
		fmt.Fprintf(stderr, "deskmonitor: unknown poller %q\n\n%s", args[0], usage)
		return 1
	}
}
