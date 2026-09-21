// Command cellctl starts, stops and scaffolds an Assay CELL on one laptop.
//
// This is the Go port of tools/cellctl/cellctl. The shell script stays in
// the tree as the parity ORACLE: tools/cellctl/tests/parity.test.sh runs both implementations
// over the same fixtures and diffs their DRY_RUN plans, stdout, stderr and exit codes, so every
// line this program prints is a CONTRACT with that script until a human signs the cutover.
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
// A source build honestly reports "dev", the same convention the shell script's CELLCTL_VERSION
// and `var statusgenVersion = "dev"` use, so a stale copy is detectable.
var cellctlVersion = "dev"

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
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. A control surface that lives in settings rather
	// than in a diff is visible only at RUN time; without the echo a NARROWING is invisible.
	deskkit.EchoEffectiveConfig(os.Stderr)
	code := run()
	os.Exit(code)
}

func run() (code int) {
	defer func() {
		if r := recover(); r != nil {
			if ec, ok := r.(exitCode); ok {
				code = ec.code
				return
			}
			panic(r)
		}
	}()
	args := os.Args[1:]

	// `--version` / `version` — pure introspection, recognised as the SOLE argument only, and
	// answered before any other parsing, so a stale copy is detectable exactly the way
	// `statusgen --version` makes a stale statusgen detectable.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Println(cellctlVersion)
		return 0
	}

	verb := ""
	if len(args) > 0 {
		verb = args[0]
	}
	rest := []string{}
	if len(args) > 1 {
		rest = args[1:]
	}

	switch verb {
	case "providers":
		cmdProviders(rest)
	case "ls":
		cmdLs()
	case "check":
		// `check`'s second parameter really is a single optional positional (a config dir), not
		// a flag set, so the oracle's fixed-arity form is kept here too.
		cfg := ""
		if len(rest) > 1 {
			cfg = rest[1]
		}
		cmdCheck(needCell(rest), cfg)
	case "deskd":
		cmdDeskd(needCell(rest))
	case "desk":
		cmdDesk(needCell(rest), rest[1:])
	case "smoke":
		cmdSmoke(needCell(rest), rest[1:])
	case "status":
		cmdStatus(needCell(rest))
	case "up":
		cmdUp(needCell(rest), rest[1:])
	case "down":
		cmdDown(needCell(rest), rest[1:])
	case "new":
		cmdNew(rest)
	case "set":
		cmdSet(needCell(rest), rest[1:])
	case "show":
		cmdShow(needCell(rest), rest[1:])
	case "-h", "--help", "":
		usage(0)
	default:
		die("unknown verb '%s' (try --help)", verb)
	}
	return 0
}

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
