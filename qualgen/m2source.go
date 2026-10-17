package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/medici-finance/assay/qualgen/adapters"
)

const fixSourceSchema = "qualgen-fix-source-v1"

// FixSource is a tracker export explicitly bound to HEAD. An absent issue is
// unavailable evidence, not an issue with no defect labels. No network or
// credentials are used. The exporter owns the declared repository and coverage.
type FixSource struct {
	SchemaVersion string `json:"schema_version"`
	Repository    string `json:"repository"`
	HeadSHA       string `json:"head_sha"`
	// Optional all-history denominator supplied by the exporter. Zero means
	// unavailable; it is never inferred from commit count or fix count.
	MergedPRCount int              `json:"merged_pr_count,omitempty"`
	Issues        []FixSourceIssue `json:"issues"`
	PullRequests  []FixSourcePR    `json:"pull_requests,omitempty"`
	digest        string
}

type FixSourceIssue struct {
	Number     int        `json:"number"`
	Labels     []string   `json:"labels"`
	ReportedAt *time.Time `json:"reported_at,omitempty"`
}

type FixSourcePR struct {
	Number   int    `json:"number"`
	MergeSHA string `json:"merge_sha"`
	Branch   string `json:"branch,omitempty"`
	Title    string `json:"title,omitempty"`
	Body     string `json:"body,omitempty"`
}

func loadFixSource(path string) (*FixSource, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fix source: %w", err)
	}
	var source FixSource
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&source); err != nil {
		return nil, fmt.Errorf("decode fix source: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("fix source must contain one JSON object")
	}
	if source.SchemaVersion != fixSourceSchema {
		return nil, fmt.Errorf("fix source schema_version must be %q", fixSourceSchema)
	}
	if strings.TrimSpace(source.Repository) == "" || !strings.Contains(source.Repository, "/") {
		return nil, fmt.Errorf("fix source repository must name owner/repository")
	}
	if !fullSHA(source.HeadSHA) {
		return nil, fmt.Errorf("fix source head_sha must be a full commit SHA")
	}
	if source.MergedPRCount < 0 {
		return nil, fmt.Errorf("fix source merged_pr_count cannot be negative")
	}
	issues := map[int]bool{}
	for _, issue := range source.Issues {
		if issue.Number <= 0 || issues[issue.Number] {
			return nil, fmt.Errorf("invalid or duplicate issue number %d", issue.Number)
		}
		if issue.Labels == nil {
			return nil, fmt.Errorf("issue %d requires an explicit labels array", issue.Number)
		}
		issues[issue.Number] = true
	}
	prs, shas := map[int]bool{}, map[string]bool{}
	for _, pr := range source.PullRequests {
		if pr.Number <= 0 || prs[pr.Number] || !fullSHA(pr.MergeSHA) || shas[pr.MergeSHA] {
			return nil, fmt.Errorf("invalid or duplicate pull request %d / merge SHA", pr.Number)
		}
		prs[pr.Number], shas[pr.MergeSHA] = true, true
	}
	if source.MergedPRCount > 0 && source.MergedPRCount < len(source.PullRequests) {
		return nil, fmt.Errorf("merged_pr_count is smaller than the supplied PR set")
	}
	sum := sha256.Sum256(raw)
	source.digest = hex.EncodeToString(sum[:])
	return &source, nil
}

func fullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == strings.ToLower(s)
}

// First-parent changes are the landed units. Inspecting both a PR merge and
// its branch commits would count the same fix more than once.
func mainline(repo *git.Repository, tip string) ([]*object.Commit, error) {
	c, err := repo.CommitObject(plumbing.NewHash(tip))
	if err != nil {
		return nil, err
	}
	var commits []*object.Commit
	for {
		commits = append(commits, c)
		if c.NumParents() == 0 {
			break
		}
		c, err = c.Parent(0)
		if err != nil {
			return nil, fmt.Errorf("first-parent history is incomplete: %w", err)
		}
	}
	return commits, nil
}

func (s *FixSource) validateRepo(repo *git.Repository, tip string) error {
	if s.HeadSHA != tip {
		return fmt.Errorf("fix source head_sha %s does not match repository HEAD %s", s.HeadSHA, tip)
	}
	commits, err := mainline(repo, tip)
	if err != nil {
		return err
	}
	landed := map[string]bool{}
	for _, c := range commits {
		landed[c.Hash.String()] = true
	}
	for _, pr := range s.PullRequests {
		if !landed[pr.MergeSHA] {
			return fmt.Errorf("PR %d merge_sha is not on the HEAD first-parent history", pr.Number)
		}
	}
	return nil
}

func (s *FixSource) IssueLabels(repo string, number int) ([]string, error) {
	if repo != "" && !strings.EqualFold(repo, s.Repository) {
		return nil, fmt.Errorf("issue repository is outside the configured source")
	}
	for _, issue := range s.Issues {
		if issue.Number == number {
			return issue.Labels, nil
		}
	}
	return nil, fmt.Errorf("issue #%d is absent from the supplied label snapshot", number)
}

func (s *FixSource) ReportDate(fix DefectFix) (time.Time, bool) {
	if fix.ClosedIssue == nil {
		return time.Time{}, false
	}
	for _, issue := range s.Issues {
		if issue.Number == fix.ClosedIssue.Number && issue.ReportedAt != nil {
			return *issue.ReportedAt, true
		}
	}
	return time.Time{}, false
}

func appendM2(store *Store, repo *git.Repository, source *FixSource, labels []string, at time.Time) error {
	commits, err := mainline(repo, source.HeadSHA)
	if err != nil {
		return err
	}
	labelSet := map[string]bool{}
	for _, label := range labels {
		labelSet[strings.ToLower(strings.TrimSpace(label))] = true
	}
	linkage := GithubLabelsLinkage{Impl: adapters.GithubLabels{Labels: source, DefectLabels: labelSet}}
	prs := map[string]FixSourcePR{}
	for _, pr := range source.PullRequests {
		prs[pr.MergeSHA] = pr
	}
	var fixes []DefectFix
	unknown := 0
	for _, commit := range commits {
		candidate := FixCandidate{CommitSHA: commit.Hash.String(), Message: commit.Message}
		if pr, ok := prs[candidate.CommitSHA]; ok {
			candidate.PRNumber, candidate.PRBranch, candidate.PRTitle = pr.Number, pr.Branch, pr.Title
			candidate.Message += "\n" + pr.Body
		}
		if fix, record := ClassifyFix(linkage, candidate); record {
			fixes = append(fixes, fix)
			if fix.Identified.State == StateCouldNotMeasure {
				unknown++
			}
		}
	}
	corpus := TraceDefects(repo, fixes, source)
	// Append only new/changed facts. Retrying a completed or partial pass does
	// not duplicate identical fix/trace rows. Metrics are timestamped snapshots.
	path, _ := store.tablePath(KindDefect)
	seen := map[string]bool{}
	if err := streamRawJSONL(path, func(raw []byte) error {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		b, _ := json.Marshal(v)
		seen[string(b)] = true
		return nil
	}); err != nil {
		return err
	}
	appendFact := func(v any) error {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var normalized any
		if err := json.Unmarshal(raw, &normalized); err != nil {
			return err
		}
		key, _ := json.Marshal(normalized)
		if seen[string(key)] {
			return nil
		}
		if err := store.Append(KindDefect, v); err != nil {
			return err
		}
		seen[string(key)] = true
		return nil
	}
	for _, fix := range fixes {
		if err := appendFact(fix); err != nil {
			return err
		}
	}
	for _, trace := range corpus.Traces {
		if err := appendFact(trace); err != nil {
			return err
		}
	}
	traced, none, unavailable := corpus.Partition()
	metrics := corpus.DerivedMetrics(source.MergedPRCount, at)
	for i := range metrics {
		if metrics[i].Metric == MetricDefectInducingRate || metrics[i].Metric == MetricTracedCFRInput {
			if unknown > 0 || (len(corpus.Traces) > 0 && traced+none == 0) {
				metrics[i].Value = CouldNotMeasure[float64]("unresolved fix identification or no traceable fixes; cannot establish the inducing rate")
			}
		}
		if err := store.Append(KindMetric, metrics[i]); err != nil {
			return err
		}
	}
	run := M2Run{Metric: metricM2Run, HeadSHA: source.HeadSHA, Repository: source.Repository,
		SourceDigest: source.digest, DefectLabels: labels, Candidates: len(commits), Unidentified: unknown,
		Traced: traced, TracedNone: none, CouldNotTrace: unavailable, MergedPRCount: source.MergedPRCount,
		TierComposition: corpus.tiers, MinedAt: at}
	run.TraceHashes = map[string]string{}
	for _, trace := range corpus.Traces {
		run.FixCommits = append(run.FixCommits, trace.FixCommit)
		digest, err := traceHash(trace)
		if err != nil {
			return err
		}
		run.TraceHashes[trace.FixCommit] = digest
	}
	// Completion marker last: readers ignore metrics from an unfinished pass.
	return store.Append(KindMetric, run)
}

func traceHash(trace DefectTrace) (string, error) {
	raw, err := json.Marshal(trace)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
