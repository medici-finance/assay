package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// checkSuitesHold is the part of checks-green that reads what the rollup cannot show: a check
// SUITE at the head that has not concluded (#6234).
//
// A suite is the container one integration opens per commit, and the rollup lists only the
// runs inside suites. A suite still queued or running with ZERO runs therefore adds nothing to
// the rollup, so latestPerRollupName judges the older green run of each name while the forge's
// merge gate waits on the stuck suite. The rollup reads green; the forge reads BLOCKED.
// Which suites hold is deskkit.PendingCheckSuites — one rule, shared with deskautolane's
// checks-green, so the two gates cannot drift apart on it.
//
// A suite read that fails, or that served fewer suites than the forge asserts, is could-not-check.
func checkSuitesHold(o flipOpts, fg deskkit.Forge, fr deskkit.ForgeRepo, head string) error {
	cs, err := fg.CheckSuitesAtHead(fr, head)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read the check suites at %s (%s) — a suite the rollup cannot see may be "+
				"what the forge's merge gate waits on, and an unread suite list is could-not-check, never green.",
			condChecksGreen, short(head), firstLine(err.Error())), err)
	}
	if cs == nil || cs.TotalCount > len(cs.Suites) {
		served, total := 0, 0
		if cs != nil {
			served, total = len(cs.Suites), cs.TotalCount
		}
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: read %d/%d check suites at %s — the forge asserts more than it served, so a "+
				"pending suite may be in the part that was not read.",
			condChecksGreen, served, total, short(head)), nil)
	}
	if held := deskkit.PendingCheckSuites(cs.Suites); len(held) > 0 {
		return deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: the rollup at %s reads green, but %d check suite(s) at that head have not "+
				"concluded (%s). A suite with no conclusion holds the forge's merge gate even when it carries "+
				"no runs, and a run-less suite never appears in the rollup — not-yet-concluded is "+
				"could-not-verify, never green.",
			condChecksGreen, short(head), len(held), strings.Join(held, ", ")), nil)
	}
	return nil
}
