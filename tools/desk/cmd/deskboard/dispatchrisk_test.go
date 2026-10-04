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

// riskFixture is one classifier input: a repo visibility, a PR body and author, an
// optional owning brief, the changed-file read, and the review/CI/draft state around it.
type riskFixture struct {
	visibility, author, body, brief, path, verdict string
	unreadable, truncated, notDraft                bool
	ci                                             string // "", "pending" or "failed"
}

// classifyRiskFixture runs the real classifier over one fixture and returns its row.
func classifyRiskFixture(t *testing.T, fx riskFixture) *actionRow {
	t.Helper()
	roster := strings.Replace(classDegradeRoster, ":ci:private", ":ci:"+fx.visibility, 1)
	// A trusted role App, so the trailer-absent App term reaches the classifier.
	roster = strings.Replace(roster, "gl-reviewer:41987965", "gl-reviewer:41987965,worker=github:sample-worker-app:9001", 1)
	installBoardRoster(t, roster)
	root := t.TempDir()
	t.Setenv(deskkit.RootsEnv, cfRepo+"="+root)
	if fx.brief != "" {
		dir := filepath.Join(root, "docs", "streams", "sample")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief-01-sample.md"), []byte(fx.brief), 0644); err != nil {
			t.Fatal(err)
		}
	}
	head := strings.Repeat("a", 40)
	p := greenRollupPR(42, head)
	p.IsDraft, p.Body = !fx.notDraft, fx.body
	if fx.author != "" {
		p.Author.Login = fx.author
	}
	switch fx.ci {
	case "pending":
		p.StatusCheckRollup[0].Status = "IN_PROGRESS"
		p.StatusCheckRollup[0].Conclusion = ""
	case "failed":
		p.StatusCheckRollup[0].Conclusion = "FAILURE"
	}
	stubForgeHooks(t, forgeHookSet{
		reviews: func(string, int) ([]deskkit.Review, error) {
			if fx.verdict == "" {
				return nil, nil
			}
			sha := head
			if fx.verdict == "APPROVED" {
				sha = strings.Repeat("b", 40)
			}
			return []deskkit.Review{{Author: deskkit.Account{Login: "gl-reviewer", ID: 41987965}, State: fx.verdict, CommitID: sha, SubmittedAt: "2026-09-14T16:00:00Z"}}, nil
		},
		getPR: func(string, int) (*deskkit.PullRequest, error) {
			count := 1
			if fx.truncated {
				count = 2
			}
			return &deskkit.PullRequest{ChangedFiles: count}, nil
		},
		changedFiles: func(string, int) ([]deskkit.ChangedFile, error) {
			if fx.unreadable {
				return nil, errors.New("unreadable fixture")
			}
			return []deskkit.ChangedFile{{Filename: fx.path}}, nil
		},
	})
	out, err := classifyPR(cfRepo, p, false, nil, nil, nil, time.Now())
	if err != nil || out.row == nil {
		t.Fatalf("classifier: %v row=%v", err, out.row)
	}
	if out.row.Action == actFlip || out.row.Action == actMergeNow {
		t.Fatalf("unapproved row cleared: %s", out.row.Action)
	}
	return out.row
}

// Classification feeds dispatch before any verdict. Exercise the real classifier
// across each risk input, with a clean private control and stale verdicts.
func TestDispatchRiskBeforeVerdict(t *testing.T) {
	for _, tc := range []struct {
		name string
		fx   riskFixture
		want bool
	}{
		{"human brief", riskFixture{visibility: "private", body: "Brief: sample/01", brief: gateHumanBrief, path: "README.md"}, true},
		{"public", riskFixture{visibility: "public", body: "Issue: #1", path: "README.md"}, true},
		{"unknown visibility", riskFixture{visibility: "", body: "Issue: #1", path: "README.md"}, true},
		{"security path", riskFixture{visibility: "private", body: "Issue: #1", path: "secrets/auth/token.go"}, true},
		{"missing brief", riskFixture{visibility: "private", body: "Brief: sample/01", path: "README.md"}, true},
		{"unreadable diff", riskFixture{visibility: "private", body: "Issue: #1", unreadable: true}, true},
		{"truncated diff", riskFixture{visibility: "private", body: "Issue: #1", path: "README.md", truncated: true}, true},
		{"clean private", riskFixture{visibility: "private", body: "Issue: #1", path: "README.md"}, false},
		{"nonrisk brief", riskFixture{visibility: "private", body: "Brief: sample/01", brief: nonRiskBrief, path: "README.md"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := classifyRiskFixture(t, tc.fx)
			if row.RiskClassed != tc.want {
				t.Errorf("risk=%v want=%v action=%s", row.RiskClassed, tc.want, row.Action)
			}
		})
	}
}

// Every risk term must fire under every review, CI and draft state. The
// NON-visibility terms run on a PRIVATE repo so the visibility term cannot
// short-circuit them. A term gated on CI, draft or a verdict goes red here for that
// state; the clean control proves the state alone does not risk-class the row.
func TestRiskTermsUngated(t *testing.T) {
	const priv = "private"
	terms := []struct {
		name string
		fx   riskFixture
		want bool
	}{
		{"public", riskFixture{visibility: "public", body: "Issue: #1", path: "README.md"}, true},
		{"unknown visibility", riskFixture{visibility: "", body: "Issue: #1", path: "README.md"}, true},
		{"security path", riskFixture{visibility: priv, body: "Issue: #1", path: "secrets/auth/token.go"}, true},
		{"human brief", riskFixture{visibility: priv, body: "Brief: sample/01", brief: gateHumanBrief, path: "README.md"}, true},
		{"missing brief", riskFixture{visibility: priv, body: "Brief: sample/01", path: "README.md"}, true},
		{"unreadable diff", riskFixture{visibility: priv, body: "Issue: #1", unreadable: true}, true},
		{"truncated diff", riskFixture{visibility: priv, body: "Issue: #1", path: "README.md", truncated: true}, true},
		{"app no trailer", riskFixture{visibility: priv, author: "app/sample-worker-app", path: "README.md"}, true},
		{"clean control", riskFixture{visibility: priv, body: "Issue: #1", path: "README.md"}, false},
	}
	states := []struct {
		name  string
		apply func(*riskFixture)
	}{
		{"green draft", func(*riskFixture) {}},
		{"pending CI", func(fx *riskFixture) { fx.ci = "pending" }},
		{"failed CI", func(fx *riskFixture) { fx.ci = "failed" }},
		{"not draft", func(fx *riskFixture) { fx.notDraft = true }},
		{"stale approval", func(fx *riskFixture) { fx.verdict = "APPROVED" }},
		{"blocking verdict", func(fx *riskFixture) { fx.verdict = "CHANGES_REQUESTED" }},
	}
	for _, term := range terms {
		for _, st := range states {
			t.Run(term.name+"/"+st.name, func(t *testing.T) {
				fx := term.fx
				st.apply(&fx)
				row := classifyRiskFixture(t, fx)
				if row.RiskClassed != term.want {
					t.Errorf("risk=%v want=%v action=%s", row.RiskClassed, term.want, row.Action)
				}
			})
		}
	}
}
