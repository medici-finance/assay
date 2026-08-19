package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeGH is a scriptable stand-in for the GitHub git-data API. EVERY request is recorded
// so a refusal test can assert that no mutating call was made — the property that matters
// most here is not "it returned 5" but "it returned 5 and wrote nothing".
type fakeGH struct {
	srv *httptest.Server

	mu   sync.Mutex
	hits []string // "METHOD /path"

	refs map[string]string // "tags/desk-tools/v0.1.3" / "heads/main" → sha

	mainType string // object type reported for heads/main (default "commit")

	// Fault injection. 0 means "behave normally".
	getStatus    int // forced status for GET  /git/ref/*
	createStatus int // forced status for POST /git/refs

	createdSHA string // when set, the POST response reports THIS sha instead of the requested one

	// Public-repo gate. visibility is what GET /repos/{owner}/{repo}
	// reports; empty means "private". visStatus forces a non-2xx on that call so a test
	// can prove the gate fails CLOSED when visibility cannot be read.
	visibility string
	visStatus  int

	posts int
}

const (
	testOwner = "medici-finance"
	testRepo  = "assay"
	mainSHA   = "1111111111111111111111111111111111111111"
	otherSHA  = "2222222222222222222222222222222222222222"
)

func newFakeGH(t *testing.T) *fakeGH {
	t.Helper()
	f := &fakeGH{
		refs:     map[string]string{"heads/main": mainSHA},
		mainType: "commit",
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)

	old := apiBaseURL
	apiBaseURL = f.srv.URL
	t.Cleanup(func() { apiBaseURL = old })
	return f
}

func (f *fakeGH) record(method, path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits = append(f.hits, method+" "+path)
}

func (f *fakeGH) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.hits...)
}

// mutatingCalls returns every non-GET request the fake saw. A refusal path must produce
// an empty result.
func (f *fakeGH) mutatingCalls() []string {
	var out []string
	for _, h := range f.calls() {
		if !strings.HasPrefix(h, "GET ") {
			out = append(out, h)
		}
	}
	return out
}

func (f *fakeGH) handle(w http.ResponseWriter, r *http.Request) {
	f.record(r.Method, r.URL.Path)
	prefix := "/repos/" + testOwner + "/" + testRepo

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, prefix+"/git/ref/"):
		if f.getStatus != 0 {
			w.WriteHeader(f.getStatus)
			return
		}
		ref := strings.TrimPrefix(r.URL.Path, prefix+"/git/ref/")
		f.mu.Lock()
		sha, ok := f.refs[ref]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		objType := "commit"
		if ref == "heads/main" {
			objType = f.mainType
		}
		writeRef(w, http.StatusOK, "refs/"+ref, sha, objType)

	case r.Method == http.MethodPost && r.URL.Path == prefix+"/git/refs":
		f.mu.Lock()
		f.posts++
		f.mu.Unlock()
		if f.createStatus != 0 {
			w.WriteHeader(f.createStatus)
			return
		}
		var in struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		body, _ := readAll(r)
		_ = json.Unmarshal(body, &in)
		ref := strings.TrimPrefix(in.Ref, "refs/")
		f.mu.Lock()
		_, exists := f.refs[ref]
		f.mu.Unlock()
		if exists {
			// GitHub's real create-only behaviour: an existing ref is 422, never a move.
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		f.mu.Lock()
		f.refs[ref] = in.SHA
		f.mu.Unlock()
		out := in.SHA
		if f.createdSHA != "" {
			out = f.createdSHA
		}
		writeRef(w, http.StatusCreated, in.Ref, out, "commit")

	// GET /repos/{owner}/{repo} — visibility check (public-repo gate).
	// Defaults to private (assay's real value, so the gate is a no-op), but a
	// test can force "public" or an API failure to exercise the gate's two refusals.
	case r.Method == http.MethodGet && r.URL.Path == prefix:
		if f.visStatus != 0 {
			w.WriteHeader(f.visStatus)
			return
		}
		vis := f.visibility
		if vis == "" {
			vis = "private"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"visibility": vis})

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeRef(w http.ResponseWriter, code int, ref, sha, objType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ref":    ref,
		"object": map[string]string{"sha": sha, "type": objType},
	})
}

func readAll(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}

// --- desktoken seam -------------------------------------------------------------

// tokenRecorder captures every external command this binary constructs. The suite
// asserts that the ONLY program ever invoked is desktoken — no git, no gh, ever.
type tokenRecorder struct {
	mu   sync.Mutex
	argv [][]string
}

func (tr *tokenRecorder) all() [][]string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return append([][]string(nil), tr.argv...)
}

// fakeDesktoken binds execCommand so that a desktoken invocation echoes a real token
// file path, exercising the production parse-stdout-then-read-file path rather than
// stubbing it out. fail=true makes the mint fail.
func fakeDesktoken(t *testing.T, fail bool) *tokenRecorder {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "desk-token-100000013")
	if err := os.WriteFile(tokenPath, []byte("ghs_FAKE_DESK_APP_TOKEN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tr := &tokenRecorder{}
	old := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		tr.mu.Lock()
		tr.argv = append(tr.argv, append([]string{name}, arg...))
		tr.mu.Unlock()
		if fail {
			return exec.Command("/usr/bin/false")
		}
		return exec.Command("/bin/echo", tokenPath)
	}
	t.Cleanup(func() { execCommand = old })
	return tr
}

// --- environment ----------------------------------------------------------------

// harness wires a test HOME (audit log + flock land there), a fake GitHub, a fake
// desktoken, and captured stdout/stderr.
type harness struct {
	gh    *fakeGH
	tok   *tokenRecorder
	home  string
	out   *bytes.Buffer
	errb  *bytes.Buffer
	tHelp *testing.T
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")

	h := &harness{
		gh:    newFakeGH(t),
		tok:   fakeDesktoken(t, false),
		home:  home,
		out:   &bytes.Buffer{},
		errb:  &bytes.Buffer{},
		tHelp: t,
	}
	oldOut, oldErr := stdout, stderr
	stdout, stderr = h.out, h.errb
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })
	return h
}

// auditLines reads every audit entry written during the test.
func (h *harness) auditLines(t *testing.T) []deskkit.Entry {
	t.Helper()
	path := filepath.Join(h.home, ".config", "assay", "audit.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read audit: %v", err)
	}
	var out []deskkit.Entry
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var e deskkit.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("bad audit line %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

// auditRaw is the raw audit file, for the "no secret ever lands here" assertion.
func (h *harness) auditRaw(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(h.home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		return ""
	}
	return string(raw)
}

func (h *harness) combined() string { return h.out.String() + h.errb.String() }

// --- outward-write budget scoping (surfaced in review) ---------

// seedCutAudit appends n charged `deskrelease` entries in the shape a real cut writes:
// Repo = repoSlug and NO PR number (writeflow.go's finishAudit sets `PR: nil`, "a release
// cut has no PR"). Seeding the shape the tool ACTUALLY records is the whole point — a seed
// written in the shape the gate merely expects would pass against a gate aimed anywhere.
func seedCutAudit(t *testing.T, h *harness, n int) {
	t.Helper()
	dir := filepath.Join(h.home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	defer f.Close()
	for i := 0; i < n; i++ {
		e := deskkit.Entry{
			TS:     time.Now().UTC().Format(time.RFC3339),
			Tool:   "deskrelease",
			Verb:   "cut:desk-tools/v0.0.1",
			Repo:   repoSlug,
			PR:     nil,
			Result: deskkit.ResultOK,
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed audit: %v", werr)
		}
	}
}

// TestCutGateCountsItsOwnWrites pins deskrelease's scoping against the mutation that
// survived the previous pass: aiming the gate at a bucket cuts never fill.
//
// deskrelease is the tool the first review found running UNBOUNDED — it passed an empty
// repo, which skipped both tiers and left it holding only the breaker, and the breaker
// counts CONSECUTIVE non-progress so a run of successful cuts never trips it. The fix
// (pass repoSlug) was correct but unpinned, which made it the easiest thing in the diff to
// undo silently. This is the test that makes it hard.
func TestCutGateCountsItsOwnWrites(t *testing.T) {
	h := newHarness(t)
	seedCutAudit(t, h, deskkit.RateLimitPerPRPerHour)

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitRateLimited {
		t.Fatalf("cut exit %d, want %d — %d successful cuts in the window must exhaust the "+
			"budget; the gate is reading a bucket cuts do not fill. stderr=%s",
			code, deskkit.ExitRateLimited, deskkit.RateLimitPerPRPerHour, h.errb.String())
	}
	if h.gh.posts != 0 {
		t.Fatalf("a ref was created past an exhausted budget: %v", h.gh.calls())
	}
}

// TestCutUnderBudgetStillCuts is the other half — a budget, not a block. Without it the
// test above is satisfied by a gate that refuses everything.
func TestCutUnderBudgetStillCuts(t *testing.T) {
	h := newHarness(t)
	seedCutAudit(t, h, deskkit.RateLimitPerPRPerHour-1)

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("cut exit %d, want 0 at cap-1; stderr=%s", code, h.errb.String())
	}
}

// TestCutGateIsScopedToItsOwnRepo distinguishes the repo-scoped gate from the tool-wide
// FALLBACK that an empty repo would select.
//
// Needed because the two coincide for this tool in the ordinary case: repoSlug is compiled
// in, so every line deskrelease writes carries the same repo and a tool-wide count sees
// exactly the same set. Reverting the gate to `AllowWrite(toolName, "", 0)` — the original
// unbounded-release bug — therefore passes TestCutGateCountsItsOwnWrites on its own. It is
// no longer unbounded (the empty-repo fallback is a tool-wide budget at the per-PR cap), but
// it IS a different scope, and this is the seed that tells them apart: lines belonging to
// another repo must not consume this repo's release budget.
func TestCutGateIsScopedToItsOwnRepo(t *testing.T) {
	h := newHarness(t)
	dir := filepath.Join(h.home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	for i := 0; i < deskkit.RateLimitPerPRPerHour; i++ {
		e := deskkit.Entry{
			TS: time.Now().UTC().Format(time.RFC3339), Tool: "deskrelease", Verb: "cut:other/v0.0.1",
			Repo: "medici-finance/somewhere-else", Result: deskkit.ResultOK,
		}
		b, _ := json.Marshal(e)
		if _, werr := f.Write(append(b, '\n')); werr != nil {
			t.Fatalf("seed audit: %v", werr)
		}
	}
	f.Close()

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitOK {
		t.Fatalf("cut exit %d, want 0 — %d writes on ANOTHER repo must not consume %s's release "+
			"budget; the gate is not scoped to its own repo. stderr=%s",
			code, deskkit.RateLimitPerPRPerHour, repoSlug, h.errb.String())
	}
}
