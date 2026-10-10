package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Scan references to the package's subprocess boundaries, not only string
// literals at calls. Unknown program/argv expressions and escaped runner values
// fail closed. Existing dynamic relays are pinned by file, receiver and exact
// call shape; this is a source boundary, not a sandbox for operator hooks/scripts.
func allocationViolations(t *testing.T, name string, src []byte) []string {
	t.Helper()
	f, e := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if e != nil {
		t.Fatal(e)
	}
	imports := map[string]string{}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		alias := filepath.Base(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		imports[alias] = path
	}
	var bad []string
	for _, decl := range f.Decls {
		site, receiver := "package-level", ""
		trustedAllocator := false
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			site = fn.Name.Name
			if fn.Recv != nil {
				receiver = guardText(fn.Recv.List[0].Type)
			}
			if name == "reviewworktree.go" && fn.Recv == nil && fn.Name.Name == "createDispatchWorktree" {
				trustedAllocator = true
			}
		}
		allowed := map[ast.Node]bool{}
		if fn != nil {
			allowed[fn.Name] = true
		}
		identity := name + ":" + receiver + ":" + site
		nested := map[ast.Node]bool{}
		ast.Inspect(decl, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok {
				ast.Inspect(lit, func(child ast.Node) bool { nested[child] = true; return true })
				return false
			}
			return true
		})
		ast.Inspect(decl, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok {
					switch id.Name {
					case "runCmd":
						if (!c.Ellipsis.IsValid() && guardSafeCommand(c.Args, 1)) || (trustedAllocator && !nested[c] && guardBindings(fn, c) && guardText(c) == `runCmd(o.root, "deskwt", args...)`) {
							allowed[id] = true
						}
					case "gitOut":
						if !c.Ellipsis.IsValid() && guardSafeGit(c.Args, 1) {
							allowed[id] = true
						}
					}
				}
				if !nested[c] && guardBindings(fn, c) && guardRelay(identity, guardText(c)) {
					allowed[c.Fun] = true
				}
			}
			// The only low-level binding and launch record. Escaping either
			// function value anywhere else is refused, including import aliases.
			if identity == "exec.go::package-level" {
				if v, ok := n.(*ast.ValueSpec); ok && guardText(v) == "execCommand = exec.Command" {
					allowed[v.Names[0]], allowed[v.Values[0]] = true, true
				}
			}
			if identity == "exec.go::runCmdEnv" && !nested[n] && guardBindings(fn, n) {
				if c, ok := n.(*ast.CompositeLit); ok && guardText(c) == "deskkit.ToolCall{Name: name, Args: args, Dir: dir, Env: env, Start: execCommand}" {
					allowed[c.Type] = true
					for _, elt := range c.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok && guardText(kv.Key) == "Start" {
							allowed[kv.Value] = true
						}
					}
				}
			}
			return true
		})
		violation := false
		ast.Inspect(decl, func(n ast.Node) bool {
			if allowed[n] {
				return true
			}
			switch v := n.(type) {
			case *ast.Ident:
				switch v.Name {
				case "runCmd", "runCmdEnv", "execCommand", "gitOut":
					violation = true
				}
			case *ast.SelectorExpr:
				if pkg, ok := v.X.(*ast.Ident); ok {
					path := imports[pkg.Name]
					if (path == "os" && v.Sel.Name == "StartProcess") || (path == "syscall" && (v.Sel.Name == "Exec" || v.Sel.Name == "ForkExec" || v.Sel.Name == "StartProcess")) ||
						(path == "os/exec" && (v.Sel.Name == "Command" || v.Sel.Name == "CommandContext" || v.Sel.Name == "Cmd")) ||
						(path == "github.com/medici-finance/assay/tools/desk/internal/deskkit" && (v.Sel.Name == "Run" || v.Sel.Name == "ToolCall")) {
						violation = true
					}
				}
			case *ast.ImportSpec:
				if v.Name != nil && v.Name.Name == "." {
					violation = true
				}
			}
			return true
		})
		if violation {
			bad = append(bad, name+":"+site)
		}
	}
	return bad
}

// Relay arguments must resolve to the original parameter/receiver or an
// immediate-body local, never a same-spelled variable from a nested block.
func guardBindings(fn *ast.FuncDecl, n ast.Node) bool {
	if fn == nil || fn.Body == nil || n == nil {
		return false
	}
	bindings := map[string]*ast.Object{}
	for _, list := range []*ast.FieldList{fn.Recv, fn.Type.Params} {
		if list != nil {
			for _, field := range list.List {
				for _, id := range field.Names {
					bindings[id.Name] = id.Obj
				}
			}
		}
	}
	for _, stmt := range fn.Body.List {
		if a, ok := stmt.(*ast.AssignStmt); ok && a.Tok == token.DEFINE {
			for _, lhs := range a.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					bindings[id.Name] = id.Obj
				}
			}
		}
	}
	fields := map[*ast.Ident]bool{}
	ast.Inspect(n, func(node ast.Node) bool {
		if sel, ok := node.(*ast.SelectorExpr); ok {
			fields[sel.Sel] = true
		}
		return true
	})
	valid := true
	ast.Inspect(n, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && !fields[id] {
			switch id.Name {
			case "o", "auth", "script", "args", "claimKey", "repo", "dir", "name", "env", "b", "key", "call", "plan", "branch", "ref":
				if bindings[id.Name] == nil || bindings[id.Name] != id.Obj {
					valid = false
				}
			}
		}
		return true
	})
	return valid
}

func guardText(n ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), n); err != nil {
		panic(err)
	}
	return b.String()
}

func guardLiteral(args []ast.Expr, i int) string {
	if i >= len(args) {
		return ""
	}
	v, ok := args[i].(*ast.BasicLit)
	if !ok || v.Kind != token.STRING {
		return ""
	}
	s, _ := strconv.Unquote(v.Value)
	return s
}

func guardSafeCommand(args []ast.Expr, i int) bool {
	switch guardLiteral(args, i) {
	case "git":
		return guardSafeGit(args, i+1)
	case "deskroster":
		return guardLiteral(args, i+1) == "set"
	}
	return false
}

func guardSafeGit(args []ast.Expr, i int) bool {
	// Git is not inherently a read boundary: fetch can select an executable,
	// remote can fetch, and config can install an executable for a later call.
	// Permit only the existing non-executing shapes. The sole existing fetch
	// is inventoried below with its fixed remote and refspec expression.
	switch guardLiteral(args, i) {
	case "rev-parse":
		return true
	case "remote":
		return len(args) == i+3 && guardLiteral(args, i+1) == "get-url" && guardLiteral(args, i+2) == "origin"
	case "config":
		if len(args) == i+3 {
			return guardLiteral(args, i+1) == "extensions.worktreeConfig" && guardLiteral(args, i+2) == "true"
		}
		if len(args) != i+4 || guardLiteral(args, i+1) != "--worktree" {
			return false
		}
		switch guardLiteral(args, i+2) {
		// assay.dispatchRef is a data-only key in the
		// assay.* namespace like assay.runKey: git interprets no assay.* key,
		// so it cannot select an executable for a later call.
		case "user.name", "user.email", "assay.runKey", "assay.dispatchRef":
			return true
		}
	}
	return false
}

func guardRelay(identity, call string) bool {
	// Audited forwarding inventory, not whole-function exemptions. Changing
	// a target, receiver, argument source or call shape requires a new review.
	for _, allowed := range map[string][]string{
		"exec.go::runCmd":           {`runCmdEnv(dir, nil, name, args...)`},
		"exec.go::runCmdEnv":        {`deskkit.Run(call)`},
		"worktree.go::worktreeBase": {`runCmd(o.root, "git", "fetch", "--quiet", "origin", "+refs/heads/"+branch+":"+ref)`},
		"worktree.go::gitOut":       {`runCmd(dir, "git", args...)`},
		"dispatch.go::stepClaim":    {`runCmdEnv(o.root, auth.env, script, append(args, auth.args...)...)`, `runCmdEnv(o.root, auth.env, script, append([]string{"show", claimKey, "--repo", repo}, auth.args...)...)`},
		"dispatch.go::releaseClaim": {`runCmdEnv(o.root, auth.env, script, append([]string{"release", claimKey, "--repo", repo}, auth.args...)...)`},
		"dispatch.go::stepDecision": {`runCmdEnv(o.root, auth.scriptEnv, script, "ensure", briefArg(o), "--repo", repo, "--at", "start")`},
		"repairadmission.go:*claimToolAdmissionBackend:acquireLease": {`runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool, append([]string{"acquire", key, "--repo", b.repo}, b.auth.args...)...)`},
		"repairadmission.go:*claimToolAdmissionBackend:releaseLease": {`runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool, append([]string{"release", key, "--repo", b.repo}, b.auth.args...)...)`},
		"repairadmission.go:*claimToolAdmissionBackend:occupancy":    {`runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool, append([]string{"list", "--repo", b.repo}, b.auth.args...)...)`},
	}[identity] {
		if call == allowed {
			return true
		}
	}
	return false
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
	src := []byte(`package main; type example struct{}; func (example) createDispatchWorktree(o dispatchOpts, plan dispatchPlan) { args := []string{}; runCmd(o.root, "deskwt", args...) }`)
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
