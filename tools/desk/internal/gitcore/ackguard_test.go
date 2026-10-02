package gitcore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The bounded class is use of go-git's ReceivePack method anywhere in the desk
// module. Inventory references, not only direct calls, so taking a method value
// cannot evade the single validated sink. This is not arbitrary Go dataflow
// analysis or a claim about other Git clients/shell commands.
func ackSites(name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	allowed := map[*ast.SelectorExpr]bool{}
	if name == "internal/gitcore/claimack.go" && f.Name.Name == "gitcore" {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "receiveClaim" || fn.Body == nil {
				continue
			}
			params := map[string]*ast.Object{}
			for _, p := range fn.Type.Params.List {
				for _, n := range p.Names {
					params[n.Name] = n.Obj
				}
			}
			for _, stmt := range fn.Body.List {
				a, ok := stmt.(*ast.AssignStmt)
				if !ok || len(a.Rhs) != 1 {
					continue
				}
				c, ok := a.Rhs[0].(*ast.CallExpr)
				if !ok || c.Ellipsis.IsValid() || len(c.Args) != 2 {
					continue
				}
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "ReceivePack" {
					continue
				}
				recv, ok := sel.X.(*ast.Ident)
				if !ok || recv.Name != "sess" || recv.Obj != params["sess"] {
					continue
				}
				ctx, ok := c.Args[0].(*ast.Ident)
				if !ok || ctx.Name != "ctx" || ctx.Obj != params["ctx"] {
					continue
				}
				req, ok := c.Args[1].(*ast.Ident)
				if !ok || req.Name != "req" || req.Obj != params["req"] {
					continue
				}
				allowed[sel] = true
			}
		}
	}
	var bad []string
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "ReceivePack" && !allowed[s] {
			bad = append(bad, name+":ReceivePack")
		}
		return true
	})
	// Exactly one allowed source reference is intentional: deleting the sink or
	// duplicating it is not evidence that a scanner looked at the correct file.
	if name == "internal/gitcore/claimack.go" && len(allowed) != 1 {
		bad = append(bad, name+":expected one audited sink")
	}
	return bad, nil
}

func TestAckReferenceGuard(t *testing.T) {
	root := filepath.Join("..", "..")
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "internal/gitcore/claimack.go" {
			found = true
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		bad, err := ackSites(rel, src)
		if err != nil {
			return err
		}
		for _, s := range bad {
			t.Errorf("unvalidated receive-pack reference: %s", s)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("audited source not found")
	}
}

func TestAckGuardPlants(t *testing.T) {
	good := `package gitcore
func receiveClaim(ctx C, sess S, req R) { rs, err := sess.ReceivePack(ctx, req); _, _ = rs, err }
`
	bad, err := ackSites("internal/gitcore/claimack.go", []byte(good))
	if err != nil || len(bad) != 0 {
		t.Fatalf("healthy: %v %v", bad, err)
	}
	for name, plant := range map[string]string{
		"direct":           `func other(s S) {s.ReceivePack(ctx,req)}`,
		"alias":            `var call = session.ReceivePack`,
		"expression":       `var call = S.ReceivePack`,
		"closure":          `func other() { f := func(){ session.ReceivePack(ctx,req) }; f() }`,
		"same-name-method": `func (x X) receiveClaim(ctx C,sess S,req R) { rs,err:=sess.ReceivePack(ctx,req); _,_=rs,err }`,
	} {
		t.Run(name, func(t *testing.T) {
			bad, err := ackSites("internal/gitcore/claimack.go", []byte(good+plant))
			if err != nil || len(bad) == 0 {
				t.Fatalf("plant missed: %v %v", bad, err)
			}
		})
	}
	bad, err = ackSites("cmd/other.go", []byte(good))
	if err != nil || len(bad) == 0 {
		t.Fatalf("wrong file missed: %v %v", bad, err)
	}
	nested := strings.Replace(good, "rs, err :=", "f := func() { rs, err :=", 1)
	nested = strings.Replace(nested, "_, _ = rs, err }", "_, _ = rs, err }; f() }", 1)
	bad, err = ackSites("internal/gitcore/claimack.go", []byte(nested))
	if err != nil || len(bad) == 0 {
		t.Fatalf("nested exemption missed: %v %v", bad, err)
	}
}
