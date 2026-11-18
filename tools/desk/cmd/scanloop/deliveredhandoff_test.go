package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/issueflow"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6507
func TestDeliveredRepairHandoff(t *testing.T) {
	for _, tc := range []struct {
		name, route, body, evidence, successor string
		delivered, merged, want                bool
	}{
		{"merged repair", "qualification", "Issue: #77", "#88", "#77", true, true, true},
		{"merged resolving repair", "qualification", "Fixes: example-org/tracker#77", "example-org/tracker#88", "#77", true, true, true},
		{"closed unmerged", "qualification", "Issue: #77", "#88", "#77", true, false, false},
		{"different source", "qualification", "Issue: #78", "#88", "#77", true, true, false},
		{"different closed successor", "qualification", "Issue: #78", "#88", "#78", true, true, false},
		{"foreign source", "qualification", "Fixes: example-org/other#77", "#88", "#77", true, true, false},
		{"unreadable change", "qualification", "Issue: #77", "#89", "#77", true, true, false},
		{"forge-linked repair", "qualification", "", "closure event with a merged change", "#77", true, true, true},
		{"only linked change still open", "qualification", "", "only linked change still open", "#77", true, false, false},
		{"incomplete closure history", "qualification", "", "incomplete closure event", "#77", true, true, false},
		{"no evidence ref", "qualification", "Issue: #77", "source already delivered", "#77", true, true, false},
		{"closed issue as implementation", "repair", "Issue: #77", "#88", "#77", false, true, false},
		{"no delivered judgment", "qualification", "Issue: #77", "#88", "#77", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolatedDeskHome(t)
			if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster+"\nASSAY_REPO_FORGES=example-org/tracker=github\n"), 0600); err != nil {
				t.Fatal(err)
			}
			deskkit.ReloadConfig()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch req.URL.Path {
				case "/repos/example-org/tracker/issues/77":
					json.NewEncoder(w).Encode(map[string]any{"number": 77, "state": "closed", "user": map[string]any{"login": "ada", "id": 2001, "type": "User"}})
				case "/repos/example-org/tracker/issues/78":
					json.NewEncoder(w).Encode(map[string]any{"number": 78, "state": "closed", "user": map[string]any{"login": "ada", "id": 2001, "type": "User"}})
				case "/repos/example-org/tracker/pulls/88":
					json.NewEncoder(w).Encode(map[string]any{"number": 88, "state": "closed", "merged": tc.merged, "body": tc.body})
				case "/graphql":
					nodes := []any{}
					if tc.name == "only linked change still open" {
						nodes = append(nodes, map[string]any{"number": 88, "state": "OPEN", "merged": false, "repository": map[string]any{"nameWithOwner": "example-org/tracker"}})
					}
					if tc.name == "forge-linked repair" || tc.name == "incomplete closure history" {
						nodes = append(nodes, map[string]any{"number": 88, "state": "MERGED", "merged": true, "repository": map[string]any{"nameWithOwner": "example-org/tracker"}})
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
						"timelineItems":                  map[string]any{"nodes": []any{}},
						"closedByPullRequestsReferences": map[string]any{"nodes": nodes, "pageInfo": map[string]any{"hasNextPage": tc.name == "incomplete closure history"}},
					}}}})
				default:
					http.NotFound(w, req)
				}
			}))
			defer server.Close()
			oldBase := forgeAPIBase
			forgeAPIBase = server.URL
			t.Cleanup(func() { forgeAPIBase = oldBase })
			stubIdentity(t, func(string) (string, string, error) { return "issue-loop", "intake-desk", nil }, func(string, string) (string, string, error) { return "fixture", "/fixture", nil })
			h := issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: "example-org/tracker#77", Opened: "2026-08-24T11:50:00Z", Revision: 1, DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: tc.route, Delivered: tc.delivered, Successor: tc.successor, Evidence: tc.evidence}
			raw, err := json.Marshal(h)
			if err != nil {
				t.Fatal(err)
			}
			file := writeTemp(t, "judgment.json", string(raw))
			args := []string{"--item", h.Item, "--opened", h.Opened, "--exit", "placeholder", "--artifact", "#77", "--handoff", file, "--handoff-reader", issueflow.IntakeHandoffSchema, "--tier", "strong"}
			for range 2 {
				err := cmdLand(args, "intake-desk", &strings.Builder{})
				if tc.successor != "#77" && (!deskkit.IsRefused(err) || !strings.Contains(err.Error(), "successor is not open")) {
					t.Fatalf("closed successor of a different item must be refused by the scope guard: %v", err)
				}
				if (err == nil) != tc.want {
					t.Fatalf("land err=%v, want accepted=%v", err, tc.want)
				}
			}
			records, err := readHandoffRecords(filepath.Join(dir, handoffRecordsFile))
			if err != nil {
				t.Fatal(err)
			}
			if tc.want {
				if len(records) != 1 || records[0].Route != "qualification" {
					t.Fatalf("recorded judgments: %+v", records)
				}
				result := issueflow.AssessIntakeHandoff(records[0], issueflow.IntakeFacts{Item: h.Item, Opened: h.Opened, Complete: true})
				if result.State != "observed" || result.DirectRepair {
					t.Fatalf("unexpected scheduling assessment: %+v", result)
				}
			} else if len(records) != 0 {
				t.Fatalf("refused handoff recorded: %+v", records)
			}
		})
	}
}

type repairEvidenceForge struct {
	deskkit.Forge
	change  deskkit.PullRequest
	history deskkit.IssueStateHistory
}

func (f *repairEvidenceForge) IssueStateEvents(deskkit.ForgeRepo, int) (*deskkit.IssueStateHistory, error) {
	return &f.history, nil
}

func (f *repairEvidenceForge) GetPullRequest(_ deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error) {
	if number != f.change.Number {
		return nil, deskkit.Unverifiable("missing change", nil)
	}
	return &f.change, nil
}

// regression: #6507
func TestRepairEvidenceIdentityAndLatestReopen(t *testing.T) {
	source := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	const (
		prURL  = "https://github.com/example-org/tracker/pull/88"
		before = "2026-08-24T10:00:00Z"
		reopen = "2026-08-24T11:00:00Z"
		after  = "2026-08-24T12:00:00Z"
	)
	for _, tc := range []struct {
		name, evidence, state, mergedAt string
		reopened, want                  bool
	}{
		{"explicit PR URL", prURL, "closed", after, false, true},
		{"wrong PR URL with another merged closer", "https://github.com/example-org/tracker/pull/89", "closed", after, false, false},
		{"foreign short-form ref", "example-org/other#88", "closed", after, false, false},
		{"foreign PR URL", "https://github.com/example-org/other/pull/88", "closed", after, false, false},
		{"other forge same PR coordinates", "https://another.example/example-org/tracker/pull/88", "closed", after, false, false},
		{"issue URL with same number", "https://github.com/example-org/tracker/issues/88", "closed", after, false, false},
		{"run URL with merged closer", "https://github.com/example-org/tracker/actions/runs/88", "closed", after, false, false},
		{"wrong typed change", "example-org/tracker!89", "closed", after, false, false},
		{"typed merge request", "example-org/tracker!88", "closed", after, false, true},
		{"open with merge signals", "#88", "open", after, false, false},
		{"missing state with merge flag", "#88", "", "", false, false},
		{"reopen with undated closer", "closed by the repair", "closed", after, true, false},
		{"previous repair after reopening", "#88", "closed", before, true, false},
		{"unreadable merge timestamp", "#88", "closed", "unreadable", true, false},
		{"same second as reopen", "#88", "closed", reopen, true, false},
		{"undated repair after reopening", "#88", "closed", "", true, false},
		{"dated repair after reopening", "#88", "closed", after, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &repairEvidenceForge{
				change:  deskkit.PullRequest{Number: 88, URL: prURL, Body: "Issue: #77", State: tc.state, Merged: true, MergedAt: tc.mergedAt},
				history: deskkit.IssueStateHistory{Complete: true, ClosingChanges: []deskkit.ClosingChange{{Repo: source.Slug(), Number: 88, Merged: true}}},
			}
			if tc.reopened {
				f.history.Events = []deskkit.IssueStateEvent{
					{Kind: deskkit.IssueEventReopened, CreatedAt: reopen},
					{Kind: deskkit.IssueEventReopened, CreatedAt: before},
				}
			}
			err := checkMergedRepair(f, source, 77, tc.evidence)
			if (err == nil) != tc.want {
				t.Errorf("accepted=%v, want %v; error=%v", err == nil, tc.want, err)
			}
		})
	}
}
