package deskkit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MergeRequirements is the forge's merge requirements for one change, read from fields that
// say the same thing to every reader (medici-finance/assay#6508). GitHub computes
// mergeStateStatus for the identity that reads it, so a role App that may not merge reads
// BLOCKED on a change that is ready to merge. The post-flip backstop therefore judges a BLOCKED
// read by these fields instead: the change's conflict verdict, its review decision, and the
// required contexts in the head's check rollup.
type MergeRequirements struct {
	// HeadSHA is the head the read describes.
	HeadSHA string
	// Mergeable is the conflict verdict in the GraphQL spelling: MERGEABLE, CONFLICTING or
	// UNKNOWN (not computed yet).
	Mergeable string
	// ReviewDecision is APPROVED, REVIEW_REQUIRED or CHANGES_REQUESTED, or "" when the forge
	// requires no review for the change.
	ReviewDecision string
	// Required is one entry per check the forge requires for this change: every context named
	// by classic branch protection or a ruleset on the base branch, and every rollup context the
	// forge marks required. Each carries the latest run of that name at the head
	// (LatestRunPerName), or the state "ABSENT" when no run of that name reported.
	Required []RequiredContext
	// Complete is false when the rollup or the base branch's rules hold more entries than the
	// read covered, or the base branch could not be read, so a requirement may be missing.
	Complete bool
	// Unevaluated names each base-branch rule the read can see but this check does not judge:
	// a ruleset rule of a type outside rulesetRuleEvaluated and rulesetRulePushOnly, and each
	// classic-protection setting that can hold a merge beyond the required checks and the
	// reviews reviewDecision already reflects. Each counts as unmet, so a change blocked by
	// such a rule reverts (#6509, security review).
	Unevaluated []string
}

// rulesetRulePushOnly lists the ruleset rule types that govern pushing to or deleting the
// base branch and never what a pull request needs before it merges. They are skipped.
var rulesetRulePushOnly = []string{"CREATION", "DELETION", "NON_FAST_FORWARD"}

// rulesetRuleEvaluated lists the ruleset rule types the read judges. A PULL_REQUEST rule is
// judged through reviewDecision (approval count, code-owner review, stale-review dismissal,
// last-push approval), except review-thread resolution, which counts as unevaluated when
// set. A REQUIRED_STATUS_CHECKS rule is judged through its contexts, except the strict
// up-to-date policy, which counts as unevaluated when set.
var rulesetRuleEvaluated = []string{"PULL_REQUEST", "REQUIRED_STATUS_CHECKS"}

// RequiredContext is one required context of a head's check rollup and its current result.
type RequiredContext struct {
	Name string
	// State is the run's conclusion once completed (SUCCESS, FAILURE, ...), else its status
	// (QUEUED, IN_PROGRESS, ...). A commit status reports its state (SUCCESS, PENDING, ...).
	State string
	// StartedAt orders runs of one name (RFC3339); "" sorts oldest.
	StartedAt string
}

// requiredContextAbsent is the State of a required check no run of which reported at the head.
const requiredContextAbsent = "ABSENT"

// requiredContextMet lists the results that satisfy a required context.
var requiredContextMet = []string{"SUCCESS", "NEUTRAL", "SKIPPED"}

// Unmet returns one line per merge requirement the read does not show as met. A read that
// cannot show a requirement as met counts as unmet: a nil read, an incomplete rollup, a
// conflict verdict that is not MERGEABLE, or a review decision this check does not recognise.
// An empty result means every requirement the read covers is met.
func (m *MergeRequirements) Unmet() []string {
	if m == nil {
		return []string{"the merge requirements could not be read"}
	}
	var out []string
	switch strings.ToUpper(m.Mergeable) {
	case "MERGEABLE":
	case "CONFLICTING":
		out = append(out, "the change has a merge conflict (mergeable is CONFLICTING)")
	default:
		out = append(out, fmt.Sprintf("the forge has not computed whether the change merges cleanly (mergeable is %q)", m.Mergeable))
	}
	switch strings.ToUpper(m.ReviewDecision) {
	case "", "APPROVED":
	case "REVIEW_REQUIRED", "CHANGES_REQUESTED":
		out = append(out, "the review decision is "+strings.ToUpper(m.ReviewDecision))
	default:
		out = append(out, fmt.Sprintf("the review decision %q is not one this check recognises", m.ReviewDecision))
	}
	for _, c := range m.Required {
		switch st := strings.ToUpper(c.State); {
		case st == requiredContextAbsent:
			out = append(out, fmt.Sprintf("the required check %s has not reported at the head", c.Name))
		case !backstopIn(requiredContextMet, st):
			out = append(out, fmt.Sprintf("the required check %s is %s", c.Name, st))
		}
	}
	for _, r := range m.Unevaluated {
		out = append(out, "the base branch requires "+r+", which this check does not evaluate")
	}
	if !m.Complete {
		out = append(out, "the read did not cover the whole check rollup and base-branch rules, so a required check may be missing")
	}
	return out
}

// mergeRequirementsQuery reads the fields MergeRequirements carries in one GraphQL request:
// the conflict verdict, the review decision, the head's check rollup (isRequired is evaluated
// for this pull request), and the checks the base branch requires through classic protection
// and rulesets, with every classic-protection and ruleset setting that can hold a merge, so
// a setting the check does not evaluate is seen and counted as unmet. Each connection is
// bounded at 100; a longer one leaves Complete false.
const mergeRequirementsQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){` +
	`pullRequest(number:$number){mergeable reviewDecision headRefOid ` +
	`baseRef{branchProtectionRule{requiredStatusCheckContexts requiresStatusChecks requiresStrictStatusChecks ` +
	`requiresConversationResolution requiresCommitSignatures requiresLinearHistory requiresDeployments ` +
	`requiredDeploymentEnvironments lockBranch} rules(first:100){totalCount nodes{type ` +
	`parameters{__typename ... on RequiredStatusChecksParameters{strictRequiredStatusChecksPolicy requiredStatusChecks{context}} ` +
	`... on PullRequestParameters{requiredReviewThreadResolution}}}}} ` +
	`commits(last:1){nodes{commit{oid statusCheckRollup{contexts(first:100){totalCount nodes{__typename ` +
	`... on CheckRun{name status conclusion startedAt isRequired(pullRequestNumber:$number)} ` +
	`... on StatusContext{context state createdAt isRequired(pullRequestNumber:$number)}}}}}}}}}}`

// MergeRequirementsRequest is the GraphQL request body for MergeRequirements on GitHub. Both
// GitHub clients (GitHubForge and deskpost's App client) send it, so they cannot diverge.
func MergeRequirementsRequest(owner, name string, number int) map[string]any {
	return map[string]any{"query": mergeRequirementsQuery,
		"variables": map[string]any{"owner": owner, "name": name, "number": number}}
}

type mergeRequirementsWire struct {
	Data struct {
		Repository *struct {
			PullRequest *struct {
				Mergeable      string  `json:"mergeable"`
				ReviewDecision *string `json:"reviewDecision"`
				HeadRefOid     string  `json:"headRefOid"`
				BaseRef        *struct {
					BranchProtectionRule *struct {
						RequiredStatusCheckContexts    []string `json:"requiredStatusCheckContexts"`
						RequiresStatusChecks           bool     `json:"requiresStatusChecks"`
						RequiresStrictStatusChecks     bool     `json:"requiresStrictStatusChecks"`
						RequiresConversationResolution bool     `json:"requiresConversationResolution"`
						RequiresCommitSignatures       bool     `json:"requiresCommitSignatures"`
						RequiresLinearHistory          bool     `json:"requiresLinearHistory"`
						RequiresDeployments            bool     `json:"requiresDeployments"`
						RequiredDeploymentEnvironments []string `json:"requiredDeploymentEnvironments"`
						LockBranch                     bool     `json:"lockBranch"`
					} `json:"branchProtectionRule"`
					Rules struct {
						TotalCount int `json:"totalCount"`
						Nodes      []struct {
							Type       string `json:"type"`
							Parameters *struct {
								Typename                         string `json:"__typename"`
								StrictRequiredStatusChecksPolicy bool   `json:"strictRequiredStatusChecksPolicy"`
								RequiredStatusChecks             []struct {
									Context string `json:"context"`
								} `json:"requiredStatusChecks"`
								RequiredReviewThreadResolution bool `json:"requiredReviewThreadResolution"`
							} `json:"parameters"`
						} `json:"nodes"`
					} `json:"rules"`
				} `json:"baseRef"`
				Commits struct {
					Nodes []struct {
						Commit struct {
							Oid               string `json:"oid"`
							StatusCheckRollup *struct {
								Contexts struct {
									TotalCount int `json:"totalCount"`
									Nodes      []struct {
										Typename   string `json:"__typename"`
										Name       string `json:"name"`
										Status     string `json:"status"`
										Conclusion string `json:"conclusion"`
										StartedAt  string `json:"startedAt"`
										Context    string `json:"context"`
										State      string `json:"state"`
										CreatedAt  string `json:"createdAt"`
										IsRequired bool   `json:"isRequired"`
									} `json:"nodes"`
								} `json:"contexts"`
							} `json:"statusCheckRollup"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// DecodeMergeRequirements maps the response to MergeRequirementsRequest. A GraphQL error, a
// missing pull request, or a head commit that differs from headRefOid is an Unverifiable error.
func DecodeMergeRequirements(raw []byte) (*MergeRequirements, error) {
	var w mergeRequirementsWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, Unverifiable("cannot decode the merge-requirements read", err)
	}
	if len(w.Errors) > 0 {
		msgs := make([]string, 0, len(w.Errors))
		for _, e := range w.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, Unverifiable("merge-requirements GraphQL error: "+strings.Join(msgs, "; "), nil)
	}
	if w.Data.Repository == nil || w.Data.Repository.PullRequest == nil {
		return nil, Unverifiable("the merge-requirements read returned no pull request", nil)
	}
	pr := w.Data.Repository.PullRequest
	out := &MergeRequirements{HeadSHA: pr.HeadRefOid, Mergeable: strings.ToUpper(pr.Mergeable), Complete: true}
	if pr.ReviewDecision != nil {
		out.ReviewDecision = strings.ToUpper(*pr.ReviewDecision)
	}
	if len(pr.Commits.Nodes) != 1 || !strings.EqualFold(pr.Commits.Nodes[0].Commit.Oid, pr.HeadRefOid) {
		return nil, Unverifiable(fmt.Sprintf("the merge-requirements read did not return the head commit %s", pr.HeadRefOid), nil)
	}
	// The checks the base branch requires, from classic protection and every ruleset.
	var names []string
	seen := map[string]bool{}
	want := func(n string) {
		if n = strings.TrimSpace(n); n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	if b := pr.BaseRef; b == nil {
		out.Complete = false
	} else {
		if p := b.BranchProtectionRule; p != nil {
			for _, c := range p.RequiredStatusCheckContexts {
				want(c)
			}
			// Classic settings that can hold a merge and that no field above evaluates. The
			// approval, code-owner, stale-review and last-push settings are reflected in
			// reviewDecision. Restricting who may push is the per-reader cause of BLOCKED
			// that this read exists to see past (#6508). The rest govern pushes only.
			for _, f := range []struct {
				on   bool
				what string
			}{
				{p.RequiresStatusChecks && p.RequiresStrictStatusChecks, "branch protection's up-to-date branch rule"},
				{p.RequiresConversationResolution, "branch protection's conversation resolution"},
				{p.RequiresCommitSignatures, "branch protection's signed commits"},
				{p.RequiresLinearHistory, "branch protection's linear history"},
				{p.RequiresDeployments || len(p.RequiredDeploymentEnvironments) > 0, "branch protection's required deployments"},
				{p.LockBranch, "branch protection's branch lock"},
			} {
				if f.on {
					out.Unevaluated = append(out.Unevaluated, f.what)
				}
			}
		}
		if b.Rules.TotalCount > len(b.Rules.Nodes) {
			out.Complete = false
		}
		for _, r := range b.Rules.Nodes {
			t := strings.ToUpper(r.Type)
			switch {
			case backstopIn(rulesetRulePushOnly, t):
			case !backstopIn(rulesetRuleEvaluated, t):
				out.Unevaluated = append(out.Unevaluated, "a ruleset rule of type "+t)
			case r.Parameters == nil:
				// The read asks for both evaluated types' settings; a rule returned without
				// them cannot be judged.
				out.Unevaluated = append(out.Unevaluated, "a ruleset rule of type "+t+" whose settings were not returned")
			case t == "REQUIRED_STATUS_CHECKS":
				for _, c := range r.Parameters.RequiredStatusChecks {
					want(c.Context)
				}
				if r.Parameters.StrictRequiredStatusChecksPolicy {
					out.Unevaluated = append(out.Unevaluated, "a ruleset's up-to-date branch rule")
				}
			case r.Parameters.RequiredReviewThreadResolution:
				out.Unevaluated = append(out.Unevaluated, "a ruleset's conversation resolution")
			}
		}
	}
	// Every run in the head's rollup, with the ones the forge marks required added to names.
	var runs []RequiredContext
	if rollup := pr.Commits.Nodes[0].Commit.StatusCheckRollup; rollup != nil {
		if rollup.Contexts.TotalCount > len(rollup.Contexts.Nodes) {
			out.Complete = false
		}
		for _, n := range rollup.Contexts.Nodes {
			run := RequiredContext{Name: n.Name, State: n.Status, StartedAt: n.StartedAt}
			switch {
			case n.Typename == "StatusContext":
				run = RequiredContext{Name: n.Context, State: n.State, StartedAt: n.CreatedAt}
			case strings.EqualFold(n.Status, "COMPLETED"):
				run.State = n.Conclusion
			}
			run.State = strings.ToUpper(run.State)
			runs = append(runs, run)
			if n.IsRequired {
				want(run.Name)
			}
		}
	}
	latest := map[string]RequiredContext{}
	for _, r := range LatestRunPerName(runs,
		func(c RequiredContext) string { return c.Name },
		func(c RequiredContext) string { return c.StartedAt }) {
		latest[r.Name] = r
	}
	for _, n := range names {
		r, ok := latest[n]
		if !ok {
			r = RequiredContext{Name: n, State: requiredContextAbsent}
		}
		out.Required = append(out.Required, r)
	}
	return out, nil
}
