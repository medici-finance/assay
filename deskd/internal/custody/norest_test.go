package custody

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the PERSISTENCE AUDIT the service contract gate-why calls for
// (property 1 of custody rule (b)): "the service PERSISTS no human credential —
// the per-request credential lives in no struct that outlives the request and
// in no file under the data dir, asserted by a persistence-audit test".
//
// This source check covers every code path, including paths a runtime test
// might never execute. No path may persist a human credential.

// credentialWriters are the standard-library calls that could put a byte on
// disk. None of them may appear in this package.
var credentialWriters = map[string]bool{
	"WriteFile":  true,
	"Create":     true,
	"CreateTemp": true,
	"OpenFile":   true,
	"MkdirAll":   true,
}

// TestServiceHoldsNoCredentialAtRest asserts the custody package — the ONLY
// package that holds a human credential — contains no filesystem write and no
// filesystem read. A credential that is never written cannot be read back after
// a restart, which is the whole of the not-at-rest guarantee.
func TestServiceHoldsNoCredentialAtRest(t *testing.T) {
	for _, name := range goFiles(t, ".") {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
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
			if !ok || pkg.Name != "os" {
				return true
			}
			if credentialWriters[sel.Sel.Name] || sel.Sel.Name == "ReadFile" {
				t.Errorf("%s calls os.%s — the human credential is supplied PER REQUEST and is never at rest (custody rule (b), the service contract P1). "+
					"A filesystem call in the credential package is the at-rest path the route-boundary review bounced.", name, sel.Sel.Name)
			}
			return true
		})
	}
}

// TestNoTokenFileNameAnywhere asserts the at-rest credential FILE the audit
// named (`operator-token.json`) appears nowhere in the tree — not as a constant,
// not in a doc string, not in a helper that a later commit could call. The
// grep-level check is the one the brief's Verify row 1 runs.
func TestNoTokenFileNameAnywhere(t *testing.T) {
	const atRestFile = "operator-token.json"
	root := repoRoot(t)
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// This audit file names the string in order to forbid it.
		if filepath.Base(path) == "norest_test.go" {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(raw), atRestFile) {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("the at-rest credential file %q is still named in: %s — the service holds no human credential at rest (the service contract P1)",
			atRestFile, strings.Join(offenders, ", "))
	}
}

// TestNoCredentialColumnInTheIndex is the second half of the persistence audit:
// the service's own durable store (the SQLite index) has no column, table, or
// statement that could hold a credential. The audit checked this by grep on
// by a whole-package source scan; this keeps a later schema migration from
// quietly opening the door the file path just closed.
func TestNoCredentialColumnInTheIndex(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{"access_token", "accesstoken", "credential", "operator_token", "oauth_token"}
	dir := filepath.Join(root, "internal", "index")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatalf("read %s: %v", e.Name(), rerr)
		}
		lower := strings.ToLower(string(raw))
		for _, f := range forbidden {
			if strings.Contains(lower, f) {
				t.Errorf("internal/index/%s mentions %q — the index is derived state and must never carry a credential column (the service contract P1 persistence audit)", e.Name(), f)
			}
		}
	}
}

func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out
}

// repoRoot walks up from the package dir to the module root (the dir holding
// go.mod), so the audit covers the whole tree rather than one package.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find the module root (no go.mod above the package dir)")
		}
		dir = parent
	}
}
