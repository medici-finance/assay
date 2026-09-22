package deskkit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forge_listchanges_test.go — the behaviour contract for ListChanges (the states-scoped,
// bounded, cursor-paginated changes read behind the phantom / already-represented check) and the
// deskkit.RepresentedPRRefs reduction wired on top of it.

// ghListChangesServer serves the paginated GraphQL read. It routes on the `after` cursor in the
// request variables: an absent/null cursor is page 1. Each entry in `pages` is one page's
// response nodes; hasNextPage is true for every page but the last unless alwaysMore is set (the
// truncation case, where the forge keeps advertising more past the client's ceiling).
type ghListChangesServer struct {
	pages       [][]map[string]any
	alwaysMore  bool
	seenStates  []any // the `states` variable of the FIRST request, for the states-scope assertion
	pageRequest int
}

func (s *ghListChangesServer) forge(t *testing.T) *GitHubForge {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var in struct {
			Variables struct {
				After  *string `json:"after"`
				States []any   `json:"states"`
			} `json:"variables"`
		}
		_ = json.Unmarshal(body, &in)
		if s.pageRequest == 0 {
			s.seenStates = in.Variables.States
		}
		idx := s.pageRequest
		s.pageRequest++
		if idx >= len(s.pages) {
			idx = len(s.pages) - 1
		}
		nodes := s.pages[idx]
		hasNext := s.alwaysMore || idx < len(s.pages)-1
		resp := map[string]any{"data": map[string]any{"repository": map[string]any{
			"pullRequests": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": hasNext, "endCursor": "cursor-after-page"},
				"nodes":    nodes,
			},
		}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return &GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()}
}

func node(number int, state, body string) map[string]any {
	return map[string]any{
		"number": number, "state": state, "headRefOid": "sha-" + state,
		"headRefName": "feat/x", "title": "t", "body": body, "mergedAt": "",
	}
}

// TestListChangesGitHubWalksPagesToTheEnd: a two-page population is walked to exhaustion and
// returned whole, NOT incomplete — the walk stops on hasNextPage=false, not on a short page.
func TestListChangesGitHubWalksPagesToTheEnd(t *testing.T) {
	s := &ghListChangesServer{pages: [][]map[string]any{
		{node(10, "OPEN", "Brief: example-a/00")},
		{node(11, "MERGED", "Brief: example-b/01")},
	}}
	cl, err := s.forge(t).ListChanges(forgeTestRepo, OpenAndMerged())
	if err != nil {
		t.Fatalf("ListChanges: %v", err)
	}
	if len(cl.Changes) != 2 {
		t.Fatalf("want both pages' changes (2), got %d: %+v", len(cl.Changes), cl.Changes)
	}
	if cl.Incomplete {
		t.Errorf("a population read to hasNextPage=false is COMPLETE, not incomplete: %+v", cl)
	}
	if cl.Changes[1].State != "MERGED" || cl.Changes[1].MergedAt != "" {
		t.Errorf("merged state must survive the read distinct from closed: %+v", cl.Changes[1])
	}
	// The states variable must carry EXACTLY the requested set (open+merged), never a whole-repo scan.
	got := make([]string, 0, len(s.seenStates))
	for _, v := range s.seenStates {
		got = append(got, v.(string))
	}
	if strings.Join(got, ",") != "OPEN,MERGED" {
		t.Errorf("the query must request states [OPEN MERGED], got %v", got)
	}
}

// TestListChangesGitHub_ReportsIncompleteAtCeiling is the FAIL-FIRST proof of the truncation
// contract: a forge that keeps advertising more past the page ceiling must come back
// Incomplete=true (never a silent partial). Pre-fix, the walk that ignored hasNextPage at the
// ceiling — or dropped the Incomplete flag — returns Incomplete=false and reddens this test.
func TestListChangesGitHub_ReportsIncompleteAtCeiling(t *testing.T) {
	s := &ghListChangesServer{
		pages:      [][]map[string]any{{node(1, "OPEN", "Brief: example-a/00")}},
		alwaysMore: true,
	}
	cl, err := s.forge(t).ListChanges(forgeTestRepo, OpenAndMerged())
	if err != nil {
		t.Fatalf("ListChanges: %v", err)
	}
	if !cl.Incomplete {
		t.Fatalf("a read that hit the page ceiling with the forge still paginating must be Incomplete: %+v", cl)
	}
	if cl.PageCap != forgeListChangesMaxPages {
		t.Errorf("PageCap must report the ceiling %d, got %d", forgeListChangesMaxPages, cl.PageCap)
	}
	if s.pageRequest != forgeListChangesMaxPages {
		t.Errorf("the walk must stop at the ceiling (%d pages), made %d requests", forgeListChangesMaxPages, s.pageRequest)
	}
}

// TestListChangesRefusesNoStates: a states-less request is refused before any request is built —
// the state set must be STATED, never defaulted to a whole-repo scan.
func TestListChangesRefusesNoStates(t *testing.T) {
	s := &ghListChangesServer{pages: [][]map[string]any{{}}}
	if _, err := s.forge(t).ListChanges(forgeTestRepo, ChangeStates{}); err == nil {
		t.Fatal("ListChanges with no states requested must be refused")
	} else if ExitCodeOf(err) != ExitUnverifiable {
		t.Errorf("a states-less request is could-not-check (exit %d), got %d", ExitUnverifiable, ExitCodeOf(err))
	}
	if s.pageRequest != 0 {
		t.Errorf("no request may be built for a states-less read, made %d", s.pageRequest)
	}
}

// listChangesForge is a fake Forge that answers ONLY ListChanges (every other method promotes
// off the embedded nil interface and would panic if called) — enough to exercise
// RepresentedPRRefs without a live forge.
type listChangesForge struct {
	Forge
	cl *ChangeList
}

func (f listChangesForge) ListChanges(ForgeRepo, ChangeStates) (*ChangeList, error) { return f.cl, nil }

// TestRepresentedPRRefs_ReducesChangeListToPRRefs proves the seam→reconciliation reduction: the
// ChangeList maps to []PRRef (number, state, body) and the incomplete flag propagates unchanged.
func TestRepresentedPRRefs_ReducesChangeListToPRRefs(t *testing.T) {
	f := listChangesForge{cl: &ChangeList{
		Incomplete: true,
		Changes: []ChangeRef{
			{Number: 700, State: "MERGED", Body: "Brief: example-b/08"},
			{Number: 303, State: "OPEN", Body: "Brief: example-a/00"},
		},
	}}
	refs, incomplete, err := RepresentedPRRefs(f, forgeTestRepo)
	if err != nil {
		t.Fatalf("RepresentedPRRefs: %v", err)
	}
	if !incomplete {
		t.Error("the ChangeList's Incomplete flag must propagate to the caller")
	}
	if len(refs) != 2 || refs[0].Number != 700 || refs[0].State != "MERGED" || refs[0].Body != "Brief: example-b/08" {
		t.Fatalf("PRRef reduction wrong: %+v", refs)
	}
	// It must feed the reconciliation: a merged brief is represented.
	if n, ok := BriefRepresentedPR("example-b/08", refs); !ok || n != 700 {
		t.Errorf("reduced refs must drive BriefRepresentedPR: got (%d,%v)", n, ok)
	}
}
