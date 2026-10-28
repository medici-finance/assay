package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/identity"
)

// reachedHeader is set by every sentinel handler. A probe that comes back
// carrying it reached a real handler, not the identity wrap's refusal.
const reachedHeader = "X-Route-Walk-Reached"

// sentinel is a handler that records, on the response, that it was reached.
func sentinel(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(reachedHeader, name)
		w.WriteHeader(http.StatusOK)
	})
}

// gatedPolicy is the strict cell policy: the surface is login-gated and no
// cell has an anonymous-read opt-in. Under it, every route behind the wrap
// must refuse a caller with no session. It is the configuration the walk
// checks against, because it is the one in which "goes through the wrap" and
// "refuses an unauthenticated request" mean the same thing.
type gatedPolicy struct{}

func (gatedPolicy) AnswerAsForRepo(string, string) string    { return "" }
func (gatedPolicy) AnonymousReadAllowed(string, string) bool { return false }
func (gatedPolicy) SurfaceLoginGated() bool                  { return true }

// walkDeps is buildServedMux's input for the walk: sentinel handlers, the
// route-owned stand-ins answering "not authenticated" (the probe carries no
// bearer, no loopback caller and no signed intent), and a REAL identity.Guard
// with an empty session store. The guard is the production type, so the
// refusal the walk requires is the wrap's own, not a test double's.
func walkDeps() servedDeps {
	no := func(*http.Request) bool { return false }
	return servedDeps{
		Routes: routeHandlers{
			HTMLConsole:      sentinel("html-console"),
			LocalState:       sentinel("local-state"),
			Answer:           sentinel("answer"),
			Inbox:            sentinel("inbox"),
			V1:               sentinel("v1"),
			Root:             sentinel("root"),
			MCP:              sentinel("mcp"),
			MCPAuthenticated: no,
			V1Authenticated:  no,
		},
		Auth:    sentinel("auth"),
		Static:  sentinel("static"),
		Session: sentinel("session"),
		Guard: &identity.Guard{
			Sessions: identity.NewSessionStore(nil),
			Policy:   gatedPolicy{},
		},
	}
}

// wildcard matches a ServeMux path wildcard: {name}, {name...} or {$}.
var wildcard = regexp.MustCompile(`\{[^}]*\}`)

// probesFor turns a mux pattern ("[METHOD ][HOST]/path") into the requests the
// walk sends: the pattern's own path with each wildcard filled in, and, for a
// prefix pattern, one path beneath it as well.
func probesFor(pattern string) []*http.Request {
	method, rest := http.MethodGet, pattern
	if m, p, ok := strings.Cut(pattern, " "); ok {
		method, rest = m, strings.TrimSpace(p)
	}
	host := ""
	if i := strings.Index(rest, "/"); i > 0 {
		host, rest = rest[:i], rest[i:]
	}
	path := wildcard.ReplaceAllStringFunc(rest, func(w string) string {
		if w == "{$}" {
			return ""
		}
		return "probe"
	})
	paths := []string{path}
	if strings.HasSuffix(path, "/") {
		paths = append(paths, path+"probe")
	}
	var out []*http.Request
	for _, p := range paths {
		r := httptest.NewRequest(method, p, nil)
		if host != "" {
			r.Host = host
		}
		out = append(out, r)
	}
	return out
}

// walkServed probes every pattern the served mux holds, through the served mux
// itself, with an unauthenticated request. Each pattern outside
// unauthenticatedMounts must come back with the wrap's refusal: a 401, and no
// handler reached. It returns one line per pattern that did not.
func walkServed(top *servedMux) []string {
	var bad []string
	for _, pattern := range top.Mounts() {
		if _, ok := unauthenticatedMounts[pattern]; ok {
			continue
		}
		for _, req := range probesFor(pattern) {
			rec := httptest.NewRecorder()
			top.ServeHTTP(rec, req)
			if got := rec.Header().Get(reachedHeader); got != "" || rec.Code != http.StatusUnauthorized {
				bad = append(bad, fmt.Sprintf("mount %q: unauthenticated %s %s got %d (handler reached: %q), want the identity wrap's 401",
					pattern, req.Method, req.URL.Path, rec.Code, got))
			}
		}
	}
	return bad
}

// TestServedRouteWalk is the brief's route-walk row, run on the mux deskd
// SERVES (the authorization contract). It calls buildServedMux, the function main()
// calls, and probes every mounted pattern unauthenticated. Every one outside
// the explicit allow-list must be refused by the identity wrap.
//
// regression: #242
func TestServedRouteWalk(t *testing.T) {
	top, err := buildServedMux(walkDeps())
	if err != nil {
		t.Fatalf("buildServedMux refused the production route table: %v", err)
	}
	for _, line := range walkServed(top) {
		t.Error(line)
	}

	// The walk is only as good as what it saw. It must have seen every table
	// route and every allow-list entry, and nothing it did not account for.
	mounts := map[string]bool{}
	for _, p := range top.Mounts() {
		mounts[p] = true
	}
	for _, r := range stubs() {
		if !mounts[r.Pattern] {
			t.Errorf("table route %q is not mounted on the served mux", r.Pattern)
		}
	}
	for p := range unauthenticatedMounts {
		if !mounts[p] {
			t.Errorf("allow-list entry %q is not mounted — a stale exemption; remove it", p)
		}
	}
	if want := len(stubs()) + len(unauthenticatedMounts); len(top.Mounts()) != want {
		t.Errorf("the served mux holds %d mounts %v, want %d (the table plus the allow-list)", len(top.Mounts()), top.Mounts(), want)
	}
}

// TestServedWalkCatchesPlants plants routes that skip the wrap on the mux
// buildServedMux returned, and requires the walk to name each one. The first
// is the issue's mutation verbatim (a read route beside the /session mounts);
// the others are the same defect in its other shapes: a new prefix tree, a
// HandleFunc mount, and a method+wildcard pattern under a gated tree.
//
// regression: #242
func TestServedWalkCatchesPlants(t *testing.T) {
	plants := []struct {
		name    string
		pattern string
		plant   func(top *servedMux, d servedDeps)
	}{
		{"issue mutation: /v1/inbox beside /session", "/v1/inbox", func(top *servedMux, d servedDeps) {
			v1API := d.Routes.V1
			top.Handle("/v1/inbox", v1API)
		}},
		{"new prefix tree /v2/", "/v2/", func(top *servedMux, d servedDeps) {
			top.Handle("/v2/", d.Routes.V1)
		}},
		{"HandleFunc mount", "GET /healthz", func(top *servedMux, _ servedDeps) {
			top.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		}},
		{"method+wildcard under a gated tree", "POST /answer/{id}/ack", func(top *servedMux, d servedDeps) {
			top.Handle("POST /answer/{id}/ack", d.Routes.Answer)
		}},
	}
	for _, p := range plants {
		t.Run(p.name, func(t *testing.T) {
			d := walkDeps()
			top, err := buildServedMux(d)
			if err != nil {
				t.Fatal(err)
			}
			p.plant(top, d)
			bad := walkServed(top)
			caught := false
			for _, line := range bad {
				if strings.Contains(line, fmt.Sprintf("mount %q", p.pattern)) {
					caught = true
				}
			}
			if !caught {
				t.Fatalf("the walk did not catch %q mounted outside the identity wrap (violations: %v)", p.pattern, bad)
			}
		})
	}
}

// TestUnauthMountsAllowList pins the allow-list: exactly these patterns, each
// with a one-line reason. Widening it exempts a route from the identity chain,
// so it must be a change to this test as well as to served.go.
func TestUnauthMountsAllowList(t *testing.T) {
	want := []string{"/auth/", "/session", "/session/", "/static/"}
	var got []string
	for p, reason := range unauthenticatedMounts {
		got = append(got, p)
		if strings.TrimSpace(reason) == "" || strings.Contains(reason, "\n") {
			t.Errorf("allow-list entry %q needs a one-line reason, got %q", p, reason)
		}
	}
	sort.Strings(got)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("unauthenticatedMounts = %v, want exactly %v — every entry bypasses the identity wrap", got, want)
	}
}

// TestServedMuxIsTheOnlyMux keeps buildServedMux the single place a route is
// mounted. The served walk sees only what that function mounts; a pattern
// added to the mux anywhere else — main() mounting on the returned mux, a
// second mux, a different Handler on the server — would be served and never
// walked. Positive controls first, so a matcher that stopped matching fails
// here instead of passing the package clean.
//
// regression: #242
func TestServedMuxIsTheOnlyMux(t *testing.T) {
	const clean = `package main
import "net/http"
func main() {
	served, err := buildServedMux(servedDeps{})
	_ = err
	srv := &http.Server{Handler: served}
	_ = srv
}`
	// A minimal served.go: the shapes the real one has (the builder, the
	// constructor, the two recorder methods), all of which are allowed to mount.
	const servedClean = `package main
import "net/http"
type servedMux struct{ mux *http.ServeMux; mounts []string }
func newServedMux() *servedMux { return &servedMux{mux: http.NewServeMux()} }
func (m *servedMux) Handle(p string, h http.Handler) { m.mux.Handle(p, h); m.mounts = append(m.mounts, p) }
func (m *servedMux) HandleFunc(p string, f func(http.ResponseWriter, *http.Request)) { m.Handle(p, http.HandlerFunc(f)) }
func (m *servedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.mux.ServeHTTP(w, r) }
func (m *servedMux) Mounts() []string { return append([]string(nil), m.mounts...) }
func buildServedMux(d servedDeps) (*servedMux, error) {
	top := newServedMux()
	top.Handle("/auth/", d.Auth)
	return top, nil
}`
	mainWith := func(old, repl string) map[string]string {
		return map[string]string{"main.go": strings.Replace(clean, old, repl, 1), "served.go": servedClean}
	}
	controls := map[string]map[string]string{
		"main() mounts on the returned mux": mainWith("_ = err", `_ = err
	served.Handle("/v1/inbox", v1API)`),
		"main() mounts with HandleFunc": mainWith("_ = err", `_ = err
	served.HandleFunc("/healthz", nil)`),
		"a second mux is served":          mainWith("Handler: served", "Handler: http.NewServeMux()"),
		"the server wraps the served mux": mainWith("Handler: served", "Handler: http.StripPrefix(\"/x\", served)"),
		"http.ListenAndServe beside the server": mainWith("_ = err", `_ = err
	go http.ListenAndServe(":0", nil)`),
		// regression: the authorization contract review F1 — a helper in served.go itself.
		"a helper in served.go mounts on the returned mux": {
			"main.go":   strings.Replace(clean, "_ = err", "_ = err\n\tmountExtras(served, v1API)", 1),
			"served.go": servedClean + "\nfunc mountExtras(m *servedMux, h http.Handler) { m.Handle(\"/v1/inbox\", h) }",
		},
		"a servedMux method other than the recorder mounts": {
			"main.go":   clean,
			"served.go": servedClean + "\nfunc (m *servedMux) Extras(h http.Handler) { m.mux.Handle(\"/v1/inbox\", h) }",
		},
		"a package-level var in served.go mounts": {
			"main.go":   clean,
			"served.go": servedClean + "\nvar _ = http.NewServeMux()",
		},
		// regression: the authorization contract review F2 — assigning srv.Handler after the literal.
		"srv.Handler is reassigned after the literal": mainWith("_ = srv", `srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/inbox" {
			v1API.ServeHTTP(w, r)
			return
		}
		served.ServeHTTP(w, r)
	})`),
		// regression: the authorization contract security review S1 — sibling shapes of F1/F2.
		"a second http.Server built with new()": mainWith("_ = srv", `_ = srv
	s2 := new(http.Server)
	s2.Handler = v1API
	go s2.ListenAndServe()`),
		"an http.Server declared with var": mainWith("_ = srv", `_ = srv
	var s2 http.Server
	_ = s2`),
		"ListenAndServe through an import alias": {
			"main.go":   strings.Replace(strings.Replace(clean, `import "net/http"`, `import nh "net/http"`, 1), "http.Server", "nh.Server", 1) + "\nfunc other() { go nh.ListenAndServe(\":1\", v1API) }",
			"served.go": servedClean,
		},
		"an http.Server literal through an import alias with a foreign Handler": {
			"main.go":   strings.Replace(strings.Replace(strings.Replace(clean, `import "net/http"`, `import nh "net/http"`, 1), "http.Server", "nh.Server", 1), "Handler: served", "Handler: v1API", 1),
			"served.go": servedClean,
		},
		"net/http is dot-imported": {
			"main.go":   strings.Replace(clean, `import "net/http"`, `import . "net/http"`, 1),
			"served.go": servedClean,
		},
		"a method value of Handle is taken in main()": mainWith("_ = err", `_ = err
	h := served.Handle
	h("/v1/inbox", v1API)`),
		"http.ListenAndServe is taken as a value": mainWith("_ = err", `_ = err
	f := http.ListenAndServe
	_ = f`),
		"servedMux.ServeHTTP dispatches a path itself": {
			"main.go": clean,
			"served.go": strings.Replace(servedClean, "func (m *servedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.mux.ServeHTTP(w, r) }",
				"func (m *servedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n\tif r.URL.Path == \"/v1/inbox\" {\n\t\treturn\n\t}\n\tm.mux.ServeHTTP(w, r)\n}", 1),
		},
		"servedMux.ServeHTTP delegates elsewhere": {
			"main.go": clean,
			"served.go": strings.Replace(servedClean, "func (m *servedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.mux.ServeHTTP(w, r) }",
				"func (m *servedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }", 1),
		},
		"servedMux.Handle mounts without recording": {
			"main.go": clean,
			"served.go": strings.Replace(servedClean, "m.mux.Handle(p, h); m.mounts = append(m.mounts, p)",
				"m.mux.Handle(p, h)", 1),
		},
		// regression: the authorization contract correctness review F3 — net/http imported twice.
		"net/http imported under a second name, ListenAndServe through it": {
			"main.go":   strings.Replace(clean, `import "net/http"`, "import (\n\t\"net/http\"\n\tnh \"net/http\"\n)", 1) + "\nfunc other() { _ = http.StatusOK; go nh.ListenAndServe(\":1\", v1API) }",
			"served.go": servedClean,
		},
		"net/http imported under a second name, new(nh.Server)": {
			"main.go":   strings.Replace(clean, `import "net/http"`, "import (\n\t\"net/http\"\n\tnh \"net/http\"\n)", 1) + "\nfunc other() { s := new(nh.Server); _ = s }",
			"served.go": servedClean,
		},
		"net/http imported under a second name, a foreign &nh.Server literal": {
			"main.go":   strings.Replace(clean, `import "net/http"`, "import (\n\t\"net/http\"\n\tnh \"net/http\"\n)", 1) + "\nfunc other() { _ = &nh.Server{Handler: v1API} }",
			"served.go": servedClean,
		},
		// regression: the authorization contract correctness review F4(a) and security review S1(a)
		// — reaching the inner mux inside buildServedMux skips the recorder.
		"buildServedMux mounts on the inner mux": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "return top, nil", "top.mux.Handle(\"/v9/\", d.Auth)\n\treturn top, nil", 1),
		},
		"buildServedMux drops the record": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "return top, nil", "top.mounts = nil\n\treturn top, nil", 1),
		},
		// F4(b) and S1(d) — swapping or building the served mux outside the recorder.
		"main() swaps the inner mux for http.DefaultServeMux": mainWith("_ = srv", "served.mux = http.DefaultServeMux"),
		"http.DefaultServeMux is read in another file": {
			"main.go":   clean,
			"served.go": servedClean,
			"extra.go":  "package main\nimport \"net/http\"\nvar _ = http.DefaultServeMux",
		},
		"net/http/pprof is blank-imported":                 {"main.go": clean, "served.go": servedClean, "extra.go": "package main\nimport _ \"net/http/pprof\""},
		"expvar is blank-imported":                         {"main.go": clean, "served.go": servedClean, "extra.go": "package main\nimport _ \"expvar\""},
		"net/http/fcgi is imported":                        {"main.go": clean, "served.go": servedClean, "extra.go": "package main\nimport \"net/http/fcgi\"\nvar _ = fcgi.Serve"},
		"net/http/httptest is imported in a non-test file": {"main.go": clean, "served.go": servedClean, "extra.go": "package main\nimport \"net/http/httptest\"\nvar _ = httptest.NewServer"},
		"net/http/cgi is imported":                         {"main.go": clean, "served.go": servedClean, "extra.go": "package main\nimport \"net/http/cgi\"\nvar _ = cgi.Handler{}"},
		"golang.org/x/net/http2/h2c is imported":           {"main.go": clean, "served.go": servedClean, "extra.go": "package main\nimport \"golang.org/x/net/http2/h2c\"\nvar _ = h2c.NewHandler"},
		"a servedMux literal in main()":                    mainWith("_ = srv", "m2 := &servedMux{}\n\t_ = m2"),
		"new(servedMux) in main()":                         mainWith("_ = srv", "m2 := new(servedMux)\n\t_ = m2"),
		"var servedMux in main()":                          mainWith("_ = srv", "var m2 servedMux\n\t_ = m2"),
		"an http.ServeMux literal in main()":               mainWith("_ = srv", "m2 := &http.ServeMux{}\n\t_ = m2"),
		"var http.ServeMux in main()":                      mainWith("_ = srv", "var m2 http.ServeMux\n\t_ = m2"),
		"main() reads served.mounts":                       mainWith("_ = srv", "_ = served.mounts"),
		"servedMux embeds the standard mux": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "type servedMux struct{ mux *http.ServeMux; mounts []string }", "type servedMux struct{ *http.ServeMux; mux *http.ServeMux; mounts []string }", 1),
		},
		"servedMux grows a field": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "mounts []string }", "mounts []string; extra http.Handler }", 1),
		},
		"servedMux is declared outside served.go": {
			"main.go":   clean,
			"served.go": servedClean,
			"extra.go":  "package main\ntype servedMux struct{}",
		},
		// A3 (correctness advisory): the recorder pinned by content, not statement count.
		"servedMux.Handle records selectively": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "m.mounts = append(m.mounts, p)", "if p != \"/v9/\" { m.mounts = append(m.mounts, p) }", 1),
		},
		"servedMux.Handle mounts a different handler than it was given": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "m.mux.Handle(p, h);", "m.mux.Handle(p, http.NotFoundHandler());", 1),
		},
		"servedMux.Handle records the wrong value": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "append(m.mounts, p)", "append(m.mounts, \"/x\")", 1),
		},
		"servedMux.HandleFunc does not delegate to Handle": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "m.Handle(p, http.HandlerFunc(f))", "m.mux.HandleFunc(p, f)", 1),
		},
		// regression: the authorization contract review F6 — the two recorder pieces the matcher
		// used to leave unpinned: the constructor body and the Mounts accessor.
		"newServedMux mounts on the inner mux before returning": {
			"main.go": clean,
			"served.go": strings.Replace(servedClean, "func newServedMux() *servedMux { return &servedMux{mux: http.NewServeMux()} }",
				"func newServedMux() *servedMux {\n\tm := &servedMux{mux: http.NewServeMux()}\n\tm.mux.Handle(\"/v9/\", http.FileServer(http.Dir(\"/\")))\n\treturn m\n}", 1),
		},
		"newServedMux calls Handle on a local mux it returns": {
			"main.go": clean,
			"served.go": strings.Replace(servedClean, "func newServedMux() *servedMux { return &servedMux{mux: http.NewServeMux()} }",
				"func newServedMux() *servedMux {\n\tmx := http.NewServeMux()\n\tmx.Handle(\"/v9/\", http.NotFoundHandler())\n\treturn &servedMux{mux: mx}\n}", 1),
		},
		"newServedMux returns a mux that carries a preloaded record": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "&servedMux{mux: http.NewServeMux()}", "&servedMux{mux: http.NewServeMux(), mounts: []string{\"/auth/\"}}", 1),
		},
		"servedMux.Mounts filters the record": {
			"main.go": clean,
			"served.go": strings.Replace(servedClean, "return append([]string(nil), m.mounts...)",
				"return slices.DeleteFunc(append([]string(nil), m.mounts...), func(p string) bool { return p == \"/v9/\" })", 1),
		},
		"servedMux.Mounts returns a different list": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "return append([]string(nil), m.mounts...)", "return []string{\"/auth/\"}", 1),
		},
		"servedMux.Mounts copies a different field": {
			"main.go":   clean,
			"served.go": strings.Replace(servedClean, "return append([]string(nil), m.mounts...)", "return append([]string(nil), \"/auth/\")", 1),
		},
		// S1(b) — the variable holding buildServedMux's result is tracked by name.
		"served is rebound in main()":                      mainWith("_ = srv", "served = nil\n\t_ = srv"),
		"served is shadowed beside a second server":        mainWith("_ = srv", "served := http.NotFoundHandler()\n\ts2 := &http.Server{Handler: served}\n\t_ = s2"),
		"served is a parameter of a closure":               mainWith("_ = srv", "f := func(served http.Handler) { _ = &http.Server{Handler: served} }\n\t_ = f"),
		"served is a range variable":                       mainWith("_ = srv", "for _, served := range []http.Handler{v1API} { _ = served }"),
		"served is redeclared with var":                    mainWith("_ = srv", "var served http.Handler = v1API\n\t_ = served"),
		"a second http.Server literal with the served mux": mainWith("_ = srv", "s2 := &http.Server{Handler: served}\n\t_ = s2"),
		"srv.Handler is assigned in a tuple":               mainWith("_ = srv", "srv.Handler, _ = http.NewServeMux(), 0"),
		"srv.Handler is assigned from another mux":         mainWith("_ = srv", "other := http.NotFoundHandler()\n\tsrv.Handler = other"),
	}
	// Clean controls: the shapes that must NOT be flagged, so the matcher is
	// not simply flagging everything.
	cleanControls := map[string]map[string]string{
		"the clean pair": {"main.go": clean, "served.go": servedClean},
		"net/http imported twice and only one name used": {
			"main.go":   strings.Replace(clean, `import "net/http"`, "import (\n\t\"net/http\"\n\tnh \"net/http\"\n)", 1) + "\nvar _ nh.Handler",
			"served.go": servedClean,
		},
		"a closure parameter that is not named served": {
			"main.go":   strings.Replace(clean, "_ = srv", "f := func(h http.Handler) { _ = h }\n\t_ = f", 1),
			"served.go": servedClean,
		},
		"srv.Handler reassigned to the served mux": {
			"main.go":   strings.Replace(clean, "_ = srv", "srv.Handler = served", 1),
			"served.go": servedClean,
		},
	}
	for name, files := range cleanControls {
		if v := mountSiteViolations(files); len(v) != 0 {
			t.Errorf("clean control %q was flagged: %v", name, v)
		}
	}
	for name, files := range controls {
		v := mountSiteViolations(files)
		if len(v) == 0 {
			t.Errorf("positive control %q was not flagged — the choke-point check is blind to it", name)
		}
		for _, line := range v {
			if strings.Contains(line, "could not parse") {
				t.Errorf("positive control %q is flagged only because its source does not parse: %s", name, line)
			}
		}
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not read the package directory: %v", err)
	}
	files := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("could not read %s: %v", n, err)
		}
		files[n] = string(b)
	}
	if _, ok := files["served.go"]; !ok {
		t.Fatal("served.go not found — the check is not looking at cmd/deskd")
	}
	for _, v := range mountSiteViolations(files) {
		t.Error(v)
	}
}
