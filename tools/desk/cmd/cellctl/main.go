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
