package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// One checked source feeds the prompt and the sole worktree allocation boundary.
// verified distinguishes a dry-run forge read from a refreshed, matching remote tip.
type resumeSource struct {
	branch, head string
	verified     bool
}

var resumeHeadRe = regexp.MustCompile("^[0-9a-f]{40}$|^[0-9a-f]{64}$")

// Tests replace the transport only; branch and head admission always runs.
var readResumeChange = liveResumeChange

func liveResumeChange(o dispatchOpts, repo string) (deskkit.PullRequest, error) {
	fr, err := forgeRepoOf(repo)
	if err != nil {
		return deskkit.PullRequest{}, err
	}
	fg, _, err := deskkit.ResolveForge(fr, deskkit.DispatcherRole)
	if err != nil {
		return deskkit.PullRequest{}, err
	}
	change, err := fg.GetPullRequest(fr, o.pr)
	if err != nil {
		return deskkit.PullRequest{}, err
	}
	if change == nil {
		return deskkit.PullRequest{}, fmt.Errorf("forge returned no change")
	}
	return *change, nil
}

func workerResume(o dispatchOpts) bool {
	return o.pr > 0 && !reviewKit(o.kit) && !verifierKit(o.kit)
}

func resolveResume(o dispatchOpts, plan *dispatchPlan) error {
	if !workerResume(o) {
		return nil
	}
	change, err := readResumeChange(o, plan.repo)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("cannot read resume source for %s#%d; nothing was claimed", plan.repo, o.pr), err)
	}
	if change.State != "open" || change.MergedAt != "" {
		return deskkit.Refused("resume requires an open change; nothing was claimed")
	}
	if change.CrossRepo != deskkit.CrossRepoSame {
		return deskkit.Refused("resume requires a source branch confirmed in the target repository; fork or unknown source repository cannot use origin")
	}
	branch := change.HeadRef
	if !branchNameRe.MatchString(branch) || strings.Contains(branch, "..") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") || strings.Contains(branch, "//") || strings.Contains(branch, "/.") || strings.HasSuffix(branch, ".lock") {
		return deskkit.Unverifiable("resume source branch is missing or invalid; nothing was claimed", nil)
	}
	if !resumeHeadRe.MatchString(change.HeadSHA) {
		return deskkit.Unverifiable("resume head is missing or invalid; nothing was claimed", nil)
	}
	if o.branch != "" && o.branch != branch {
		return deskkit.Refused(fmt.Sprintf("--branch %q differs from the change's source branch %q; nothing was claimed", o.branch, branch))
	}
	source := &resumeSource{branch: branch, head: change.HeadSHA}
	if !o.dryRun {
		if _, err := worktreeBase(o, branch, change.HeadSHA); err != nil {
			return err
		}
		source.verified = true
	}
	plan.branch, plan.resume = branch, source
	return nil
}

// Called at the allocation boundary, including by future callers that bypass planning.
func dispatchBase(o dispatchOpts, plan dispatchPlan) (string, error) {
	if !workerResume(o) {
		return mainlineRef, nil
	}
	source := plan.resume
	if source == nil || !source.verified || source.branch != plan.branch || !resumeHeadRe.MatchString(source.head) || plan.detached {
		return "", deskkit.Refused("resume allocation lacks a verified source branch and head")
	}
	return source.head, nil
}
