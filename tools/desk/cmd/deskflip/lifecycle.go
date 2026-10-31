package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lifecycleProducerApp is the only App whose lifecycle-preflight check run this condition
// accepts: the staged workflow runs as a GitHub Actions job, and a check run records the
// App that created it. A commit status records no producer App at all, so a status with
// this context is never evidence (#4139 review S1).
const lifecycleProducerApp = "github-actions"

// lifecycleDelivery classifies only whether the PR declares a delivery. Every Brief: or
// Cross-repo-brief: declaration counts; statusgen alone owns the authoring-only and
// declared-partial interpretation, and its successful preflight carries that exemption
// (#4139 review F8: deskflip no longer keeps a second copy of that meaning). A malformed
// or mixed declaration never grants an exemption.
func lifecycleDelivery(body string) bool {
	trailers, err := deskkit.ParseTrailers([]byte(body))
	if err != nil {
		return true
	}
	for _, tr := range trailers {
		switch tr.Kind {
		case deskkit.TrailerBrief, deskkit.TrailerCrossRepoBrief:
			return true
		}
	}
	return false
}

// lifecycleAdoptedOn is the adoption predicate (#4139 review F3): the condition binds only
// where the base branch's required status checks name lifecycle-preflight. deskflip ships
// in one release to every adopter, so a repository that has not provisioned the workflow
// keeps today's behaviour until it requires the check. The required set is read from the
// server; a read that fails is could-not-check, never "not adopted".
func lifecycleAdoptedOn(required func() ([]string, error)) (bool, error) {
	set, err := required()
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: cannot read the base branch's required status checks, so whether %s is adopted "+
				"there cannot be established — could-not-check is never 'not adopted'",
			condLifecyclePreflight, condLifecyclePreflight), err)
	}
	for _, c := range set {
		if strings.TrimSpace(c) == condLifecyclePreflight {
			return true, nil
		}
	}
	return false, nil
}

// lifecycleWorkflowPath reports whether a changed path is a workflow or composite-action
// definition: a PR that edits one can run its own job named lifecycle-preflight under the
// expected App, so that run cannot vouch for the PR.
func lifecycleWorkflowPath(p string) bool {
	p = strings.TrimPrefix(strings.TrimSpace(p), "./")
	return strings.HasPrefix(p, ".github/workflows/") || strings.HasPrefix(p, ".github/actions/")
}

// checkLifecyclePreflight judges the delivery condition. It reports adopted=true when the
// condition bound (a delivery declared on a base that requires the check) and passed.
//
// checks comes exclusively from the complete rollup fetched by commit SHA in flip. A
// previous-head run is absent from this observation, never evidence for this head. Unlike
// checks-green, skipped/neutral are not successes: this mandatory check must run.
func checkLifecyclePreflight(body string, files []fileInfo, checks []rollupEntry, head string, required func() ([]string, error), editedAt func() (string, error)) (bool, error) {
	if !lifecycleDelivery(body) {
		return false, nil
	}
	adopted, err := lifecycleAdoptedOn(required)
	if err != nil || !adopted {
		return false, err
	}
	for _, f := range files {
		if lifecycleWorkflowPath(f.Path) || lifecycleWorkflowPath(f.PreviousFilename) {
			return false, deskkit.Unverifiable(fmt.Sprintf(
				"condition %s: the PR changes %s, a workflow or action definition — a %s run at %s may come from "+
					"the PR's own workflow, so it cannot vouch for this PR; a maintainer must verify the delivery",
				condLifecyclePreflight, f.Path, condLifecyclePreflight, short(head)), nil)
		}
	}
	runs := make([]rollupEntry, 0, len(checks))
	sawStatus := false
	for _, c := range checks {
		if c.groupKey() != condLifecyclePreflight {
			continue
		}
		if c.Name == "" {
			sawStatus = true // a commit status names no producer App: never evidence
			continue
		}
		runs = append(runs, c)
	}
	for _, c := range latestPerRollupName(runs) {
		if c.AppSlug != lifecycleProducerApp {
			return false, deskkit.Unverifiable(fmt.Sprintf(
				"condition %s: the latest %s check run at %s was created by App %q, not %s — a check run "+
					"from any other producer is not the lifecycle verdict",
				condLifecyclePreflight, condLifecyclePreflight, short(head), c.AppSlug, lifecycleProducerApp), nil)
		}
		if !strings.EqualFold(c.Status, "completed") || !strings.EqualFold(c.Conclusion, "success") {
			return false, deskkit.Unverifiable(fmt.Sprintf("condition %s: the mandatory lifecycle check at %s has not completed successfully; statusgen must validate the proposed implementation transition before ready-flip", condLifecyclePreflight, short(head)), nil)
		}
		if editedAt != nil {
			stamp, err := editedAt()
			if err != nil {
				return false, deskkit.Unverifiable(fmt.Sprintf("condition %s: cannot read the current PR body edit time", condLifecyclePreflight), err)
			}
			if stamp != "" {
				edited, err := time.Parse(time.RFC3339Nano, stamp)
				started, startErr := time.Parse(time.RFC3339Nano, c.StartedAt)
				if err != nil || startErr != nil || started.Before(edited) {
					return false, deskkit.Unverifiable(fmt.Sprintf("condition %s: the successful check predates the current delivery declaration, or its time cannot be proven; rerun lifecycle-preflight after the PR body edit", condLifecyclePreflight), nil)
				}
			}
		}
		return true, nil
	}
	note := ""
	if sawStatus {
		note = " (a commit status with that context is not accepted: only a check run created by " + lifecycleProducerApp + ")"
	}
	return false, deskkit.Unverifiable(fmt.Sprintf("condition %s: delivering PR has no successful %s check run at %s%s; an absent or previous-head result is not a pass", condLifecyclePreflight, condLifecyclePreflight, short(head), note), nil)
}
