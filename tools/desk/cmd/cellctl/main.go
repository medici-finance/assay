// Command cellctl starts, stops and scaffolds an Assay CELL on one laptop.
//
// This is the Go port of the original bash cellctl, and it is the launcher releases ship. The
// shell script stays in the tree as a TEST FIXTURE only, at
// tools/cellctl/testdata/cellctl-shell-oracle.sh — the parity ORACLE:
// tools/cellctl/tests/parity.test.sh runs both implementations over the same fixtures and diffs
// their DRY_RUN plans, stdout, stderr and exit codes, so every line this program prints is a
// CONTRACT with that script until a human signs the cutover.
//
// Layout mirrors the brief's files: cell.go (cell.env + kind/harness/forge validation), env.go
// (the scrubbed allowlist, stated once), plan.go (the [plan] grammar), and one file per verb.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cellctlVersion is the release tag this binary ships at, stamped at link time exactly like
// every other tools/desk/cmd/* program:
//
//	go build -ldflags '-X main.cellctlVersion=v1.2.3' ./cmd/cellctl
//
// A source build leaves it "dev", the same convention the shell script's CELLCTL_VERSION and
// `var statusgenVersion = "dev"` use. `--version` then reports "dev-<commit>[-dirty]" from the
// toolchain's embedded VCS stamp when one exists (version.go, versionString), so two source builds
// from different commits — a stale copy and a fresh one — print different versions.
var cellctlVersion = unstampedVersion

// exitCode is the panic payload die() raises; main recovers it so every exit path runs deferred
// cleanup (the session lock, above all) instead of calling os.Exit from deep in a call tree.
type exitCode struct{ code int }

// die prints the oracle's refusal shape — "cellctl: <msg>" on stderr — and exits 3.
func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "cellctl: "+format+"\n", args...)
	panic(exitCode{3})
}

// exitWith leaves with a specific code without the "cellctl: " prefix (the caller has already
// printed whatever it wanted to say). Exit 4 is the session-lock refusal.
func exitWith(code int) { panic(exitCode{code}) }

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found SetToolClass had no caller anywhere, so "ClassWrite is the
	// safe default" was true only by luck — and TestEveryRosterReadingMainDeclaresClassAndEchoes
	// fails any main that reads the roster without one). cellctl reads the cell home's roster to
	// answer `check`'s write-authorisation rows — rosterParses() via deskkit.LoadConfig, and
	// rosterAllowedRepos() for the scrubbed exact-scope row — so ciEligible=false, matching
	// deskpr/deskboard/deskfile: the answer must come from the cell's config-home FILE and never
	// from the environment. That is not a stylistic match. ClassCI would let the surrounding
	// environment supply the very scope line `check` exists to audit, so a cell whose roster file
	// is missing or wider than its CELL_REPO_SLUG could pass its own check on inherited env vars.
	// The cell home's file is the thing under audit, so it is the only admissible source.
	// The model-policy hook's deadline starts before anything that can block, the roster echo
	// included (policy_enforce.go, armHookDeadline).
	armHookDeadline(commandArgs(os.Args[1:]))
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// The roster echo (deskkit.EchoEffectiveConfig) runs from the command tree's pre-run hook
	// (cobra.go), once per run, so that --help and --version stay pure introspection.
	os.Exit(run())
}

// echoEffectiveConfig is P3: echo the effective roster once per run. A control surface that
// lives in settings rather than in a diff is visible only at RUN time; without the echo a
// NARROWING is invisible.
func echoEffectiveConfig() { deskkit.EchoEffectiveConfig(os.Stderr) }

// run parses and executes os.Args[1:] through the Cobra tree and returns the exit code.
func run() int { return runTree(os.Args[1:]) }

// needCell mirrors the oracle's `"${2:?cell}"` — bash's own message on an unset parameter is
// `<script>: line N: 2: cell`, which is not a contract anything reads; what IS the contract is
// that a missing cell name refuses with a non-zero exit and says the word `cell`.
func needCell(rest []string) string {
	if len(rest) == 0 || rest[0] == "" {
		fmt.Fprintln(os.Stderr, "cellctl: cell")
		exitWith(1)
	}
	return rest[0]
}

// commandArgs recognizes the one global selector before hook deadline detection, in every
// spelling the tree accepts (--cells-root <abs>, --cells-root=<abs>, and the Go single-dash
// forms). Validation still happens in run, but no alternate hook spelling can defer its
// watchdog until after the potentially blocking roster echo.
func commandArgs(args []string) []string {
	if n := selectorSpan(args); n > 0 && len(args) > n {
		return args[n:]
	}
	return args
}
