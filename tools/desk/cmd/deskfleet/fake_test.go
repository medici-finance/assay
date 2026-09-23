package main

// fake_test.go — an in-process fake GitLab (and GitHub labels endpoint) for deskfleet's
// tests. Nothing here contacts a real forge: every test either installs a transport that
// FAILS on any request, or points the verb at this httptest server on loopback.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	fakeOwnerToken = "example-owner-token"
	fakeGroupPath  = "example-group"
	fakeGroupID    = 77
	fakeProject    = "example-group/example-project"
	fakeProjectID  = 900
	fakePrefix     = "example"
)

// fakeToken is the obviously-fake PAT value the fake mints for the n-th mint (1-based).
func fakeToken(n int) string { return fmt.Sprintf("example-token-%d", n) }

type fakeReq struct {
	Method, Path string // Path is the ESCAPED path, no query
	Body         map[string]any
	Auth         string
}

type fakeRule struct {
	push, merge int
	pushUser    int64
	force       bool
}

type fakeForge struct {
	t   *testing.T
	mu  sync.Mutex
	srv *httptest.Server

	plan       string
	accounts   map[string]int64 // username -> id
	nextUserID int64
	members    map[int64]int
	mints      int
	failMintAt int // 1-based mint number that fails; 0 = never
	// failMintStatus is the status the failing mint answers (0 = 500). failMintHangup instead
	// drops the connection without any reply — a transport error with the outcome unknown.
	failMintStatus int
	failMintHangup bool
	// failMemberPost refuses every group-membership POST (403), after the account exists.
	failMemberPost bool
	// omitMergeLevels answers protected_branches/main with an EMPTY merge_access_levels.
	omitMergeLevels bool
	// avatars: auth header -> "<filename>:<content>" of each PUT /user/avatar; avatarStatus is
	// its reply (0 = 200).
	avatars      map[string]string
	avatarStatus int
	rule         *fakeRule
	refuseIntent bool // refuse a protected-branch POST that carries the INTENDED rule
	deleteStatus int  // 0 = 204
	approvalsErr bool
	tagsPostErr  bool
	tags         []string
	labels       map[string]bool // gitlab labels present
	ghLabels     map[string]bool // github labels present
	gitlab400Dup bool            // answer a gitlab duplicate with 400 instead of 409

	reqs []fakeReq
	// unprotectedObserved counts GET protected_branches/main reads that found no rule.
	unprotectedObserved int
}

func newFakeForge(t *testing.T) *fakeForge {
	f := &fakeForge{t: t, plan: "premium", accounts: map[string]int64{}, nextUserID: 1000,
		members: map[int64]int{}, labels: map[string]bool{}, ghLabels: map[string]bool{}, avatars: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeForge) base() string { return f.srv.URL + "/api/v4" }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeForge) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	auth := r.Header.Get("PRIVATE-TOKEN")
	if auth == "" {
		auth = r.Header.Get("Authorization")
	}
	p := r.URL.EscapedPath()
	f.reqs = append(f.reqs, fakeReq{Method: r.Method, Path: p, Body: body, Auth: auth})

	// The avatar endpoint: multipart, authenticated as the ROLE's own token.
	if r.Method == "PUT" && p == "/api/v4/user/avatar" {
		r.Body = io.NopCloser(bytes.NewReader(raw))
		file, hdr, err := r.FormFile("avatar")
		if err != nil {
			f.t.Errorf("fake forge: avatar upload is not a multipart 'avatar' file: %v", err)
			writeJSON(w, 400, map[string]any{"message": "bad avatar"})
			return
		}
		content, _ := io.ReadAll(file)
		f.avatars[auth] = hdr.Filename + ":" + string(content)
		st := f.avatarStatus
		if st == 0 {
			st = 200
		}
		writeJSON(w, st, map[string]any{"avatar_url": "https://gitlab.example.com/avatar.png"})
		return
	}

	// GitHub labels endpoint.
	if strings.HasPrefix(p, "/repos/") && strings.HasSuffix(p, "/labels") && r.Method == "POST" {
		name, _ := body["name"].(string)
		if f.ghLabels[name] {
			writeJSON(w, 422, map[string]any{"message": "Validation Failed",
				"errors": []map[string]any{{"resource": "Label", "code": "already_exists", "field": "name"}}})
			return
		}
		f.ghLabels[name] = true
		writeJSON(w, 201, map[string]any{"name": name})
		return
	}

	api := strings.TrimPrefix(p, "/api/v4")
	gid := "/groups/" + strconv.Itoa(fakeGroupID)
	pid := "/projects/" + strconv.Itoa(fakeProjectID)
	switch {
	case r.Method == "GET" && api == "/groups/"+fakeGroupPath:
		writeJSON(w, 200, map[string]any{"id": fakeGroupID, "plan": f.plan})
	case r.Method == "GET" && api == gid+"/service_accounts":
		var out []map[string]any
		for u, id := range f.accounts {
			out = append(out, map[string]any{"id": id, "username": u})
		}
		writeJSON(w, 200, out)
	case r.Method == "POST" && api == gid+"/service_accounts":
		u, _ := body["username"].(string)
		f.nextUserID++
		f.accounts[u] = f.nextUserID
		writeJSON(w, 201, map[string]any{"id": f.nextUserID, "username": u})
	case r.Method == "GET" && strings.HasPrefix(api, gid+"/members/"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(api, gid+"/members/"), 10, 64)
		if lvl, ok := f.members[id]; ok {
			writeJSON(w, 200, map[string]any{"id": id, "access_level": lvl})
		} else {
			writeJSON(w, 404, map[string]any{"message": "404 Not found"})
		}
	case r.Method == "POST" && api == gid+"/members":
		if f.failMemberPost {
			writeJSON(w, 403, map[string]any{"message": "403 Forbidden"})
			return
		}
		id := int64(body["user_id"].(float64))
		f.members[id] = int(body["access_level"].(float64))
		writeJSON(w, 201, map[string]any{"id": id})
	case r.Method == "POST" && strings.HasPrefix(api, gid+"/service_accounts/") &&
		strings.HasSuffix(api, "/personal_access_tokens"):
		f.mints++
		if f.failMintAt != 0 && f.mints == f.failMintAt {
			if f.failMintHangup {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
				return
			}
			st := f.failMintStatus
			if st == 0 {
				st = 500
			}
			writeJSON(w, st, map[string]any{"message": http.StatusText(st)})
			return
		}
		writeJSON(w, 201, map[string]any{"id": 5000 + f.mints, "name": body["name"],
			"token": fakeToken(f.mints), "expires_at": body["expires_at"], "scopes": body["scopes"]})
	case r.Method == "GET" && api == "/projects/"+strings.ReplaceAll(fakeProject, "/", "%2F"):
		writeJSON(w, 200, map[string]any{"id": fakeProjectID})
	case r.Method == "GET" && api == pid+"/protected_branches/main":
		if f.rule == nil {
			f.unprotectedObserved++
			writeJSON(w, 404, map[string]any{"message": "404 Not found"})
			return
		}
		push := map[string]any{"access_level": f.rule.push}
		if f.rule.pushUser != 0 {
			push["user_id"] = f.rule.pushUser
		}
		merge := []any{map[string]any{"access_level": f.rule.merge}}
		if f.omitMergeLevels {
			merge = []any{}
		}
		writeJSON(w, 200, map[string]any{"name": "main", "push_access_levels": []any{push},
			"merge_access_levels": merge, "allow_force_push": f.rule.force})
	case r.Method == "PATCH" && api == pid+"/protected_branches/main":
		if f.rule == nil {
			writeJSON(w, 404, nil)
			return
		}
		f.rule.force = false
		writeJSON(w, 200, map[string]any{})
	case r.Method == "DELETE" && api == pid+"/protected_branches/main":
		st := f.deleteStatus
		if st == 0 {
			st = 204
		}
		if st == 204 {
			f.rule = nil
		}
		w.WriteHeader(st)
	case r.Method == "POST" && api == pid+"/protected_branches":
		if f.rule != nil {
			writeJSON(w, 409, map[string]any{"message": "Protected branch 'main' already exists"})
			return
		}
		nr := ruleFromBody(body)
		if f.refuseIntent && nr.merge == mergeAccessLevel && !nr.force {
			writeJSON(w, 403, map[string]any{"message": "403 Forbidden"})
			return
		}
		f.rule = &nr
		writeJSON(w, 201, map[string]any{"name": "main"})
	case r.Method == "POST" && api == pid+"/approvals":
		if f.approvalsErr {
			writeJSON(w, 403, map[string]any{"message": "403 Forbidden"})
			return
		}
		writeJSON(w, 201, map[string]any{})
	case r.Method == "GET" && api == pid+"/approvals":
		writeJSON(w, 200, map[string]any{"merge_requests_author_approval": false,
			"merge_requests_disable_committers_approval": true, "merge_request_approvers_available": true})
	case r.Method == "GET" && api == pid+"/protected_tags":
		var out []map[string]any
		for _, n := range f.tags {
			out = append(out, map[string]any{"name": n})
		}
		writeJSON(w, 200, out)
	case r.Method == "POST" && api == pid+"/protected_tags":
		if f.tagsPostErr {
			writeJSON(w, 403, map[string]any{"message": "403 Forbidden"})
			return
		}
		f.tags = append(f.tags, body["name"].(string))
		writeJSON(w, 201, map[string]any{})
	case r.Method == "PUT" && api == pid:
		writeJSON(w, 200, map[string]any{})
	case r.Method == "GET" && api == pid:
		writeJSON(w, 200, map[string]any{"id": fakeProjectID, "only_allow_merge_if_pipeline_succeeds": true,
			"only_allow_merge_if_all_discussions_are_resolved": true})
	case r.Method == "POST" && strings.HasPrefix(api, "/projects/") && strings.HasSuffix(api, "/labels"):
		name, _ := body["name"].(string)
		if f.labels[name] {
			if f.gitlab400Dup {
				writeJSON(w, 400, map[string]any{"message": map[string]any{"title": []string{"has already been taken"}}})
			} else {
				writeJSON(w, 409, map[string]any{"message": "Label already exists"})
			}
			return
		}
		f.labels[name] = true
		writeJSON(w, 201, map[string]any{"name": name})
	default:
		f.t.Errorf("fake forge: unexpected request %s %s", r.Method, p)
		writeJSON(w, 404, map[string]any{"message": "unexpected"})
	}
}

func ruleFromBody(b map[string]any) fakeRule {
	r := fakeRule{}
	if v, ok := b["push_access_level"].(float64); ok {
		r.push = int(v)
	}
	if v, ok := b["merge_access_level"].(float64); ok {
		r.merge = int(v)
	}
	if v, ok := b["allow_force_push"].(bool); ok {
		r.force = v
	}
	if arr, ok := b["allowed_to_push"].([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			if v, ok := m["user_id"].(float64); ok {
				r.pushUser = int64(v)
			}
		}
	}
	if arr, ok := b["allowed_to_merge"].([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			if v, ok := m["access_level"].(float64); ok {
				r.merge = int(v)
			}
		}
	}
	return r
}

func (f *fakeForge) requests() []fakeReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeReq(nil), f.reqs...)
}

// failingTransport fails the test on ANY request — the instrument for "zero network calls".
type failingTransport struct {
	t     *testing.T
	calls int
	mu    sync.Mutex
}

func (ft *failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ft.mu.Lock()
	ft.calls++
	ft.mu.Unlock()
	ft.t.Errorf("network call attempted: %s %s", r.Method, r.URL.Redacted())
	return nil, fmt.Errorf("network disabled in this test")
}

// firstContactTransport records what the run had printed to stdout at the moment of its FIRST
// request, then forwards every request to the real transport (the loopback fake).
type firstContactTransport struct {
	out      *bytes.Buffer
	mu       sync.Mutex
	seen     bool
	atFirst  string
	requests int
}

func (ft *firstContactTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ft.mu.Lock()
	if !ft.seen {
		ft.seen, ft.atFirst = true, ft.out.String()
	}
	ft.requests++
	ft.mu.Unlock()
	return http.DefaultTransport.RoundTrip(r)
}

// harness is one test's env: captured output, fake clock, a temp config home, and the
// production createRestricted / classifyCustody unless a test overrides them.
type harness struct {
	e          *env
	out, err   *bytes.Buffer
	dir        string
	ownerFile  string
	apiBase    string
	forge      *fakeForge
	classified []string
}

func newHarness(t *testing.T, f *fakeForge) *harness {
	t.Helper()
	h := &harness{out: &bytes.Buffer{}, err: &bytes.Buffer{}, dir: t.TempDir(), forge: f}
	h.ownerFile = filepath.Join(t.TempDir(), "owner.token")
	if err := os.WriteFile(h.ownerFile, []byte(fakeOwnerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if f != nil {
		h.apiBase = f.base()
	}
	h.e = &env{
		stdout:           h.out,
		stderr:           h.err,
		http:             &http.Client{Timeout: 10 * time.Second},
		getenv:           func(k string) string { return map[string]string{"GITLAB_API_BASE": h.apiBase}[k] },
		now:              func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) },
		createRestricted: createRestricted,
		githubAPIBase:    "https://api.github.invalid",
	}
	if f != nil {
		h.e.githubAPIBase = f.srv.URL
	}
	h.e.classifyCustody = func(path string) deskkit.CustodyVerdict {
		h.classified = append(h.classified, path)
		return deskkit.ClassifyCustodyOwnerOnly(path)
	}
	return h
}

func (h *harness) provisionArgs(extra ...string) []string {
	args := []string{"provision", "--group", fakeGroupPath, "--prefix", fakePrefix,
		"--owner-token-file", h.ownerFile, "--out-dir", h.dir}
	return append(args, extra...)
}

// allText is every byte the run wrote to stdout and stderr plus every non-token file it left
// in the out dir (the partial-run report).
func (h *harness) allText(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(h.out.String())
	b.WriteString(h.err.String())
	ents, _ := os.ReadDir(h.dir)
	for _, en := range ents {
		if strings.HasSuffix(en.Name(), ".token") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(h.dir, en.Name()))
		if err == nil {
			b.Write(data)
		}
	}
	return b.String()
}
