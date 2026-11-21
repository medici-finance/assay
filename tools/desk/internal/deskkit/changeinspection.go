package deskkit

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// These are source observations for a worker investigating one change. Readiness,
// dispatch and write authority remain with the existing review/merge gates.
type ChangeInspection struct {
	StartedAt         string            `json:"startedAt"`
	Repo              string            `json:"repo"`
	Number            int               `json:"number"`
	Head              string            `json:"head"`
	ExpectedHead      string            `json:"expectedHead,omitempty"`
	State             string            `json:"state"`
	Draft             bool              `json:"draft"`
	Mergeable         string            `json:"mergeable,omitempty"`
	MergeState        string            `json:"mergeState,omitempty"`
	URL               string            `json:"url,omitempty"`
	Reviews           []InspectedReview `json:"reviews"`
	Checks            []InspectedCheck  `json:"checks"`
	ReviewRead        string            `json:"reviewRead"`
	CheckRead         string            `json:"checkRead"`
	ReviewError       string            `json:"reviewError,omitempty"`
	CheckError        string            `json:"checkError,omitempty"`
	InlineDiscussions string            `json:"inlineDiscussions"`
	FinishedAt        string            `json:"finishedAt"`
}

type InspectedReview struct {
	Author      string `json:"author"`
	AuthorID    int64  `json:"authorId,omitempty"`
	State       string `json:"state"`
	Commit      string `json:"commit,omitempty"`
	Binding     string `json:"binding"`
	Lane        string `json:"lane"`
	ID          int64  `json:"id,omitempty"`
	URL         string `json:"url,omitempty"`
	SubmittedAt string `json:"submittedAt,omitempty"`
	Body        string `json:"body"`
}

type InspectedCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	State      string `json:"state,omitempty"`
	Result     string `json:"result"`
	ID         string `json:"checkId,omitempty"`
	JobID      string `json:"jobId,omitempty"`
	RunID      string `json:"runId,omitempty"`
	URL        string `json:"url,omitempty"`
}

type observedCheck struct {
	CICheck
	state             string
	id, job, run, url string
}

type changeInspectionSource interface {
	GetPullRequest(ForgeRepo, int) (*PullRequest, error)
	ChecksAtHead(ForgeRepo, string) (*ChecksAtHead, error)
}

// Only an adapter with a bounded inspection read can supply review history.
// Owned wrappers delegate precisely this read; a future backend without it reports unknown.
func inspectionReviews(f changeInspectionSource, repo ForgeRepo, number int) ([]Review, string, error) {
	switch reader := f.(type) {
	case *outboundForge:
		return inspectionReviews(reader.Forge, repo, number)
	case *readOnlyForge:
		return inspectionReviews(reader.inner, repo, number)
	case *GitLabForge:
		reviews, err := reader.ReviewsAtHead(repo, number)
		return reviews, "", err
	case interface {
		InspectionReviewWindow(ForgeRepo, int) ([]Review, string, error)
	}:
		return reader.InspectionReviewWindow(repo, number)
	default:
		return nil, "", Unverifiable("bounded review history unavailable for this provider", nil)
	}
}

// InspectChange reads facts for one explicitly supplied review repository, change number
// and observed commit. The three reads are sequential observations, not a transaction.
func InspectChange(f changeInspectionSource, fr ForgeRepo, number int, expected string) (*ChangeInspection, error) {
	if !ValidRepoSlug(fr.Slug()) || number <= 0 || (expected != "" && !IsFullSHA(expected)) {
		return nil, Refused("change: expected an owner/repo, a positive change number and an optional full commit SHA")
	}
	started := time.Now().UTC().Format(time.RFC3339Nano)
	p, err := f.GetPullRequest(fr, number)
	if err != nil {
		return nil, err
	}
	if p == nil || p.Number != number || !IsFullSHA(p.HeadSHA) {
		return nil, Unverifiable("change: requested change number and full head could not be established", nil)
	}
	r := ChangeInspection{StartedAt: started, InlineDiscussions: "not-read", Repo: fr.Slug(), Number: number, Head: p.HeadSHA, State: p.State, Draft: p.Draft,
		Mergeable: p.Mergeable, MergeState: p.MergeStateStatus, URL: p.URL}
	if p.Merged || p.MergedAt != "" {
		r.State = "merged"
	}
	if r.MergeState == "" {
		r.MergeState = p.GitLabMergeStatus
	}
	if expected != "" {
		r.ExpectedHead = expected
		if r.ExpectedHead != p.HeadSHA {
			return nil, Unverifiable(fmt.Sprintf("change: expected %s, observed %s on %s#%d", r.ExpectedHead, r.Head, fr.Slug(), number), nil)
		}
	}
	reviews, omission, err := inspectionReviews(f, fr, number)
	if err != nil {
		r.ReviewRead, r.ReviewError = "unknown", err.Error()
	} else {
		// This operation returns provider submissions; GitLab's historical notes
		// window is bounded. A returned window is not evidence of no older review.
		r.ReviewRead = "provider-window"
		if omission != "" {
			r.ReviewRead, r.ReviewError = "partial", omission
		}
		r.Reviews = make([]InspectedReview, 0, len(reviews))
		for _, v := range reviews {
			binding := "unknown"
			if v.CommitID == r.Head {
				binding = "at-head"
			} else if IsFullSHA(v.CommitID) {
				binding = "other-head"
			}
			lane := "unmarked"
			switch {
			case HasSecurityReviewPass(v.Body) || HasSecurityReviewFail(v.Body):
				lane = "security"
			case v.State == "APPROVED" || v.State == "CHANGES_REQUESTED" || HasCorrectnessVerdictLine(v.Body):
				lane = "correctness"
			}
			r.Reviews = append(r.Reviews, InspectedReview{Author: v.Author.Login, AuthorID: v.Author.ID,
				State: v.State, Commit: v.CommitID, Binding: binding, Lane: lane, ID: v.SubmissionID(),
				URL: v.URL, Body: v.Body, SubmittedAt: v.SubmittedAt})
		}
	}
	checks, err := f.ChecksAtHead(fr, r.Head)
	switch {
	case err != nil:
		r.CheckRead, r.CheckError = "unknown", err.Error()
	case checks == nil:
		r.CheckRead, r.CheckError = "unknown", "no check rollup returned"
	default:
		r.CheckRead = "observed"
		if len(checks.CheckRuns)+len(checks.Statuses) == 0 {
			r.CheckRead = "empty"
		}
		if checks.CheckRunsTotalCount != len(checks.CheckRuns) || checks.StatusTotalCount != len(checks.Statuses) {
			r.CheckRead, r.CheckError = "partial", "provider counts differ from returned entries"
		}
		r.Checks = changeChecks(checks)
	}
	r.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return &r, nil
}

func changeChecks(cs *ChecksAtHead) []InspectedCheck {
	rows := make([]observedCheck, 0, len(cs.Statuses)+len(cs.CheckRuns))
	for _, s := range cs.Statuses {
		rows = append(rows, observedCheck{CICheck: CICheck{Context: s.Context, CreatedAt: s.CreatedAt}, state: strings.ToUpper(s.State), url: s.URL, run: s.RunID})
	}
	for _, c := range cs.CheckRuns {
		rows = append(rows, observedCheck{CICheck: CICheck{Name: c.Name, Status: strings.ToUpper(c.Status), Conclusion: strings.ToUpper(c.Conclusion),
			StartedAt: c.StartedAt, CompletedAt: c.CompletedAt}, id: c.ID, job: c.JobID, run: c.RunID, url: c.URL})
	}
	out := make([]InspectedCheck, 0, len(rows))
	for _, c := range LatestRunPerName(rows, observedCheck.groupKey, observedCheck.recencyKey) {
		pass, pending, fail, unknown := reviewflow.CountChecks([]reviewflow.CheckFact{{Status: c.Status, Conclusion: c.Conclusion, State: c.state}})
		result := "skipped-or-neutral"
		switch {
		case fail > 0:
			result = "failed"
		case pass > 0:
			result = "passed"
		case pending > 0:
			result = "pending"
		case unknown > 0:
			result = "unknown"
		}
		out = append(out, InspectedCheck{Name: c.groupKey(), Status: c.Status, Conclusion: c.Conclusion, State: c.state, Result: result, ID: c.id,
			JobID: c.job, RunID: c.run, URL: c.url})
	}
	return out
}
