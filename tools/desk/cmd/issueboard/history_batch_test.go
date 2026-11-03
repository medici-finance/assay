package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// This opaque composition deliberately has no optional history capability.
type serialEventForge struct{ deskkit.Forge }

type historyFixture struct {
	t          *testing.T
	root       string
	count      int
	kind       string
	fault      string
	delay      time.Duration
	queries    int
	lists      int
	trustReads int
	events     []int
	histories  []int
	external   bool
	plain      bool
	duplicate  bool
	paths      []string
	resolve    func(string, ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error)
}

func newHistoryFixture(t *testing.T, count int, kind string) *historyFixture {
	t.Helper()
	resolve := forgeFor
	installForge(t, nil)
	config := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	var forges []string
	for _, repo := range deskkit.ScanRepos() {
		forges = append(forges, repo+"=github")
	}
	roster := filepath.Join(config, "roster.env")
	raw, err := os.ReadFile(roster)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roster, append(raw, []byte("\nASSAY_REPO_FORGES="+strings.Join(forges, ",")+"\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSAY_CONFIG_HOME", config)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	oldMint, oldRole, oldBase, oldTransport := mintTokenFn, sessionRoleFn, forgeAPIBase, http.DefaultTransport
	t.Cleanup(func() {
		mintTokenFn, sessionRoleFn, forgeAPIBase, http.DefaultTransport = oldMint, oldRole, oldBase, oldTransport
	})
	mintTokenFn = func(string, string) (string, string, error) { return "fixture-token", "", nil }
	sessionRoleFn = func(string) (string, string, error) { return "worker", "worker-desk", nil }
	forgeAPIBase = ""
	f := &historyFixture{t: t, root: t.TempDir(), count: count, kind: kind, resolve: resolve}
	http.DefaultTransport = perfWire(f.answer)
	return f
}

func (f *historyFixture) answer(r *http.Request) (*http.Response, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.paths = append(f.paths, r.Method+" "+r.URL.RequestURI())
	var body any
	if strings.HasSuffix(r.URL.Path, "/issues") && r.Method == "GET" {
		f.lists++
		rows := []any{}
		if strings.Contains(r.URL.Path, "/tracker/") && r.URL.Query().Get("page") == "1" {
			for n := 1; n <= f.count; n++ {
				label := "question"
				if f.kind == "addressed" || f.kind == "mixed" && n%2 == 0 {
					label = "to:worker"
				}
				author := "shared-agent"
				if f.external && n == 2 {
					author = "external-user"
				}
				rows = append(rows, map[string]any{"number": n, "title": fmt.Sprintf("required-%d", n), "state": "open", "user": map[string]any{"login": author, "id": 2002}, "labels": []any{map[string]string{"name": label}}, "created_at": "2026-01-01T00:00:00Z"})
			}
			if f.plain {
				rows = append(rows, map[string]any{"number": f.count + 1, "title": "plain-history", "state": "open", "user": map[string]any{"login": "shared-agent", "id": 2002}, "labels": []any{}, "created_at": "2026-01-01T00:00:00Z"})
			}
			if f.duplicate {
				rows = append(rows, rows[0])
			}
		}
		body = rows
	} else if strings.HasSuffix(r.URL.Path, "/timeline") && r.Method == "GET" {
		f.histories = append(f.histories, f.count+1)
		body = []any{}
	} else if r.Method == "POST" && r.URL.Path == "/graphql" {
		f.queries++
		var in struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			f.t.Fatal(err)
		}
		if strings.Contains(in.Query, "mutation") {
			f.t.Fatal("history operation became a write")
		}
		repository := map[string]any{}
		if raw, ok := in.Variables["number"].(float64); ok {
			cursor, _ := in.Variables["after"].(string)
			if strings.Contains(in.Query, "lastEditedAt comments(first:100)") {
				f.trustReads++
				repository["issue"] = map[string]any{"lastEditedAt": nil, "comments": map[string]any{"pageInfo": map[string]bool{"hasNextPage": false}, "nodes": []any{}}}
			} else {
				f.events = append(f.events, int(raw))
				repository["issue"] = f.issuePage(int(raw), cursor)
			}
		} else {
			for slot := 0; slot < 20; slot++ {
				if in.Variables[fmt.Sprintf("use%d", slot)] != true {
					continue
				}
				n, ok := in.Variables[fmt.Sprintf("n%d", slot)].(float64)
				if !ok || n < 1 || n > float64(f.count) {
					f.t.Fatal("wrong history identity", in.Variables)
				}
				cursor, _ := in.Variables[fmt.Sprintf("after%d", slot)].(string)
				f.events = append(f.events, int(n))
				repository[fmt.Sprintf("a%d", slot)] = f.issuePage(int(n), cursor)
			}
			if len(repository) == 0 {
				f.t.Fatal("empty batched request", in.Query)
			}
		}
		body = map[string]any{"data": map[string]any{"repository": repository}}
	} else {
		f.t.Fatalf("unexpected provider request %s %s", r.Method, r.URL.Path)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		f.t.Fatal(err)
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(raw)), Request: r}, nil
}

func (f *historyFixture) issuePage(n int, cursor string) any {
	more, next := false, ""
	if f.fault == "pagination" && n%2 == 0 && cursor == "" {
		more, next = true, "second"
	}
	return map[string]any{"number": n, "comments": map[string]any{
		"pageInfo": map[string]any{"hasNextPage": more, "endCursor": next},
		"nodes":    []any{map[string]any{"createdAt": "2026-01-02T00:00:00Z", "author": map[string]any{"login": "reader", "__typename": "User", "databaseId": 7}}},
	}}
}

func (f *historyFixture) run(serial bool, args ...string) (int, string) {
	f.queries, f.lists, f.paths = 0, 0, nil
	f.trustReads, f.events, f.histories = 0, nil, nil
	forgeFor = func(repo string, observers ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
		native, fr, err := f.resolve(repo, observers...)
		if err != nil {
			return nil, fr, err
		}
		native = deskkit.ReadOnly(native)
		if serial {
			return &serialEventForge{native}, fr, nil
		}
		return native, fr, nil
	}
	var out, errs bytes.Buffer
	code := run(append([]string{"issues", "--root", f.root}, args...), &out, &errs)
	return code, projectionStable(out.String() + errs.String())
}

// regression: #4622
func TestHistoryBatchFlow(t *testing.T) {
	for _, kind := range []string{"question", "addressed", "mixed"} {
		t.Run(kind, func(t *testing.T) {
			f := newHistoryFixture(t, 41, kind)
			before, want := f.run(true)
			serialQueries := f.queries
			after, got := f.run(false)
			if before != 0 || after != 0 || got != want {
				t.Fatalf("output/exit changed: %d/%d\n%s\n%s", before, after, want, got)
			}
			if serialQueries != 41 || f.queries != 9 {
				t.Fatalf("required comment queries serial=%d native=%d; want 41/9", serialQueries, f.queries)
			}
			_, repeated := f.run(false)
			if f.queries != 9 || repeated != got {
				t.Fatal("history reused across invocation", f.queries)
			}
		})
	}
}

// regression: #4622
func TestHistoryTrustLabels(t *testing.T) {
	f := newHistoryFixture(t, 8, "mixed")
	f.external, f.plain = true, true
	before, want := f.run(true)
	after, got := f.run(false)
	if before != 0 || after != 0 || want != got || f.trustReads != 1 || len(f.histories) != 1 {
		t.Fatal("trust or REST label history changed", before, after, f.trustReads, f.histories)
	}
	for _, n := range f.events {
		if n == 2 {
			t.Fatal("unblessed author was prefetched")
		}
	}
}

// regression: #4622
func TestHistoryDuplicateRows(t *testing.T) {
	f := newHistoryFixture(t, 8, "mixed")
	f.duplicate = true
	before, want := f.run(true)
	after, got := f.run(false)
	if before != 0 || after != 0 || want != got || f.queries != 9 {
		t.Fatal("duplicate listing reads were coalesced", before, after, f.queries)
	}
}

// regression: #4622
func TestHistorySecondCaller(t *testing.T) {
	// A planted second caller reaches the same required-history class through
	// the addressed inbox, independently of the default full-board caller.
	f := newHistoryFixture(t, 41, "addressed")
	before, want := f.run(true, "--projection", "inbox", "--to", "worker")
	after, got := f.run(false, "--projection", "inbox", "--to", "worker")
	if before != 0 || after != 0 || got != want || f.queries != 9 {
		t.Fatalf("planted inbox caller retained per-issue reads: exit=%d/%d queries=%d", before, after, f.queries)
	}
}

// regression: #4622
func TestHistoryPages(t *testing.T) {
	f := newHistoryFixture(t, 21, "mixed")
	f.fault = "pagination"
	before, want := f.run(true)
	after, got := f.run(false)
	if before != 0 || after != 0 || got != want || f.queries != 9 {
		t.Fatalf("per-thread cursor walk changed: exit=%d/%d queries=%d\n%s\n%s", before, after, f.queries, want, got)
	}
}

// regression: #4622
func TestHistoryCostProbe(t *testing.T) {
	q := reviewtest.Read(t)
	f := newHistoryFixture(t, q.Size, "mixed")
	f.delay = 3 * time.Millisecond
	var args []string
	switch q.Case {
	case "full":
	case "pages":
		f.fault = "pagination"
	case "intake":
		args = []string{"--projection", "intake"}
	case "inbox":
		args = []string{"--projection", "inbox", "--to", "worker"}
	case "worker":
		args = []string{"--projection", "work-ready"}
	default:
		t.Fatal("unknown fixed history fixture")
	}
	code, output := f.run(false, args...)
	reviewtest.Write(t, reviewtest.Response{Exit: code, Output: output, Calls: f.paths, HTTPComplete: true,
		Facts: map[string]string{"comments": fmt.Sprint(f.queries), "lists": fmt.Sprint(f.lists), "http": fmt.Sprint(len(f.paths)), "delayMs": "3"}})
}
