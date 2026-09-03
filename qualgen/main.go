// Command qualgen is a repo-agnostic git-history quality miner — a
// statusgen-sibling Go binary (spec: docs/streams/quality/spec.md). It reads a
// git repository's full history into diffable, auditable, append-only artifacts
// that every later quality metric ("are we getting better / where is it brittle /
// how do we compare") is computed from.
//
// This is the wave-0 skeleton: the `mine` mode extracts commits + diffs through
// the three-state instrument wrapper and records its horizon. `report` (M1
// trend view), `check` (brittleness screen), and `pr` (per-PR risk-feature
// feed) are filled in by later briefs.
//
// The tool is repo-agnostic and OSS: no house specifics are hard-coded here;
// they arrive as configuration in later briefs.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// dispatch routes to the four modes. Exit codes follow the statusgen house
// convention: 0 success, 1 runtime error, 2 usage error.
func dispatch(args []string, stdout, stderr io.Writer) int {
	// `qualgen --version` / `qualgen version` — pure introspection, answered as
	// the SOLE argument (a consumer checking a pin invokes it alone).
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintln(stdout, qualgenVersion)
		return 0
	}
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	mode, rest := args[0], args[1:]
	switch mode {
	case "mine":
		return runMine(rest, stdout, stderr)
	case "report":
		return runReport(rest, stdout, stderr)
	case "pr":
		return runPR(rest, stdout, stderr)
	case "check":
		return runCheck(rest, stdout, stderr)
	case "sweep":
		return runSweep(rest, stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "qualgen: unknown mode %q (want one of: mine, report, pr, check, sweep)\n", mode)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `qualgen — repo-agnostic git-history quality miner

usage:
  qualgen mine   --repo <dir> --out <dir>   extract history into the tracking root
  qualgen report --out <dir>                render trend views          (not yet implemented)
  qualgen pr <n> --out <dir> [--repo <dir>] [--head <ref> --base <ref>]  per-PR risk-feature feed
  qualgen check <paths> --out <dir> [--repo <dir>]  brittleness screen for a named file set
  qualgen sweep  --repo <dir> --out <dir> --config <file> [--reverify-all]  code-slop forensic sweep lane
  qualgen --version                         print the release tag

All modes are read-only against the mined repo; artifacts land only under --out.
`)
}
