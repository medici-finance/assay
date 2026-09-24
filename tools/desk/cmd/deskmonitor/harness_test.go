package main

// harness_test.go — the recorded-forge harness the parity and property tests share.
//
// A FIXTURE is a sequence of poll cycles; each cycle records, per repo, the forge's answer to that
// cycle's read: an open set (issues or PRs), or a failure (status + message, optionally a rate-limit
// signature). One fixture is served two ways:
//
//   - to the bash ORACLE as a stub `gh` on PATH that replays what `gh issue list` / `gh pr list`
//     would print for that answer (JSON on stdout truncated to --limit exactly as gh truncates, or
//     `HTTP <status>: <message>` on stderr and exit 1);
//   - to the VERB as an httptest server replaying the forge API JSON the resolved GitHub client
//     reads (REST /repos/{o}/{n}/issues pages for issues, the /graphql open-changes read for PRs,
//     or the status + message the forge answers a failure with).
//
// Both record which repos they were asked for, in order, so the parity test can assert the two
// read the SAME repos in the SAME order — the stop-on-limit and cursor properties are about which
// reads happen, not only about what is printed.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type fixtureIssue struct {
	Number    int    `json:"number"`
	UpdatedAt string `json:"updatedAt"`
}

type fixturePR struct {
	Number           int    `json:"number"`
	HeadRefOid       string `json:"headRefOid"`
	IsDraft          bool   `json:"isDraft"`
	State            string `json:"state,omitempty"` // default OPEN
	MergeStateStatus string `json:"mergeStateStatus"`
}

// fixtureRead is one repo's answer in one cycle. Status 0 = a successful read of Issues/PRs.
type fixtureRead struct {
	Issues      []fixtureIssue `json:"issues,omitempty"`
	PRs         []fixturePR    `json:"prs,omitempty"`
	Status      int            `json:"status,omitempty"`
	Message     string         `json:"message,omitempty"`
	RateLimited bool           `json:"rateLimited,omitempty"`
}

type fixtureCycle struct {
	Note  string                 `json:"note"`
	Reads map[string]fixtureRead `json:"reads"`
}

type fixture struct {
	Description string            `json:"description"`
	Kind        string            `json:"kind"` // inbound | pr
	Env         map[string]string `json:"env"`
	Repos       []string          `json:"repos"`
	Cycles      []fixtureCycle    `json:"cycles"`
}

func loadFixtures(t *testing.T, kind string) map[string]fixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "parity", kind+"-*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no %s parity fixtures under testdata/parity (%v) — a parity test over nothing proves nothing", kind, err)
	}
	out := map[string]fixture{}
	for _, p := range paths {
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatal(rerr)
		}
		var f fixture
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.DisallowUnknownFields()
		if derr := dec.Decode(&f); derr != nil {
			t.Fatalf("%s: %v", p, derr)
		}
		if f.Kind != kind {
			t.Fatalf("%s: kind %q, want %q", p, f.Kind, kind)
		}
		out[strings.TrimSuffix(filepath.Base(p), ".json")] = f
	}
	return out
}

// forgeFake is the verb-side replay: an httptest server answering the current cycle's reads.
type forgeFake struct {
	srv   *httptest.Server
	mu    sync.Mutex
	cycle fixtureCycle
	reads []string // repo per read, in order (pagination collapsed)
	auths []string // every Authorization header seen
}

func newForgeFake(t *testing.T) *forgeFake {
	t.Helper()
	f := &forgeFake{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	old := forgeAPIBase
	forgeAPIBase = f.srv.URL
	t.Cleanup(func() { forgeAPIBase = old })
	return f
}

func (f *forgeFake) setCycle(c fixtureCycle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cycle = c
}

func (f *forgeFake) takeReads() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.reads
	f.reads = nil
	return r
}

func (f *forgeFake) note(repo string) {
	if n := len(f.reads); n == 0 || f.reads[n-1] != repo {
		f.reads = append(f.reads, repo)
	}
}

func (f *forgeFake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auths = append(f.auths, r.Header.Get("Authorization"))
	w.Header().Set("Content-Type", "application/json")

	var repo string
	isIssues := false
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/") && strings.HasSuffix(r.URL.Path, "/issues"):
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		repo = parts[1] + "/" + parts[2]
		isIssues = true
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		repo = fmt.Sprint(body.Variables["owner"]) + "/" + fmt.Sprint(body.Variables["name"])
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"no such fake route"}`)
		return
	}
	f.note(repo)
	rd, ok := f.cycle.Reads[repo]
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"the fixture recorded no answer for `+repo+` this cycle"}`)
		return
	}
	if rd.Status != 0 {
		if rd.RateLimited {
			w.Header().Set("Retry-After", "60")
		}
		w.WriteHeader(rd.Status)
		b, _ := json.Marshal(map[string]string{"message": rd.Message})
		_, _ = w.Write(b)
		return
	}
	if isIssues {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if per < 1 {
			per = 30
		}
		lo, hi := (page-1)*per, page*per
		if lo > len(rd.Issues) {
			lo = len(rd.Issues)
		}
		if hi > len(rd.Issues) {
			hi = len(rd.Issues)
		}
		if hi < len(rd.Issues) {
			w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=%d>; rel="next"`, f.srv.URL, r.URL.Path, page+1))
		}
		items := make([]map[string]any, 0, hi-lo)
		for _, is := range rd.Issues[lo:hi] {
			items = append(items, map[string]any{
				"number": is.Number, "title": "fixture issue", "user": map[string]any{"login": "someone", "id": 7},
				"labels": []any{}, "created_at": "2026-01-01T00:00:00Z", "updated_at": is.UpdatedAt,
				"html_url": "https://github.com/" + repo + "/issues/" + strconv.Itoa(is.Number),
			})
		}
		_ = json.NewEncoder(w).Encode(items)
		return
	}
	// The open-changes GraphQL read, newest first (number descending stands in for createdAt).
	prs := append([]fixturePR(nil), rd.PRs...)
	sort.Slice(prs, func(i, j int) bool { return prs[i].Number > prs[j].Number })
	nodes := make([]map[string]any, 0, len(prs))
	for _, p := range prs {
		state := p.State
		if state == "" {
			state = "OPEN"
		}
		nodes = append(nodes, map[string]any{
			"number": p.Number, "title": "fixture pr", "body": "", "state": state, "isDraft": p.IsDraft,
			"createdAt": "2026-01-01T00:00:00Z", "lastEditedAt": "", "author": map[string]any{"login": "someone", "__typename": "User"},
			"mergeStateStatus": p.MergeStateStatus, "headRefOid": p.HeadRefOid, "headRefName": "feat", "baseRefName": "main",
			"labels": map[string]any{"nodes": []any{}}, "commits": map[string]any{"nodes": []any{}},
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{
		"pullRequests": map[string]any{"nodes": nodes}}}})
}

// ghStub is the oracle-side replay: a stub `gh` on PATH reading the current cycle's rendered
// answers from a directory the test rewrites before each cycle.
type ghStub struct {
	binDir   string
	cycleDir string
	logPath  string
}

const ghStubScript = `#!/bin/sh
# parity stub gh — replays the recorded answer for --repo from $PARITY_CYCLE_DIR.
repo=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo) repo="$2"; shift 2 ;;
    *) shift ;;
  esac
done
printf '%s\n' "$repo" >> "$PARITY_READ_LOG"
key=$(printf '%s' "$repo" | sed 's#/#__#g')
d="$PARITY_CYCLE_DIR"
if [ ! -f "$d/$key.rc" ]; then
  echo "HTTP 500: the fixture recorded no answer for $repo this cycle" >&2
  exit 1
fi
[ -f "$d/$key.out" ] && cat "$d/$key.out"
[ -f "$d/$key.err" ] && cat "$d/$key.err" >&2
exit "$(cat "$d/$key.rc")"
`

func newGHStub(t *testing.T) *ghStub {
	t.Helper()
	g := &ghStub{binDir: t.TempDir(), cycleDir: t.TempDir()}
	g.logPath = filepath.Join(t.TempDir(), "reads.log")
	if err := os.WriteFile(filepath.Join(g.binDir, "gh"), []byte(ghStubScript), 0o755); err != nil {
		t.Fatal(err)
	}
	return g
}

// render writes one cycle's answers as gh would print them for a poller asking with limit.
func (g *ghStub) render(t *testing.T, kind string, c fixtureCycle, limit int) {
	t.Helper()
	entries, _ := os.ReadDir(g.cycleDir)
	for _, e := range entries {
		_ = os.Remove(filepath.Join(g.cycleDir, e.Name()))
	}
	for repo, rd := range c.Reads {
		key := strings.ReplaceAll(repo, "/", "__")
		write := func(ext, body string) {
			if err := os.WriteFile(filepath.Join(g.cycleDir, key+ext), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if rd.Status != 0 {
			write(".err", fmt.Sprintf("HTTP %d: %s\n", rd.Status, rd.Message))
			write(".rc", "1")
			continue
		}
		var payload []byte
		if kind == "inbound" {
			// gh issue list keeps the newest `--limit` (issue creation order; the fixture lists
			// newest first) — only the COUNT matters to the poller once it is at the limit.
			iss := rd.Issues
			if len(iss) > limit {
				iss = iss[:limit]
			}
			if iss == nil {
				iss = []fixtureIssue{}
			}
			payload, _ = json.Marshal(iss)
		} else {
			prs := append([]fixturePR(nil), rd.PRs...)
			sort.Slice(prs, func(i, j int) bool { return prs[i].Number > prs[j].Number })
			if len(prs) > limit {
				prs = prs[:limit]
			}
			rows := make([]map[string]any, 0, len(prs))
			for _, p := range prs {
				state := p.State
				if state == "" {
					state = "OPEN"
				}
				rows = append(rows, map[string]any{"number": p.Number, "headRefOid": p.HeadRefOid,
					"isDraft": p.IsDraft, "state": state, "mergeStateStatus": p.MergeStateStatus})
			}
			payload, _ = json.Marshal(rows)
		}
		write(".out", string(payload)+"\n")
		write(".rc", "0")
	}
}

func (g *ghStub) takeReads(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(g.logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(g.logPath)
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// plantRoster installs a roster under a private HOME that binds every fixture repo to GitHub, so
// deskkit.ForgeFor resolves the forge from configuration — never from whatever checkout the test
// happens to run in.
func plantRoster(t *testing.T, repos []string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=ada:2001\n" +
		"ASSAY_TRUSTED_LOGINS=ada:2001\n" +
		"ASSAY_ALLOWED_REPOS=" + strings.Join(repos, ":ci:private,") + ":ci:private\n" +
		"ASSAY_REPO_FORGES=" + strings.Join(repos, "=github,") + "=github\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// stubSessionIdentity replaces the session-role and mint seams so no test ever reaches the real
// token minter: the session acts as role, and every mint answers token.
func stubSessionIdentity(t *testing.T, role, token string, roleErr error) {
	t.Helper()
	oldRole, oldMint := sessionRoleFn, mintTokenFn
	sessionRoleFn = func(string) (string, string, error) {
		if roleErr != nil {
			return "", "", roleErr
		}
		return role, "worker-desk", nil
	}
	mintTokenFn = func(string, string) (string, string, error) { return token, "", nil }
	t.Cleanup(func() { sessionRoleFn, mintTokenFn = oldRole, oldMint })
}

// ownerTokenFile writes an owner-only token file and returns its path.
func ownerTokenFile(t *testing.T, value string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "installation-token")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// owners returns the distinct owners of repos, sorted.
func owners(repos []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range repos {
		o, _, _ := strings.Cut(r, "/")
		if !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	sort.Strings(out)
	return out
}

// snapshotState reads every file in a state dir, keyed by name.
func snapshotState(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		out[e.Name()] = string(b)
	}
	return out
}
