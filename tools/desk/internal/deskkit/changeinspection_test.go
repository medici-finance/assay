package deskkit

import (
	"fmt"
	"strings"
	"testing"
)

const inspectionHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const inspectionOld = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestInspectChangeGitHubReadCost(t *testing.T) {
	for _, jobs := range []int{1, 32} {
		t.Run(fmt.Sprint(jobs), func(t *testing.T) {
			s := newGoldenServer(t)
			repo := ForgeRepo{Owner: "example-org", Name: "sample"}
			s.pull = map[string]any{"number": 7, "state": "open", "draft": true, "head": map[string]any{"sha": inspectionHead}, "mergeable": true, "mergeable_state": "blocked", "html_url": "https://example.org/example-org/sample/pull/7"}
			const reviewLink = "https://example.org/example-org/sample/pull/7#pullrequestreview-27"
			s.reviews = []map[string]any{
				{"id": 26, "state": "APPROVED", "commit_id": inspectionOld, "body": "Earlier review"},
				{"id": 27, "state": "CHANGES_REQUESTED", "commit_id": inspectionHead, "html_url": reviewLink, "body": ""},
				{"id": 28, "state": "CHANGES_REQUESTED", "body": "Security-Review: fail\nF1: retain the original reason."},
			}
			s.status = map[string]any{"state": "pending", "total_count": 0, "statuses": []any{}}
			checks := make([]map[string]any, 0, jobs)
			for i := 1; i <= jobs; i++ {
				link := strings.Replace(s.srv.URL, "http://", "https://", 1) + fmt.Sprintf("/example-org/sample/actions/runs/90/job/%d", i)
				checks = append(checks, map[string]any{"id": 100 + i, "name": fmt.Sprintf("build-%d", i), "status": "completed", "conclusion": "failure", "app": map[string]string{"slug": "github-actions"}, "details_url": link})
			}
			s.checks = map[string]any{"total_count": jobs, "check_runs": checks}
			r, err := InspectChange(s.forge(), repo, 7, inspectionHead)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.requests) != 4 {
				t.Fatalf("want four HTTP requests for %d checks: %+v", jobs, s.requests)
			}
			if r.ExpectedHead != inspectionHead || r.Head != inspectionHead || r.MergeState != "BLOCKED" || r.CheckRead != "observed" || len(r.Checks) != jobs || r.Checks[0].Result != "failed" || r.Checks[0].ID != "101" || r.Checks[0].RunID != "90" || r.Checks[0].JobID != "1" {
				t.Fatalf("bad subject or checks: %+v", r)
			}
			if r.ReviewRead != "provider-window" || r.InlineDiscussions != "not-read" || len(r.Reviews) != 3 || r.Reviews[0].Binding != "other-head" || r.Reviews[1].Binding != "at-head" || r.Reviews[1].ID != 27 || r.Reviews[1].URL != reviewLink || r.Reviews[1].Body != "" || r.Reviews[2].Lane != "security" || r.Reviews[2].Binding != "unknown" || r.Reviews[2].Body != "Security-Review: fail\nF1: retain the original reason." {
				t.Fatalf("incorrect review evidence: %+v", r.Reviews)
			}
		})
	}
}

func TestInspectChangeRetainsUncertainty(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "sample"}
	s := newGoldenServer(t)
	s.pull = map[string]any{"number": 7, "merged": true, "state": "closed", "head": map[string]any{"sha": inspectionHead}}
	_, err := InspectChange(s.forge(), repo, 7, inspectionOld)
	if err == nil || !strings.Contains(err.Error(), inspectionHead) || len(s.requests) != 1 {
		t.Fatalf("head mismatch: requests=%v err=%v", s.requests, err)
	}
	s.forceStatus["/reviews"] = 403
	s.status = map[string]any{"total_count": 1, "statuses": []any{}}
	s.checks = map[string]any{"total_count": 0}
	r, err := InspectChange(s.forge(), repo, 7, "")
	if err != nil || r.State != "merged" || r.CheckRead != "partial" || r.ReviewRead != "unknown" || r.ReviewError == "" || r.CheckError == "" || r.Reviews != nil {
		t.Fatalf("incomplete evidence: %+v %v", r, err)
	}
}

func TestInspectChangeGitLabReadCost(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "sample"}
	s := newGLServer(t)
	url := "https://example.org/example-org/sample/-/merge_requests/7"
	s.mr = map[string]any{"iid": 7, "state": "opened", "sha": inspectionHead, "changes_count": "1", "detailed_merge_status": "mergeable", "web_url": url}
	s.versions = []map[string]any{{"head_commit_sha": inspectionHead, "created_at": "2026-01-01T00:00:00Z"}}
	s.projApproval = map[string]any{"reset_approvals_on_push": false}
	s.approvals = map[string]any{"approved_by": []any{map[string]any{"user": map[string]any{"id": 14, "username": "reader"}}}}
	s.notes = []map[string]any{{"id": 88, "body": "Verdict: request-changes", "author": map[string]any{"id": 18, "username": "reviewer"}, "created_at": "2026-05-01T00:00:00Z"}}
	s.commit = map[string]any{"id": inspectionHead, "last_pipeline": map[string]any{"id": 77, "sha": inspectionHead, "status": "failed"}}
	s.statuses = []map[string]any{}
	for i := 1; i <= 32; i++ {
		s.jobs = append(s.jobs, map[string]any{"id": i, "name": fmt.Sprintf("build-%d", i), "status": "failed", "web_url": fmt.Sprintf("https://example.org/example-org/sample/-/jobs/%d", i)})
	}
	r, err := InspectChange(s.forge(), repo, 7, inspectionHead)
	if err != nil {
		t.Fatal(err)
	}
	if r.CheckRead != "observed" || len(r.Checks) != 33 || len(r.Reviews) != 2 || r.Reviews[0].ID != 0 || r.Reviews[0].Binding != "unknown" || r.Reviews[1].ID != 88 || r.Reviews[1].URL != url+"#note_88" || r.Reviews[1].Binding != "at-head" {
		t.Fatalf("gitlab evidence: %+v", r)
	}
	var seen bool
	for _, c := range r.Checks {
		if c.Name == "build-32" {
			seen = c.JobID == "32" && c.RunID == "77" && c.URL != "" && c.Result == "failed"
		}
	}
	if !seen || len(s.requests) != 9 {
		t.Fatalf("gitlab named-job evidence=%v; HTTP requests: %+v", seen, s.requests)
	}
}

func TestInspectChangeCheckIdentifiers(t *testing.T) {
	f := &GitHubForge{}
	repo := ForgeRepo{Owner: "example-org", Name: "sample"}
	if run, job := f.actionsJobIDs(repo, "https://github.com/example-org/sample/actions/runs/19"); run != "19" || job != "" {
		t.Fatalf("workflow-only link: run=%q job=%q", run, job)
	}
	for _, u := range []string{"https://third-party.example/example-org/sample/actions/runs/1/job/2", "https://github.com/example-org/other/actions/runs/1/job/2", "https://github.com/example-org/sample/actions/runs/1/job/missing", "https://github.com/example-org/sample/runs/1"} {
		if run, job := f.actionsJobIDs(repo, u); run != "" || job != "" {
			t.Fatalf("unrelated URL became run/job: %q -> %q/%q", u, run, job)
		}
	}
	rows := changeChecks(&ChecksAtHead{CheckRuns: []CheckRun{
		{ID: "17", Name: "build", Status: "completed", Conclusion: "failure", CompletedAt: "2026-02-21T00:00:00Z"},
		{ID: "18", Name: "build", Status: "completed", Conclusion: "success", CompletedAt: "2026-02-22T00:00:00Z"},
	}})
	if len(rows) != 1 || rows[0].ID != "18" || rows[0].Result != "passed" || rows[0].RunID != "" || rows[0].JobID != "" {
		t.Fatalf("older failure or fabricated IDs: %+v", rows)
	}
}
