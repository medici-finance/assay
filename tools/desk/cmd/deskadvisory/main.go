package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskadvisory -- recompute-at-the-gate verification for advisory fixes.

USAGE:
  deskadvisory check <base-repo> <ghsa-id>

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (correctness review finding: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskadvisory sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "check":
		err = cmdCheck(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskadvisory: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskadvisory:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}

func cmdCheck(args []string) error {
	if len(args) != 2 {
		return deskkit.Refused("usage: deskadvisory check <base-repo> <ghsa-id>")
	}
	return checkAdvisory(args[0], args[1])
}
