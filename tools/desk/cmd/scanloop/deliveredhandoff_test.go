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
		name, route, body, evidence string
		delivered, merged, want     bool
	}{
		{"merged repair", "qualification", "Issue: #77", "#88", true, true, true},
		{"merged resolving repair", "qualification", "Fixes: example-org/tracker#77", "example-org/tracker#88", true, true, true},
		{"closed unmerged", "qualification", "Issue: #77", "#88", true, false, false},
		{"different source", "qualification", "Issue: #78", "#88", true, true, false},
		{"foreign source", "qualification", "Fixes: example-org/other#77", "#88", true, true, false},
		{"unreadable change", "qualification", "Issue: #77", "#89", true, true, false},
		{"forge-linked repair", "qualification", "", "closure event with a merged change", true, true, true},
		{"incomplete closure history", "qualification", "", "incomplete closure event", true, true, false},
		{"no evidence ref", "qualification", "Issue: #77", "source already delivered", true, true, false},
		{"closed issue as implementation", "repair", "Issue: #77", "#88", false, true, false},
		{"no delivered judgment", "qualification", "Issue: #77", "#88", false, true, false},
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
				case "/repos/example-org/tracker/pulls/88":
					json.NewEncoder(w).Encode(map[string]any{"number": 88, "state": "closed", "merged": tc.merged, "body": tc.body})
				case "/graphql":
					nodes := []any{}
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
			h := issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: "example-org/tracker#77", Opened: "2026-08-24T11:50:00Z", Revision: 1, DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: tc.route, Delivered: tc.delivered, Successor: "#77", Evidence: tc.evidence}
			raw, err := json.Marshal(h)
			if err != nil {
				t.Fatal(err)
			}
			file := writeTemp(t, "judgment.json", string(raw))
			args := []string{"--item", h.Item, "--opened", h.Opened, "--exit", "placeholder", "--artifact", "#77", "--handoff", file, "--handoff-reader", issueflow.IntakeHandoffSchema, "--tier", "strong"}
			for range 2 {
				err := cmdLand(args, "intake-desk", &strings.Builder{})
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
