package avatar

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// goldenclass_test.go — the CLASS guard for the defect #1952 fixed in TestGolden20px.
//
// The defect class: a test that reads a committed file in a COMPRESSED format (PNG, gzip, zlib,
// zip, …) and compares it BYTE-for-byte with freshly encoded output. The encoded bytes pin the Go
// toolchain's compressor, not the content — compress/flate's output changed between Go releases
// for identical input — so the check fails on any machine whose Go differs from the one that wrote
// the file while the content is the same. Compare the DECODED content instead (pixelMismatch in
// golden_test.go is the model for PNG).
//
// This scan walks every _test.go file in the tools/desk module (cmd/ and internal/, testdata
// skipped) and flags a file that names a compressed-extension string literal AND, inside one
// function, passes a value read from a committed file (os.ReadFile / ioutil.ReadFile of a path
// that mentions "testdata") to a byte-equality primitive (bytes.Equal / bytes.Compare /
// reflect.DeepEqual). Comparing two outputs of the same process (a determinism check) is not the
// class and is not flagged: both sides come from one toolchain. A flagged file that is NOT a class
// site goes on the allow-list below with its reason.

// compressedGoldenAllowList maps files (relative to tools/desk, forward slashes) the scan flags
// but that are not class sites, to the reason. Empty: no file currently needs it.
var compressedGoldenAllowList = map[string]string{}

var compressedExts = []string{".png", ".gz", ".tgz", ".zip", ".zlib", ".zst", ".bz2", ".xz"}

// compressedGoldenSites parses every _test.go file under root/<tops...> (skipping testdata unless
// includeTestdata) and returns the files matching the class shape, relative to root, plus the
// count of files parsed so "found none" is distinguishable from "looked at nothing".
func compressedGoldenSites(root string, tops []string, includeTestdata bool) ([]string, int, error) {
	var hits []string
	scanned := 0
	fset := token.NewFileSet()
	for _, top := range tops {
		base := filepath.Join(root, top)
		if _, err := os.Stat(base); err != nil {
			return nil, 0, err
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" && !includeTestdata {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			scanned++
			if isCompressedGoldenSite(f) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				hits = append(hits, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, 0, err
		}
	}
	sort.Strings(hits)
	return hits, scanned, nil
}

func isCompressedGoldenSite(f *ast.File) bool {
	ext := false
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := stringLit(n); ok {
			ls := strings.ToLower(s)
			for _, e := range compressedExts {
				if strings.HasSuffix(ls, e) {
					ext = true
				}
			}
		}
		return true
	})
	if !ext {
		return false
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && funcByteComparesCommitted(fd.Body) {
			return true
		}
	}
	return false
}

// funcByteComparesCommitted reports whether body byte-compares a value it read from a committed
// file: an identifier assigned from os.ReadFile / ioutil.ReadFile whose path mentions "testdata"
// (directly, or through an identifier assigned from an expression that does), passed to
// bytes.Equal / bytes.Compare / reflect.DeepEqual. Assignments are tracked in source order.
func funcByteComparesCommitted(body *ast.BlockStmt) bool {
	testdataVars := map[string]bool{}
	committedVars := map[string]bool{}
	mentionsTestdata := func(e ast.Node) bool {
		found := false
		ast.Inspect(e, func(n ast.Node) bool {
			if s, ok := stringLit(n); ok && strings.Contains(s, "testdata") {
				found = true
			}
			if id, ok := n.(*ast.Ident); ok && testdataVars[id.Name] {
				found = true
			}
			return !found
		})
		return found
	}
	hit := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Rhs) != 1 || len(v.Lhs) == 0 {
				return true
			}
			id, ok := v.Lhs[0].(*ast.Ident)
			if !ok || id.Name == "_" {
				return true
			}
			if call, ok := v.Rhs[0].(*ast.CallExpr); ok && callName(call) == "os.ReadFile" || ok && callName(call) == "ioutil.ReadFile" {
				if len(call.Args) == 1 && mentionsTestdata(call.Args[0]) {
					committedVars[id.Name] = true
				}
			} else if mentionsTestdata(v.Rhs[0]) {
				testdataVars[id.Name] = true
			}
		case *ast.CallExpr:
			switch callName(v) {
			case "bytes.Equal", "bytes.Compare", "reflect.DeepEqual":
				for _, a := range v.Args {
					if id, ok := a.(*ast.Ident); ok && committedVars[id.Name] {
						hit = true
					}
				}
			}
		}
		return true
	})
	return hit
}

func callName(c *ast.CallExpr) string {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + sel.Sel.Name
}

func stringLit(n ast.Node) (string, bool) {
	bl, ok := n.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	return s, err == nil
}

// TestNoByteCompareOfCompressedGoldens fails naming any test file in the module that repeats the
// #1952 shape and is not on the allow-list, and any allow-list entry the scan no longer flags.
func TestNoByteCompareOfCompressedGoldens(t *testing.T) {
	root := filepath.Join("..", "..") // tools/desk
	hits, scanned, err := compressedGoldenSites(root, []string{"cmd", "internal"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 50 {
		t.Fatalf("scanned only %d test files under %s — the walk is not seeing the module", scanned, root)
	}
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h] = true
		if _, ok := compressedGoldenAllowList[h]; !ok {
			t.Errorf("%s: reads a committed compressed file and byte-compares — compare decoded content, not encoder bytes (#1952; see pixelMismatch in internal/avatar/golden_test.go), or allow-list it here with a reason", h)
		}
	}
	for f := range compressedGoldenAllowList {
		if !seen[f] {
			t.Errorf("%s: on the allow-list but no longer flagged — remove the entry", f)
		}
	}
}

// TestCompressedGoldenGuardPositiveControl proves the matcher still fires: the committed fixture
// under testdata/classguard repeats the pre-fix shape and must be flagged; its pixel-comparing
// sibling must not be.
func TestCompressedGoldenGuardPositiveControl(t *testing.T) {
	hits, scanned, err := compressedGoldenSites(".", []string{filepath.Join("testdata", "classguard")}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"testdata/classguard/planted_test.go"}
	if scanned != 2 || strings.Join(hits, ",") != strings.Join(want, ",") {
		t.Fatalf("positive control: scanned %d files, flagged %v; want 2 scanned, flagged %v", scanned, hits, want)
	}
}
