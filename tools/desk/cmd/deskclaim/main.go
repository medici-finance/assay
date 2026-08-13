// Command deskclaim is the CLI over the flock-backed claimable-action LOCK primitive
// (deskkit/claim.go). It is the mutual-exclusion gate the desk skills
// call INSTEAD of the shell noclobber idiom `(set -C; … > "$f")` — the idiom the
// writeguard hook blocks, forcing a Write-tool fallback that has no O_EXCL and lets every
// racer "succeed" (double-dispatch). Moving atomicity into this binary
// is that fix.
//
// Verbs:
//
//	deskclaim acquire --kind K --item I [--branch B] [--owner O]
//	deskclaim release --item I
//	deskclaim list
//
// acquire exits 0 when the claim is acquired (fresh or a stale reclaim), 5 when the item is
// already claimed by a live holder (refused — do NOT proceed), and 6 when the lock could
// not be held or the claim could not be read/written (fail closed — NEVER "assume free").
//
// These are local-state verbs on ~/.config/assay/claims: they take the full audit line
// and the kill switch but not the outward-write rate limit.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited
// (unused here) · 5 refused · 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskclaim — flock-backed claimable-action lock (dispatch/route/file/close/verify).

USAGE:
  deskclaim acquire --kind K --item I [--branch B] [--owner O]
  deskclaim release --item I
  deskclaim list
  deskclaim --version

acquire — atomically claim <item>. Exits 0 acquired (fresh or a stale reclaim), 5 if a live
          holder already owns it (do NOT proceed), 6 if the lock could not be held or the
          claim could not be read/written (fail closed — never "assume free", #146). --kind
          is one of dispatch|route|file|close|verify. --owner defaults to the session id
          ($CLAUDE_SESSION_ID / $DESK_SESSION). --branch, when set, protects the claim from
          age-only reclaim while that branch has live work.

release — remove <item>'s claim (hand the slot back). A missing claim is a no-op (exit 0).

list    — print every claim in the claims dir, read tolerantly so claims written by any
          writer (this CLI, loopengine, the legacy roster/bash idiom) all appear.

Exit: 0 ok/noop · 3 disabled · 5 refused (already claimed) · 6 unverifiable.`

func main() {
	// Explicit roster class (a correctness review): deskclaim acts on
	// local state, ciEligible=false — it reads the config-home file, never the environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskclaim sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill-switch check is the FIRST gated action. Guard writes its own
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
	case "acquire":
		err = cmdAcquire(rest)
	case "release":
		err = cmdRelease(rest)
	case "list":
		err = cmdList(rest)
	default:
		err = deskkit.Refused("refused: unknown verb " + verb + " (want one of: acquire, release, list)")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
