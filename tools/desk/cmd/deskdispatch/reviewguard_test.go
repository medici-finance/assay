package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Any new subprocess allocation must use the one seam that keeps reviewer
// worktrees detached. Scan every non-test source file, not only today's caller.
func allocationViolations(t *testing.T, name string, src []byte) []string {
	t.Helper()
	f, e := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if e != nil {
		t.Fatal(e)
	}
	var bad []string
	for _, decl := range f.Decls {
		site := "package-level"
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			site = fn.Name.Name
			if name == "reviewworktree.go" && fn.Recv == nil && fn.Name.Name == "createDispatchWorktree" {
				continue
			}
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, a := range c.Args {
				v, ok := a.(*ast.BasicLit)
				if !ok || v.Kind != token.STRING {
					continue
				}
				s, _ := strconv.Unquote(v.Value)
				if s == "deskwt" {
					bad = append(bad, name+":"+site)
				}
			}
			return true
		})
	}
	return bad
}
func TestAllocationClassGuard(t *testing.T) {
	paths, e := filepath.Glob("*.go")
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		count++
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if bad := allocationViolations(t, path, b); len(bad) > 0 {
			t.Errorf("allocation outside detached review seam: %v", bad)
		}
	}
	if count == 0 {
		t.Fatal("no source scanned")
	}
}
func TestAllocationGuardPlant(t *testing.T) {
	b, e := os.ReadFile("testdata/review-allocation.go.txt")
	if e != nil {
		t.Fatal(e)
	}
	bad := allocationViolations(t, "planted.go", b)
	if len(bad) != 1 || bad[0] != "planted.go:plantedAllocation" {
		t.Fatalf("guard missed second allocation: %v", bad)
	}
}
func TestAllocationMethodPlant(t *testing.T) {
	src := []byte("package main; type example struct{}; func (example) createDispatchWorktree() { runCmd(\"\", \"deskwt\", \"add\") }")
	if bad := allocationViolations(t, "reviewworktree.go", src); len(bad) != 1 {
		t.Fatalf("same-name method escaped guard: %v", bad)
	}
}

func TestReviewNameBoundary(t *testing.T) {
	old := reviewEntropy
	t.Cleanup(func() { reviewEntropy = old })
	reviewEntropy = bytes.NewReader(append(bytes.Repeat([]byte{1}, 8), bytes.Repeat([]byte{2}, 8)...))
	first, e := freshReviewName(strings.Repeat("x", 64))
	if e != nil {
		t.Fatal(e)
	}
	second, e := freshReviewName(strings.Repeat("x", 64))
	if e != nil {
		t.Fatal(e)
	}
	if first == second || len(first) != 64 || !worktreeNameRe.MatchString(first) || !strings.HasSuffix(first, "-0101010101010101") || !strings.HasSuffix(second, "-0202020202020202") {
		t.Fatalf("nonce lost at boundary: %s %s", first, second)
	}
	if name, e := freshReviewName("short"); e == nil || name != "" {
		t.Fatalf("entropy failure guessed name=%q err=%v", name, e)
	}
}

func TestReviewEntropyPreClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	gh := installGHStamp(t)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "unused"))
	old := reviewEntropy
	reviewEntropy = bytes.NewReader(nil)
	t.Cleanup(func() { reviewEntropy = old })
	rc, _ := runCapturingStderr(t, []string{"assay--pr-77", "--root", root, "--repo", allowedRepo, "--kit", "review", "--pr", "77", "--model", "example-model-1", "--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != 6 || s.ran("dispatch-claim.sh acquire") || len(s.deskwtCalls()) != 0 || len(gh.requests) != 0 {
		t.Fatalf("entropy failure reached durable step: rc=%d calls=%v", rc, s.calls)
	}
}

func TestAllocationGlobalPlant(t *testing.T) {
	src := []byte("package main; var planted = runCmd(\"\", \"deskwt\", \"add\")")
	if bad := allocationViolations(t, "planted.go", src); len(bad) != 1 {
		t.Fatalf("package initializer escaped guard: %v", bad)
	}
}
