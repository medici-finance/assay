// Command deskwt is the worktree-lifecycle desk tool.
// It encodes TWO local-only verbs — "add a worktree under a sanctioned prefix" and
// "remove a worktree I can prove is safe to delete" — so the mandated isolate-first rule
// is the path of least resistance instead of
// a prompt, and so cleanup can never wipe another session's uncommitted work the way a raw
// rm-class delete can.
//
// Safety is by construction:
//   - both verbs act only under the RESOLVED (EvalSymlinks) prefixes `/private/tmp/tracker-*`
//     and `<repo-root>/.claude/worktrees/`; a path that resolves elsewhere is refused;
//   - the shared checkout is refused by IDENTITY (git-common-dir's parent), not prefix;
//   - remove refuses a dirty TRACKED tree, unpushed commits, or a branch with no upstream,
//     and there is NO --force / override verb anywhere;
//   - add never clobbers an existing target and refuses an unresolvable --base.
//
// These are local-only verbs: they take the full audit line and the kill switch
// but NOT the outward-write rate limit (deskkit/ratelimit.go "Verb classes").
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled, 4 rate-limited
// (unused here), 5 refused, 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskwt — add, remove, or prune git worktrees, only under sanctioned prefixes.

USAGE:
  deskwt add <name> [--branch B] [--base origin/main]
  deskwt remove <path>
  deskwt prune [--repo <path>] [--interval <dur>]
  deskwt --version

deskwt is safe by construction: every verb acts ONLY on paths that RESOLVE under
/private/tmp/tracker-* or <repo-root>/.claude/worktrees/, the shared checkout is refused by
identity, and remove/prune refuse a dirty tracked tree or unpushed commits. There is NO
--force flag. On any state it cannot positively verify it refuses.

prune first runs ` + "`git worktree prune`" + ` (drops entries for dirs already gone), then removes
ONLY worktrees it can prove safe: tracked-clean AND fully merged into origin/main. An
UNMERGED branch (an open PR in flight) is LEFT untouched — that is the active-worker guard.
With --interval (e.g. 30m) it loops forever, sweeping every interval (for a k8s desk pod's
prune loop); it honors the kill switch / STOP flags between ticks and exits 0 on SIGTERM.

Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskwt sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action of the tool. Guard writes its own
	// result=disabled audit line.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "add":
		err = cmdAdd(rest)
	case "remove":
		err = cmdRemove(rest)
	case "prune":
		err = cmdPrune(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskwt: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
