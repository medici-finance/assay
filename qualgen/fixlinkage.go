package main

import (
	"fmt"

	"github.com/medici-finance/assay/qualgen/adapters"
)

// EvidenceTier is the three precedence-ordered evidence tiers a candidate fix
// is classified into (spec §5.1), strongest first. Tier composition (the
// tier-1/2/3 split across a fix set) is itself a reported metric — see
// TierComposition — and tier 3 is NEVER silently merged with tiers 1-2.
type EvidenceTier string

const (
	// Tier1 — the candidate closes/references a DEFECT-CLASSED issue.
	Tier1 EvidenceTier = "tier-1-defect-labeled-issue"
	// Tier2 — the PR is classified `fix` by the repo's PR taxonomy (branch
	// prefix fix/, conventional-commit fix: title).
	Tier2 EvidenceTier = "tier-2-pr-taxonomy"
	// Tier3 — keyword fallback in the message. The weakest tier; reported
	// separately, never silently merged with tiers 1-2.
	Tier3 EvidenceTier = "tier-3-keyword-fallback"
)

// IssueRef is a repo-agnostic reference to an issue closed by a candidate fix.
// Repo is "owner/repo"; empty means the mined repo itself.
type IssueRef struct {
	Repo   string `json:"repo,omitempty"`
	Number int    `json:"number"`
}

// FixCandidate is one commit or PR under consideration as a possible defect
// fix. It carries only the raw facts a linkage adapter needs to classify it —
// no adapter-specific state.
type FixCandidate struct {
	// CommitSHA is always set: every candidate has a fix commit.
	CommitSHA string `json:"commit_sha"`
	// PRNumber is 0 when the candidate is a bare commit with no PR.
	PRNumber int `json:"pr_number,omitempty"`
	// PRBranch is the PR's head branch name (e.g. "fix/thing"), empty when
	// there is no PR.
	PRBranch string `json:"pr_branch,omitempty"`
	// PRTitle is the PR's title (e.g. "fix: thing"), empty when there is no
	// PR.
	PRTitle string `json:"pr_title,omitempty"`
	// Message is the commit message (or PR body, for a squash-merged PR):
	// the tier-3 keyword fallback and the Fixes/Closes linkage are both read
	// from here.
	Message string `json:"message"`
}

// FixID is the candidate's stable identity — the PR when there is one
// (multiple commits on a PR share one fix identity), else the bare commit SHA.
func (c FixCandidate) FixID() string {
	if c.PRNumber != 0 {
		return fmt.Sprintf("pr:%d", c.PRNumber)
	}
	return c.CommitSHA
}

// LinkageAdapter is the pluggable fix-linkage adapter interface (spec §5.1,
// §3.1 profile-B): given a candidate commit/PR, answer the two questions fix
// identification needs — does it close a defect-classed issue, and is it
// classified `fix` by the repo's own PR taxonomy or message keywords — without
// this package (or quality/07, which consumes DefectFix) ever hardcoding a
// GitHub-specific path. adapters.GithubLabels is the first reference
// implementation; any other linkage source (a different issue tracker, a
// house verdict-issue lane) is a new adapter, never new classifier code.
//
// A method returning a non-nil error means the linkage could not be resolved
// (an unreachable issue, a rate limit, a permission failure) — ClassifyFix
// turns that into DefectFix.Identified = could-not-identify, never a silent
// non-fix (spec §3.2).
type LinkageAdapter interface {
	// ClosedIssue resolves the issue a candidate's commit/PR closes or
	// references (e.g. "Fixes #N"). ok is false when no such reference is
	// present — that is not itself an error, just tier-1 ineligibility.
	ClosedIssue(c FixCandidate) (ref IssueRef, ok bool, err error)
	// IssueDefectClassed decides whether ref carries a defect-classed label.
	IssueDefectClassed(ref IssueRef) (bool, error)
	// PRIsFixTaxonomy decides tier 2: the PR's branch/title taxonomy.
	PRIsFixTaxonomy(c FixCandidate) (bool, error)
	// MessageHasFixKeyword decides tier 3, the keyword fallback. It never
	// fails — a keyword match is a plain string test.
	MessageHasFixKeyword(c FixCandidate) bool
}

// DefectFix is the record quality/07's B-SZZ inducing-commit trace consumes
// (interface-contract item declared by this brief). It carries, at minimum,
// the fix commit/PR identity, the closed-issue reference (or none), the
// evidence tier, and a three-state `identified` flag — quality/07 blames the
// fix's changed lines at its parent and MUST NOT re-derive fix identification
// from this record's inputs; it reads exactly these fields.
//
// Identified reuses the frozen Measure[T] three-state wrapper (spec §3.2):
//   - Measured(true)  — the candidate was identified as a fix; Tier is set.
//   - CouldNotMeasure — the linkage could not be resolved (an adapter error);
//     Tier is empty. Never silently treated as a non-fix.
//
// A candidate confirmed NOT to be a fix (every tier check ran cleanly and
// none matched) is not recorded as a DefectFix at all — ClassifyFix reports
// that case via its second return value, not via this record.
type DefectFix struct {
	FixCommitSHA string        `json:"fix_commit_sha"`
	FixPRNumber  int           `json:"fix_pr_number,omitempty"`
	ClosedIssue  *IssueRef     `json:"closed_issue,omitempty"`
	Tier         EvidenceTier  `json:"tier,omitempty"`
	Identified   Measure[bool] `json:"identified"`
}

// ClassifyFix runs the three-tier precedence classifier (spec §5.1) for one
// candidate against adapter, stopping at the strongest tier that matches:
//
//	tier 1 — closes a defect-classed issue
//	tier 2 — PR taxonomy `fix`
//	tier 3 — keyword fallback (weakest; reported separately, see
//	         ComputeTierComposition)
//
// Returns (fix, true) when the candidate is identified as a fix (Tier set,
// Identified = Measured(true)) or when linkage could not be resolved
// (Identified = CouldNotMeasure, Tier empty) — both cases are recorded.
// Returns (DefectFix{}, false) when the candidate is confirmed NOT a fix; the
// caller must not record it (spec: a non-fix is never emitted as a
// DefectFix).
func ClassifyFix(adapter LinkageAdapter, c FixCandidate) (DefectFix, bool) {
	fix := DefectFix{FixCommitSHA: c.CommitSHA, FixPRNumber: c.PRNumber}

	ref, ok, err := adapter.ClosedIssue(c)
	if err != nil {
		fix.Identified = CouldNotMeasure[bool](fmt.Sprintf("resolving closed-issue linkage: %v", err))
		return fix, true
	}
	if ok {
		fix.ClosedIssue = &ref
		defectClassed, err := adapter.IssueDefectClassed(ref)
		if err != nil {
			fix.Identified = CouldNotMeasure[bool](fmt.Sprintf("checking issue defect-class: %v", err))
			return fix, true
		}
		if defectClassed {
			fix.Tier = Tier1
			fix.Identified = Measured(true)
			return fix, true
		}
	}

	isFixTaxonomy, err := adapter.PRIsFixTaxonomy(c)
	if err != nil {
		fix.Identified = CouldNotMeasure[bool](fmt.Sprintf("checking PR taxonomy: %v", err))
		return fix, true
	}
	if isFixTaxonomy {
		fix.Tier = Tier2
		fix.Identified = Measured(true)
		return fix, true
	}

	if adapter.MessageHasFixKeyword(c) {
		fix.Tier = Tier3
		fix.Identified = Measured(true)
		return fix, true
	}

	// Confirmed non-fix: every tier check ran cleanly and none matched.
	return DefectFix{}, false
}

// TierComposition is the reported tier-composition metric (spec §5.1, §10): the
// count of IDENTIFIED fixes at each evidence tier, over a fix set. Tier3Count
// is carried on its own field and MUST NOT be folded into Tier1And2Count — a
// keyword-guess fix is never silently merged with a defect-labeled or
// taxonomy-classified one.
type TierComposition struct {
	Tier1Count int `json:"tier1_count"`
	Tier2Count int `json:"tier2_count"`
	// Tier3Count is reported SEPARATELY — see Tier1And2Count.
	Tier3Count int `json:"tier3_count"`
}

// Tier1And2Count is the combined strong-evidence count (tiers 1-2). It
// deliberately excludes Tier3Count — callers that want the keyword-fallback
// count read Tier3Count directly, never merged in here.
func (tc TierComposition) Tier1And2Count() int { return tc.Tier1Count + tc.Tier2Count }

// ComputeTierComposition computes the tier-composition metric over fixes.
// Only records with Identified == Measured(true) carry a tier and are
// counted; a could-not-identify record contributes to none of the three
// counts (it is neither a tiered fix nor absent — its own three-state field
// is how a consumer distinguishes "0 fixes" from "some fixes unidentified").
func ComputeTierComposition(fixes []DefectFix) TierComposition {
	var tc TierComposition
	for _, f := range fixes {
		if f.Identified.State != StateMeasured {
			continue
		}
		switch f.Tier {
		case Tier1:
			tc.Tier1Count++
		case Tier2:
			tc.Tier2Count++
		case Tier3:
			tc.Tier3Count++
		}
	}
	return tc
}

// GithubLabelsLinkage adapts the repo-agnostic adapters.GithubLabels reference
// adapter (which knows nothing about this package's types) to the
// LinkageAdapter interface declared above. adapters.GithubLabels is
// deliberately written against plain strings/ints so it has zero dependency
// on the qualgen command package; this type is the thin translation between
// the two.
type GithubLabelsLinkage struct {
	Impl adapters.GithubLabels
}

func (g GithubLabelsLinkage) ClosedIssue(c FixCandidate) (IssueRef, bool, error) {
	n, ok := adapters.ClosedIssueNumber(c.Message)
	if !ok {
		return IssueRef{}, false, nil
	}
	return IssueRef{Number: n}, true, nil
}

func (g GithubLabelsLinkage) IssueDefectClassed(ref IssueRef) (bool, error) {
	return g.Impl.IssueDefectClassed(ref.Repo, ref.Number)
}

func (g GithubLabelsLinkage) PRIsFixTaxonomy(c FixCandidate) (bool, error) {
	return g.Impl.IsFixTaxonomy(c.PRBranch, c.PRTitle), nil
}

func (g GithubLabelsLinkage) MessageHasFixKeyword(c FixCandidate) bool {
	return g.Impl.HasFixKeyword(c.Message)
}
