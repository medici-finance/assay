package deskkit

import (
	"fmt"
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
// derives a repository slug uses SlugRemoteURL or gitcore's ScopedRemoteURLs (both refuse
// when that config could apply) or native git; a NEW raw read must say here why it needs none.
var localRemoteReads = map[string]bool{"RemoteURL": true, "RemoteURLs": true}

// localRemoteReadAllowed is the ratchet: every non-test caller that existed when the scoped
// read was introduced, keyed file:function, with how many references to a read it holds and
// what it decides. A count, not a presence bit, so a second read added inside an allowed
// function still fails. Shrink it; do not grow it.
var localRemoteReadAllowed = map[string]struct {
	reads   int
	decides string
}{
	"cmd/deskclaim-ref/gogit.go:gogitOriginURL":      {1, "claim host; go-git refusal falls through to native git"},
	"cmd/deskgit/deskgit.go:fetchOriginURL":          {1, "in-process fetch gate on the URL it connects to"},
	"cmd/deskpushguard/main.go:deriveRepo":           {1, "hook fallback when git passes no remote URL"},
	"internal/deskkit/gitread.go:CheckoutGitRead":    {1, "in-process fetch gate on the URL it connects to"},
	"internal/deskkit/preflight.go:landingRemoteURL": {1, "landing host"},
}

// localRemoteReaders lists file:function once per non-test reference to a localRemoteReads
// name, sorted, so a function holding two references is listed twice.
func localRemoteReaders(t *testing.T, root string) []string {
	t.Helper()
	var out []string
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
		// Any selector naming a read counts, called or not: a method value or method
		// expression (r.RemoteURLs, (*gitcore.Repo).RemoteURLs) reaches the read too.
		// Package-level var/const initializers (a func literal or a method value bound
		// at package scope) are keyed by the declared names.
		mark := func(node ast.Node, key string) {
			ast.Inspect(node, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && localRemoteReads[sel.Sel.Name] {
					out = append(out, filepath.ToSlash(rel)+":"+key)
				}
				return true
			})
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body != nil {
					mark(d.Body, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					var names []string
					for _, n := range vs.Names {
						names = append(names, n.Name)
					}
					for _, v := range vs.Values {
						mark(v, strings.Join(names, ","))
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
	found := map[string]int{}
	for _, hit := range localRemoteReaders(t, module) {
		found[hit]++
	}
	var hits, stale []string
	for k, n := range found {
		if n > localRemoteReadAllowed[k].reads {
			hits = append(hits, fmt.Sprintf("%s (%d, allowed %d)", k, n, localRemoteReadAllowed[k].reads))
		}
	}
	sort.Strings(hits)
	if len(hits) != 0 {
		t.Fatalf("non-test code reads a remote URL from repository-local config only; a slug decided "+
			"from it can differ from native git's (use SlugRemoteURL, gitcore's ScopedRemoteURLs or native git): %v", hits)
	}
	// The ratchet only shrinks: an entry whose caller is gone, or that now holds fewer reads,
	// must be lowered, so a later raw read cannot reuse its slot.
	for k, a := range localRemoteReadAllowed {
		if found[k] < a.reads {
			stale = append(stale, fmt.Sprintf("%s (%d, allowed %d)", k, found[k], a.reads))
		}
	}
	sort.Strings(stale)
	if len(stale) != 0 {
		t.Fatalf("allow-list entries hold fewer remote URL reads than listed; lower or remove them: %v", stale)
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
		"e.go":      "package x\nvar e = func(r R) { r.RemoteURLs(\"origin\") }\n",
		"f.go":      "package x\nfunc f(r R) { read := r.RemoteURL; _, _ = read(\"origin\") }\n",
		"g.go":      "package x\nvar g = (*gitcore.Repo).RemoteURLs\n",
		"h.go":      "package x\nfunc h(r R) { r.RemoteURL(\"origin\"); r.RemoteURLs(\"upstream\") }\n",
	}
	for name, src := range plants {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Join(localRemoteReaders(t, dir), ",")
	if got != "a.go:a,b.go:b,e.go:e,f.go:f,g.go:g,h.go:h,h.go:h" {
		t.Fatalf("scanner flagged %q, want the planted non-test reads only (calls, a package-level "+
			"func literal, a method value, a method expression, two reads in one function)", got)
	}
}
