// Command deskreply is the WORKER-side desk tool.
// It encodes ONE workflow verb — "post a plain reply comment on MY OWN open PR" — as the
// worker App (the worker App), so the worker can satisfy the PR review loop policy's
// step 2 (workers MUST reply on findings) without a permission prompt every
// iteration.
//
// Identity separation is the POINT of this tool: the worker voice and the reviewer
// App voice never share a tool. deskreply mints the WORKER App token via desktoken worker;
// it NEVER mints the reviewer App token — it never reads REVIEWER_APP_ID /
// REVIEWER_INSTALL_ID / any reviewer App key, and never imports deskpost's mint code.
// The unforgeable review gate depends on the reviewer App's voice being mintable ONLY
// via deskpost's audited path; a worker tool that could post as the reviewer App would
// break it. deskreply also has no review/verdict/ready verb: a worker tool
// cannot emit anything a board or desk would read as a reviewer signal.
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled,
// 4 rate-limited, 5 refused, 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskreply — post a plain reply comment on YOUR OWN open pull request.

USAGE:
  deskreply <owner/repo> <pr> --body-file F
  deskreply --version

deskreply posts a plain issue comment as the worker App (the worker App) — never
as the reviewer App. It has ONLY this verb: no review, no verdict, no ready, no merge,
no edit. Before posting it re-verifies in-tool that the PR is OPEN and that the
branch checked out in this worktree matches the PR's head branch — a worker replies on
ITS OWN PR. On any state it cannot positively verify it refuses.

The body is read from --body-file only (no stdin / inline body), is capped at 16 KiB, and
is secret-scanned; there is no override flag.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (an earlier correctness review found SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskreply sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill-switch check is the FIRST action of the tool. Guard writes its own
	// result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	err := cmdReply(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
