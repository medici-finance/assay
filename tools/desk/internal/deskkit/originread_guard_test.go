package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// localRemoteReads are gitcore's repository-local remote URL reads. They see the shared
// config file alone: no worktree-scope config and no include/includeIf, so a repository a
// caller decides from them can differ from the one native git resolves. A caller that
// derives a repository slug uses gitcore's ScopedRemoteURLs (which refuses when that config
// could apply) or native git; a NEW raw read must say here why it needs neither.
var localRemoteReads = map[string]bool{"RemoteURL": true, "RemoteURLs": true}

// localRemoteReadAllowed is the ratchet: every non-test caller that existed when the scoped
// read was introduced, keyed file:function, with what it decides. Shrink it; do not grow it.
var localRemoteReadAllowed = map[string]string{
	"cmd/deskclaim-ref/gogit.go:gogitOriginURL":      "claim host; go-git refusal falls through to native git",
	"cmd/deskgit/deskgit.go:fetchOriginURL":          "in-process fetch gate on the URL it connects to",
	"cmd/deskpushguard/main.go:deriveRepo":           "hook fallback when git passes no remote URL",
	"cmd/deskreply/deskreply.go:preflight":           "reply slug; follow-up candidate",
	"cmd/deskwt/deskwt.go:currentRepo":               "worktree slug; follow-up candidate",
	"internal/deskkit/gitread.go:CheckoutGitRead":    "in-process fetch gate on the URL it connects to",
	"internal/deskkit/preflight.go:deriveRepoSlug":   "landing slug; follow-up candidate",
	"internal/deskkit/preflight.go:landingRemoteURL": "landing host",
	"internal/deskkit/remoterepo.go:OriginRepoSlug":  "origin slug helper; follow-up candidate",
}

// localRemoteReaders lists file:function for every non-test call of a localRemoteReads name.
func localRemoteReaders(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && localRemoteReads[sel.Sel.Name] {
						seen[filepath.ToSlash(rel)+":"+fn.Name.Name] = true
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// regression: #6606
func TestNoNewLocalOriginRead(t *testing.T) {
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		t.Fatalf("module root not found at %s: %v", module, err)
	}
	var hits []string
	for _, hit := range localRemoteReaders(t, module) {
		if _, ok := localRemoteReadAllowed[hit]; !ok {
			hits = append(hits, hit)
		}
	}
	if len(hits) != 0 {
		t.Fatalf("non-test code reads a remote URL from repository-local config only; a slug decided "+
			"from it can differ from native git's (use gitcore's ScopedRemoteURLs or native git): %v", hits)
	}
}

// Positive control: the scanner must flag planted reads under any receiver, or a broken
// matcher would report the module clean.
func TestNoNewLocalOriginReadPlant(t *testing.T) {
	dir := t.TempDir()
	plants := map[string]string{
		"a.go":      "package x\nfunc a(r R) { r.RemoteURLs(\"origin\") }\n",
		"b.go":      "package x\nfunc b(r R) { _, _ = gitcore.RemoteURL(r, \"origin\") }\n",
		"c.go":      "package x\nfunc c(r R) { r.ScopedRemoteURLs(\"origin\") }\n",
		"d_test.go": "package x\nfunc d(r R) { r.RemoteURL(\"origin\") }\n",
	}
	for name, src := range plants {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Join(localRemoteReaders(t, dir), ",")
	if got != "a.go:a,b.go:b" {
		t.Fatalf("scanner flagged %q, want the two planted non-test reads only", got)
	}
}
