package comment

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// ── Structural invariants (Verify 1) ──────────────────────────────────────

// TestOneUserWritePath is the sibling of internal/answer's
// TestExactlyOneWritePath: it parses every non-test file in this package and
// asserts outbound requests are built in exactly one file (github.go). A second
// user write path anywhere in the package would build an http.NewRequest* in
// another file and fail here — so "exactly one user write site" (the capability model facts,
// the restated single-write-path invariant) is checked on every build.
//
// Fail-first: add a second file to this package that calls
// http.NewRequestWithContext (or move the builder out of github.go) and this
// test fails — the mutation the reviewer can re-run.
func TestOneUserWritePath(t *testing.T) {
	builders := requestBuildingFiles(t)
	if len(builders) == 0 {
		t.Fatal("no file builds an outbound request — the check is not load-bearing (did github.go stop using net/http?)")
	}
	for f := range builders {
		if f != "github.go" {
			t.Fatalf("outbound request built in %s — the user comment path must construct writes only in github.go (UserWriter), so there is exactly ONE user write path", f)
		}
	}
}

// TestUserWriterConstructsNoClose asserts the ONE write builder posts a comment
// and NOTHING that could close an issue: no PATCH method, and no `state:closed`
// payload anywhere in the package. "A comment can never carry a close" is thus a
// property a diff would break, not a claim (the capability model facts, §6.6 injection
// posture).
//
// Fail-first: add a Close (issues PATCH state:closed) to github.go and this
// test fails.
func TestUserWriterConstructsNoClose(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// Scan CODE only (string literals + identifiers), never comments/prose: the
	// package doc legitimately describes what it does NOT do ("no PATCH ...").
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatal(perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BasicLit:
				if node.Kind == token.STRING {
					v := strings.ToLower(strings.Trim(node.Value, "`\""))
					if v == "closed" || v == "state" {
						t.Fatalf("%s has a %q string literal — the user comment path must never construct an issue-state close", name, v)
					}
				}
			case *ast.SelectorExpr:
				if id, ok := node.X.(*ast.Ident); ok && id.Name == "http" {
					switch node.Sel.Name {
					case "MethodPatch", "MethodPut", "MethodDelete":
						t.Fatalf("%s references http.%s — the comment path posts only, it never closes or deletes", name, node.Sel.Name)
					}
				}
			}
			return true
		})
	}
}

func requestBuildingFiles(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	builders := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatal(perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "http" {
				return true
			}
			if sel.Sel.Name == "NewRequest" || sel.Sel.Name == "NewRequestWithContext" {
				builders[name] = true
			}
			return true
		})
	}
	return builders
}

// ── Behavioural (Verify 3, 5) ─────────────────────────────────────────────

type fakeWriter struct {
	token string
	ref   Ref
	body  string
	calls int
}

func (f *fakeWriter) Comment(_ context.Context, token string, ref Ref, body string) error {
	f.token, f.ref, f.body = token, ref, body
	f.calls++
	return nil
}

type fakeLookup struct {
	ref   Ref
	found bool
}

func (f fakeLookup) Ref(string) (Ref, bool, error) { return f.ref, f.found, nil }

type fakeSessions struct {
	login, token string
	ok           bool
}

func (f fakeSessions) Authenticated(*http.Request) (string, string, bool) {
	return f.login, f.token, f.ok
}

func testRef() Ref { return Ref{Repo: "owner/name", Number: 7} }

// TestCommentAuthoredWithSessionToken proves the comment goes out with the
// SESSION user's token and the ref the index resolved — never anything from the
// request body (Verify 3).
func TestCommentAuthoredWithSessionToken(t *testing.T) {
	w := &fakeWriter{}
	svc := NewService(w, fakeLookup{ref: testRef(), found: true})
	if err := svc.Comment(context.Background(), "alice", "alice-token", "id-1", "hello"); err != nil {
		t.Fatal(err)
	}
	if w.calls != 1 {
		t.Fatalf("expected one comment, got %d", w.calls)
	}
	if w.token != "alice-token" {
		t.Fatalf("comment authored with %q, want the session token", w.token)
	}
	if w.ref != testRef() {
		t.Fatalf("comment on %+v, want the index-resolved ref", w.ref)
	}
}

// TestCommentIgnoresForgedIdentity proves the HANDLER authors as the session,
// not any identity field or header the request supplies (Verify 5). A body
// carrying actor=mallory and an X-Actor header must change nothing.
func TestCommentIgnoresForgedIdentity(t *testing.T) {
	w := &fakeWriter{}
	h := &Handlers{
		Service:  NewService(w, fakeLookup{ref: testRef(), found: true}),
		Sessions: fakeSessions{login: "alice", token: "alice-token", ok: true},
	}
	srv := h.Handler()

	form := url.Values{"text": {"hi"}, "actor": {"mallory"}, "author": {"mallory"}, "login": {"mallory"}}
	req := httptest.NewRequest(http.MethodPost, "/comment/id-1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Actor", "mallory")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	if w.token != "alice-token" {
		t.Fatalf("authored with %q — a forged field/header changed the author", w.token)
	}
	if !strings.Contains(rr.Body.String(), `"as": "alice"`) {
		t.Fatalf("response should attribute the comment to the session login, got %s", rr.Body.String())
	}
}

// TestCommentRequiresAuthenticatedViewer proves an unauthenticated POST is
// refused with 401 and no write happens.
func TestCommentRequiresAuthenticatedViewer(t *testing.T) {
	w := &fakeWriter{}
	h := &Handlers{
		Service:  NewService(w, fakeLookup{ref: testRef(), found: true}),
		Sessions: fakeSessions{ok: false},
	}
	srv := h.Handler()
	req := httptest.NewRequest(http.MethodPost, "/comment/id-1", strings.NewReader("text=hi"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated comment got %d, want 401", rr.Code)
	}
	if w.calls != 0 {
		t.Fatal("an unauthenticated comment wrote")
	}
}

// TestCommentUnknownItemIsNotFound proves an id absent from the index is a 404,
// never a write against a guessed ref.
func TestCommentUnknownItemIsNotFound(t *testing.T) {
	w := &fakeWriter{}
	svc := NewService(w, fakeLookup{found: false})
	err := svc.Comment(context.Background(), "alice", "tok", "nope", "hi")
	if err != ErrUnknownItem {
		t.Fatalf("expected ErrUnknownItem, got %v", err)
	}
	if w.calls != 0 {
		t.Fatal("wrote against an unresolved ref")
	}
}
