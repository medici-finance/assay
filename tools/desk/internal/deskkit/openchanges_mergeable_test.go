package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Exercise both real bulk readers: the board must receive policy and conflict
// verdicts from the same change node, without a follow-up PR read.
func TestOpenChangesCarryIndependentMergeable(t *testing.T) {
	for _, snapshot := range []bool{false, true} {
		for _, mergeable := range []string{"MERGEABLE", "CONFLICTING", "UNKNOWN", ""} {
			t.Run(map[bool]string{false: "list", true: "snapshot"}[snapshot]+"/"+mergeable, func(t *testing.T) {
				calls := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var req struct {
						Query string `json:"query"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					if !strings.Contains(req.Query, "mergeStateStatus mergeable headRefOid") {
						t.Error("bulk query omitted separate mergeable verdict")
					}
					node := map[string]any{"number": 7, "state": "OPEN", "isDraft": true, "mergeStateStatus": "BLOCKED", "headRefOid": "abc",
						"reviews": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{}}}
					if mergeable != "" {
						node["mergeable"] = mergeable
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"visibility": "PUBLIC", "pullRequests": map[string]any{
						"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{node},
					}}}})
				}))
				defer srv.Close()
				f := &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}
				var got OpenChange
				if snapshot {
					q, e := f.ReviewQueueSnapshot(pagedRepo)
					if e != nil {
						t.Fatal(e)
					}
					if len(q.Changes) != 1 {
						t.Fatal("missing change")
					}
					got = q.Changes[0].OpenChange
				} else {
					q, e := f.ListOpenChanges(pagedRepo)
					if e != nil {
						t.Fatal(e)
					}
					if len(q.Changes) != 1 {
						t.Fatal("missing change")
					}
					got = q.Changes[0]
				}
				if got.MergeStateStatus != "BLOCKED" || got.Mergeable != mergeable {
					t.Fatalf("lost merge verdicts: %+v", got)
				}
				if calls != 1 {
					t.Fatalf("made %d reads, want one bulk read", calls)
				}
			})
		}
	}
}
