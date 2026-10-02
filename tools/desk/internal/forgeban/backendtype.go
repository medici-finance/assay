package forgeban

// backendtype.go — the second rule (desktools-v2/10): no write path is built around the
// outbound-write check.
//
// Every Forge a verb holds comes from deskkit.ResolveForge, which returns the backend wrapped
// by the outbound-write check. That is only a proof if nothing outside deskkit can build a
// backend of its own (a `deskkit.GitHubForge{...}` literal, a `new(deskkit.GitLabForge)`, a
// conversion) or reach through the wrapper to one (a type assertion or type switch naming a
// backend). This rule makes naming either backend type outside internal/deskkit a finding,
// in any syntactic position: a composite literal, a conversion, a type assertion, a type
// switch case, a declared type, an alias, a pointer or slice of it. Comments and strings are
// not code and never match.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// BackendTypeNames are the two backend types the rule confines to deskkit.
var BackendTypeNames = map[string]bool{"GitHubForge": true, "GitLabForge": true}

// deskkitImportSuffix identifies the deskkit package by import path.
const deskkitImportSuffix = "/internal/deskkit"

// deskkitDir is the one directory (relative to the scanned root) allowed to name a backend.
const deskkitDir = "internal/deskkit/"

// BackendUse is one place a non-test file outside deskkit names a backend type.
type BackendUse struct {
	File string
	Line int
	Type string
}

func (u BackendUse) String() string {
	return fmt.Sprintf("%s:%d names deskkit.%s outside internal/deskkit", u.File, u.Line, u.Type)
}

// ScanBackendTypes walks root (in practice `tools/desk`) and returns every use of a backend
// type outside internal/deskkit in shipped (non-test) Go source. Like Scan, it returns an
// error rather than an empty result when it read nothing.
func ScanBackendTypes(root string) ([]BackendUse, error) {
	root = filepath.Clean(root)
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no shipped Go source found under %s — a scan that reads nothing clears everything", root)
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	var out []BackendUse
	for _, p := range files {
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, deskkitDir) {
			continue
		}
		src, rerr := os.ReadFile(p) //nolint:gosec // path comes from the walk of the scanned root
		if rerr != nil {
			return nil, fmt.Errorf("reading %s: %w", p, rerr)
		}
		f, perr := parser.ParseFile(fset, p, src, 0)
		if perr != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, perr)
		}
		out = append(out, backendUses(fset, f, rel)...)
	}
	return out, nil
}

// backendUses finds every reference to a backend type in one file: a selector on any name
// the file imports deskkit under, or — when deskkit is dot-imported — the bare identifier.
func backendUses(fset *token.FileSet, f *ast.File, rel string) []BackendUse {
	names := map[string]bool{}
	dot := false
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !strings.HasSuffix(path, deskkitImportSuffix) {
			continue
		}
		switch {
		case imp.Name == nil:
			names["deskkit"] = true
		case imp.Name.Name == ".":
			dot = true
		case imp.Name.Name != "_":
			names[imp.Name.Name] = true
		}
	}
	if len(names) == 0 && !dot {
		return nil
	}
	var out []BackendUse
	// The Sel half of any selector, and a declared field or parameter name, is never a bare
	// dot-imported type reference; those idents are skipped while everything around them is
	// still walked (a selector's X can hold a reference).
	sel := map[*ast.Ident]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Field:
			// A declared field or parameter NAME is not a type reference (an embedded
			// field has no Names, so its type is still walked).
			for _, nm := range x.Names {
				sel[nm] = true
			}
		case *ast.SelectorExpr:
			sel[x.Sel] = true
			id, ok := x.X.(*ast.Ident)
			if ok && names[id.Name] && BackendTypeNames[x.Sel.Name] {
				out = append(out, BackendUse{File: rel, Line: fset.Position(x.Pos()).Line, Type: x.Sel.Name})
			}
		case *ast.Ident:
			if dot && !sel[x] && BackendTypeNames[x.Name] {
				out = append(out, BackendUse{File: rel, Line: fset.Position(x.Pos()).Line, Type: x.Name})
			}
		}
		return true
	})
	return out
}
