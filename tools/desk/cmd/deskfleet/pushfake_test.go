package main

// pushfake_test.go — a stateful in-process fake GitLab for `push-token` (forge-gitlab/18). It
// is separate from fake_test.go's provisioning fake because rotation needs state the
// provisioning routes never model: tokens that rotate, revoke and expire, a variable whose
// update RESETS any flag the request omits, and the receive-pack advertisement. It encodes the
// vendor behaviour the brief documents; rows 19 and 20 check that behaviour live. Every value
// here is an obviously fake placeholder, and nothing contacts a real forge.

import (
	"bytes"
	"encoding/base64"
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

	"github.com/medici-finance/assay/tools/desk/internal/custodytest"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	ptRotator   = "example-rotator-cred"
	ptReader    = "example-reader-cred"
	ptOld       = "example-push-old"
	ptName      = "assay-statusgen-push"
	ptOldID     = 6001
	ptProjectID = 900
	ptBotUser   = 3001
)

// ptNew is the fake value the fake issues for token id.
func ptNew(id int64) string { return fmt.Sprintf("example-push-new-%d", id) }

var ptToday = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func ptDate(days int) string { return ptToday.AddDate(0, 0, days).Format("2006-01-02") }

type ptTok struct {
	id              int64
	name            string
	active, revoked bool
	scopes          []string
	level           int
	expires         string
	value           string
}

type ptVar struct {
	exists, masked, protected, hidden bool
	scope, value                      string
}

type ptReq struct {
	Method, Path, Query string
	Body                map[string]any
	Raw                 string // the raw request body
	Private             string // PRIVATE-TOKEN header
	Basic               string // the HTTP Basic password, decoded
	BasicUser           string
	HasAuthorization    bool
}

type ptFake struct {
	t   *testing.T
	srv *httptest.Server
	mu  sync.Mutex

	prefix          string // a path prefix the instance is served under ("" or "/prefix")
	branchProtected bool
	cloneHost       string // the host the project record's clone URL names
	v               ptVar
	toks            []*ptTok
	nextID          int64
	supportsHidden  bool

	// faults
	listStatus    int
	rotateStatus  int
	dropRotate    bool // perform the rotate, then drop the connection
	writeStatuses []int
	writeMsgEcho  bool                             // a failing write's message quotes the submitted value
	storeOverride func(v *ptVar)                   // applied after every variable write (the forge stores other flags)
	readValue     string                           // a read-back answers this value instead of the stored one
	revokeStatus  int                              // non-zero: DELETE answers it and does nothing
	dropRevoke    bool                             // DELETE drops the connection and does nothing
	revokeNoop    bool                             // DELETE answers 204 and does nothing
	byIDMutate    func(id int64, m map[string]any) // applied to every by-id answer
	selfStatus    int
	selfBody      string
	selfID        int64
	pushStatus    int
	pushLocation  string
	requestsLog   []ptReq
}

func newPTFake(t *testing.T) *ptFake {
	f := &ptFake{t: t, branchProtected: true, nextID: 7001, supportsHidden: true,
		v: ptVar{exists: true, masked: true, protected: true, scope: "*", value: ptOld},
		toks: []*ptTok{{id: ptOldID, name: ptName, active: true, scopes: []string{"write_repository"}, level: 40,
			expires: ptDate(3), value: ptOld}}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	f.cloneHost = f.srv.URL
	return f
}

func (f *ptFake) base() string { return f.srv.URL + f.prefix + "/api/v4" }

func (f *ptFake) requests() []ptReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ptReq(nil), f.requestsLog...)
}

// count returns how many recorded requests have method and a path ending in suffix.
func (f *ptFake) count(method, suffix string) int {
	n := 0
	for _, r := range f.requests() {
		if r.Method == method && strings.HasSuffix(r.Path, suffix) {
			n++
		}
	}
	return n
}

func (f *ptFake) writes() int {
	n := 0
	for _, r := range f.requests() {
		if r.Method != "GET" {
			n++
		}
	}
	return n
}

// activeByName is the fake's own count of active, unrevoked tokens by name.
func (f *ptFake) activeByName() []*ptTok {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*ptTok
	for _, t := range f.toks {
		if t.name == ptName && t.active && !t.revoked {
			out = append(out, t)
		}
	}
	return out
}

func (f *ptFake) tok(id int64) *ptTok {
	for _, t := range f.toks {
		if t.id == id {
			return t
		}
	}
	return nil
}

func ptRecord(t *ptTok) map[string]any {
	return map[string]any{"id": t.id, "name": t.name, "active": t.active, "revoked": t.revoked,
		"scopes": t.scopes, "access_level": t.level, "expires_at": t.expires, "user_id": ptBotUser}
}

func ptJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ptDrop(w http.ResponseWriter) {
	if hj, ok := w.(http.Hijacker); ok {
		if c, _, err := hj.Hijack(); err == nil {
			_ = c.Close()
		}
	}
}

func (f *ptFake) varJSON() map[string]any {
	m := map[string]any{"key": pushTokenVariable, "masked": f.v.masked, "protected": f.v.protected,
		"hidden": f.v.hidden, "environment_scope": f.v.scope, "variable_type": "env_var"}
	switch {
	case f.v.hidden:
		m["value"] = nil
	case f.readValue != "":
		m["value"] = f.readValue
	default:
		m["value"] = f.v.value
	}
	return m
}

func (f *ptFake) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	req := ptReq{Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, Raw: string(raw),
		Private: r.Header.Get("PRIVATE-TOKEN"), HasAuthorization: r.Header.Get("Authorization") != ""}
	if u, p, ok := r.BasicAuth(); ok {
		req.BasicUser, req.Basic = u, p
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req.Body)
	}
	f.mu.Lock()
	f.requestsLog = append(f.requestsLog, req)
	f.mu.Unlock()

	p := strings.TrimPrefix(req.Path, f.prefix)
	if strings.HasSuffix(p, ".git/info/refs") {
		f.handlePush(w, r, req)
		return
	}
	if !strings.HasPrefix(p, "/api/v4/") {
		f.t.Errorf("fake: unexpected request %s %s", r.Method, req.Path)
		w.WriteHeader(404)
		return
	}
	p = strings.TrimPrefix(p, "/api/v4")
	if p == "/personal_access_tokens/self" {
		f.handleSelf(w, req)
		return
	}
	if req.Private != ptRotator && req.Private != ptReader {
		ptJSON(w, 401, map[string]any{"message": "401 Unauthorized"})
		return
	}
	writer := req.Private == ptRotator
	f.mu.Lock()
	defer f.mu.Unlock()
	pp := "/projects/" + strconv.Itoa(ptProjectID)
	switch {
	case r.Method == "GET" && (p == "/projects/"+strings.ReplaceAll(fakeProject, "/", "%2F") || p == pp):
		ptJSON(w, 200, map[string]any{"id": ptProjectID, "path_with_namespace": fakeProject,
			"default_branch": "main", "http_url_to_repo": f.cloneHost + "/" + fakeProject + ".git"})
	case r.Method == "GET" && p == pp+"/protected_branches/main":
		if f.branchProtected {
			ptJSON(w, 200, map[string]any{"name": "main"})
		} else {
			ptJSON(w, 404, map[string]any{"message": "404 Not found"})
		}
	case p == pp+"/variables/"+pushTokenVariable:
		f.handleVar(w, r, req, writer)
	case r.Method == "POST" && p == pp+"/variables":
		f.handleVar(w, r, req, writer)
	case r.Method == "GET" && p == pp+"/access_tokens":
		if f.listStatus != 0 {
			ptJSON(w, f.listStatus, map[string]any{"message": "boom"})
			return
		}
		out := []map[string]any{}
		for _, t := range f.toks {
			out = append(out, ptRecord(t))
		}
		ptJSON(w, 200, out)
	case r.Method == "POST" && p == pp+"/access_tokens" && writer:
		t := &ptTok{id: f.nextID, name: fmt.Sprint(req.Body["name"]), active: true, level: int(num(req.Body["access_level"])),
			expires: fmt.Sprint(req.Body["expires_at"])}
		for _, s := range req.Body["scopes"].([]any) {
			t.scopes = append(t.scopes, fmt.Sprint(s))
		}
		f.nextID++
		t.value = ptNew(t.id)
		f.toks = append(f.toks, t)
		rec := ptRecord(t)
		rec["token"] = t.value
		ptJSON(w, 201, rec)
	case strings.HasPrefix(p, pp+"/access_tokens/"):
		f.handleTok(w, r, req, strings.TrimPrefix(p, pp+"/access_tokens/"), writer)
	default:
		f.t.Errorf("fake: unexpected request %s %s", r.Method, req.Path)
		w.WriteHeader(404)
	}
}

func num(v any) float64 {
	n, _ := v.(float64)
	return n
}

func (f *ptFake) handleVar(w http.ResponseWriter, r *http.Request, req ptReq, writer bool) {
	if r.Method == "GET" {
		if !f.v.exists {
			ptJSON(w, 404, map[string]any{"message": "404 Variable Not Found"})
			return
		}
		ptJSON(w, 200, f.varJSON())
		return
	}
	if !writer {
		ptJSON(w, 403, map[string]any{"message": "403 Forbidden"})
		return
	}
	if len(f.writeStatuses) > 0 {
		st := f.writeStatuses[0]
		f.writeStatuses = f.writeStatuses[1:]
		msg := "boom"
		if f.writeMsgEcho {
			msg = "value " + fmt.Sprint(req.Body["value"]) + " is invalid"
		}
		ptJSON(w, st, map[string]any{"message": msg})
		return
	}
	created := r.Method == "POST"
	if created && f.v.exists {
		ptJSON(w, 400, map[string]any{"message": "already exists"})
		return
	}
	// update RESETS any flag the request omits (the vendor behaviour the verb defends against).
	f.v.exists = true
	f.v.value = fmt.Sprint(req.Body["value"])
	f.v.masked = req.Body["masked"] == true
	f.v.protected = req.Body["protected"] == true
	if s, ok := req.Body["environment_scope"].(string); ok {
		f.v.scope = s
	} else {
		f.v.scope = "*"
	}
	if created && f.supportsHidden && req.Body["masked_and_hidden"] == true {
		f.v.hidden, f.v.masked = true, true
	}
	if f.storeOverride != nil {
		f.storeOverride(&f.v)
	}
	m := f.varJSON()
	if !f.v.hidden {
		m["value"] = f.v.value // the write response echoes the value
	}
	if created {
		ptJSON(w, 201, m)
	} else {
		ptJSON(w, 200, m)
	}
}

func (f *ptFake) handleTok(w http.ResponseWriter, r *http.Request, req ptReq, rest string, writer bool) {
	idStr, action, _ := strings.Cut(rest, "/")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	t := f.tok(id)
	switch {
	case r.Method == "GET" && action == "":
		if t == nil {
			ptJSON(w, 404, map[string]any{"message": "404 Not Found"})
			return
		}
		rec := ptRecord(t)
		if f.byIDMutate != nil {
			f.byIDMutate(id, rec)
		}
		ptJSON(w, 200, rec)
	case r.Method == "POST" && action == "rotate" && writer:
		if f.rotateStatus != 0 {
			ptJSON(w, f.rotateStatus, map[string]any{"message": "example-forge-message"})
			return
		}
		if t == nil || !t.active || t.revoked {
			ptJSON(w, 400, map[string]any{"message": "token already revoked"})
			return
		}
		t.active, t.revoked = false, true
		nt := &ptTok{id: f.nextID, name: t.name, active: true, scopes: t.scopes, level: t.level,
			expires: fmt.Sprint(req.Body["expires_at"])}
		f.nextID++
		nt.value = ptNew(nt.id)
		f.toks = append(f.toks, nt)
		if f.dropRotate {
			f.dropRotate = false
			ptDrop(w)
			return
		}
		rec := ptRecord(nt)
		rec["token"] = nt.value
		ptJSON(w, 200, rec)
	case r.Method == "DELETE" && action == "" && writer:
		switch {
		case f.dropRevoke:
			ptDrop(w)
		case f.revokeStatus != 0:
			ptJSON(w, f.revokeStatus, map[string]any{"message": "boom"})
		case f.revokeNoop:
			w.WriteHeader(204)
		case t == nil:
			ptJSON(w, 404, map[string]any{"message": "404 Not Found"})
		default:
			t.active, t.revoked = false, true
			w.WriteHeader(204)
		}
	default:
		f.t.Errorf("fake: unexpected token request %s %s", r.Method, req.Path)
		w.WriteHeader(404)
	}
}

func (f *ptFake) handleSelf(w http.ResponseWriter, req ptReq) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.selfStatus != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.selfStatus)
		_, _ = io.WriteString(w, f.selfBody)
		return
	}
	for _, t := range f.toks {
		if t.value == req.Private && t.active && !t.revoked {
			rec := ptRecord(t)
			if f.selfID != 0 {
				rec["id"] = f.selfID
			}
			ptJSON(w, 200, rec)
			return
		}
	}
	ptJSON(w, 401, map[string]any{"message": "401 Unauthorized"})
}

func (f *ptFake) handlePush(w http.ResponseWriter, r *http.Request, req ptReq) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Query().Get("service") != "git-receive-pack" {
		w.WriteHeader(400)
		return
	}
	if f.pushStatus != 0 {
		if f.pushLocation != "" {
			w.Header().Set("Location", f.pushLocation)
		}
		w.WriteHeader(f.pushStatus)
		return
	}
	for _, t := range f.toks {
		if req.Basic != "" && t.value == req.Basic && t.active && !t.revoked {
			w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
			w.WriteHeader(200)
			_, _ = io.WriteString(w, "001f# service=git-receive-pack\n0000")
			return
		}
	}
	w.WriteHeader(401)
}

// ptHarness runs the verb against a ptFake with owner-only credential files.
type ptHarness struct {
	e           *env
	out, err    *bytes.Buffer
	rot, reader string
	envs        map[string]string
	forge       *ptFake
	tmp         string
}

func newPTHarness(t *testing.T, f *ptFake) *ptHarness {
	t.Helper()
	dir := custodytest.PrivateTempDir(t)
	h := &ptHarness{out: &bytes.Buffer{}, err: &bytes.Buffer{}, forge: f,
		rot: filepath.Join(dir, "rotator.cred"), reader: filepath.Join(dir, "reader.cred"),
		envs: map[string]string{}, tmp: t.TempDir()}
	for path, v := range map[string]string{h.rot: ptRotator, h.reader: ptReader} {
		if err := os.WriteFile(path, []byte(v+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if f != nil {
		h.envs["GITLAB_API_BASE"] = f.base()
	}
	t.Setenv("TMPDIR", h.tmp)
	h.e = &env{
		stdout: h.out, stderr: h.err,
		http: &http.Client{Timeout: 10 * time.Second},
		lookupEnv: func(k string) (string, bool) {
			v, ok := h.envs[k]
			return v, ok
		},
		now:              func() time.Time { return ptToday },
		createRestricted: createRestricted,
		classifyCustody:  deskkit.ClassifyCustodyOwnerOnly,
		githubAPIBase:    "https://api.github.invalid",
	}
	return h
}

func (h *ptHarness) rotate(extra ...string) int {
	h.out.Reset()
	h.err.Reset()
	args := append([]string{"push-token", "rotate", "--project", fakeProject, "--rotator-token-file", h.rot,
		"--token-name", ptName}, extra...)
	return run(args, h.e)
}

func (h *ptHarness) check(extra ...string) int {
	h.out.Reset()
	h.err.Reset()
	args := append([]string{"push-token", "check", "--project", fakeProject, "--reader-token-file", h.reader,
		"--token-name", ptName}, extra...)
	return run(args, h.e)
}

func (h *ptHarness) text() string { return h.out.String() + h.err.String() }

// tmpText is every file created under TMPDIR during the run.
func (h *ptHarness) tmpText() string {
	var b strings.Builder
	_ = filepath.Walk(h.tmp, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			if data, rerr := os.ReadFile(p); rerr == nil {
				b.Write(data)
			}
		}
		return nil
	})
	return b.String()
}

func basicPass(hdr string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(hdr, "Basic "))
	if err != nil {
		return ""
	}
	_, p, _ := strings.Cut(string(raw), ":")
	return p
}
