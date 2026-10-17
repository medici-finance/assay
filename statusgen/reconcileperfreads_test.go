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

// perBriefHandSaid is the per-brief walk handSaidBeforeGeneration performed
// before the per-stream replay replaced it, kept verbatim as the test oracle so
// the replay is compared against an independent implementation, not itself.
func perBriefHandSaid(root, stream, num string) (state, sha string, found bool) {
	rel := filepath.ToSlash(filepath.Join("docs", "streams", stream, "README.md"))
	commits, err := streamReadmeCommits(root, rel)
	if err != nil || len(commits) == 0 {
		return "", "", false
	}
	for _, c := range commits {
		content, ok := readmeAtCommit(root, c.SHA, rel)
		if !ok {
			continue
		}
		if strings.Contains(content, generatedTableMarker) {
			break
		}
		_, briefs, ok := parseStreamSnapshot(content, stream)
		if !ok {
			continue
		}
		for _, b := range briefs {
			if b.Num == num {
				state, sha, found = b.Status, c.SHA, true
			}
		}
	}
	return state, sha, found
}

// TestHandSaidLookupMatchesPerBrief pins that the per-stream replay answers
// exactly what the old per-brief walk answered (an independent oracle), and that
// a stream's history is read once: after the first answer the .git directory is
// removed, and the lookup still answers every brief of that stream from its
// replay.
func TestHandSaidLookupMatchesPerBrief(t *testing.T) {
	dir := initHandSaidRepo(t)
	lookup := newHandSaidLookup(dir)
	for _, c := range []struct{ stream, num string }{{"demo", "01"}, {"demo", "99"}, {"no-such-stream", "01"}} {
		ws, wsha, wfound := perBriefHandSaid(dir, c.stream, c.num)
		for name, got := range map[string]func(string, string) (string, string, bool){
			"lookup":                   lookup,
			"handSaidBeforeGeneration": func(s, n string) (string, string, bool) { return handSaidBeforeGeneration(dir, s, n) },
		} {
			gs, gsha, gfound := got(c.stream, c.num)
			if gs != ws || gsha != wsha || gfound != wfound {
				t.Fatalf("%s/%s: %s = (%q, %q, %v), the per-brief walk = (%q, %q, %v)", c.stream, c.num, name, gs, gsha, gfound, ws, wsha, wfound)
			}
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

// TestHandSaidReplayTwoBriefs pins the replay over a README that carries more
// than one brief, whose histories differ: every row of every snapshot must be
// recorded, not only the first. 01 moves todo → implemented and then stays;
// 02 stays todo until the third commit, then goes done.
func TestHandSaidReplayTwoBriefs(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	rel := filepath.Join("docs", "streams", "demo", "README.md")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	readme := func(s1, s2 string) string {
		return "---\nstream: demo\nstatus: active\npriority: P1\n---\n\n" +
			"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
			"|---|-------|------|--------|--------|----------|----------|\n" +
			"| 01 | First thing | 0 | M | " + s1 + " | — | — |\n" +
			"| 02 | Second thing | 0 | M | " + s2 + " | — | — |\n"
	}
	for i, st := range [][2]string{{"todo", "todo"}, {"implemented", "todo"}, {"implemented", "done"}} {
		if err := os.WriteFile(abs, []byte(readme(st[0], st[1])), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", rel)
		date := "2026-07-0" + strconv.Itoa(i+1) + "T10:00:00Z"
		runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "-q", "-m", "step")
	}
	lookup := newHandSaidLookup(dir)
	for num, want := range map[string]string{"01": "implemented", "02": "done"} {
		ws, wsha, wfound := perBriefHandSaid(dir, "demo", num)
		gs, gsha, gfound := lookup("demo", num)
		if !wfound || ws != want {
			t.Fatalf("fixture drift: the per-brief walk gives %s = (%q, %v), want %q", num, ws, wfound, want)
		}
		if gs != ws || gsha != wsha || gfound != wfound {
			t.Fatalf("demo/%s: replay = (%q, %q, %v), per-brief walk = (%q, %q, %v)", num, gs, gsha, gfound, ws, wsha, wfound)
		}
	}
}

// regression: #3018
func TestHandSaidKeepsDeletedRow(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	rel := filepath.Join("docs", "streams", "demo", "README.md")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	readme := func(state string, includeDeleted bool) string {
		content := "---\nstream: demo\nstatus: active\npriority: P1\n---\n\n" +
			"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
			"|---|-------|------|--------|--------|----------|----------|\n" +
			"| 01 | Retained thing | 0 | M | todo | — | — |\n"
		if includeDeleted {
			content += "| 02 | Deleted thing | 0 | M | " + state + " | — | — |\n"
		}
		return content
	}
	for i, snapshot := range []struct {
		state string
		keep  bool
	}{{"todo", true}, {"done", true}, {"", false}} {
		if err := os.WriteFile(abs, []byte(readme(snapshot.state, snapshot.keep)), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", rel)
		date := "2026-07-0" + strconv.Itoa(i+1) + "T10:00:00Z"
		runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "-q", "-m", "snapshot")
	}

	wantState, wantSHA, found := perBriefHandSaid(dir, "demo", "02")
	if !found || wantState != "done" {
		t.Fatalf("fixture drift: per-brief walk = (%q, %v), want retained done state", wantState, found)
	}
	hashes := gitLogHashes(t, dir, rel)
	if len(hashes) != 3 || wantSHA != hashes[1] {
		t.Fatalf("fixture drift: deleted row's last state came from %q, want second snapshot %v", wantSHA, hashes)
	}

	for name, got := range map[string]func() (string, string, bool){
		"replay": func() (string, string, bool) {
			h, ok := handSaidReplay(dir, "demo")["02"]
			return h.state, h.sha, ok
		},
		"per-brief API": func() (string, string, bool) {
			return handSaidBeforeGeneration(dir, "demo", "02")
		},
		"memoized lookup": func() (string, string, bool) {
			return newHandSaidLookup(dir)("demo", "02")
		},
	} {
		state, sha, found := got()
		if state != wantState || sha != wantSHA || !found {
			t.Errorf("%s = (%q, %q, %v), per-brief walk = (%q, %q, %v)", name, state, sha, found, wantState, wantSHA, true)
		}
	}
}
