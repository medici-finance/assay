package deskkit

// forgebodytransport_test.go — the body cap in the forge clients' transport: reads the client
// libraries make themselves (an error body go-gh or the GitLab client reads whole, a 2xx the
// GitLab client stream-decodes, a GraphQL batch decoded from the stream) are bounded, and an
// over-cap body is a refusal carrying *ForgeBodyCapError, never a prefix read as the answer.

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
)

// bodyMeter counts the response-body bytes actually pulled from the wire side of the client.
type bodyMeter struct {
	next http.RoundTripper
	n    atomic.Int64
}

func (m *bodyMeter) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := m.next.RoundTrip(req)
	if err == nil && resp != nil && resp.Body != nil {
		resp.Body = &meteredBody{rc: resp.Body, n: &m.n}
	}
	return resp, err
}

type meteredBody struct {
	rc io.ReadCloser
	n  *atomic.Int64
}

func (b *meteredBody) Read(p []byte) (int, error) {
	k, err := b.rc.Read(p)
	b.n.Add(int64(k))
	return k, err
}

func (b *meteredBody) Close() error { return b.rc.Close() }

const (
	tinyCap  = 64
	hugeBody = 200 << 10
)

// hugeServer answers every request with status and body.
func hugeServer(t *testing.T, status int, body []byte) (*httptest.Server, *bodyMeter) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodyMeter{next: srv.Client().Transport}
}

// assertCapRefusal: a refusal carrying *ForgeBodyCapError, with at most a few cap-widths read.
func assertCapRefusal(t *testing.T, err error, m *bodyMeter) {
	t.Helper()
	if err == nil {
		t.Fatalf("an over-cap body read as success (%d bytes read)", m.n.Load())
	}
	if !IsForgeBodyOverCap(err) || ExitCodeOf(err) != 6 {
		t.Errorf("want an over-cap refusal (exit 6, *ForgeBodyCapError), got exit %d: %v", ExitCodeOf(err), err)
	}
	if got := m.n.Load(); got > 4*tinyCap {
		t.Errorf("read %d body bytes under a %d-byte cap — the read is not bounded", got, tinyCap)
	}
}

func bigMessage() []byte {
	return []byte(`{"message":"` + strings.Repeat("a", hugeBody) + `"}`)
}

// A GitHub non-2xx JSON answer: go-gh reads it whole to build its error.
//
// regression: #6730
func TestGitHubErrorBodyCapped(t *testing.T) {
	lowCap(t, tinyCap)
	srv, m := hugeServer(t, http.StatusUnprocessableEntity, bigMessage())
	g := &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: &http.Client{Transport: m}}
	var out []map[string]any
	err := g.doJSON(http.MethodGet, "/repos/o/r/labels", nil, &out)
	assertCapRefusal(t, err, m)
	var fae *ForgeAPIError
	if errors.As(err, &fae) {
		t.Errorf("an over-cap error body became a ForgeAPIError built from its prefix: %v", err)
	}
}

// A GraphQL 2xx batch stream-decoded by readCommentBatch.
//
// regression: #6730
func TestGraphQLBatchCapped(t *testing.T) {
	lowCap(t, tinyCap)
	body := []byte(`{` + strings.Repeat(" ", hugeBody) +
		`"data":{"repository":{"a0":{"number":7,"comments":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}}`)
	srv, m := hugeServer(t, http.StatusOK, body)
	g := &GitHubForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: &http.Client{Transport: m}}
	got, err := g.issueCommentEvents(ForgeRepo{Owner: "o", Name: "r"}, []int{7})
	if err == nil {
		t.Fatalf("an over-cap GraphQL batch decoded as %v", got)
	}
	assertCapRefusal(t, err, m)
}

// A GitLab 2xx the client stream-decodes.
//
// regression: #6730
func TestGitLabOkBodyCapped(t *testing.T) {
	lowCap(t, tinyCap)
	body := []byte(`[{"id":1,"name":"` + strings.Repeat("a", hugeBody) + `"}]`)
	srv, m := hugeServer(t, http.StatusOK, body)
	g := &GitLabForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: &http.Client{Transport: m}}
	labels, err := g.ListLabels(ForgeRepo{Owner: "o", Name: "r"})
	if err == nil {
		t.Fatalf("an over-cap GitLab answer decoded as %d labels", len(labels))
	}
	assertCapRefusal(t, err, m)
}

// A GitLab non-2xx the client reads whole for its ErrorResponse.
//
// regression: #6730
func TestGitLabErrBodyCapped(t *testing.T) {
	lowCap(t, tinyCap)
	srv, m := hugeServer(t, http.StatusBadRequest, bigMessage())
	g := &GitLabForge{Token: "synthetic-minted", BaseURL: srv.URL, Client: &http.Client{Transport: m}}
	_, err := g.ListLabels(ForgeRepo{Owner: "o", Name: "r"})
	assertCapRefusal(t, err, m)
	var fae *ForgeAPIError
	if errors.As(err, &fae) {
		t.Errorf("an over-cap error body became a ForgeAPIError: %v", err)
	}
}

// Bodies at or under the cap pass unchanged, on both statuses.
func TestCapTransportUnderCap(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write(paddedJSON(tinyCap))
		}))
		c := cappedForgeClient(srv.Client())
		lowCap(t, tinyCap)
		resp, err := c.Get(srv.URL)
		if err != nil {
			t.Fatalf("status %d at the cap: %v", status, err)
		}
		raw, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		srv.Close()
		if rerr != nil || !bytes.Equal(raw, paddedJSON(tinyCap)) {
			t.Errorf("status %d at the cap: %d bytes, err %v; want the whole body", status, len(raw), rerr)
		}
	}
}

// A per-request limit (the run-log readers' own larger bound) replaces the shared cap.
func TestCapTransportRequestLimit(t *testing.T) {
	lowCap(t, tinyCap)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(paddedJSON(4 * tinyCap))
	}))
	defer srv.Close()
	req, _ := http.NewRequestWithContext(withForgeBodyLimit(t.Context(), 4*tinyCap), http.MethodGet, srv.URL, nil)
	resp, err := cappedForgeClient(srv.Client()).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if raw, rerr := io.ReadAll(resp.Body); rerr != nil || len(raw) != 4*tinyCap {
		t.Errorf("per-request limit: %d bytes, err %v", len(raw), rerr)
	}
}

// regression: #6730
func TestForgeClientsCapped(t *testing.T) {
	hits, err := scanForgeClients(deskTreeRoot, "desk")
	if err != nil {
		t.Fatalf("the forge-client guard could not scan the tree: %v — could-not-check, NOT clean", err)
	}
	for _, h := range hits {
		t.Errorf("%s — build forge clients on the capped transport: go-gh ClientOptions{Transport: "+
			"capForgeBodies(...)} and gitlab.WithHTTPClient(cappedForgeClient(...)); a client without it reads "+
			"error bodies and decoded answers with no byte bound (#6730)", h)
	}
}

// regression: #6730
func TestForgeClientGuardPlanted(t *testing.T) {
	hits, err := scanForgeClients(filepath.Join("testdata", "forgeclientcap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"planted/planted.go:glBareClient uncapped-gitlab-client",
		"planted/planted.go:glNoHTTPClient uncapped-gitlab-client",
		"planted/planted.go:restNoCap uncapped-gogh-client",
		"planted/planted.go:restNoTransport uncapped-gogh-client",
	}
	sort.Strings(hits)
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Errorf("planted hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}

const (
	goghAPIPath   = "github.com/cli/go-gh/v2/pkg/api"
	gitlabLibPath = "gitlab.com/gitlab-org/api/client-go"
)

// scanForgeClients reports every go-gh client built without Transport: capForgeBodies(...),
// every gitlab.WithHTTPClient whose argument is not cappedForgeClient(...), and every function
// that builds a GitLab client with no WithHTTPClient option at all (the library's own,
// uncapped default client).
func scanForgeClients(dir, label string) ([]string, error) {
	var out []string
	fset := token.NewFileSet()
	files := 0
	werr := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if path != dir && (e.Name() == "testdata" || strings.HasPrefix(e.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		files++
		gh, gl := importName(f, goghAPIPath, "api"), importName(f, gitlabLibPath, "gitlab")
		if gh == "" && gl == "" {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			where := fmt.Sprintf("%s/%s:%s", label, filepath.ToSlash(rel), fn.Name.Name)
			builds, withHTTP := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if gh != "" && isAnyPkgCall(c, gh, "NewRESTClient", "NewGraphQLClient", "NewHTTPClient",
					"DefaultRESTClient", "DefaultGraphQLClient", "DefaultHTTPClient") && !goghOptsCapped(c) {
					out = append(out, where+" uncapped-gogh-client")
				}
				if gl != "" && isAnyPkgCall(c, gl, "NewClient", "NewAuthSourceClient", "NewOAuthClient",
					"NewJobClient", "NewBasicAuthClient") {
					builds = true
				}
				if gl != "" && isPkgCall(c, gl, "WithHTTPClient") {
					withHTTP = true
					if len(c.Args) != 1 || !isCallTo(c.Args[0], "cappedForgeClient") {
						out = append(out, where+" uncapped-gitlab-client")
					}
				}
				return true
			})
			if builds && !withHTTP {
				out = append(out, where+" uncapped-gitlab-client")
			}
		}
		return nil
	})
	if werr != nil {
		return nil, werr
	}
	if files == 0 {
		return nil, fmt.Errorf("no Go source under %s — a guard that scans nothing certifies nothing", dir)
	}
	return out, nil
}

func isAnyPkgCall(c *ast.CallExpr, pkg string, fns ...string) bool {
	for _, fn := range fns {
		if isPkgCall(c, pkg, fn) {
			return true
		}
	}
	return false
}

func isCallTo(e ast.Expr, fn string) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	return ok && id.Name == fn
}

// goghOptsCapped: the call's ClientOptions literal sets Transport: capForgeBodies(...).
func goghOptsCapped(c *ast.CallExpr) bool {
	if len(c.Args) != 1 {
		return false
	}
	lit, ok := c.Args[0].(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, el := range lit.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Transport" {
				return isCallTo(kv.Value, "capForgeBodies")
			}
		}
	}
	return false
}
