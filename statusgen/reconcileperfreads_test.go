package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// TestHandSaidLookupMatchesPerBrief pins that the per-stream replay answers
// exactly what the per-brief walk answers, and that a stream's history is read
// once: after the first answer the .git directory is removed, and the lookup
// still answers every brief of that stream from its replay.
func TestHandSaidLookupMatchesPerBrief(t *testing.T) {
	dir := initHandSaidRepo(t)
	lookup := newHandSaidLookup(dir)
	for _, c := range []struct{ stream, num string }{{"demo", "01"}, {"demo", "99"}, {"no-such-stream", "01"}} {
		ws, wsha, wfound := handSaidBeforeGeneration(dir, c.stream, c.num)
		gs, gsha, gfound := lookup(c.stream, c.num)
		if gs != ws || gsha != wsha || gfound != wfound {
			t.Fatalf("%s/%s: lookup = (%q, %q, %v), per-brief walk = (%q, %q, %v)", c.stream, c.num, gs, gsha, gfound, ws, wsha, wfound)
		}
	}
	want, _, _ := handSaidBeforeGeneration(dir, "demo", "01")
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	if got, _, found := lookup("demo", "01"); !found || got != want {
		t.Fatalf("a stream already replayed must be answered from the replay, not re-read; got (%q, %v), want %q", got, found, want)
	}
}

// TestPrefetchBackfillReadsTheSequentialSet pins that the parallel prefetch reads
// EXACTLY the file lists the sequential newest-first match reads — never a
// speculative superset — so overlapping the waits costs no extra forge requests.
// demo/01 has three candidates: #30 only authored it (read, then the match moves
// on), #20 delivered it (read, the match stops), #10 is never asked about.
// demo/02's only candidate #40 delivered it.
func TestPrefetchBackfillReadsTheSequentialSet(t *testing.T) {
	pull := func(n int, ref string) ghPull {
		p := ghPull{Number: n, MergedAt: "2026-10-01T00:00:00Z", MergeCommitSHA: strings.Repeat("a", 40)}
		p.Head.Ref = ref
		return p
	}
	pulls := []ghPull{pull(30, "docs/demo-01-author"), pull(20, "feat/demo-01-build"), pull(10, "feat/demo-01-old"), pull(40, "feat/demo-02-build")}
	files := map[int][]prFile{
		30: {{Filename: "docs/streams/demo/brief-01-thing.md", Status: "added"}, {Filename: "docs/streams/demo/README.md", Status: "modified"}},
		20: {{Filename: "internal/demo/demo.go", Status: "modified"}},
		10: {{Filename: "internal/demo/old.go", Status: "modified"}},
		40: {{Filename: "internal/demo/two.go", Status: "modified"}},
	}
	briefs := [][2]string{{"demo", "01"}, {"demo", "02"}}

	reads := func(prefetch bool) []int {
		var mu sync.Mutex
		var got []int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			n, _ := strconv.Atoi(parts[len(parts)-2])
			mu.Lock()
			got = append(got, n)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(files[n])
		}))
		defer srv.Close()
		cl := newAuthoringClassifier(&ghClient{doer: srv.Client(), base: srv.URL}, "example-org/board", nil)
		if prefetch {
			cl.prefetchBackfill(pulls, briefs)
		}
		for _, b := range briefs {
			if _, _, why := matchBackfillPRChecked(pulls, b[0], b[1], cl.authoringOnly); why != "" {
				t.Fatalf("%s/%s: unexpected unread: %s", b[0], b[1], why)
			}
		}
		sort.Ints(got)
		return got
	}

	seq, par := reads(false), reads(true)
	if want := []int{20, 30, 40}; !equalInts(seq, want) {
		t.Fatalf("fixture drift: the sequential match read %v, want %v", seq, want)
	}
	if !equalInts(par, seq) {
		t.Fatalf("prefetch read %v, the sequential match reads %v — the prefetch must read exactly that set, each PR once", par, seq)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// countingRepo is an evaluation.Repo that counts the questions it is asked.
type countingRepo struct {
	evaluation.Repo
	mu    sync.Mutex
	calls map[string]int
}

func (c *countingRepo) count(op string) {
	c.mu.Lock()
	c.calls[op]++
	c.mu.Unlock()
}
func (c *countingRepo) IsShallow() (bool, bool) { c.count("shallow"); return false, true }
func (c *countingRepo) ResolveCommit(tok string) (string, bool) {
	c.count("commit:" + tok)
	return tok + "-full", true
}
func (c *countingRepo) DiffNames(a, b string) ([]string, bool) {
	c.count("diff:" + a + ".." + b)
	return []string{"x.go", "y.go"}, true
}

// TestMemoRepoAsksEachQuestionOnce pins the coverage memo: a repeated question
// reaches the git-backed Repo once, distinct arguments are distinct questions,
// and a caller that mutates a returned slice cannot change a later answer.
func TestMemoRepoAsksEachQuestionOnce(t *testing.T) {
	inner := &countingRepo{calls: map[string]int{}}
	m := newMemoRepo(inner)
	for i := 0; i < 5; i++ {
		m.IsShallow()
		m.ResolveCommit("abc")
		names, _ := m.DiffNames("a", "b")
		names[0] = "mutated"
	}
	m.ResolveCommit("def")
	if got, _ := m.DiffNames("a", "b"); got[0] != "x.go" {
		t.Fatalf("a caller's mutation leaked into the memo: %v", got)
	}
	for op, want := range map[string]int{"shallow": 1, "commit:abc": 1, "commit:def": 1, "diff:a..b": 1} {
		if inner.calls[op] != want {
			t.Errorf("%s asked %d time(s), want %d", op, inner.calls[op], want)
		}
	}
}
