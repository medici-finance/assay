package deskkit

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// GitHub can provide one document. GitLab declares the REST population incomplete:
// bounded requests and equal stable population must not be confused with atomic reviews.
func TestReadPatternBackends(t *testing.T) {
	repo := ForgeRepo{Owner: "example", Name: "repo"}
	t.Run("github", func(t *testing.T) {
		fake := &movingForge{version: 3, frozen: true, changes: []int{7, 8}}
		srv := httptest.NewServer(fake)
		defer srv.Close()
		f := &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: srv.Client()}
		q, err := f.ReviewQueueSnapshot(repo)
		if err != nil {
			t.Fatal(err)
		}
		if fake.requests != 1 || len(q.Changes) != 2 {
			t.Fatalf("requests=%d queue=%+v", fake.requests, q)
		}
		for _, c := range q.Changes {
			if !c.ReviewsComplete || !consistentAtHead(c.HeadSHA, c.Reviews) {
				t.Fatalf("torn/incomplete snapshot: %+v", c)
			}
		}
	})
	t.Run("gitlab", func(t *testing.T) {
		s := newGLServer(t)
		s.mrList = []map[string]any{glMR(map[string]any{"iid": 7, "sha": "example-head"})}
		// A pipeline belonging to a different head cannot provide this queue's verdict.
		s.pipelines = []map[string]any{{"id": 77, "sha": "other-head", "status": "success"}}
		f := s.forge()
		q, err := f.ReviewQueueSnapshot(repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(q.Changes) != 1 || len(s.requests) != 2 {
			t.Fatalf("unbounded/incomplete population: %+v requests=%v", q, s.requests)
		}
		c := q.Changes[0]
		if c.ReviewsComplete || len(c.Reviews) != 0 {
			t.Fatalf("REST population claimed atomic reviews: %+v", c)
		}
		for _, r := range s.requests {
			if strings.Contains(r.Path, "approvals") || strings.Contains(r.Path, "notes") {
				t.Fatalf("snapshot widened into per-review reads: %v", s.requests)
			}
		}
		oc, err := f.ListOpenChanges(repo)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.OpenChange, oc.Changes[0]) {
			t.Fatalf("stable population changed: %+v != %+v", c.OpenChange, oc.Changes[0])
		}
	})
	// The same backend-specific failure boundaries remain part of the access-pattern leg.
	for _, name := range []string{"ce_404_approvals", "free_tier_403", "last_pipeline_empty", "last_pipeline_absent", "x_next_page"} {
		t.Run("gitlab/"+name, func(t *testing.T) { platformContract(t, "gitlab", name) })
	}
}
