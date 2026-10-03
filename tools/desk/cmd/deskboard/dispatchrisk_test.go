package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Classification feeds dispatch before any verdict. Exercise the real classifier
// across each risk input, with a clean private control and stale verdicts.
func TestDispatchRiskBeforeVerdict(t *testing.T) {
	for _, tc := range []struct {
		name, visibility, body, brief, path, verdict string
		unreadable, truncated, want                  bool
	}{
		{name: "human brief", visibility: "private", body: "Brief: sample/01", brief: gateHumanBrief, path: "README.md", want: true},
		{name: "public", visibility: "public", body: "Issue: #1", path: "README.md", want: true},
		{name: "unknown visibility", visibility: "", body: "Issue: #1", path: "README.md", want: true},
		{name: "pending CI", visibility: "public", body: "Issue: #1", path: "README.md", want: true},
		{name: "failed CI", visibility: "public", body: "Issue: #1", path: "README.md", want: true},
		{name: "ready draft flag", visibility: "public", body: "Issue: #1", path: "README.md", want: true},
		{name: "security path", visibility: "private", body: "Issue: #1", path: "secrets/auth/token.go", want: true},
		{name: "missing brief", visibility: "private", body: "Brief: sample/01", path: "README.md", want: true},
		{name: "unreadable diff", visibility: "private", body: "Issue: #1", unreadable: true, want: true},
		{name: "truncated diff", visibility: "private", body: "Issue: #1", path: "README.md", truncated: true, want: true},
		{name: "clean private", visibility: "private", body: "Issue: #1", path: "README.md"},
		{name: "nonrisk brief", visibility: "private", body: "Brief: sample/01", brief: nonRiskBrief, path: "README.md"},
		{name: "stale verdict", visibility: "public", body: "Issue: #1", path: "README.md", verdict: "APPROVED", want: true},
		{name: "blocking verdict", visibility: "public", body: "Issue: #1", path: "README.md", verdict: "CHANGES_REQUESTED", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installBoardRoster(t, strings.Replace(classDegradeRoster, ":ci:private", ":ci:"+tc.visibility, 1))
			root := t.TempDir()
			t.Setenv(deskkit.RootsEnv, cfRepo+"="+root)
			if tc.brief != "" {
				dir := filepath.Join(root, "docs", "streams", "sample")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "brief-01-sample.md"), []byte(tc.brief), 0644); err != nil {
					t.Fatal(err)
				}
			}
			head := strings.Repeat("a", 40)
			p := greenRollupPR(42, head)
			p.IsDraft, p.Body = true, tc.body
			if tc.name == "pending CI" {
				p.StatusCheckRollup[0].Status = "IN_PROGRESS"
				p.StatusCheckRollup[0].Conclusion = ""
			}
			if tc.name == "failed CI" {
				p.StatusCheckRollup[0].Conclusion = "FAILURE"
			}
			if tc.name == "ready draft flag" {
				p.IsDraft = false
			}
			stubForgeHooks(t, forgeHookSet{
				reviews: func(string, int) ([]deskkit.Review, error) {
					if tc.verdict == "" {
						return nil, nil
					}
					sha := head
					if tc.verdict == "APPROVED" {
						sha = strings.Repeat("b", 40)
					}
					return []deskkit.Review{{Author: deskkit.Account{Login: "gl-reviewer", ID: 41987965}, State: tc.verdict, CommitID: sha, SubmittedAt: "2026-09-14T16:00:00Z"}}, nil
				},
				getPR: func(string, int) (*deskkit.PullRequest, error) {
					count := 1
					if tc.truncated {
						count = 2
					}
					return &deskkit.PullRequest{ChangedFiles: count}, nil
				},
				changedFiles: func(string, int) ([]deskkit.ChangedFile, error) {
					if tc.unreadable {
						return nil, errors.New("unreadable fixture")
					}
					return []deskkit.ChangedFile{{Filename: tc.path}}, nil
				},
			})
			out, err := classifyPR(cfRepo, p, false, nil, nil, nil, time.Now())
			if err != nil || out.row == nil {
				t.Fatalf("classifier: %v row=%v", err, out.row)
			}
			if out.row.RiskClassed != tc.want {
				t.Errorf("risk=%v want=%v action=%s", out.row.RiskClassed, tc.want, out.row.Action)
			}
			if out.row.Action == actFlip || out.row.Action == actMergeNow {
				t.Fatalf("unapproved row cleared: %s", out.row.Action)
			}
		})
	}
}
