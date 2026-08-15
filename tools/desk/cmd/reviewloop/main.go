package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident.
	// reviewloop READS only: it plans, it does not act on the roster, so ciEligible=true
	// is wrong here for the same reason it is wrong in verifyloop — this tool's plan
	// drives outward verbs, so it takes the config-home-only class.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("reviewloop sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Guard (kill switch + stop flags) is the FIRST action, before any board read — the
	// iteration boundary of a reactor whose iteration is one plan pass. Precedence
	// DISABLED > STOP > STOP.<DESK_LOOP> is enforced inside deskkit.Guard.
	if os.Getenv("DESK_LOOP") == "" {
		_ = os.Setenv("DESK_LOOP", "pr-review-desk")
	}
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	switch args[0] {
	case "plan":
		err := cmdPlan(args[1:], os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return deskkit.ExitCodeOf(err)
	default:
		fmt.Fprintf(os.Stderr, "reviewloop: unknown subcommand %q\n\n%s", args[0], usage)
		return deskkit.ExitRefused
	}
}

const usage = `reviewloop — pr-review-desk's board-reactor driver (archetype B; NOT a drain).

USAGE:
  reviewloop plan --actions <actions.json|-> [--prs <prs.json>] [--now <RFC3339>]
  reviewloop --version

'plan' classifies every row of a deskboard sweep against the reactor's action table,
coalesces the outward verbs on (repo, pr, head, verb), and states the THREE-STATE idle
verdict. It spawns nothing, writes nothing outward, and makes no GitHub call.

  --actions   ` + "`deskboard actions`" + ` JSON. REQUIRED: no sweep means BLIND, not idle.
  --prs       ` + "`deskboard prs`" + ` JSON. Supplies the head SHAs the actions verb omits;
              without it every outward verb is SUPPRESSED as could-not-check.

Exit: 0 idle-or-busy and positively measured · 3 disabled · 5 refused · 6 unverifiable
(includes: the board could not be read, an ACTION the table does not know, or an idle
question the board could not answer — rc=0 never means "all clear" on an unread board).

The standing-window cutover is gate: human — BLOCKED-ON-HUMAN. The desk never merges.
`
