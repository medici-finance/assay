// Command deskfile is the filing-gate desk tool.
//
// It makes the filing-discipline ruling binding: dedupe BEFORE filing, attach
// instances to class issues, and budget filings. An issue that should have been a comment
// on a class issue never gets minted.
//
// Repo scope comes from deskkit.IsAllowedRepo — deskfile introduces NO new repo list. The
// filing identity is the caller's ambient gh credential (worker/App, unchanged); deskfile
// gates WHETHER and WHERE, not WHO, and never mints an App token.
//
// Exit codes (deskkit contract): 0 success/noop · 3 disabled ·
// 4 rate-limited · 5 refused · 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskfile — filing gate: mandatory dedupe, class-issue attach, per-pass budget.

USAGE:
  deskfile new    -R <owner/repo> --title <t> --body-file <f> [--label ...] [--force-new --reason <r>]
  deskfile attach -R <owner/repo> --to <N> --body-file <f>
  deskfile check  -R <owner/repo> --title <t>
  deskfile --version

new    — file a new issue. Runs a dedupe search against the repo's OPEN issues first: if a
         candidate scores at/above the match threshold it REFUSES (exit 5) and prints the
         exact ` + "`attach`" + ` command for the top candidate. A search API failure fails CLOSED
         (exit 6) — minting a possibly-duplicate issue is the expensive direction. The
         escape hatch is --force-new --reason <r>, which bypasses the search and is
         audit-logged. A session may file at most 3 new issues per repo per rolling 24h
         (exit 4 over); attach comments are never budgeted.

attach — post an observation as a comment on issue N (a class issue or duplicate target).
         Never budgeted. Refuses (exit 5) if N is CLOSED, with reopen-or-new guidance.

check  — dry-run dedupe: prints candidates and exits 0/5 the same as ` + "`new`" + ` would, but
         writes nothing. The verb skills embed in authoring loops.

The body is read from --body-file only (no stdin/inline), capped at 16 KiB, and secret-
scanned; there is no override flag. <owner/repo> must be in the desk-tools repo set
(deskkit.allowedRepos — deskfile adds no list of its own).

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck — and
	// TestEveryRosterReadingMainDeclaresClassAndEchoes now fails any main that reads
	// the roster without one). deskfile ACTS on the roster (files issues, posts
	// comments), so ciEligible=false: it reads the config-home file and never the
	// environment, in CI as well as locally — matching deskpr/deskboard.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. A control surface that lives in
	// settings rather than in a diff is visible only at RUN time; without the echo a
	// NARROWING is invisible.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskfile sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill-switch check is the FIRST action of the tool. Guard writes its own
	// result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	verb := args[0]
	rest := args[1:]
	var err error
	switch verb {
	case "new":
		err = cmdNew(rest)
	case "attach":
		err = cmdAttach(rest)
	case "check":
		err = cmdCheck(rest)
	default:
		err = deskkit.Refused("refused: unknown verb " + verb + " (want one of: new, attach, check)")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
