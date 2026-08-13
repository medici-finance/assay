// Command deskroster manages a self-declared session roster.
//
// Each desk session declares what it's working on via set/drop; list joins
// the roster with live GitHub PR state and dispatch claims to answer
// "open work -> session" without a manual round-trip.
//
// Storage is OUT-OF-GIT: ~/.config/assay/roster/<session>.json.
// This is runtime machine-local state, never committed.
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled,
// 4 rate-limited, 5 refused, 6 unverifiable.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskroster — self-declared open-work → session roster (out-of-git).

Each desk session declares what it's working on; list joins the roster with
live GitHub PR state and dispatch claims for a full "who owns what" view.

USAGE:
  deskroster set   --repo R --pr N --what "..." [--role "..."] [--session NAME]
  deskroster drop  --repo R --pr N [--session NAME]
  deskroster list
  deskroster mine  [--session NAME]
  deskroster --version

Session resolution: $DESK_SESSION → $CLAUDE_SESSION_ID → --session flag.
Unresolvable → exit 6 (never guess a session identity).

Storage (out-of-git, machine-local): ~/.config/assay/roster/<session>.json

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskroster sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action (before flag parsing).
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "set":
		err = cmdSet(rest)
	case "drop":
		err = cmdDrop(rest)
	case "list":
		err = cmdList()
	case "mine":
		err = cmdMine(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskroster: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
