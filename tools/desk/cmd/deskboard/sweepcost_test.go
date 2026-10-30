package main

// sweepcost_test.go — #2421 acceptance: the read cost of a SECOND `deskboard actions` sweep
// over an unchanged board must not grow with the number of open PRs.
//
// The board runs end to end — run(), the real resolver, the real GitHub backend — against a
// fake forge that behaves like GitHub where it matters here: every GET carries an ETag, a
// GET whose If-None-Match matches gets a 304, and only non-304 answers are charged. The
// tests use nothing but symbols that predate the fix, so they compile against the base code
// too, where the constant-cost test is red (the base re-reads every PR in full each sweep).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const fakeRepo = "example-org/tracker"

type fakePR struct {
	num      int
	head     string
	updated  string
	reviewed string // the sha the reviewer App's APPROVE landed at
	ci       string // the check run's conclusion
	base     string // the base branch; "" reads as main
	ciExtra  bool   // a second (successful) check run in the rollup
}

// etagForge is the GitHub stand-in. Charged holds "METHOD path" for every answer that was not
// a 304; All holds every request; Conditional counts requests that carried a validator.
type etagForge struct {
	mu  sync.Mutex
	prs []fakePR
	// Optional live repository scalar carried by the queue snapshot. Nil keeps
	// the older-backend fixture, which must still use the metadata fallback.
	snapshotVisibility *string
	metadataFailure    bool
	remaining          int
	reset              int64
	charged            []string
	all                []string
	conditional        int
	unknown            []string
}

type sweepResult struct {
	code        int
	stdout      []byte
	stderr      string
	charged     []string
	all         []string
	conditional int
}

// newEtagForge starts a fake forge with n open PRs and points a fresh HOME (roster, state
// dir, cache) and the board's resolver at it.
func newEtagForge(t *testing.T, n int) *etagForge {
	t.Helper()
	ff := &etagForge{remaining: 4900, reset: time.Now().Add(30 * time.Minute).Unix()}
	for i := 1; i <= n; i++ {
		head := fmt.Sprintf("%040x", i)
		ff.prs = append(ff.prs, fakePR{num: i, head: head, updated: "2026-10-12T10:00:00Z", reviewed: head, ci: "SUCCESS"})
	}
	srv := httptest.NewServer(http.HandlerFunc(ff.serve))
	t.Cleanup(srv.Close)

	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\n" +
		"ASSAY_TRUSTED_LOGINS=ada:2001\n" +
		"ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,reviewer=assay-reviewer-app:300000004\n" +
		"ASSAY_ALLOWED_REPOS=" + fakeRepo + ":ci:private\n" +
		"ASSAY_REPO_FORGES=" + fakeRepo + "=github\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("DESK_LOOP", "the-desk")
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESKBOARD_GATE_SCORES_JSON", "[]")
	t.Setenv("DESKBOARD_RATE_FLOOR", "")
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	oldMint, oldBase := mintTokenFn, forgeAPIBase
	mintTokenFn = func(role, repo string) (string, string, error) { return "test-token", "", nil }
	forgeAPIBase = srv.URL
	t.Cleanup(func() { mintTokenFn, forgeAPIBase = oldMint, oldBase })
	return ff
}

func (ff *etagForge) edit(fn func(prs []fakePR)) {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	fn(ff.prs)
}

// sweep runs one `deskboard actions` and returns what it spent.
func (ff *etagForge) sweep(t *testing.T, args ...string) sweepResult {
	t.Helper()
	ff.mu.Lock()
	ff.charged, ff.all, ff.conditional, ff.unknown = nil, nil, 0, nil
	ff.mu.Unlock()
	var out, errb bytes.Buffer
	code := run(append([]string{"actions"}, args...), &out, &errb)
	ff.mu.Lock()
	defer ff.mu.Unlock()
	if len(ff.unknown) > 0 {
		t.Fatalf("the board sent requests the fake forge does not serve: %v\nstderr:\n%s", ff.unknown, errb.String())
	}
	return sweepResult{code: code, stdout: out.Bytes(), stderr: errb.String(),
		charged: append([]string(nil), ff.charged...), all: append([]string(nil), ff.all...), conditional: ff.conditional}
}

var (
	rePull      = regexp.MustCompile(`^/repos/example-org/tracker/pulls/(\d+)$`)
	rePullFiles = regexp.MustCompile(`^/repos/example-org/tracker/pulls/(\d+)/files$`)
	reCommits   = regexp.MustCompile(`^/repos/example-org/tracker/commits$`)
	reChecks    = regexp.MustCompile(`^/repos/example-org/tracker/commits/([0-9a-f]+)/check-runs$`)
	reStatus    = regexp.MustCompile(`^/repos/example-org/tracker/commits/([0-9a-f]+)/status$`)
)

func (ff *etagForge) serve(w http.ResponseWriter, r *http.Request) {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	key := r.Method + " " + r.URL.Path
	ff.all = append(ff.all, key)
	resource := "core"
	if r.URL.Path == "/graphql" {
		resource = "graphql"
	}
	w.Header().Set("X-RateLimit-Resource", resource)
	w.Header().Set("X-RateLimit-Limit", "5000")
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(ff.remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(ff.reset, 10))
	w.Header().Set("Content-Type", "application/json")

	var body any
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		raw, _ := io.ReadAll(r.Body)
		if !bytes.Contains(raw, []byte("pullRequests(states:OPEN")) {
			ff.unknown = append(ff.unknown, key+" "+string(raw))
			http.Error(w, `{"message":"not served"}`, http.StatusNotFound)
			return
		}
		ff.charged = append(ff.charged, key)
		_ = json.NewEncoder(w).Encode(ff.snapshotLocked())
		return
	case r.Method != http.MethodGet:
		ff.unknown = append(ff.unknown, key)
		http.Error(w, `{"message":"not served"}`, http.StatusNotFound)
		return
	case r.URL.Path == "/repos/example-org/tracker":
		if ff.metadataFailure {
			http.Error(w, `{"message":"metadata unavailable"}`, http.StatusForbidden)
			return
		}
		body = map[string]any{"name": "tracker", "full_name": fakeRepo, "private": true, "visibility": "private"}
	case reCommits.MatchString(r.URL.Path):
		body = []any{map[string]any{"sha": "aaaa000000000000000000000000000000000001",
			"commit":  map[string]any{"message": "main", "committer": map[string]any{"date": "2026-10-12T09:00:00Z"}},
			"parents": []any{map[string]any{"sha": "aaaa000000000000000000000000000000000000"}}}}
	case reChecks.MatchString(r.URL.Path):
		body = map[string]any{"total_count": 1, "check_runs": []any{map[string]any{"id": 1, "name": "ci",
			"status": "completed", "conclusion": "success", "started_at": "2026-10-12T09:00:00Z", "completed_at": "2026-10-12T09:05:00Z"}}}
	case reStatus.MatchString(r.URL.Path):
		body = map[string]any{"state": "success", "statuses": []any{}, "total_count": 0}
	case rePullFiles.MatchString(r.URL.Path):
		n, _ := strconv.Atoi(rePullFiles.FindStringSubmatch(r.URL.Path)[1])
		// The file list is a function of the base: a retargeted PR's diff differs (GitHub
		// recomputes it against the new base at the same head).
		name := fmt.Sprintf("src/file%d.go", n)
		if pr, ok := ff.prLocked(n); ok && pr.base != "" && pr.base != "main" {
			name = fmt.Sprintf("src/file%d-vs-%s.go", n, pr.base)
		}
		body = []any{map[string]any{"filename": name, "status": "modified"}}
	case rePull.MatchString(r.URL.Path):
		n, _ := strconv.Atoi(rePull.FindStringSubmatch(r.URL.Path)[1])
		pr, ok := ff.prLocked(n)
		if !ok {
			ff.unknown = append(ff.unknown, key)
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		body = map[string]any{"number": n, "state": "open", "changed_files": 1,
			"head": map[string]any{"sha": pr.head}, "updated_at": pr.updated}
	default:
		ff.unknown = append(ff.unknown, key)
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		return
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		ff.conditional++
		if inm == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	ff.charged = append(ff.charged, key)
	_, _ = w.Write(raw)
}

func (ff *etagForge) prLocked(n int) (fakePR, bool) {
	for _, p := range ff.prs {
		if p.num == n {
			return p, true
		}
	}
	return fakePR{}, false
}

func (ff *etagForge) snapshotLocked() any {
	nodes := make([]any, 0, len(ff.prs))
	for _, p := range ff.prs {
		base := p.base
		if base == "" {
			base = "main"
		}
		checks := []any{map[string]any{"__typename": "CheckRun", "databaseId": p.num,
			"name": "ci", "status": "COMPLETED", "conclusion": p.ci,
			"startedAt": "2026-10-12T09:40:00Z", "completedAt": "2026-10-12T09:45:00Z"}}
		if p.ciExtra {
			checks = append(checks, map[string]any{"__typename": "CheckRun", "databaseId": 10000 + p.num,
				"name": "lint", "status": "COMPLETED", "conclusion": "SUCCESS",
				"startedAt": "2026-10-12T09:40:00Z", "completedAt": "2026-10-12T09:46:00Z"})
		}
		nodes = append(nodes, map[string]any{
			"number": p.num, "title": fmt.Sprintf("change %d", p.num), "body": "Issue: #123",
			"state": "OPEN", "isDraft": false, "createdAt": "2026-10-12T09:30:00Z", "lastEditedAt": nil,
			"updatedAt": p.updated, "author": map[string]any{"login": "ada", "__typename": "User"},
			"mergeStateStatus": "CLEAN", "headRefOid": p.head, "headRefName": fmt.Sprintf("feat-%d", p.num),
			"baseRefName": base,
			"reviews": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{map[string]any{
				"databaseId": 1000 + p.num, "author": map[string]any{"login": "assay-reviewer-app", "__typename": "Bot", "databaseId": 300000004},
				"state": "APPROVED", "commit": map[string]any{"oid": p.reviewed}, "body": "Verdict: APPROVE",
				"submittedAt": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)}}},
			"labels": map[string]any{"nodes": []any{}},
			// The head commit's oid + committer date ride on the rollup node, as the live
			// query serves them (#3093) — so the stale-grant filter needs no per-commit read.
			"commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{
				"oid": p.head, "committedDate": "2026-10-12T09:35:00Z", "statusCheckRollup": map[string]any{
					"contexts": map[string]any{"nodes": checks}}}}}},
		})
	}
	repository := map[string]any{"pullRequests": map[string]any{"nodes": nodes}}
	if ff.snapshotVisibility != nil {
		repository["visibility"] = *ff.snapshotVisibility
	}
	return map[string]any{"data": map[string]any{"repository": repository}}
}

// boardRows decodes the actions JSON rows by PR number.
func boardRows(t *testing.T, res sweepResult) map[int]actionRow {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("actions exited %d, want 0\nstderr:\n%s", res.code, res.stderr)
	}
	var rep struct {
		Rows []actionRow `json:"rows"`
	}
	if err := json.Unmarshal(res.stdout, &rep); err != nil {
		t.Fatalf("decode actions JSON: %v\n%s", err, res.stdout)
	}
	rows := map[int]actionRow{}
	for _, r := range rep.Rows {
		rows[r.Number] = r
	}
	return rows
}

// perPRPaths returns the PR numbers any request in paths was about.
func perPRPaths(paths []string) []int {
	seen := map[int]bool{}
	for _, k := range paths {
		p := strings.TrimPrefix(strings.TrimPrefix(k, "GET "), "POST ")
		for _, re := range []*regexp.Regexp{rePull, rePullFiles} {
			if m := re.FindStringSubmatch(p); m != nil {
				n, _ := strconv.Atoi(m[1])
				seen[n] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// regression: #2421
func TestSecondSweepCostConstant(t *testing.T) {
	charged := map[int][]string{}
	for _, n := range []int{5, 50} {
		ff := newEtagForge(t, n)
		first := ff.sweep(t)
		rows := boardRows(t, first)
		if len(rows) != n {
			t.Fatalf("N=%d: first sweep printed %d rows, want %d", n, len(rows), n)
		}
		if rows[1].Action != actMergeNow {
			t.Fatalf("N=%d: fixture PR #1 should classify %s, got %s (%s)", n, actMergeNow, rows[1].Action, rows[1].Note)
		}
		second := ff.sweep(t)
		if got := boardRows(t, second); len(got) != n {
			t.Fatalf("N=%d: second sweep printed %d rows, want %d", n, len(got), n)
		}
		charged[n] = second.charged
		t.Logf("N=%d: first sweep charged %d of %d requests; second sweep charged %d of %d requests",
			n, len(first.charged), len(first.all), len(second.charged), len(second.all))
	}
	if len(charged[5]) != len(charged[50]) {
		t.Fatalf("the second sweep over an UNCHANGED board charged %d calls at 5 PRs but %d at 50 — "+
			"its cost grows with the PR count.\ncharged at 5 PRs: %v", len(charged[5]), len(charged[50]), charged[5])
	}
}

// regression: #2421
func TestHeadChangeRereadsOnePR(t *testing.T) {
	ff := newEtagForge(t, 5)
	boardRows(t, ff.sweep(t))
	newHead := fmt.Sprintf("%040x", 0xbeef)
	ff.edit(func(prs []fakePR) {
		prs[2].head, prs[2].reviewed, prs[2].updated = newHead, newHead, "2026-10-12T11:00:00Z"
	})
	res := ff.sweep(t)
	rows := boardRows(t, res)
	if got := perPRPaths(res.all); len(got) != 1 || got[0] != 3 {
		t.Fatalf("after PR #3's head moved, the sweep read per-PR endpoints of PRs %v, want exactly [3]\nrequests: %v", got, res.all)
	}
	if !contains(res.charged, "GET /repos/example-org/tracker/pulls/3") {
		t.Fatalf("PR #3's metadata was not re-read at its new head; charged: %v", res.charged)
	}
	if rows[3].Action != actMergeNow {
		t.Fatalf("PR #3 approved at its new head should be %s, got %s (%s)", actMergeNow, rows[3].Action, rows[3].Note)
	}
}

// regression: #2421
// TestSkipKeyEveryPart — the per-PR skip reuses a PR's changed-file set only while EVERY part
// of its key is unchanged. Each subtest moves exactly one part at an unchanged head and
// asserts that PR — and only that PR — is re-read from the forge. The base case matters most:
// a retarget at the same head changes the diff GitHub reports, and the reused set feeds the
// risk-path scan, so a stale set could leave a security-path PR un-risk-classed.
func TestSkipKeyEveryPart(t *testing.T) {
	cases := []struct {
		name  string
		move  func(p *fakePR)
		fresh bool // the new file list differs, so the re-read must come back charged (not a 304)
	}{
		{"base", func(p *fakePR) { p.base = "release" }, true},
		{"updatedAt", func(p *fakePR) { p.updated = "2026-10-12T12:00:00Z" }, false},
		{"rollup", func(p *fakePR) { p.ciExtra = true }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ff := newEtagForge(t, 5)
			boardRows(t, ff.sweep(t))
			ff.edit(func(prs []fakePR) { tc.move(&prs[3]) })
			res := ff.sweep(t)
			boardRows(t, res)
			if got := perPRPaths(res.all); len(got) != 1 || got[0] != 4 {
				t.Fatalf("after only PR #4's %s moved (head unchanged), the sweep read per-PR endpoints of PRs %v, want exactly [4]\nrequests: %v",
					tc.name, got, res.all)
			}
			if !contains(res.all, "GET /repos/example-org/tracker/pulls/4/files") {
				t.Fatalf("PR #4's changed files were not re-read after its %s moved; requests: %v", tc.name, res.all)
			}
			if tc.fresh && !contains(res.charged, "GET /repos/example-org/tracker/pulls/4/files") {
				t.Fatalf("PR #4's changed files after a %s change came back unchanged; charged: %v", tc.name, res.charged)
			}
		})
	}
}

// TestCheckFlipNoStaleGreen — a check that goes red behind an unchanged head is still seen
// on the board. What it pins is that the check rollup is never cached: it arrives fresh in
// the uncached GraphQL snapshot every sweep. It does NOT pin the per-PR skip key — no change
// to the reuse can turn it red; TestSkipKeyEveryPart pins the key's rollup component.
func TestCheckFlipNoStaleGreen(t *testing.T) {
	ff := newEtagForge(t, 5)
	if rows := boardRows(t, ff.sweep(t)); rows[2].Action != actMergeNow {
		t.Fatalf("fixture PR #2 should start %s, got %s", actMergeNow, rows[2].Action)
	}
	// The check run goes red behind an UNCHANGED head and updatedAt (a check-run change does
	// not move GitHub's updatedAt).
	ff.edit(func(prs []fakePR) { prs[1].ci = "FAILURE" })
	rows := boardRows(t, ff.sweep(t))
	if rows[2].CIFail != 1 || rows[2].Action == actMergeNow {
		t.Fatalf("a check that went red behind an unchanged head was not seen: PR #2 ciFail=%d action=%s (%s)",
			rows[2].CIFail, rows[2].Action, rows[2].Note)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// TestRateFloorRefusesSweep — a sweep during which the forge reports quota below the floor
// exits 4 with no board, naming the reset time; the flag and env var move the floor.
func TestRateFloorRefusesSweep(t *testing.T) {
	ff := newEtagForge(t, 3)
	ff.remaining = 100
	res := ff.sweep(t)
	if res.code != deskkit.ExitRateLimited {
		t.Fatalf("quota 100 under the default floor 500: exit %d, want %d\nstderr:\n%s", res.code, deskkit.ExitRateLimited, res.stderr)
	}
	if len(res.stdout) != 0 {
		t.Fatalf("a sweep stopped at the floor printed a board:\n%s", res.stdout)
	}
	reset := time.Unix(ff.reset, 0).UTC().Format(time.RFC3339)
	if !strings.Contains(res.stderr, "resets at "+reset) || !strings.Contains(res.stderr, "deskboard: cost ") {
		t.Fatalf("refusal must name the reset time %s and carry the cost line; stderr:\n%s", reset, res.stderr)
	}
	if res := ff.sweep(t, "--rate-floor", "50"); res.code != 0 {
		t.Fatalf("--rate-floor 50 with 100 remaining: exit %d, want 0\nstderr:\n%s", res.code, res.stderr)
	}
	t.Setenv("DESKBOARD_RATE_FLOOR", "50")
	if res := ff.sweep(t); res.code != 0 {
		t.Fatalf("DESKBOARD_RATE_FLOOR=50 with 100 remaining: exit %d, want 0\nstderr:\n%s", res.code, res.stderr)
	}
	if res := ff.sweep(t, "--rate-floor", "500"); res.code != deskkit.ExitRateLimited {
		t.Fatalf("--rate-floor must win over the env var: exit %d, want %d", res.code, deskkit.ExitRateLimited)
	}
	t.Setenv("DESKBOARD_RATE_FLOOR", "lots")
	if res := ff.sweep(t); res.code != deskkit.ExitRefused {
		t.Fatalf("a malformed DESKBOARD_RATE_FLOOR must refuse (exit %d), got %d", deskkit.ExitRefused, res.code)
	}
}

// TestSweepCostReported — the cost rides in the JSON header and as one stderr line.
func TestSweepCostReported(t *testing.T) {
	ff := newEtagForge(t, 4)
	ff.sweep(t)
	res := ff.sweep(t)
	boardRows(t, res)
	var rep struct {
		Cost *struct {
			Requests    int                       `json:"requests"`
			Charged     int                       `json:"charged"`
			NotModified int                       `json:"notModified"`
			Skipped     int                       `json:"skipped"`
			ByClass     map[string]map[string]int `json:"byClass"`
			RateLimit   []map[string]any          `json:"rateLimit"`
			Floor       int                       `json:"floor"`
		} `json:"cost"`
	}
	if err := json.Unmarshal(res.stdout, &rep); err != nil || rep.Cost == nil {
		t.Fatalf("actions JSON carries no cost field (err=%v):\n%s", err, res.stdout)
	}
	c := rep.Cost
	if c.Requests != len(res.all) || c.Charged != len(res.charged) || c.NotModified != len(res.all)-len(res.charged) {
		t.Fatalf("cost %+v disagrees with the forge's own count: %d requests, %d charged", *c, len(res.all), len(res.charged))
	}
	if c.Skipped != 2*4 || c.Floor != 500 || c.ByClass["graphql"]["requests"] != 1 || len(c.RateLimit) != 2 {
		t.Fatalf("cost fields wrong: %+v", *c)
	}
	lines := 0
	for _, l := range strings.Split(res.stderr, "\n") {
		if strings.HasPrefix(l, "deskboard: cost ") {
			lines++
			if !strings.Contains(l, "core.remaining=4900") || !strings.Contains(l, "skipped=8") {
				t.Fatalf("cost line missing fields: %s", l)
			}
		}
	}
	if lines != 1 {
		t.Fatalf("want exactly one cost line on stderr, got %d:\n%s", lines, res.stderr)
	}
}

// TestCorruptCacheIsMiss — a garbled cache is a miss: no validator is sent, every read is
// fresh, and the board is the same board. Never an error, never a stale serve.
func TestCorruptCacheIsMiss(t *testing.T) {
	ff := newEtagForge(t, 4)
	first := ff.sweep(t)
	want := boardRows(t, first)
	root := filepath.Join(os.Getenv("HOME"), ".config", "assay", "deskboard-cache")
	n := 0
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
			if werr := os.WriteFile(p, []byte(`{"schema":1,"url":`), 0o600); werr != nil {
				t.Fatal(werr)
			}
		}
		return nil
	})
	if n == 0 {
		t.Fatalf("the first sweep wrote no cache files under %s", root)
	}
	res := ff.sweep(t)
	got := boardRows(t, res)
	if res.conditional != 0 || len(res.charged) != len(first.charged) {
		t.Fatalf("a corrupt cache must be a full miss: %d conditional requests, %d charged (first sweep %d)",
			res.conditional, len(res.charged), len(first.charged))
	}
	for num, r := range want {
		if got[num].Action != r.Action || got[num].CIPass != r.CIPass {
			t.Fatalf("PR #%d classified %s after the cache was corrupted, %s before", num, got[num].Action, r.Action)
		}
	}
}
