package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The desk-tools test suites leave nothing behind in $TMPDIR (#1195). Three shapes did:
//
//  1. a TestMain that installed the roster fixture (a private HOME under $TMPDIR) and
//     registered its cleanup with `defer`, then called os.Exit — a defer never fires past
//     os.Exit, so one fixture HOME survived per run;
//  2. a TestMain that built a fake `gh`/`desktoken` with HOME already relocated and no
//     GOMODCACHE / GOPATH / GOCACHE pin, so `go build` grew a fresh module and build cache
//     inside the fixture on every run (read-only files RemoveAll cannot delete);
//  3. a fake `desktoken` whose `worker` verb wrote its token file into the REAL temp dir
//     (os.CreateTemp("", …)), outside every fixture directory anything removes.
//
// This test is the static guard on all three, read from the test sources themselves, so a
// regression is a red run here rather than a slow disk audit. It walks every _test.go under
// cmd/ and internal/ — the same directory set the echo-coverage test walks — and it is a
// source-level check on purpose: the runtime evidence (the fixture HOME is gone after
// m.Run()) lives in each affected package's own finishFixtureRoster, which only fires if the
// package's TestMain reaches it; this test is what catches the shapes that never reach it.
func TestTestMainsLeaveNothingInTempDir(t *testing.T) {
	var files []string
	for _, root := range []string{cmdDir, filepath.Join("..", "..", "internal")} {
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), "_test.go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	if len(files) == 0 {
		t.Fatal("no _test.go files found — the walk roots are wrong")
	}

	fset := token.NewFileSet()
	checkedMains := 0
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("cannot read %s: %v", p, err)
		}
		src := string(b)
		rel, _ := filepath.Rel(filepath.Join("..", ".."), p)

		// Shape 3: a fake desktoken must write its token under a directory a fixture removes.
		// (The literal is assembled so this file does not match itself.)
		for _, prefix := range []string{"fake-worker-token-", "fleet-harness-token-"} {
			if strings.Contains(src, `os.CreateTemp("", "`+prefix) {
				t.Errorf("%s: a fake desktoken writes %s… into the REAL temp dir; route it into the "+
					"fixture directory (os.CreateTemp(os.Getenv(\"FAKE_TOKEN_DIR\"), …)) so the "+
					"fixture's RemoveAll covers it", rel, prefix)
			}
		}

		f, err := parser.ParseFile(fset, p, b, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", p, err)
		}
		var testMain *ast.FuncDecl
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "TestMain" {
				testMain = fd
			}
		}
		if testMain == nil {
			continue
		}
		installsFixture := strings.Contains(src, "installFixtureRoster()")
		if !installsFixture {
			continue
		}
		checkedMains++

		// Shape 1: no `defer` in a TestMain that installs the fixture. Every such TestMain
		// exits through os.Exit (directly, or via a run function whose code it hands on), and
		// a deferred cleanup is exactly the call that never happens. The cleanup must be an
		// explicit call between m.Run() and os.Exit — the twelve packages that always did it
		// that way never leaked.
		ast.Inspect(testMain, func(n ast.Node) bool {
			if ds, ok := n.(*ast.DeferStmt); ok {
				t.Errorf("%s: TestMain defers %s — a defer never fires past os.Exit, so the roster "+
					"fixture HOME under $TMPDIR is never removed; call the cleanup explicitly "+
					"between m.Run() and os.Exit", rel, exprString(ds.Call.Fun))
			}
			return true
		})

		// Shape 2: a fake-binary build in a fixture-installing package runs with HOME relocated,
		// so unless the host's Go caches are pinned every run downloads/compiles into the fixture.
		if strings.Contains(src, `"go", "build"`) && !strings.Contains(src, "fakeBuildEnv(") {
			t.Errorf("%s: builds a fake binary with HOME relocated into the roster fixture without "+
				"fakeBuildEnv() — the build must carry the host GOMODCACHE/GOPATH/GOCACHE the fixture "+
				"pinned before HOME moved, or every run fills the fixture with a fresh cache", rel)
		}
	}
	if checkedMains < 6 {
		t.Fatalf("checked only %d fixture-installing TestMains; the six the leak audit named must all be seen", checkedMains)
	}
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name + "()"
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name + "." + x.Sel.Name + "()"
		}
		return x.Sel.Name + "()"
	}
	return "a call"
}
