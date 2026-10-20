package main

// briefadmission.go — the two brief-side holds taken BEFORE the claim (medici-finance/assay#2744).
//
// A brief can be startable on the board and still not be startable by THIS dispatch:
//
//   - its `exec-tier:` is stricter than --tier. A worker launched below the tier the brief
//     demands stops at pickup, after the claim and the worktree exist. briefTierRefusal is
//     pure and offline, so it runs in validateCallerPreconditions — a dry run refuses too.
//
//   - it is `gate: human` and its decision issue is open, labelled and unruled. The worker
//     finds the open decision after starting and releases the claim, so a slot and a claim
//     cycle are spent for nothing. briefDecisionHold reads the forge, so it runs after the
//     dry-run return, beside the phantom check, still before the claim.
//
// Both conditions are read through deskkit (briefadmission.go there), which `deskboard
// dispatch` reads too: the board drops exactly the rows this verb would refuse.
//
// The decision hold binds a FRESH worker dispatch only. The human gate binds approval, not
// implementation, so a brief whose decision issue does not exist yet still dispatches and
// files it (stepDecision); a --pr resume is fixing an already-open change; a reviewer or a
// verifier produces what the human decides on. A decision read that fails never holds.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionHeldMarker opens the refusal a decision hold returns.
const decisionHeldMarker = "dispatch HELD on an unruled decision"

// decisionWaitForgeFn resolves the forge the decision hold reads. It is nil unless main()
// wires liveDecisionWaitForge: a run that did not wire it reads nothing and holds nothing.
var decisionWaitForgeFn func(repo string) (deskkit.DecisionForge, deskkit.ForgeRepo, error)

// liveDecisionWaitForge is the production resolution, under the dispatching role, like the
// phantom check's read.
func liveDecisionWaitForge(repo string) (deskkit.DecisionForge, deskkit.ForgeRepo, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return nil, fr, err
	}
	fg, _, err := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}

// readBriefAdmission reads the brief's admission fields. ok is false when there is no brief
// or it cannot be read; the brief's own readability is checked elsewhere.
func readBriefAdmission(o dispatchOpts) (deskkit.BriefAdmission, bool) {
	if strings.TrimSpace(o.brief) == "" {
		return deskkit.BriefAdmission{}, false
	}
	path := o.brief
	if !filepath.IsAbs(path) {
		path = filepath.Join(o.root, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return deskkit.BriefAdmission{}, false
	}
	return deskkit.ParseBriefAdmission(string(raw)), true
}

// briefLabel is how a refusal names the brief: the caller's own spelling when there is one.
func briefLabel(o dispatchOpts) string {
	if s := strings.TrimSpace(o.briefShown); s != "" {
		return s
	}
	return o.brief
}

// briefTierRefusal refuses a dispatch whose --tier is weaker than the brief's `exec-tier:`.
func briefTierRefusal(o dispatchOpts) error {
	b, ok := readBriefAdmission(o)
	if !ok {
		return nil
	}
	covers, err := deskkit.TierCovers(o.tier, b.ExecTier)
	if err != nil {
		// An exec-tier outside the vocabulary is the brief lint's finding (statusgen --lint), and
		// the dispatch record already records it as null rather than refusing the line: it is not
		// evidence the brief demands more than --tier, so it is noted and the dispatch proceeds.
		fmt.Fprintf(os.Stderr, "deskdispatch: tier check: %v (brief %s) — not compared; dispatching at --tier %s.\n",
			err, briefLabel(o), o.tier)
		return nil
	}
	if covers {
		return nil
	}
	return deskkit.Refused(fmt.Sprintf(
		"step %s: brief %s declares `exec-tier: %s`, which is stricter than this dispatch's --tier %s. "+
			"Nothing was claimed and no worktree was created. Dispatch it with --tier %s from a session that "+
			"can launch that tier, or leave it for one.",
		stepClaimAcquire, briefLabel(o), b.ExecTier, o.tier, b.ExecTier))
}

// briefDecisionHold holds a fresh worker dispatch of a human-gated brief whose decision issue
// is still waiting on a ruling.
func briefDecisionHold(o dispatchOpts, plan dispatchPlan, repo string) error {
	if o.kit != "worker" || o.pr != 0 || !plan.gateHuman || decisionWaitForgeFn == nil {
		return nil
	}
	b, ok := readBriefAdmission(o)
	if !ok {
		return nil
	}
	note := func(why string) {
		fmt.Fprintf(os.Stderr, "deskdispatch: decision hold: %s — dispatching; the human gate still binds approval.\n", why)
	}
	fg, fr, err := decisionWaitForgeFn(repo)
	if err != nil || fg == nil {
		note("could not reach the forge to read the brief's decision issue")
		return nil
	}
	var isRatifier func(string, int64) bool
	if is, ok := reviewGateRatifierFn(); ok && is != nil {
		isRatifier = is
	}
	w := deskkit.BriefDecisionWait(fg, fr, b, isRatifier)
	if w.Unknown != "" {
		note(w.Unknown)
		return nil
	}
	if !w.Waiting {
		return nil
	}
	return deskkit.Refused(fmt.Sprintf(
		"%s — step %s: brief %s is `gate: human` and its decision issue %s#%d is open, carries `%s`, and has "+
			"no ruling recorded since that label was applied. Nothing was claimed and no worktree was created; "+
			"this is a delay, not a verdict. Dispatch again once the ruling is recorded on %s#%d.",
		decisionHeldMarker, stepClaimAcquire, briefLabel(o), fr.Slug(), w.Issue, deskkit.DecisionLabel, fr.Slug(), w.Issue))
}
