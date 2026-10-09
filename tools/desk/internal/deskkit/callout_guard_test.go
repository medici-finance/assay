package deskkit

// callout_guard_test.go — class guard: exactly ONE function in the desk tree makes the
// trust check on a configured executable before it is run.
//
// The defect class is a callout trust check whose checks are made against different
// paths — the file's type and mode read through a symbolic link while the directory
// check reads the directory holding the link. calloutExecutable is the one
// implementation that resolves the path first; a second copy of the check anywhere else
// would have to get the same resolution right on its own. So the guard pins the
// absence of a second copy: it reads the AST of every shipped (non-test) Go file under
// tools/desk and flags any FUNCTION that tests BOTH the executable bits (0o111) and the
// group/world-writable bits (0o022) of a mode — the shape of that trust check — other
// than the permitted one. A runner that needs the check calls calloutExecutable.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// calloutCheckPermits are the functions allowed to carry the trust check, keyed
// `<path under tools/desk>::<func>`.
var calloutCheckPermits = map[string]bool{
	"internal/deskkit/callout.go::calloutExecutable": true,
}

func TestCalloutCheckSingleSite(t *testing.T) {
	found, err := scanCalloutChecks(deskTreeRoot)
	if err != nil {
		t.Fatalf("could not scan the desk tree: %v — could-not-check, NOT clean", err)
	}
	seen := map[string]bool{}
	for _, key := range found {
		seen[key] = true
		if !calloutCheckPermits[key] {
			t.Errorf("%s carries its own executable trust check (tests both 0o111 and 0o022 on a "+
				"mode). Call calloutExecutable instead, so the check is made against the resolved "+
				"path in one place.", key)
		}
	}
	for key := range calloutCheckPermits {
		if !seen[key] {
			t.Errorf("permitted site %s was not found — a scanner that no longer sees the one real "+
				"check would certify every copy; re-point the scan or the permit", key)
		}
	}
}

// TestCalloutGuardPositiveCtl is the guard's positive control: a planted second copy
// of the check must be flagged, so a matcher that silently stops matching fails here.
func TestCalloutGuardPositiveCtl(t *testing.T) {
	const planted = `package p

import "os"

func plantedCheck(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&0o111 == 0 || fi.Mode().Perm()&0o022 != 0 {
		return os.ErrPermission
	}
	return nil
}

func modeOnly(m uint32) bool { return m&0o022 == 0 }

func decimalOnly(n int) bool { return n == 73 || n == 18 }
`
	f, err := parser.ParseFile(token.NewFileSet(), "planted.go", planted, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := calloutChecksIn("planted.go", f)
	if len(got) != 1 || got[0] != "planted.go::plantedCheck" {
		t.Fatalf("planted copy not flagged exactly once: got %v", got)
	}
}

// scanCalloutChecks returns the sorted keys of every shipped function under root that
// carries the trust check.
func scanCalloutChecks(root string) ([]string, error) {
	fset := token.NewFileSet()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, calloutChecksIn(filepath.ToSlash(rel), f)...)
		return nil
	})
	sort.Strings(out)
	return out, err
}

// calloutChecksIn returns `<rel>::<func>` for each top-level function (or
// package-level var bound to a function literal) in f that uses both masks.
func calloutChecksIn(rel string, f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body != nil && usesBothModeMasks(d.Body) {
				out = append(out, rel+"::"+d.Name.Name)
			}
		case *ast.GenDecl:
			if d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, v := range vs.Values {
					if i < len(vs.Names) && usesBothModeMasks(v) {
						out = append(out, rel+"::"+vs.Names[i].Name)
					}
				}
			}
		}
	}
	return out
}

func usesBothModeMasks(n ast.Node) bool {
	var exec, write bool
	ast.Inspect(n, func(n ast.Node) bool {
		bl, ok := n.(*ast.BasicLit)
		if !ok || bl.Kind != token.INT {
			return true
		}
		// Octal spellings only (0o111, 0111): a decimal 73 or 18 is not a mode mask.
		lit := strings.ToLower(bl.Value)
		if len(lit) < 2 || lit[0] != '0' || lit[1] == 'x' || lit[1] == 'b' {
			return true
		}
		v, err := strconv.ParseInt(strings.ReplaceAll(bl.Value, "_", ""), 0, 64)
		if err != nil {
			return true
		}
		switch v {
		case 0o111:
			exec = true
		case 0o022:
			write = true
		}
		return true
	})
	return exec && write
}
