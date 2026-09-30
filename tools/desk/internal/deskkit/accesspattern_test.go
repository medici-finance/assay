package deskkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// movingForge is a GitHub fake whose state ADVANCES after every request it answers — the
// shape of a live queue a worker keeps pushing to while a reader walks it. At state version k
// every open change's head is `sha-k` and its reviewer has approved every head up to it, so
// its reviews are APPROVED at sha-1 … sha-k in submission order: at ANY single instant the
// newest review is AT the head. A reader that sees one instant sees that invariant hold; a
// reader stitched from several requests sees the head of one instant and the reviews of a
// later one, and the invariant breaks (a review at a commit newer than the head it read).
//
// frozen stops the advance, so the same fake can compare two reads of ONE instant.
type movingForge struct {
	mu       sync.Mutex
	version  int
	frozen   bool
	requests int
	changes  []int
}

func (m *movingForge) review(n, k int) (id int64, sha, at string) {
	return int64(n*1000 + k), fmt.Sprintf("sha-%d", k), fmt.Sprintf("2026-09-01T00:%02d:00Z", k)
}

func (m *movingForge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	k := m.version
	if !m.frozen {
		defer func() { m.version++ }()
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	if r.Method == http.MethodPost && r.URL.Path == "/graphql" {
		body, _ := io.ReadAll(r.Body)
		withReviews := strings.Contains(string(body), "reviews(first:")
		var nodes []map[string]any
		for _, n := range m.changes {
			node := map[string]any{
				"number": n, "title": "change", "body": "", "state": "OPEN", "isDraft": true,
				"createdAt": "2026-09-01T00:00:00Z", "lastEditedAt": nil,
				"author":           map[string]any{"login": "worker", "__typename": "Bot"},
				"mergeStateStatus": "CLEAN", "headRefOid": fmt.Sprintf("sha-%d", k),
				"headRefName": "feat/x", "baseRefName": "main",
				"labels":  map[string]any{"nodes": []any{}},
				"commits": map[string]any{"nodes": []any{}},
			}
			if withReviews {
				var rv []map[string]any
				for j := 1; j <= k; j++ {
					id, sha, at := m.review(n, j)
					rv = append(rv, map[string]any{
						"databaseId": id, "state": "APPROVED", "body": "Verdict: approve", "submittedAt": at,
						"author": map[string]any{"login": "reviewer", "__typename": "Bot", "databaseId": 42},
						"commit": map[string]any{"oid": sha},
					})
				}
				node["reviews"] = map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": rv}
			}
			nodes = append(nodes, node)
		}
		_ = enc.Encode(map[string]any{"data": map[string]any{"repository": map[string]any{
			"pullRequests": map[string]any{"nodes": nodes}}}})
		return
	}
	var n int
	if _, err := fmt.Sscanf(r.URL.Path, "/repos/o/r/pulls/%d/reviews", &n); err == nil && r.Method == http.MethodGet {
		var rv []map[string]any
		for j := 1; j <= k; j++ {
			id, sha, at := m.review(n, j)
			rv = append(rv, map[string]any{
				"id": id, "state": "APPROVED", "body": "Verdict: approve", "submitted_at": at,
				"user": map[string]any{"login": "reviewer[bot]", "id": 42}, "commit_id": sha,
			})
		}
		if rv == nil {
			rv = []map[string]any{}
		}
		_ = enc.Encode(rv)
		return
	}
	http.NotFound(w, r)
}

// consistentAtHead reports whether a change's newest review sits at the head the change was
// read at — the one-instant invariant movingForge holds at every state version.
func consistentAtHead(head string, reviews []Review) bool {
	return len(reviews) > 0 && reviews[len(reviews)-1].CommitID == head
}

// TestAccessPatternSingleRoundTrip is the freshness claim's dereferencing row: the
// review-queue snapshot answers the whole queue — every change's head AND its reviews — in
// ONE round-trip, so it is one consistent instant even while the forge moves under it.
//
// The control is the read it replaces: ListOpenChanges then ReviewsAtHead per change against
// the SAME moving fake makes 1+N requests and TEARS (a change is paired with reviews from a
// later instant than its head). The fix is the single document, not the fake: a snapshot
// composed of the per-item reads would tear here too.
func TestAccessPatternSingleRoundTrip(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "r"}
	changes := []int{7, 8, 9}

	t.Run("snapshot_is_one_round_trip_and_consistent", func(t *testing.T) {
		fake := &movingForge{version: 1, changes: changes}
		srv := httptest.NewServer(fake)
		defer srv.Close()
		f := &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}

		q, err := f.ReviewQueueSnapshot(repo)
		if err != nil {
			t.Fatalf("ReviewQueueSnapshot: %v", err)
		}
		if fake.requests != 1 {
			t.Fatalf("snapshot made %d requests, want exactly 1 round-trip", fake.requests)
		}
		if len(q.Changes) != len(changes) {
			t.Fatalf("snapshot carried %d changes, want %d", len(q.Changes), len(changes))
		}
		for _, c := range q.Changes {
			if !c.ReviewsComplete {
				t.Errorf("#%d: ReviewsComplete=false on a complete response", c.Number)
			}
			if !consistentAtHead(c.HeadSHA, c.Reviews) {
				t.Errorf("#%d: snapshot TORE — head %s, reviews %+v", c.Number, c.HeadSHA, c.Reviews)
			}
		}
	})

	t.Run("control_sequential_per_item_reads_tear", func(t *testing.T) {
		fake := &movingForge{version: 1, changes: changes}
		srv := httptest.NewServer(fake)
		defer srv.Close()
		f := &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}

		oc, err := f.ListOpenChanges(repo)
		if err != nil {
			t.Fatalf("ListOpenChanges: %v", err)
		}
		torn := 0
		for _, c := range oc.Changes {
			rv, err := f.ReviewsAtHead(repo, c.Number)
			if err != nil {
				t.Fatalf("ReviewsAtHead #%d: %v", c.Number, err)
			}
			if !consistentAtHead(c.HeadSHA, rv) {
				torn++
			}
		}
		if want := 1 + len(changes); fake.requests != want {
			t.Errorf("per-item path made %d requests, want %d (1 list + N review reads)", fake.requests, want)
		}
		if torn == 0 {
			t.Fatal("control did not tear — the moving fake no longer distinguishes one read from N, " +
				"so the snapshot assertion above proves nothing")
		}
	})

	// At ONE frozen instant the snapshot's reviews are exactly what ReviewsAtHead reports —
	// the GraphQL rendering (databaseId, Bot login re-suffixed, commit oid) is the REST one,
	// which is what lets the consumer swap reads with its output unchanged.
	t.Run("snapshot_reviews_equal_per_item_reviews_at_one_instant", func(t *testing.T) {
		fake := &movingForge{version: 3, frozen: true, changes: changes}
		srv := httptest.NewServer(fake)
		defer srv.Close()
		f := &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}

		q, err := f.ReviewQueueSnapshot(repo)
		if err != nil {
			t.Fatalf("ReviewQueueSnapshot: %v", err)
		}
		oc, err := f.ListOpenChanges(repo)
		if err != nil {
			t.Fatalf("ListOpenChanges: %v", err)
		}
		for i, c := range q.Changes {
			if !reflect.DeepEqual(c.OpenChange, oc.Changes[i]) {
				t.Errorf("#%d: snapshot change %+v != ListOpenChanges %+v", c.Number, c.OpenChange, oc.Changes[i])
			}
			rv, err := f.ReviewsAtHead(repo, c.Number)
			if err != nil {
				t.Fatalf("ReviewsAtHead #%d: %v", c.Number, err)
			}
			if !reflect.DeepEqual(c.Reviews, rv) {
				t.Errorf("#%d: snapshot reviews\n%+v\n!= ReviewsAtHead\n%+v", c.Number, c.Reviews, rv)
			}
		}
	})
}

// accessPatternOps is every typed access-pattern op on the Forge seam — an op that combines
// per-item reads into one backend round-trip. Each one must take typed inputs and return a
// typed result; the backend's query document never crosses the interface.
var accessPatternOps = []string{"ReviewQueueSnapshot"}

// rawQueryType reports whether t can carry a caller-authored query: a string or []byte, an
// interface (any value), or a map (free-form variables). Named domain types built on these
// (ForgeRepo's fields are strings) are checked at the parameter level, not recursed into.
func rawQueryType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Interface, reflect.Map:
		return true
	case reflect.Slice:
		return t.Elem().Kind() == reflect.Uint8 || t.Elem().Kind() == reflect.String
	}
	return false
}

// resultCarriesRawQuery walks a result type's fields and returns the path of any field whose
// NAME says it carries a query document back out (Query / GraphQL / Document / Raw) or whose
// type is free-form (interface / map / json.RawMessage) — the seam's results are data, not the
// read that produced them.
func resultCarriesRawQuery(t reflect.Type, path string, seen map[reflect.Type]bool) string {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return ""
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		fld := t.Field(i)
		p := path + "." + fld.Name
		name := strings.ToLower(fld.Name)
		for _, bad := range []string{"query", "graphql", "document", "raw"} {
			if strings.Contains(name, bad) {
				return p
			}
		}
		ft := fld.Type
		if ft.Kind() == reflect.Interface || ft.Kind() == reflect.Map || ft == reflect.TypeOf(json.RawMessage(nil)) {
			return p
		}
		if hit := resultCarriesRawQuery(ft, p, seen); hit != "" {
			return hit
		}
	}
	return ""
}

// TestForgeNoRawQueryInSignature is the "no raw query crosses the seam" row: every
// access-pattern op on Forge takes exactly a ForgeRepo (typed input — no string, map or
// interface a caller could put a GraphQL document or variables in) and returns a typed
// pointer-to-struct result plus error, and the result carries no query-shaped field. A
// missing op fails too, so the test cannot pass by the op being renamed away.
func TestForgeNoRawQueryInSignature(t *testing.T) {
	forge := reflect.TypeOf((*Forge)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	repoType := reflect.TypeOf(ForgeRepo{})

	for _, name := range accessPatternOps {
		m, ok := forge.MethodByName(name)
		if !ok {
			t.Errorf("Forge has no access-pattern op %s", name)
			continue
		}
		mt := m.Type // interface method: no receiver in In
		if mt.NumIn() != 1 || mt.In(0) != repoType {
			var ins []string
			for i := 0; i < mt.NumIn(); i++ {
				ins = append(ins, mt.In(i).String())
			}
			t.Errorf("%s takes (%s), want exactly (ForgeRepo) — typed inputs only", name, strings.Join(ins, ", "))
		}
		for i := 0; i < mt.NumIn(); i++ {
			if rawQueryType(mt.In(i)) {
				t.Errorf("%s parameter %d is %s — a raw query/variables channel across the seam", name, i, mt.In(i))
			}
		}
		if mt.NumOut() != 2 || mt.Out(1) != errType {
			t.Errorf("%s returns %s, want (typed result, error)", name, mt)
			continue
		}
		res := mt.Out(0)
		if res.Kind() != reflect.Ptr || res.Elem().Kind() != reflect.Struct {
			t.Errorf("%s result is %s, want a pointer to a typed struct", name, res)
			continue
		}
		if hit := resultCarriesRawQuery(res, res.Elem().Name(), map[reflect.Type]bool{}); hit != "" {
			t.Errorf("%s result field %s carries a query-shaped or free-form value out of the backend", name, hit)
		}
	}
}
