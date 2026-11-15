package deskkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// pagedOpenChanges is a fake GraphQL forge holding n open changes, newest first, served by the
// cursor walk's own variables: `first` (or the pre-#3096 `limit`) and `after` (an offset). A
// fault, when set, answers request i (1-based) instead of the page; returning "" serves the page.
type pagedOpenChanges struct {
	mu       sync.Mutex
	n        int
	onePage  bool // ignore first/after and answer every change in one page
	fault    func(i int, first int) string
	requests []string // "first/after" per request
}

func (f *pagedOpenChanges) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if r.Method != http.MethodPost || r.URL.Path != "/graphql" || json.NewDecoder(r.Body).Decode(&req) != nil ||
		!strings.Contains(req.Query, "pullRequests(states:OPEN") {
		http.Error(w, `{"message":"not served"}`, http.StatusNotFound)
		return
	}
	first := 100
	for _, k := range []string{"first", "limit"} {
		if v, ok := req.Variables[k].(float64); ok {
			first = int(v)
		}
	}
	start := 0
	if a, ok := req.Variables["after"].(string); ok {
		start, _ = strconv.Atoi(a)
	}
	f.requests = append(f.requests, fmt.Sprintf("%d/%d", first, start))
	if f.fault != nil {
		switch f.fault(len(f.requests), first) {
		case "504":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte("<html><body>504 Gateway Time-out</body></html>"))
			return
		case "empty":
			w.WriteHeader(http.StatusOK)
			return
		case "truncated":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"repository":{"visibility":"PUBLIC","pullRequests":{"pageInfo":{"hasNextPage":true,"endCursor":"25"},"nodes":[{"number":60,"title":"t`))
			return
		case "cutoff":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", "4096")
			_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequests":{"nodes":[`))
			return
		case "401":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		case "403":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
			return
		case "secondary":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit. Please wait a few minutes before you try again."}`))
			return
		case "429":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"too many requests"}`))
			return
		case "gqlerror":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Something went wrong"}]}`))
			return
		}
	}
	if f.onePage {
		first, start = f.n+1, 0
	}
	end := min(start+first, f.n)
	nodes := []any{}
	for i := start; i < end; i++ {
		num := f.n - i // newest first
		head := fmt.Sprintf("%040x", num)
		nodes = append(nodes, map[string]any{
			"number": num, "title": fmt.Sprintf("change %d", num), "state": "OPEN", "isDraft": num%2 == 0,
			"author":     map[string]any{"login": "ada", "__typename": "User"},
			"headRefOid": head, "headRefName": fmt.Sprintf("b%d", num), "baseRefName": "main",
			"labels":  map[string]any{"nodes": []any{map[string]any{"name": "l" + strconv.Itoa(num%3)}}},
			"reviews": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{map[string]any{"databaseId": num, "author": map[string]any{"login": "rev", "__typename": "Bot", "databaseId": 9}, "state": "APPROVED", "commit": map[string]any{"oid": head}, "body": "ok", "submittedAt": "2026-11-01T00:00:00Z"}}},
			"commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"oid": head, "committedDate": "2026-10-31T00:00:00Z"}}}},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{
		"visibility": "PUBLIC",
		"pullRequests": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": end < f.n, "endCursor": strconv.Itoa(end)},
			"nodes":    nodes,
		},
	}}})
}

func pagedForge(t *testing.T, fake *pagedOpenChanges) *GitHubForge {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return &GitHubForge{Token: "stub", BaseURL: srv.URL, Client: srv.Client()}
}

var pagedRepo = ForgeRepo{Owner: "o", Name: "r"}

// regression: #3096
func TestOpenChangesPagedEqualsOneLargePage(t *testing.T) {
	for _, n := range []int{0, 3, 25, 56, 100, 101, 140} {
		paged := &pagedOpenChanges{n: n}
		one := &pagedOpenChanges{n: n, onePage: true}
		pq, err := pagedForge(t, paged).ReviewQueueSnapshot(pagedRepo)
		if err != nil {
			t.Fatalf("n=%d paged: %v", n, err)
		}
		oq, err := pagedForge(t, one).ReviewQueueSnapshot(pagedRepo)
		if err != nil {
			t.Fatalf("n=%d one page: %v", n, err)
		}
		if !reflect.DeepEqual(pq, oq) {
			t.Fatalf("n=%d: paged snapshot differs from one large page\npaged: %+v\n  one: %+v", n, pq, oq)
		}
		if want := min(n, forgeOpenChangesCap); len(pq.Changes) != want || pq.Cap != forgeOpenChangesCap {
			t.Fatalf("n=%d: %d changes cap %d, want %d cap %d", n, len(pq.Changes), pq.Cap, want, forgeOpenChangesCap)
		}
		if pq.TruncatedAtCap != (n > forgeOpenChangesCap) {
			t.Errorf("n=%d: TruncatedAtCap=%v — a queue bigger than the cap must say so, one at or under it must not", n, pq.TruncatedAtCap)
		}
		if n > 0 && (pq.Visibility != "public" || pq.Changes[0].Number != n || !pq.Changes[0].ReviewsComplete) {
			t.Errorf("n=%d: visibility %q first #%d complete=%v", n, pq.Visibility, pq.Changes[0].Number, pq.Changes[0].ReviewsComplete)
		}
		for _, r := range paged.requests {
			if first, _ := strconv.Atoi(strings.SplitN(r, "/", 2)[0]); first > 25 {
				t.Errorf("n=%d: a page asked for %d changes, above the 25 page size", n, first)
			}
		}
		if want := max(1, (min(n, forgeOpenChangesCap)+24)/25); len(paged.requests) != want {
			t.Errorf("n=%d: %d requests %v, want %d", n, len(paged.requests), paged.requests, want)
		}
		lo, err := pagedForge(t, &pagedOpenChanges{n: n}).ListOpenChanges(pagedRepo)
		if err != nil {
			t.Fatalf("n=%d ListOpenChanges: %v", n, err)
		}
		if len(lo.Changes) != len(pq.Changes) || lo.TruncatedAtCap != pq.TruncatedAtCap {
			t.Errorf("n=%d: ListOpenChanges %d/%v != snapshot %d/%v", n, len(lo.Changes), lo.TruncatedAtCap, len(pq.Changes), pq.TruncatedAtCap)
		}
	}
}

// regression: #3096
func TestOpenChangesTransientPageRetriedSmaller(t *testing.T) {
	for _, shape := range []string{"504", "empty", "truncated", "cutoff"} {
		t.Run(shape+" once then a smaller page completes", func(t *testing.T) {
			fake := &pagedOpenChanges{n: 60, fault: func(i, _ int) string {
				if i == 2 {
					return shape
				}
				return ""
			}}
			q, err := pagedForge(t, fake).ReviewQueueSnapshot(pagedRepo)
			if err != nil {
				t.Fatalf("one transient %s failure must be retried at a smaller page: %v", shape, err)
			}
			if len(q.Changes) != 60 || q.TruncatedAtCap {
				t.Fatalf("got %d changes truncated=%v", len(q.Changes), q.TruncatedAtCap)
			}
			want := []string{"25/0", "25/25", "12/25", "12/37", "12/49"}
			if !reflect.DeepEqual(fake.requests, want) {
				t.Errorf("requests %v, want %v — the SAME cursor retried at half size", fake.requests, want)
			}
		})
		t.Run(shape+" sustained: could-not-check after a pinned request count", func(t *testing.T) {
			fake := &pagedOpenChanges{n: 60, fault: func(int, int) string { return shape }}
			for name, read := range map[string]func(*GitHubForge) error{
				"snapshot": func(f *GitHubForge) error { _, err := f.ReviewQueueSnapshot(pagedRepo); return err },
				"list":     func(f *GitHubForge) error { _, err := f.ListOpenChanges(pagedRepo); return err },
			} {
				fake.requests = nil
				err := read(pagedForge(t, fake))
				if ExitCodeOf(err) != ExitUnverifiable {
					t.Fatalf("%s: exhausted backoff must be could-not-check (exit %d), got exit %d: %v", name, ExitUnverifiable, ExitCodeOf(err), err)
				}
				want := []string{"25/0", "12/0", "6/0"}
				if !reflect.DeepEqual(fake.requests, want) {
					t.Fatalf("%s: requests under a sustained outage %v, want exactly %v", name, fake.requests, want)
				}
				if shape == "504" && !strings.Contains(err.Error(), "HTTP 504") {
					t.Errorf("%s: the forge's own answer must survive in the refusal: %v", name, err)
				}
			}
		})
	}
}

// regression: #3096
func TestOpenChangesRefusalsAreNotRetried(t *testing.T) {
	for _, shape := range []string{"401", "403", "secondary", "429", "gqlerror"} {
		fake := &pagedOpenChanges{n: 60, fault: func(int, int) string { return shape }}
		_, err := pagedForge(t, fake).ReviewQueueSnapshot(pagedRepo)
		if err == nil {
			t.Fatalf("%s: a refusal read as success", shape)
		}
		if len(fake.requests) != 1 {
			t.Errorf("%s: a refusal was retried: %v", shape, fake.requests)
		}
		switch shape {
		case "secondary", "429":
			if !IsForgeRateLimited(err) {
				t.Errorf("%s: the rate-limit answer must stay recognisable: %v", shape, err)
			}
		case "403":
			if !IsForgeForbidden(err) || IsForgeRateLimited(err) {
				t.Errorf("403: must stay a plain forbidden: %v", err)
			}
		}
	}
	// A refusal on a LATER page is equally final, and no partial set is handed back.
	fake := &pagedOpenChanges{n: 60, fault: func(i, _ int) string {
		if i == 2 {
			return "403"
		}
		return ""
	}}
	q, err := pagedForge(t, fake).ListOpenChanges(pagedRepo)
	if err == nil || q != nil || len(fake.requests) != 2 {
		t.Fatalf("a mid-walk refusal must fail the whole read: q=%v err=%v requests=%v", q, err, fake.requests)
	}
}
