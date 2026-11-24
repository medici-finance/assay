package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verifiertarget.go — the dispatch half of the pre-merge verifier target
// (#7623). deskkit.AdmitVerifierHead owns the admission rule; this file only
// refuses a malformed or misplaced target BEFORE the claim, so no claim,
// worktree or credential is spent on a run admission would refuse anyway.

const verifierTargetHelp = "Pre-merge verifier target (--target-change N --target-head SHA, verifier kit only): " +
	"the run executes an UNMERGED change head with the verifier's credential, so it is admitted only when " +
	"the run is in a credential-less, contained runner (NONE EXISTS YET, so every target is refused today: a " +
	"host-lane row runs as the desk's OS user with every same-user credential in reach), " +
	"the repository is opted in on the roster (ASSAY_VERIFIER_PREMERGE_REPOS, default off), the change is open at " +
	"EXACTLY that 40-hex head, the reviewer App's own verdict at that head is APPROVED with no reviewer-App " +
	"request for changes at it, its security lane posted Security-Review: pass at that head in a comment review of its own (one body grants one lane), the head branch is in " +
	"the base repository (never a fork), the author is a trusted role App or accountable human, and the change " +
	"author is none of the verifier, reviewer or dispatching desk Apps. " +
	"Fetch the change head into the checkout first; deskdispatch never fetches it. --attest-verifier and " +
	"--check-verifier re-read the change at every use: a moved head, a withdrawn approval, a closed change or an " +
	"opt-out refuses (exit 5) and an unreadable forge is could-not-check (exit 6). A pre-merge run never lands " +
	"Evidence and is never credited; the merged-main run stays owed. Cross-repository briefs are refused."

func (o dispatchOpts) hasVerifierTarget() bool { return o.targetChange != 0 || o.targetHead != "" }

func (o dispatchOpts) verifierTarget() deskkit.VerifierHeadTarget {
	return deskkit.VerifierHeadTarget{Change: o.targetChange, Head: o.targetHead}
}

// verifierTargetShape is the argument-only half: verifier kit, both halves, a
// full head. It reads nothing.
func verifierTargetShape(o dispatchOpts) error {
	if !o.hasVerifierTarget() {
		return nil
	}
	if !verifierKit(o.kit) {
		return deskkit.Refused("--target-change/--target-head are accepted only with --kit verifier")
	}
	if o.crossRepo {
		return deskkit.Refused("a pre-merge verifier target is never admitted with --cross-repo")
	}
	if why := o.verifierTarget().Problem(); why != "" {
		return deskkit.Refused(why)
	}
	return nil
}

// verifierTargetPreconditions is the read half, still before the claim: the
// repository's opt-in, no cross-repository brief, and the head commit present
// in the checkout the home is cut from.
func verifierTargetPreconditions(o dispatchOpts, dl deliverable, repo string) error {
	if dl.crossRepo(o.root) {
		return deskkit.Refused("a pre-merge verifier target is never admitted for a cross-repository brief")
	}
	if err := deskkit.VerifierPremergeOptedIn(repo); err != nil {
		return err
	}
	r := runCmd(o.root, "git", "rev-parse", "--verify", "--quiet", o.targetHead+"^{commit}")
	if r.err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: change %d's head %s is not in %s — fetch the change head first; deskdispatch never fetches it",
			stepWorktreeCreate, o.targetChange, o.targetHead, o.root), nil)
	}
	return premergeRunnerFn()
}

// premergeRunnerFn is deskkit.PremergeRunnerProblem: with no contained runner
// it refuses every target before the claim (#7628 review S1). A package seam
// so this package's tests can reach the steps behind it.
var premergeRunnerFn = deskkit.PremergeRunnerProblem

// detachedBaseLabel names what a detached home is cut from, for the plan and
// step lines.
func detachedBaseLabel(o dispatchOpts) string {
	if o.hasVerifierTarget() {
		return fmt.Sprintf("change %d head %s", o.targetChange, strings.TrimSpace(o.targetHead))
	}
	return "origin/main"
}
