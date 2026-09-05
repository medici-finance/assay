// Command deskclaim is the CLI over the flock-backed claimable-action LOCK primitive
// (deskkit/claim.go). It is the mutual-exclusion gate the desk skills
// call INSTEAD of the shell noclobber idiom `(set -C; … > "$f")` — the idiom the
// writeguard hook blocks, forcing a Write-tool fallback that has no O_EXCL and lets every
// racer "succeed" (double-dispatch). Moving atomicity into this binary
// is that fix.
//
// Verbs:
//
//	deskclaim acquire --kind K --item I [--branch B] [--owner O] [--repo R]
//	deskclaim release --item I
//	deskclaim list
//	deskclaim stale   --item I [--repo R]
//
// acquire exits 0 when the claim is acquired (fresh or a stale reclaim), 5 when the item is
// already claimed by a live holder (refused — do NOT proceed), and 6 when the lock could
// not be held or the claim could not be read/written (fail closed — NEVER "assume free").
//
// stale is the READ-ONLY probe of the same verdict acquire would use: exit 0 stale
// (reclaimable), 5 live (do not reclaim), 6 unreadable/missing. It never mutates the claim.
// Both acquire and stale wire a fail-closed branch-liveness probe (liveness.go): a claim
// recorded with --branch is reclaimable only once every readable liveness signal (the
// branch checked out in a worktree of --repo; the owner session's roster beacon) proves the
// branch is not doing live work. A signal that cannot be read means LIVE — a hand-delete of
// the claim file bypasses this and the flock and is never the remedy.
//
// These are local-state verbs on ~/.config/assay/claims: they take the full audit line
// and the kill switch but not the outward-write rate limit.
//
// SCOPE NOTE (2026-08-13): the `dispatch` kind no longer belongs here. A
// worker-desk dispatch claim is now the GitHub ref `refs/dispatch/<id>` in the target repo
// (tools/dispatch-claim.sh) — this directory is machine-local, so it served two dispatchers on
// one machine and did nothing for two desks on different machines, which is the case that
// double-dispatched. The other kinds (route/file/close/verify) gate a single session's own
// actions rather than a cross-machine hand-off and are unchanged.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited
// (unused here) · 5 refused · 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// version is an optional bare-`vX.Y.Z` build stamp (`-ldflags -X main.version`)
// feeding the brief-reading version gate (derived-board/06); empty on a real
// release, where the namespaced ReleaseTag stamp supplies the version.
var version string

const usage = `deskclaim — flock-backed claimable-action lock (dispatch/route/file/close/verify).

USAGE:
  deskclaim acquire --kind K --item I [--branch B] [--owner O] [--repo R]
  deskclaim release --item I
  deskclaim list
  deskclaim stale   --item I [--repo R]
  deskclaim --version

acquire — atomically claim <item>. Exits 0 acquired (fresh or a stale reclaim), 5 if a live
          holder already owns it (do NOT proceed), 6 if the lock could not be held or the
          claim could not be read/written (fail closed — never "assume free", #146). --kind
          is one of dispatch|route|file|close|verify. --owner defaults to the session id
          ($CLAUDE_SESSION_ID / $DESK_SESSION). --branch, when set, protects the claim from
          age-only reclaim while that branch has live work; --repo names the git repo whose
          worktrees prove that liveness (default: cwd if it is a git repo). stdout says
          'reclaimed' (not 'acquired') when it took over a stale claim.

release — remove <item>'s claim (hand the slot back). A missing claim is a no-op (exit 0).

list    — print every claim in the claims dir, read tolerantly so claims written by any
          writer (this CLI, loopengine, the legacy roster/bash idiom) all appear.

stale   — READ-ONLY: report whether <item>'s claim is reclaimable, WITHOUT touching it.
          Exits 0 stale (reclaimable), 5 live (do NOT reclaim), 6 unreadable or missing (a
          missing claim is not stale — nothing to reclaim). Prints one line:
          item=<I> age=<m>m ttl=<m>m branch=<B|-> holder=<owner> verdict=<stale|live|unreadable>
          because=<age-under-ttl|branch-checked-out:<path>|beacon-live|no-repo-cannot-prove|old-no-live-signal>.
          A hand-delete of a claim file bypasses the flock and is NEVER the remedy for a
          stuck --branch claim; run 'stale' then 'acquire' instead.

Exit: 0 ok/noop/stale · 3 disabled · 5 refused (already claimed / live) · 6 unverifiable.`

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
		fmt.Printf("deskclaim sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
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

	// Brief-reading version gate (derived-board/06 §6): a stamped deskclaim below
	// v1.0.0 refuses a brief-v2 tree (exit 6).
	if code := deskkit.RefuseIfTreeV2BelowV1(deskkit.RootsFromArgs(args), deskkit.EffectiveToolVersion(version), "deskclaim", os.Stderr); code != 0 {
		return code
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
	case "stale":
		err = cmdStale(rest)
	default:
		err = deskkit.Refused("refused: unknown verb " + verb + " (want one of: acquire, release, list, stale)")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
