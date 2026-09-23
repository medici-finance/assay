// deskrun — start a workflow run, or clear a deployment gate on one, under the repo's
// roster-bound RUN CREDENTIAL (forge-neutral/14).
//
// WHY THIS EXISTS. Starting a release run or approving a gated deployment used to be done
// with whoever's ambient forge CLI happened to be logged in, because GitHub grants both
// through ONE permission (`actions: write`) that also cancels runs, deletes run logs and
// disables workflows repo-wide — so no desk App can safely hold it, and no desk verb existed.
// deskrun is that verb, and the credential it acts under is decided by the roster, per repo,
// never by the session:
//
//	ASSAY_RUN_CREDENTIALS=<owner/name>=release-runner[+<gate shape>]   → proceeds, under the
//	                                                                     dedicated release-runner
//	                                                                     credential
//	ASSAY_RUN_CREDENTIALS=<owner/name>=human:<name>                    → REFUSED (exit 5): a
//	                                                                     human action today
//	(no entry)                                                         → could-not-check (exit 6)
//
// The binding is read BEFORE anything is minted, so a human-bound repo never produces a mint,
// a forge call, or a read of an ambient `gh`/`glab` credential. The token then comes from
// ForgeFor's ordinary custody path for the release-runner role, and the backend's own refusal
// of an unminted token is the second, independent layer.
//
// It never posts to GitHub's repository_dispatch endpoint (a trigger any App with
// contents: write could fire), and on GitLab it starts pipelines through the pipeline trigger
// token, not a project token.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskrun — start a workflow run, or clear a deployment gate, under the repo's roster-bound run credential.

USAGE:
  deskrun <owner/repo> <workflow> --ref <ref> [-f key=value ...] [--dry-run]
  deskrun approve <owner/repo> <run-id> --gate <name> [--dry-run]
  deskrun status  <owner/repo> <run-id>
  deskrun --version

dispatch — starts ONE run of <workflow> (GitHub: the workflow file name, e.g. release.yml;
           GitLab: .gitlab-ci.yml or "-" — a project has one pipeline definition) on --ref,
           with each -f key=value as an input/variable, and prints the run it created.
approve  — approves the ONE deployment gate named --gate that run <run-id> is waiting on.
           The gate shape (GitHub: environment; GitLab: environment or manual-job) comes from
           the repo's run-credential binding, never a flag.
status   — prints the run's lifecycle: queued | in_progress | waiting | completed (+ conclusion).

WHO ACTS: the credential is chosen by ASSAY_RUN_CREDENTIALS in the roster, per repo:
  <owner/name>=release-runner[+environment|+manual-job]  the dedicated release-runner credential
  <owner/name>=human:<name>                              refused (exit 5) — a human action today
  (no entry)                                             could-not-check (exit 6)
The binding is read before anything is minted; no ambient gh/glab credential is ever read.

--dry-run resolves the binding and the budget and prints what would happen, minting nothing.

Exit: 0 ok · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskrun sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	var err error
	switch args[0] {
	case "approve":
		err = cmdApprove(args[1:], os.Stdout)
	case "status":
		err = cmdStatus(args[1:], os.Stdout)
	default:
		err = cmdDispatch(args, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskrun:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
