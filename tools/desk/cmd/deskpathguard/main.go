// Command deskpathguard is the protected-verifier-paths check (verify-integrity/01): the
// reviewer-side control that stops a worker PR from silently editing the table it will be
// graded by in the same diff as the code that table checks.
//
// THE GAP IT CLOSES. Before this tool, nothing read whether a worker PR touched a brief's
// `## Verify` table, a CI workflow file, a client-side guard or a golden fixture in the same
// diff as the code — see docs/protected-paths.md for the exact set and the exemption
// boundary. `check` closes that: given a PR, it labels the change `wrote-to-the-test` and
// prints the `gate-forced: wrote-to-the-test` line the status transition reads, UNLESS the
// author is the desk/verifier identity, the PR carries a `regen:` label, or the diff touches
// only brief/fixture paths (pure authoring — see Evaluate in protected.go for the exact
// three-condition rule). `rederive` is verify-desk's pre-change re-read: it reports every
// Verify-table row present at HEAD but absent at the merge-base with origin/main as
// author-added, so a row the worker added is never run as a witness.
//
// THREE STATES, NEVER TWO (the stream README's own rule): a PR whose diff could not be read
// is could-not-check (exit 6), never rendered as clean. See Evaluate's doc comment.
//
// Exit codes (deskkit contract): 0 ok/clean · 3 disabled · 4 rate-limited · 5 refused ·
// 6 could-not-check.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskpathguard — protected-verifier-paths check (verify-integrity/01).

USAGE:
  deskpathguard check <owner/repo> <number> [--dry-run]
  deskpathguard rederive --root <path> --brief <path-to-brief.md> [--main <ref>]
  deskpathguard --version

check      Reads the PR's changed files, author identity and labels. A worker (or any
           non-desk, non-verifier) PR that touches a protected path (see
           docs/protected-paths.md) alongside at least one non-brief, non-fixture file is
           labelled wrote-to-the-test and prints a "gate-forced: wrote-to-the-test" line
           the status transition reads. Exempt: the desk/verifier identity, a PR carrying
           a regen: label, or a diff touching only brief/fixture paths. A diff that could
           not be read is could-not-check (exit 6) — NEVER clean.

rederive   verify-desk's pre-change re-read: reports every Verify-table row present at
           HEAD but absent (by its "#" cell) at the merge-base with origin/main as
           author-added, and prints every other row's BASE command/expect (never the
           head's — an existing row's command may have been edited to always pass).

--dry-run  (check only) resolves the verdict and prints it, but never writes the label.
--main     (rederive only) the ref to merge-base HEAD against; default origin/main.

Exit: 0 ok/clean · 3 disabled · 4 rate-limited · 5 refused · 6 could-not-check.`

func main() {
	// check performs a forge write (ApplyLabels); rederive is a local git read. Both declare
	// the same non-CI-eligible class as the sibling write verbs (desklabel, deskclose): the
	// roster is read from the config-home file, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskpathguard sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill switch is checked first, before the verb's payload is parsed.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "check":
		err = cmdCheck(rest, os.Stdout)
	case "rederive":
		err = cmdRederive(rest, os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "deskpathguard: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskpathguard:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
