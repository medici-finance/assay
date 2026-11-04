package main

// pushrotate_test.go — `push-token rotate|check` (#2910 part B) against the stateful fake in
// pushfake_test.go. Each test is one row of the brief's Verify table. Every value is an
// obviously fake placeholder; nothing contacts a real forge.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// issuedValues is every value the fake issued (one per token it created by rotate or create).
func (f *ptFake) issuedValues() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, t := range f.toks {
		if t.id != ptOldID {
			out = append(out, t.value)
		}
	}
	return out
}

// ptNoSentinel fails if any sentinel reaches the run's output, the process environment, or a
// file created under TMPDIR.
func ptNoSentinel(t *testing.T, h *ptHarness, label string) {
	t.Helper()
	sentinels := append([]string{ptRotator, ptOld}, h.forge.issuedValues()...)
	env := strings.Join(os.Environ(), "\n")
	for _, s := range sentinels {
		for where, text := range map[string]string{"output": h.text(), "environment": env, "TMPDIR": h.tmpText()} {
			if strings.Contains(text, s) {
				t.Errorf("%s: a sentinel reached the %s", label, where)
			}
		}
	}
}

// regression: #2910
func TestPushTokenNeverEscapes(t *testing.T) {
	t.Run("full rotate", func(t *testing.T) {
		f := newPTFake(t)
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d want 0\n%s", rc, h.text())
		}
		ptNoSentinel(t, h, "full rotate")
		val := ptNew(7001)
		var places []string
		for _, r := range f.requests() {
			if strings.Contains(r.Raw, val) {
				places = append(places, "body "+r.Method+" "+r.Path)
			}
			if r.Private == val {
				places = append(places, "private "+r.Path)
			}
			if r.Basic == val {
				places = append(places, "basic "+r.Path)
			}
			if strings.Contains(r.Path+r.Query, val) {
				places = append(places, "url "+r.Path)
			}
		}
		want := []string{"body PUT /api/v4/projects/900/variables/STATUSGEN_PUSH_TOKEN",
			"private /api/v4/personal_access_tokens/self",
			"basic /" + fakeProject + ".git/info/refs"}
		if strings.Join(places, "|") != strings.Join(want, "|") {
			t.Errorf("the new value reached %q, want exactly %q", places, want)
		}
	})
	t.Run("rotated-not-installed", func(t *testing.T) {
		f := newPTFake(t)
		f.writeStatuses = []int{500, 500, 500}
		f.writeMsgEcho = true
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "rotated-not-installed") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		ptNoSentinel(t, h, "not installed")
	})
	t.Run("write 400 quoting the value", func(t *testing.T) {
		f := newPTFake(t)
		f.writeStatuses = []int{400}
		f.writeMsgEcho = true
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		ptNoSentinel(t, h, "write 400")
	})
	t.Run("contained", func(t *testing.T) {
		f := newPTFake(t)
		f.storeOverride = func(v *ptVar) { v.protected = false }
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "contained: revoked") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		ptNoSentinel(t, h, "contained")
	})
	t.Run("containment-failed", func(t *testing.T) {
		f := newPTFake(t)
		f.storeOverride = func(v *ptVar) { v.protected = false }
		f.revokeStatus = 500
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitUnverifiable || !strings.Contains(h.text(), "containment-failed") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		ptNoSentinel(t, h, "containment-failed")
	})
}

// pushGuardFiles are the files whose calls the source guard checks (os/exec is checked
// package-wide).
var pushGuardFiles = []string{"pushtoken.go", "pushrotate.go", "pushcli.go"}

// pushSourceViolations walks one parsed file: an os/exec import anywhere, and (when calls is
// set) any os.Setenv, os.WriteFile, os.Create or os.OpenFile.
func pushSourceViolations(f *ast.File, calls bool) []string {
	var out []string
	osName := ""
	for _, im := range f.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		if p == "os/exec" {
			out = append(out, "import os/exec")
		}
		if p == "os" {
			osName = "os"
			if im.Name != nil {
				osName = im.Name.Name
			}
		}
	}
	if !calls || osName == "" {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == osName {
			switch sel.Sel.Name {
			case "Setenv", "WriteFile", "Create", "OpenFile":
				out = append(out, "os."+sel.Sel.Name)
			}
		}
		return true
	})
	return out
}

// regression: #2910
func TestPushTokenSourceGuard(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	guarded := map[string]bool{}
	for _, g := range pushGuardFiles {
		guarded[g] = true
	}
	seen := 0
	for _, en := range ents {
		n := en.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), n, nil, parser.ImportsOnly|parser.ParseComments)
		if guarded[n] {
			f, err = parser.ParseFile(token.NewFileSet(), n, nil, 0)
			seen++
		}
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if v := pushSourceViolations(f, guarded[n]); len(v) > 0 {
			t.Errorf("%s: %v", n, v)
		}
	}
	if seen != len(pushGuardFiles) {
		t.Fatalf("walked %d of the %d guarded files", seen, len(pushGuardFiles))
	}
	// Negative path: a planted fixture reports every forbidden call and the import.
	const fixture = `package main
import (
	"os"
	"os/exec"
)
func planted() {
	_ = os.Setenv("A", "b")
	_ = os.WriteFile("f", nil, 0o600)
	_, _ = os.Create("f")
	_, _ = os.OpenFile("f", 0, 0)
	_ = exec.Command
}`
	pf, err := parser.ParseFile(token.NewFileSet(), "fixture.go", fixture, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := pushSourceViolations(pf, true)
	sort.Strings(got)
	want := []string{"import os/exec", "os.Create", "os.OpenFile", "os.Setenv", "os.WriteFile"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("fixture: got %v, want %v", got, want)
	}
}

// regression: #2910
func TestPushTokenRedacts(t *testing.T) {
	const sentinel = "example-redact-sentinel"
	s := newPushSecret(sentinel)
	wrap := struct {
		Name string
		V    pushSecret
	}{"x", s}
	v := pushVariable{Key: "K", Value: s}
	var renders []string
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		renders = append(renders, fmt.Sprintf(verb, s), fmt.Sprintf(verb, &s), fmt.Sprintf(verb, wrap),
			fmt.Sprintf(verb, v))
	}
	renders = append(renders, s.String(), s.GoString(), fmt.Sprint(s), fmt.Sprintln(s))
	for _, x := range []any{s, wrap, v} {
		b, err := json.Marshal(x)
		if err != nil {
			t.Fatal(err)
		}
		renders = append(renders, string(b))
	}
	tb, _ := s.MarshalText()
	renders = append(renders, string(tb))
	for i, r := range renders {
		if strings.Contains(r, sentinel) {
			t.Errorf("render %d exposed the value: %s", i, r)
		}
		if !strings.Contains(r, redactedText) {
			t.Errorf("render %d did not say %s: %s", i, redactedText, r)
		}
	}
	if s.reveal() != sentinel {
		t.Error("reveal must return the bytes")
	}
}

// regression: #2910
func TestPushTokenRefusesInputs(t *testing.T) {
	type tc struct {
		name  string
		mode  string // "rotate" | "check"
		setup func(h *ptHarness)
		args  []string
		want  int
	}
	cases := []tc{
		{name: "base unset", mode: "rotate", setup: func(h *ptHarness) { delete(h.envs, "GITLAB_API_BASE") }, want: exitRefused},
		{name: "plain http base", mode: "rotate", setup: func(h *ptHarness) {
			h.envs["GITLAB_API_BASE"] = "http://gitlab.example.invalid/api/v4"
		}, want: exitRefused},
		{name: "base without /api/v4", mode: "rotate", setup: func(h *ptHarness) {
			h.envs["GITLAB_API_BASE"] = h.forge.srv.URL + "/api"
		}, want: exitRefused},
		{name: "rotator file 0644", mode: "rotate", setup: func(h *ptHarness) { _ = os.Chmod(h.rot, 0o644) }, want: exitRefused},
		{name: "rotator file missing", mode: "rotate", setup: func(h *ptHarness) { _ = os.Remove(h.rot) }, want: exitRefused},
		{name: "reader file 0644", mode: "check", setup: func(h *ptHarness) { _ = os.Chmod(h.reader, 0o644) }, want: exitRefused},
		{name: "days 0", mode: "rotate", args: []string{"--days", "0"}, want: exitRefused},
		{name: "days 91", mode: "rotate", args: []string{"--days", "91"}, want: exitRefused},
		{name: "min-days equals days", mode: "rotate", args: []string{"--days", "10", "--min-days", "10"}, want: exitRefused},
		{name: "session marker set", mode: "rotate", setup: func(h *ptHarness) { h.envs[pushSessionMarker] = "worker-desk" },
			want: exitRefused},
		{name: "value flag", mode: "rotate", args: []string{"--token", "x"}, want: exitUsage},
		{name: "check given rotator file", mode: "check", args: []string{"--rotator-token-file", "f"}, want: exitUsage},
		{name: "rotate given reader file", mode: "rotate", args: []string{"--reader-token-file", "f"}, want: exitUsage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPTFake(t)
			h := newPTHarness(t, f)
			if c.setup != nil {
				c.setup(h)
			}
			run := h.rotate
			if c.mode == "check" {
				run = h.check
			}
			rc := run(c.args...)
			if rc != c.want {
				t.Fatalf("rc=%d want %d\n%s", rc, c.want, h.text())
			}
			if n := len(f.requests()); n != 0 {
				t.Errorf("%d requests reached the fake, want 0", n)
			}
			if c.name == "session marker set" && !strings.Contains(h.text(), pushSessionMarker) {
				t.Errorf("the refusal does not name the session marker:\n%s", h.text())
			}
		})
	}
	t.Run("ambient sentinels, no file", func(t *testing.T) {
		const a, b = "example-ambient-gitlab-sentinel", "example-ambient-private-sentinel"
		f := newPTFake(t)
		h := newPTHarness(t, f)
		h.envs["GITLAB_TOKEN"], h.envs["PRIVATE_TOKEN"] = a, b
		t.Setenv("GITLAB_TOKEN", a)
		t.Setenv("PRIVATE_TOKEN", b)
		h.out.Reset()
		h.err.Reset()
		rc := run([]string{"push-token", "rotate", "--project", fakeProject, "--token-name", ptName}, h.e)
		if rc != exitRefused {
			t.Fatalf("rc=%d want 5\n%s", rc, h.text())
		}
		if n := len(f.requests()); n != 0 {
			t.Errorf("%d requests reached the fake", n)
		}
	})
}

// regression: #2910
func TestPushTokenPreflight(t *testing.T) {
	scopes := func(s ...string) func(*ptFake) { return func(f *ptFake) { f.toks[0].scopes = s } }
	cases := []struct {
		name, want string
		setup      func(f *ptFake)
	}{
		{"unmasked", "masked", func(f *ptFake) { f.v.masked = false }},
		{"unprotected", "protected", func(f *ptFake) { f.v.protected = false }},
		{"scope not *", "environment_scope", func(f *ptFake) { f.v.scope = "production" }},
		{"variable missing", "--create", func(f *ptFake) { f.v.exists = false }},
		{"group-level only", "group-level", func(f *ptFake) { f.v.exists = false }},
		{"no active token", "zero active", func(f *ptFake) { f.toks[0].active = false }},
		{"two active", "2 active", func(f *ptFake) {
			f.toks = append(f.toks, &ptTok{id: 6002, name: ptName, active: true, scopes: []string{"write_repository"},
				level: 40, expires: ptDate(20), value: "example-push-other"})
		}},
		{"scopes widened", "scopes", scopes("write_repository", "api")},
		{"scopes read", "scopes", scopes("read_repository")},
		{"level 30", "access_level", func(f *ptFake) { f.toks[0].level = 30 }},
		{"branch unprotected", "not protected", func(f *ptFake) { f.branchProtected = false }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPTFake(t)
			c.setup(f)
			h := newPTHarness(t, f)
			rc := h.rotate()
			if rc != exitRefused {
				t.Fatalf("rc=%d want 5\n%s", rc, h.text())
			}
			if !strings.Contains(h.text(), c.want) {
				t.Errorf("the refusal does not name %q:\n%s", c.want, h.text())
			}
			if n := f.writes(); n != 0 {
				t.Errorf("%d writes recorded, want 0", n)
			}
			if strings.Contains(h.text(), ptOld) {
				t.Error("a refusal printed the predecessor value")
			}
		})
	}
	t.Run("token list 500", func(t *testing.T) {
		f := newPTFake(t)
		f.listStatus = 500
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitUnverifiable {
			t.Fatalf("rc=%d want 6\n%s", rc, h.text())
		}
		if n := f.writes(); n != 0 {
			t.Errorf("%d writes recorded, want 0", n)
		}
	})
}

// regression: #2910
func TestPushTokenPreservesFlags(t *testing.T) {
	assertWrites := func(t *testing.T, f *ptFake) {
		t.Helper()
		n := 0
		for _, r := range f.requests() {
			if !strings.Contains(r.Path, "/variables") || r.Method == "GET" {
				continue
			}
			n++
			if r.Body["masked"] != true || r.Body["protected"] != true || r.Body["environment_scope"] != "*" {
				t.Errorf("variable write %s lacks masked/protected/scope: %v", r.Method, keysOf(r.Body))
			}
		}
		if n == 0 {
			t.Error("no variable write recorded")
		}
	}
	t.Run("unhidden", func(t *testing.T) {
		f := newPTFake(t)
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		assertWrites(t, f)
		if !f.v.masked || !f.v.protected || f.v.scope != "*" {
			t.Errorf("stored flags lost: %+v", f.v)
		}
	})
	t.Run("hidden stays hidden", func(t *testing.T) {
		f := newPTFake(t)
		f.v.hidden = true
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		assertWrites(t, f)
		if !f.v.hidden {
			t.Error("the variable lost hidden")
		}
		for _, l := range []string{"self: ok", "push: ok", "value=not returned (hidden)"} {
			if !strings.Contains(h.text(), l) {
				t.Errorf("missing %q:\n%s", l, h.text())
			}
		}
	})
	t.Run("create sends hidden", func(t *testing.T) {
		f := newPTFake(t)
		f.v = ptVar{}
		h := newPTHarness(t, f)
		if rc := h.rotate("--create"); rc != exitOK {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		assertWrites(t, f)
		if f.count("POST", "/variables") != 1 {
			t.Fatal("no variable create recorded")
		}
		for _, r := range f.requests() {
			if r.Method == "POST" && strings.HasSuffix(r.Path, "/variables") && r.Body["masked_and_hidden"] != true {
				t.Error("the create body does not carry masked_and_hidden")
			}
		}
		if !f.v.hidden {
			t.Error("the created variable is not hidden")
		}
	})
}

func keysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// regression: #2910
func TestPushTokenReadbackCatches(t *testing.T) {
	cases := []struct {
		field  string
		hidden bool
		store  func(v *ptVar)
		read   string
	}{
		{field: "protected", store: func(v *ptVar) { v.protected = false }},
		{field: "masked", store: func(v *ptVar) { v.masked = false }},
		{field: "environment_scope", store: func(v *ptVar) { v.scope = "production" }},
		{field: "hidden", hidden: true, store: func(v *ptVar) { v.hidden = false }},
		{field: "value", read: "example-push-different"},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			f := newPTFake(t)
			f.v.hidden = c.hidden
			f.storeOverride, f.readValue = c.store, c.read
			h := newPTHarness(t, f)
			rc := h.rotate()
			want := "contained: revoked 7001 — the read-back disagreed on " + c.field
			if rc != exitFailed || !strings.Contains(h.text(), want) {
				t.Fatalf("rc=%d, want 1 and %q\n%s", rc, want, h.text())
			}
			if n := f.count("DELETE", "/access_tokens/7001"); n != 1 {
				t.Errorf("%d revokes of the new id, want 1", n)
			}
			if n := len(f.activeByName()); n != 0 {
				t.Errorf("%d active tokens by name afterwards, want 0", n)
			}
			ptNoProof(t, h, f)
		})
	}
	faults := map[string]func(f *ptFake){
		"revoke 500":     func(f *ptFake) { f.revokeStatus = 500 },
		"revoke dropped": func(f *ptFake) { f.dropRevoke = true },
		"revoke no-op":   func(f *ptFake) { f.revokeNoop = true },
	}
	for name, fault := range faults {
		t.Run(name, func(t *testing.T) {
			f := newPTFake(t)
			f.storeOverride = func(v *ptVar) { v.protected = false }
			fault(f)
			h := newPTHarness(t, f)
			rc := h.rotate()
			if rc != exitUnverifiable || !strings.Contains(h.text(), "containment-failed: revoke 7001 by hand") {
				t.Fatalf("rc=%d want 6 and containment-failed\n%s", rc, h.text())
			}
			if strings.Contains(h.text(), "contained:") {
				t.Errorf("printed a contained: line on a failed containment:\n%s", h.text())
			}
			ptNoProof(t, h, f)
		})
	}
}

// ptNoProof checks a contained (or containment-failed) run stopped there: no proof probe ran
// after the containment, so its exit code is containment's own and never a later probe's.
func ptNoProof(t *testing.T, h *ptHarness, f *ptFake) {
	t.Helper()
	if strings.Contains(h.text(), "by-id:") || strings.Contains(h.text(), "outcome:") {
		t.Errorf("a proof step ran after containment:\n%s", h.text())
	}
	if n := f.count("GET", "/personal_access_tokens/self") + f.count("GET", ".git/info/refs"); n != 0 {
		t.Errorf("%d self/push probes after containment, want 0", n)
	}
}

// regression: #2910
func TestPushTokenProvesLiveness(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		f := newPTFake(t)
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		for _, l := range []string{"by-id: ok", "self: ok", "push: ok", "outcome: rotated"} {
			if !strings.Contains(h.text(), l) {
				t.Errorf("missing %q", l)
			}
		}
	})
	byID := func(mut func(m map[string]any)) func(f *ptFake) {
		return func(f *ptFake) {
			f.byIDMutate = func(id int64, m map[string]any) {
				if id != ptOldID {
					mut(m)
				}
			}
		}
	}
	cases := []struct {
		name, want string
		args       []string
		setup      func(f *ptFake)
	}{
		{"by-id revoked", "by-id: FAIL field revoked", nil, byID(func(m map[string]any) { m["revoked"] = true })},
		{"by-id inactive", "by-id: FAIL field active", nil, byID(func(m map[string]any) { m["active"] = false })},
		{"by-id scopes widened", "by-id: FAIL field scopes", nil,
			byID(func(m map[string]any) { m["scopes"] = []string{"write_repository", "api"} })},
		{"by-id 3 days left", "by-id: FAIL field expires_at", []string{"--min-days", "7"},
			byID(func(m map[string]any) { m["expires_at"] = ptDate(3) })},
		{"by-id 5 days early", "by-id: FAIL field expires_at", nil,
			byID(func(m map[string]any) { m["expires_at"] = ptDate(25) })},
		{"self is predecessor", "self: FAIL field id", nil, func(f *ptFake) { f.selfID = ptOldID }},
		{"self scopes widened", "self: FAIL field scopes", nil, func(f *ptFake) {
			f.selfMutate = func(m map[string]any) { m["scopes"] = []string{"write_repository", "api"} }
		}},
		{"push 401", "push: FAIL field status", nil, func(f *ptFake) { f.pushStatus = 401 }},
		{"push 403", "push: FAIL field status", nil, func(f *ptFake) { f.pushStatus = 403 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPTFake(t)
			c.setup(f)
			h := newPTHarness(t, f)
			rc := h.rotate(c.args...)
			if rc != exitFailed || !strings.Contains(h.text(), c.want) {
				t.Fatalf("rc=%d, want 1 and %q\n%s", rc, c.want, h.text())
			}
			ptNoSentinel(t, h, c.name)
		})
	}
}

// regression: #2910
func TestPushTokenSelfScope(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		want       int
	}{
		{"403 insufficient scope", `{"error":"insufficient_scope"}`, 403, exitOK},
		{"401", `{"message":"401 Unauthorized"}`, 401, exitFailed},
		{"403 other", `{"message":"403 Forbidden"}`, 403, exitFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPTFake(t)
			f.selfStatus, f.selfBody = c.status, c.body
			h := newPTHarness(t, f)
			if rc := h.rotate(); rc != c.want {
				t.Fatalf("rc=%d want %d\n%s", rc, c.want, h.text())
			}
			if c.want == exitOK && !strings.Contains(h.text(), "self: not-permitted-by-scope") {
				t.Errorf("missing the not-permitted-by-scope line:\n%s", h.text())
			}
		})
	}
}

// regression: #2910
func TestPushTokenProbeHostBound(t *testing.T) {
	var bHits atomic.Int64
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		bHits.Add(1)
		w.WriteHeader(200)
	}))
	t.Cleanup(b.Close)
	probes := func(f *ptFake) []ptReq {
		var out []ptReq
		for _, r := range f.requests() {
			if strings.HasSuffix(r.Path, ".git/info/refs") {
				out = append(out, r)
			}
		}
		return out
	}
	assertAuth := func(t *testing.T, ps []ptReq) {
		t.Helper()
		for _, p := range ps {
			if !p.HasAuthorization || p.BasicUser != pushProbeUser || strings.Contains(p.Path, "@") {
				t.Errorf("push probe %s does not carry its credential in Authorization alone", p.Path)
			}
		}
	}
	t.Run("clone URL on B", func(t *testing.T) {
		bHits.Store(0)
		f := newPTFake(t)
		f.cloneHost = b.URL
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		ps := probes(f)
		if len(ps) != 1 {
			t.Fatalf("the API host recorded %d push probes, want 1", len(ps))
		}
		assertAuth(t, ps)
		if n := bHits.Load(); n != 0 {
			t.Errorf("B recorded %d requests, want 0", n)
		}
	})
	t.Run("3xx to B", func(t *testing.T) {
		bHits.Store(0)
		f := newPTFake(t)
		f.pushStatus, f.pushLocation = 302, b.URL+"/"+fakeProject+".git/info/refs?service=git-receive-pack"
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed {
			t.Fatalf("rc=%d want 1\n%s", rc, h.text())
		}
		if n := bHits.Load(); n != 0 {
			t.Errorf("B recorded %d requests, want 0", n)
		}
	})
	t.Run("path prefix", func(t *testing.T) {
		f := newPTFake(t)
		f.prefix = "/prefix"
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		ps := probes(f)
		if len(ps) != 1 || ps[0].Path != "/prefix/"+fakeProject+".git/info/refs" {
			t.Fatalf("push probes: %+v", ps)
		}
		assertAuth(t, ps)
	})
}

// ptOneActive checks the fake ends with exactly one active token by name, and that the
// variable holds that token's value.
func ptOneActive(t *testing.T, f *ptFake) {
	t.Helper()
	act := f.activeByName()
	if len(act) != 1 {
		t.Fatalf("%d active tokens by name, want 1", len(act))
	}
	if f.v.value != act[0].value {
		t.Error("the variable does not hold the active token's value")
	}
}

// regression: #2910
func TestPushTokenInstallFails(t *testing.T) {
	t.Run("write 500 x3 then re-run", func(t *testing.T) {
		f := newPTFake(t)
		f.writeStatuses = []int{500, 500, 500}
		h := newPTHarness(t, f)
		rc := h.rotate()
		if rc != exitFailed || !strings.Contains(h.text(), "rotated-not-installed") ||
			!strings.Contains(h.text(), "re-run `push-token rotate`") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		if n := f.count("PUT", "/variables/"+pushTokenVariable); n != 3 {
			t.Errorf("%d write attempts, want 3", n)
		}
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("re-run rc=%d\n%s", rc, h.text())
		}
		ptOneActive(t, f)
	})
	t.Run("rotate response lost", func(t *testing.T) {
		f := newPTFake(t)
		f.dropRotate = true
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "rotate-response-lost") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		if rc := h.rotate(); rc != exitOK {
			t.Fatalf("re-run rc=%d\n%s", rc, h.text())
		}
		ptOneActive(t, f)
	})
	t.Run("concurrent run", func(t *testing.T) {
		f := newPTFake(t)
		f.rotateStatus = 400
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "rotate-refused") ||
			!strings.Contains(h.text(), "HTTP 400") || strings.Contains(h.text(), "example-forge-message") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		if n := f.count("PUT", "/variables/"+pushTokenVariable) + f.count("POST", "/variables"); n != 0 {
			t.Errorf("%d variable writes, want 0", n)
		}
	})
}

// regression: #2910
func TestPushTokenCheckMode(t *testing.T) {
	healthy := func(f *ptFake) { f.toks[0].expires = ptDate(20) }
	cases := []struct {
		name, want string
		code       int
		setup      func(f *ptFake)
	}{
		{"healthy", "outcome: healthy", exitOK, healthy},
		{"revoked", "by-id: FAIL field revoked", exitFailed, func(f *ptFake) {
			healthy(f)
			f.byIDMutate = func(_ int64, m map[string]any) { m["revoked"] = true }
		}},
		{"none", "no-active-token", exitFailed, func(f *ptFake) { f.toks[0].active = false }},
		{"two", "2 active tokens", exitFailed, func(f *ptFake) {
			healthy(f)
			f.toks = append(f.toks, &ptTok{id: 6002, name: ptName, active: true, scopes: []string{"write_repository"},
				level: 40, expires: ptDate(20)})
		}},
		{"list 500", "could-not-check", exitUnverifiable, func(f *ptFake) { f.listStatus = 500 }},
		{"2 days out of band", "by-id: FAIL field expires_at", exitFailed, func(f *ptFake) { f.toks[0].expires = ptDate(2) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPTFake(t)
			c.setup(f)
			h := newPTHarness(t, f)
			if rc := h.check("--min-days", "7"); rc != c.code || !strings.Contains(h.text(), c.want) {
				t.Fatalf("rc=%d, want %d and %q\n%s", rc, c.code, c.want, h.text())
			}
			for _, r := range f.requests() {
				if r.Private != ptReader {
					t.Errorf("%s %s not under the reader credential", r.Method, r.Path)
				}
				if r.Method != "GET" {
					t.Errorf("check sent %s %s", r.Method, r.Path)
				}
			}
			if len(f.requests()) == 0 {
				t.Error("check made no request")
			}
		})
	}
}

// regression: #2910
func TestPushTokenDryRun(t *testing.T) {
	f := newPTFake(t)
	h := newPTHarness(t, f)
	delete(h.envs, "GITLAB_API_BASE")
	if rc := h.rotate("--dry-run"); rc != exitOK {
		t.Fatalf("rc=%d\n%s", rc, h.text())
	}
	for _, step := range []string{"refusals:", "preflight: read project", "preflight: read project-level variable",
		"preflight: list access tokens", "rotate (or create):", "install:", "read-back:", "proof by-id:",
		"proof self:", "proof push:", "zero requests made"} {
		if !strings.Contains(h.out.String(), "dry-run: "+step) {
			t.Errorf("dry-run does not name %q", step)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests, want 0", n)
	}
}

// regression: #2910
func TestPushTokenCreatePath(t *testing.T) {
	f := newPTFake(t)
	f.toks[0].active = false // expired, as after #2910
	h := newPTHarness(t, f)
	if rc := h.rotate(); rc != exitRefused || !strings.Contains(h.text(), "zero active") ||
		!strings.Contains(h.text(), "--create") {
		t.Fatalf("rc=%d want 5\n%s", rc, h.text())
	}
	if n := f.writes(); n != 0 {
		t.Fatalf("%d writes, want 0", n)
	}
	if rc := h.rotate("--create"); rc != exitOK {
		t.Fatalf("--create rc=%d\n%s", rc, h.text())
	}
	var create *ptReq
	for _, r := range f.requests() {
		if r.Method == "POST" && strings.HasSuffix(r.Path, "/access_tokens") {
			r := r
			create = &r
		}
	}
	if create == nil {
		t.Fatal("no create recorded")
	}
	sc, _ := json.Marshal(create.Body["scopes"])
	if create.Body["name"] != ptName || string(sc) != `["write_repository"]` || create.Body["access_level"] != float64(40) ||
		create.Body["expires_at"] != ptDate(30) {
		t.Errorf("create body: %v", keysOf(create.Body))
	}
	ptOneActive(t, f)

	t.Run("containment recovery", func(t *testing.T) {
		f := newPTFake(t)
		f.storeOverride = func(v *ptVar) { v.protected = false }
		h := newPTHarness(t, f)
		if rc := h.rotate(); rc != exitFailed || !strings.Contains(h.text(), "contained: revoked") {
			t.Fatalf("rc=%d\n%s", rc, h.text())
		}
		f.mu.Lock()
		f.storeOverride, f.v.protected = nil, true // the operator fixes the flags, as the line says
		f.mu.Unlock()
		if rc := h.rotate("--create"); rc != exitOK {
			t.Fatalf("recovery rc=%d\n%s", rc, h.text())
		}
		ptOneActive(t, f)
	})
}
