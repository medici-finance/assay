package cachequiet

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

func TestMain(m *testing.M) { Isolate(); os.Exit(m.Run()) }

// regression: #2491
// TestIsolateClearsPolicy — the behaviour. With a policy and a free-space floor planted in
// the environment, Isolate leaves neither in this process nor in a child that inherits
// os.Environ(), and cellcache.FromEnv over that environment finds no managed cache.
func TestIsolateClearsPolicy(t *testing.T) {
	root := t.TempDir()
	t.Setenv(cellcache.EnvPolicy, `{"root":"`+root+`","domain":"`+strings.Repeat("ab", 32)+
		`","budget_bytes":8589934592,"minimum_free_bytes":0}`)
	t.Setenv("DESK_MIN_FREE_BYTES", "1")
	// Control: FromEnv sees the planted policy (whether or not this host would accept its
	// root), so the assertion after Isolate cannot pass vacuously.
	if p, err := cellcache.FromEnv(os.Environ()); p == nil {
		t.Fatalf("control: FromEnv does not see the planted policy (%v)", err)
	}
	Isolate()
	for _, k := range Vars {
		if v, ok := os.LookupEnv(k); ok {
			t.Errorf("%s is still set (%q) after Isolate", k, v)
		}
	}
	if p, err := cellcache.FromEnv(os.Environ()); p != nil || err != nil {
		t.Errorf("FromEnv after Isolate = %v, %v; want no managed cache", p, err)
	}
	out, err := exec.Command("/usr/bin/env").Output()
	if err != nil {
		t.Skipf("no env binary to observe a child's environment: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, k := range Vars {
			if strings.HasPrefix(line, k+"=") {
				t.Errorf("a child started after Isolate still inherits %s", k)
			}
		}
	}
}

// --- class guard ------------------------------------------------------------------

type pkgInfo struct {
	dir      string          // relative to the scan root, slash-separated
	imports  map[string]bool // imports of the non-test files
	hasTests bool
	isolates bool
}

// isolates reports whether a _test.go file declares a TestMain that calls
// cachequiet.Isolate (or Isolate, inside this package), directly or through one
// package-level function that does.
func isolates(files []*ast.File, names []string) bool {
	calls := func(f *ast.File, body *ast.BlockStmt, via map[string]bool) bool {
		found := false
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return !found
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok && x.Name == "cachequiet" && fun.Sel.Name == "Isolate" {
					found = true
				}
			case *ast.Ident:
				if (f.Name.Name == "cachequiet" && fun.Name == "Isolate") || via[fun.Name] {
					found = true
				}
			}
			return !found
		})
		return found
	}
	type decl struct {
		f  *ast.File
		fn *ast.FuncDecl
	}
	var funcs []decl
	for i, f := range files {
		if !strings.HasSuffix(names[i], "_test.go") {
			continue
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				funcs = append(funcs, decl{f, fn})
			}
		}
	}
	via := map[string]bool{}
	for _, d := range funcs {
		if d.fn.Name.Name != "TestMain" && calls(d.f, d.fn.Body, nil) {
			via[d.fn.Name.Name] = true
		}
	}
	for _, d := range funcs {
		if d.fn.Name.Name == "TestMain" && calls(d.f, d.fn.Body, via) {
			return true
		}
	}
	return false
}

// modulePath reads the module line of root/go.mod.
func modulePath(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fields := strings.Fields(sc.Text()); len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fs.ErrNotExist
}

// unisolated walks the module at root, whose module path is module, and returns, sorted and relative to root, every
// package with tests whose own (non-test) code links <module>/internal/cellcache, directly
// or through other packages of the module, and whose TestMain does not call Isolate. It
// also returns every such package it examined, so a scan that matched nothing is visible
// as such. cellcache itself is not examined: it reads no ambient environment, its API
// takes the environment as an argument. A package whose TEST files alone import cellcache
// is not examined either: such a test drives cellcache with the explicit policy it builds.
// testdata, vendor and dot directories below root are not packages.
func unisolated(root, module string) (missing, examined []string, err error) {
	target := module + "/internal/cellcache"
	pkgs := map[string]*pkgInfo{} // by import path
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		entries, rerr := os.ReadDir(path)
		if rerr != nil {
			return rerr
		}
		fset := token.NewFileSet()
		var files []*ast.File
		var names []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			f, perr := parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, parser.ImportsOnly|parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				// Full parse: the TestMain body is needed.
				if f, perr = parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, parser.SkipObjectResolution); perr != nil {
					return perr
				}
			}
			files = append(files, f)
			names = append(names, e.Name())
		}
		if len(files) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		p := &pkgInfo{dir: rel, imports: map[string]bool{}, isolates: isolates(files, names)}
		for i, f := range files {
			if strings.HasSuffix(names[i], "_test.go") {
				p.hasTests = true
				continue
			}
			for _, imp := range f.Imports {
				s, _ := strconv.Unquote(imp.Path.Value)
				p.imports[s] = true
			}
		}
		ip := module
		if rel != "." {
			ip = module + "/" + rel
		}
		pkgs[ip] = p
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	memo := map[string]int{} // 0 unvisited, 1 in progress, 2 links, 3 does not
	var links func(ip string) bool
	links = func(ip string) bool {
		switch memo[ip] {
		case 1, 3:
			return false
		case 2:
			return true
		}
		memo[ip] = 1
		p := pkgs[ip]
		ok := false
		if p != nil {
			for imp := range p.imports {
				if imp == target || (strings.HasPrefix(imp, module+"/") && links(imp)) {
					ok = true
					break
				}
			}
		}
		memo[ip] = 3
		if ok {
			memo[ip] = 2
		}
		return ok
	}
	for ip, p := range pkgs {
		if ip == target || !p.hasTests || !links(ip) {
			continue
		}
		examined = append(examined, p.dir)
		if !p.isolates {
			missing = append(missing, p.dir)
		}
	}
	sort.Strings(missing)
	sort.Strings(examined)
	return missing, examined, nil
}

// regression: #2491
// TestCachePackagesIsolate — the class guard. Any package under tools/desk whose code
// links internal/cellcache and which has tests must clear the cell's storage-admission
// variables in its TestMain, so a NEW package that reaches the managed cache cannot put a
// test run back on the operator's live root. Fix a failure by adding
//
//	func TestMain(m *testing.M) { cachequiet.Isolate(); os.Exit(m.Run()) }
//
// to the named package, or by calling cachequiet.Isolate() first in its existing TestMain.
func TestCachePackagesIsolate(t *testing.T) {
	root := filepath.Join("..", "..")
	module, err := modulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	missing, examined, err := unisolated(root, module)
	if err != nil {
		t.Fatal(err)
	}
	// Non-vacuous: the packages the defect was found in are cache-linking test packages.
	for _, want := range []string{"cmd/cellctl", "cmd/deskdispatch", "cmd/deskwt", "internal/cellprocess"} {
		found := false
		for _, e := range examined {
			found = found || e == want
		}
		if !found {
			t.Fatalf("the scan did not examine %s (examined %v); the matcher has stopped matching", want, examined)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d package(s) link internal/cellcache and run tests without cachequiet.Isolate in "+
			"TestMain, so a test run from a cell shell takes the live cache's lock:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// regression: #2491
// TestGuardControl — the guard's positive control. testdata/guard is module
// example.com/fixture (no go.mod of its own, so the go command never sees it) with a stub
// internal/cellcache and: one package that links it directly with no TestMain (flagged),
// one that links it only through another package (flagged), that other package, which has
// no tests (ignored), one whose TestMain calls Isolate (passes), one that calls it through a
// helper (passes), one whose TestMain only calls m.Run (flagged), one whose test files alone
// import cellcache (ignored), and one that never links it (ignored).
func TestGuardControl(t *testing.T) {
	missing, examined, err := unisolated(filepath.Join("testdata", "guard"), "example.com/fixture")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"direct", "plainmain", "transitive"}
	if strings.Join(missing, ",") != strings.Join(want, ",") || len(examined) != 5 {
		t.Fatalf("guard over testdata/guard flagged %v of %v; want %v of 5", missing, examined, want)
	}
}
