// Command deskboot is the BOOT-seam adapter verb for the desk drain loops.
//
// WHAT IT IS. Every desk role opens its window with the same six-step ceremony — declare
// the loop identity, prune stale worktrees, lock this session's worktree, register the
// role on the roster, run the operating-envelope preflight, prove a token mints — and then
// reads the board. That ceremony was carried as ~40-90 lines of prose in each role's
// skill, five copies of one procedure, and five copies drift. deskboot is the one
// implementation: two sessions booting the same role now execute the same steps in the
// same order, because the machinery is COMPUTED rather than re-interpreted.
//
// THE ENGINE SEAM. The drain engine's contract has a pre-loop phase (boot) before its
// per-item Dispatch and its per-result Land. deskboot is the ADAPTER a loop's boot phase
// calls; it is not an engine hook, and the engine's Go API is not its interface. Desk
// skills and desk sessions bind to this CLI, never to the engine internals — which is what
// lets the engine change shape underneath without a prose rewrite.
//
// FAIL CLOSED, WITH THE STEP NAMED. Exit 0 means the WHOLE boot completed. Any other exit
// names the step that stopped it and what to do about it. There is no partial success and
// no silent state: a boot that got through four steps and could not do the fifth is a
// FAILED boot, because the role is then operating with an envelope nobody checked. The
// half-booted desk that claims work anyway is the failure mode this verb exists to remove.
//
// WHAT IT DOES NOT DO. It never claims work, never files an issue about the desk's own
// envelope (each failing check already names its own remedy), never prints a token value,
// and never retries a rejected probe under another identity. A probe REJECTION is a STOP.
//
// Exit codes (deskkit contract): 0 boot complete · 3 disabled · 5 refused (a precondition
// the caller controls: unknown role, loop identity unset or mismatched, shared checkout) ·
// 6 unverifiable (a step ran and could not be proven green — a red preflight, a mint that
// did not produce a usable token, a board fetch that failed). See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskboot — one command for a desk role's boot sequence (engine seam: BOOT).

USAGE:
  deskboot <role> [--root DIR] [--repo OWNER/NAME] [--quiet] [--dry-run]
  deskboot --version

<role> is the LOOP name the role presents in $DESK_LOOP — one of the names the kill
switch knows (deskboot --help prints the live set below). The token/preflight role is
DERIVED from it, so a session names its role once and cannot spell the two halves apart.

STEPS, in order. Each prints one line; the first red one stops the boot and NAMES itself.

  1 loop-identity     $DESK_LOOP is set and resolves to <role>'s loop class, so a
                      STOP.<name> flag a human is holding actually halts this session.
  2 worktree-prune    ` + "`deskwt prune`" + ` — bounded worktree growth. Only tracked-clean,
                      fully-merged worktrees are removed; active work is left alone.
  3 worktree-lock     locks THIS session's worktree so the prune supervisor cannot
                      reclaim it underneath a live desk. Refuses to boot in the shared
                      checkout — isolate first (` + "`deskwt role-init --role <role>`" + `).
  4 roster-set        ` + "`deskroster set --role <role>`" + ` — self-declares the session so
                      "who owns this desk" is answerable without a round-trip.
  5 roster-preflight  ` + "`deskroster preflight --role <token-role> --root <root>`" + ` — the
                      five-check operating envelope. A red preflight is could-not-run for
                      the WHOLE pass: nothing is claimed.
  6 token-mint        ` + "`desktoken <token-role> --repo <repo>`" + ` — proves the role's App
                      token mints from THIS shell before auto-mode tightens. Prints the
                      cache PATH and its age; never the token value.
  7 board-fetch       fetches origin/main read-only and summarises the board at
                      FETCH_HEAD. Read-only by construction: a write-mode board regen from
                      a session home strews generated files across the shared checkout.

--dry-run prints the plan (every step, its command, and the derived role/repo) and stops
before step 2 — nothing is pruned, locked, registered, minted or fetched.
--quiet suppresses the per-step OK lines; failures and the summary always print.

Exit: 0 boot complete · 3 disabled · 5 refused (caller precondition) · 6 unverifiable
(a step ran and could not be proven green).`

func main() {
	// Explicit roster class: deskboot ACTS on the roster (it registers a role and
	// resolves the repo set), so ciEligible=false — it reads the config-home file and
	// never the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run: a control surface that lives in settings
	// rather than in a diff is only visible at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskboot sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		fmt.Fprintln(os.Stderr, "\nKNOWN ROLES (derived from the kill switch's loop roster):")
		for _, r := range knownRoles() {
			fmt.Fprintf(os.Stderr, "  %-16s token-role %s\n", r, tokenRoleFor(r))
		}
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

	err := cmdBoot(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
