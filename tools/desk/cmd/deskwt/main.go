// Command deskwt is the worktree-lifecycle desk tool.
// Its lifecycle verbs add under a sanctioned prefix and remove only proven-safe
// worktrees; its maintenance verb compacts the shared Git store offline. The isolate-first rule
// is the path of least resistance instead of
// a prompt, and so cleanup can never wipe another session's uncommitted work the way a raw
// rm-class delete can.
//
// Safety is by construction:
//   - add/remove act only under the RESOLVED (EvalSymlinks) temporary-root worktree prefix
//     and `<repo-root>/.claude/worktrees/`; a path that resolves elsewhere is refused;
//   - the shared checkout is refused by IDENTITY (git-common-dir's parent), not prefix;
//   - remove refuses a dirty TRACKED tree, unpushed commits, or a branch with no upstream,
//     and there is NO --force / override verb anywhere;
//   - add never clobbers an existing target and refuses an unresolvable --base.
//
// These are local-only verbs: they take the full audit line and the kill switch
// but NOT the outward-write rate limit (deskkit/ratelimit.go "Verb classes").
//
// Exit codes (deskkit contract): 0 success/noop, 2 usage (a malformed command line, refused
// before any effect), 3 disabled, 4 rate-limited
// (unused here), 5 refused, 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"os"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// perfVerbs is deskwt's registered first-argument set for the opt-in local perf record
// (deskkit.PerfRun): args[0] is recorded only when it is one of these, and as
// "other" otherwise, so no argv text can reach the record.
var perfVerbs = []string{"add", "remove", "prune", "maintain", "role-init", "role-clean", "--version", "--help"}

func main() {
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	// main owns the declared class, the effective-config echo and activation discovery.
	// Cobra invokes them only for an execution, after help, version and parsing: a help
	// screen or a malformed command line reads no roster and writes no audit row.
	env := wtCLI{
		before: func() error {
			// The roster class is an EXPLICIT declaration, never the zero value by accident.
			// This tool ACTS on the roster, so it is ciEligible=false: it reads the
			// config-home file and never the environment, in CI as well as locally.
			deskkit.SetToolClass(deskkit.ClassForTool(false))
			// P3: echo the effective roster once per run, so a narrowing is as visible as a
			// widening.
			deskkit.EchoEffectiveConfig(os.Stderr)
			if !deskkit.CheckVerbActivation(os.Stderr) {
				return deskkit.Unverifiable("deskwt activation could not be established", nil)
			}
			return nil
		},
		handle: guardedExecute,
	}
	// The one measurement finalizer wraps the whole command, parsing included.
	os.Exit(deskkit.PerfRunMeasured("deskwt", perfVerbs, os.Args[1:], func(a []string, measurement *perfrec.Session) (int, perfrec.Context) {
		env.performance = func(c perfrec.Context) {
			selected = c
			if measurement != nil {
				measurement.SelectContext(c)
			}
		}
		return parseWT(a, env, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}), selected
	}))
}

// run is one in-process invocation for the regression tests: the same tree, the kill switch
// and verb, without main's activation discovery.
func run(args []string) int {
	return parseWT(args, wtCLI{handle: guardedExecute}, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
}
