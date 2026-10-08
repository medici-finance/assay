package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Issue #2355, part 2. A dispatch refused AFTER its claim was placed left the claim HELD, so the
// next attempt was told "already claimed by a LIVE holder" for a dispatcher that never dispatched.
// The decision-issue gate was the field case; the model stamp, prompt assembly and the prompt write
// had the same gap.

func TestGateRefusalReleasesClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = append(happyReplies("/private/tmp/worker-home"),
		reply{match: "decision-issue.sh ensure", stderr: "refused: no decision surface", code: deskkit.ExitRefused})

	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--gate-human", "--brief", "spec.md",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc == deskkit.ExitOK {
		t.Fatal("a failed decision gate must fail the dispatch")
	}
	if !s.ran("dispatch-claim.sh acquire") {
		t.Fatal("the claim was never acquired — the precondition for this test does not hold")
	}
	if !s.ran("dispatch-claim.sh release") {
		t.Error("the decision-gate refusal left the claim HELD — a re-run reads it as a LIVE holder")
	}
	if !strings.Contains(stderr, "item-1 was released") {
		t.Errorf("the refusal must say the claim was released:\n%s", stderr)
	}
}

func TestPromptWriteFailReleases(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")

	// The destination's parent exists (the pre-claim check passes) but the destination is itself a
	// directory, so the prompt write — the last step — fails.
	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--prompt-file", t.TempDir()})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d:\n%s", rc, deskkit.ExitUnverifiable, stderr)
	}
	if !s.ran("dispatch-claim.sh release") {
		t.Error("a prompt-write failure left the claim HELD")
	}
	if !strings.Contains(stderr, "item-1 was released") {
		t.Errorf("the refusal must say the claim was released:\n%s", stderr)
	}
}

// TestClaimReleaseChokepoint is the class guard. The defect class is "a return path after the
// claim that does not release it"; each fix that added one more release call closed one instance
// and left the next. The guard pins the SHAPE instead: stepClaim is called once, from dispatch, and
// every return after that call (outside the claim's own error branch) is `<held>.settle(...)`, and
// releaseClaim is reached only through heldClaim. A new post-claim step that returns its error raw
// reddens here.
func TestClaimReleaseChokepoint(t *testing.T) {
	files, fset := parsePackage(t, ".")
	if leaks := claimReleaseLeaks(fset, files); len(leaks) > 0 {
		t.Errorf("post-claim paths that bypass heldClaim.settle:\n  %s", strings.Join(leaks, "\n  "))
	}
}

// The guard's positive control: a planted second instance — a raw return after the claim, and a
// direct releaseClaim call outside heldClaim — must be flagged, so the guard is never vacuous.
func TestChokepointGuardFlags(t *testing.T) {
	const planted = `package main
func dispatch(o dispatchOpts) error {
	if err := stepClaim(o, "", "", false, claimAuth{}, ""); err != nil {
		return err
	}
	held := &heldClaim{}
	if err := somethingNew(); err != nil {
		return err
	}
	_ = releaseClaim(o, "", claimAuth{}, "", "")
	return held.settle(nil)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	leaks := claimReleaseLeaks(fset, []*ast.File{f})
	joined := strings.Join(leaks, "\n")
	if !strings.Contains(joined, "planted.go:8") || !strings.Contains(joined, "planted.go:10") {
		t.Errorf("the guard missed the planted instances (want lines 8 and 10):\n%s", joined)
	}
}

func parsePackage(t *testing.T, dir string) ([]*ast.File, *token.FileSet) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, filepath.Base(p), src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files, fset
}

// claimReleaseLeaks returns one line per violation of the release choke point.
func claimReleaseLeaks(fset *token.FileSet, files []*ast.File) []string {
	var out []string
	pos := func(n ast.Node) string { p := fset.Position(n.Pos()); return fmt.Sprintf("%s:%d", p.Filename, p.Line) }
	claimCallers := 0
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			inHeld := fn.Recv != nil && len(fn.Recv.List) == 1 && strings.Contains(exprString(fn.Recv.List[0].Type), "heldClaim")
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch exprString(call.Fun) {
				case "stepClaim":
					claimCallers++
					if fn.Name.Name != "dispatch" {
						out = append(out, pos(call)+": stepClaim called outside dispatch ("+fn.Name.Name+")")
					}
				case "releaseClaim":
					if !inHeld {
						out = append(out, pos(call)+": releaseClaim called outside heldClaim ("+fn.Name.Name+")")
					}
				}
				return true
			})
			if fn.Name.Name == "dispatch" && fn.Recv == nil {
				out = append(out, postClaimReturns(fn, pos)...)
			}
		}
	}
	if claimCallers == 0 {
		out = append(out, "stepClaim is never called — the guard has nothing to anchor on")
	}
	return out
}

// postClaimReturns checks every return in dispatch after the statement that places the claim.
func postClaimReturns(fn *ast.FuncDecl, pos func(ast.Node) string) []string {
	var out []string
	claimed := false
	for _, st := range fn.Body.List {
		if !claimed {
			if ifs, ok := st.(*ast.IfStmt); ok && ifs.Init != nil && containsCall(ifs.Init, "stepClaim") {
				claimed = true // the claim's own error branch returns before anything is held
			}
			continue
		}
		ast.Inspect(st, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(ret.Results) != 1 || !isSettle(ret.Results[0]) {
				out = append(out, pos(ret)+": return after the claim does not go through settle")
			}
			return true
		})
	}
	if !claimed {
		out = append(out, pos(fn)+": dispatch places no claim via `if err := stepClaim(...)`")
	}
	return out
}

func isSettle(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "settle"
}

func containsCall(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		if c, ok := m.(*ast.CallExpr); ok && exprString(c.Fun) == name {
			found = true
		}
		return !found
	})
	return found
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	}
	return ""
}
