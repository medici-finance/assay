// Package adapters holds the reference implementations of qualgen's pluggable
// fix-linkage adapters (spec §5.1, §3.1 profile-B). This package is
// deliberately self-contained — no dependency on the qualgen command package —
// so a target-specific linkage source (GitHub today, another tracker
// tomorrow) is pure configuration, never new mining code wired into main.
package adapters

import (
	"fmt"
	"regexp"
	"strings"
)

// IssueLabelSource is a generic, per-target-configured source of an issue's
// labels. The reference implementation talks to the GitHub REST/GraphQL API;
// a test (or another tracker) supplies a stub. No internal identifiers are
// hardcoded anywhere in this package — the caller decides which labels count
// as defect-classed via GithubLabels.DefectLabels.
type IssueLabelSource interface {
	// IssueLabels returns the label set for issue `number`. repo is
	// "owner/repo"; an empty repo means "the mined repo itself". An error
	// means the linkage could not be resolved (not found, rate-limited,
	// permission denied, network failure) — the caller reports this as
	// could-not-identify, never as a silent non-fix.
	IssueLabels(repo string, number int) ([]string, error)
}

// GithubLabels is the reference fix-linkage adapter (spec §5.1): it resolves
// `Fixes #N` / `Closes #N` linkage out of a commit or PR message, and decides
// whether the referenced issue is defect-classed by reading its labels
// through the configured IssueLabelSource.
//
// DefectLabels is the full configured defect-classed label set, lower-cased.
// It is supplied by the caller — the default bug/defect/incident set plus, if
// the target has one, its own configured verdict-issue label lane (spec §3.1:
// "a repo's configured verdict-issue label set... no internal identifiers are
// hardcoded"). This package never hardcodes any label belonging to a specific
// house or repo; DefaultDefectLabels is the only built-in default, and it is
// generic across any GitHub repo.
type GithubLabels struct {
	Labels       IssueLabelSource
	DefectLabels map[string]bool
}

// DefaultDefectLabels is the generic starting label set every GitHub repo can
// reasonably be expected to use for a defect report. NewGithubLabels merges
// this with any additional per-target labels (e.g. a house's own verdict-issue
// lane) supplied by the caller.
func DefaultDefectLabels() map[string]bool {
	return map[string]bool{"bug": true, "defect": true, "incident": true}
}

// NewGithubLabels builds a GithubLabels adapter over source, with the default
// defect-label set merged with any extra labels the target configures (e.g. a
// repo's own verdict-issue label lane). extra is matched case-insensitively.
func NewGithubLabels(source IssueLabelSource, extra ...string) GithubLabels {
	labels := DefaultDefectLabels()
	for _, l := range extra {
		labels[strings.ToLower(strings.TrimSpace(l))] = true
	}
	return GithubLabels{Labels: source, DefectLabels: labels}
}

// closedIssuePattern matches GitHub's `Fixes #N` / `Closes #N` closing
// keywords (case-insensitive, singular or plural, an optional colon). Spec
// §5.1 names exactly these two keyword families; this is deliberately not a
// superset of every GitHub closing keyword (resolves/resolved), which is out
// of this brief's scope.
var closedIssuePattern = regexp.MustCompile(`(?i)\b(?:fix|fixes|close|closes)\s*:?\s*#(\d+)`)

// ClosedIssueNumber parses the first `Fixes #N` / `Closes #N` reference out of
// message. Returns (0, false) when no such reference is present — a candidate
// with no closing keyword is simply not tier-1 eligible, which is not itself
// an error.
func ClosedIssueNumber(message string) (int, bool) {
	m := closedIssuePattern.FindStringSubmatch(message)
	if m == nil {
		return 0, false
	}
	n := 0
	if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

// IssueDefectClassed decides whether issue `number` in `repo` carries a
// defect-classed label (spec §5.1 tier 1). An IssueLabelSource error is
// returned as-is — the caller (the qualgen classifier) turns it into
// could-not-identify rather than a silent non-fix.
func (g GithubLabels) IssueDefectClassed(repo string, number int) (bool, error) {
	labels, err := g.Labels.IssueLabels(repo, number)
	if err != nil {
		return false, fmt.Errorf("adapters: reading labels for issue #%d: %w", number, err)
	}
	for _, l := range labels {
		if g.DefectLabels[strings.ToLower(strings.TrimSpace(l))] {
			return true, nil
		}
	}
	return false, nil
}

// IsFixTaxonomy decides tier 2 (spec §5.1): the PR's branch carries the
// `fix/` prefix, or its title is a conventional-commit `fix:` title.
func (g GithubLabels) IsFixTaxonomy(branch, title string) bool {
	if strings.HasPrefix(branch, "fix/") {
		return true
	}
	t := strings.ToLower(strings.TrimSpace(title))
	return strings.HasPrefix(t, "fix:") || strings.HasPrefix(t, "fix(")
}

// fixKeywordPattern is the tier-3 fallback (spec §5.1) — the weakest evidence,
// reported separately and never silently merged with tiers 1-2.
var fixKeywordPattern = regexp.MustCompile(`(?i)\b(fix|fixes|fixed|bug|defect|regression)\b`)

// HasFixKeyword reports whether message contains a tier-3 keyword.
func (g GithubLabels) HasFixKeyword(message string) bool {
	return fixKeywordPattern.MatchString(message)
}
