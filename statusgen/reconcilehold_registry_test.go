package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// statusKeyedRuleScan walks run()'s package-local call graph (depth-bounded)
// and returns every PROBLEM-producing callee — named check* or *Problems, the
// lint's naming convention — whose body, directly or through package-local
// helpers, reads a `.Status` selector. Those are the rules whose verdict a
// Status-cell write can change, i.e. the set reconcile --apply's lint hold must
// evaluate.
func statusKeyedRuleScan(files []*ast.File) []string {
	decls := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
				decls[fd.Name.Name] = fd
			}
		}
	}
	callees := func(fd *ast.FuncDecl) []string {
		var out []string
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok && decls[id.Name] != nil {
					out = append(out, id.Name)
				}
			}
			return true
		})
		return out
	}
	readsMemo := map[string]bool{}
	var readsStatus func(name string, depth int, visiting map[string]bool) bool
	readsStatus = func(name string, depth int, visiting map[string]bool) bool {
		if v, ok := readsMemo[name]; ok {
			return v
		}
		if visiting[name] || depth > 6 {
			return false
		}
		visiting[name] = true
		defer delete(visiting, name)
		fd := decls[name]
		hit := false
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "Status" {
				hit = true
			}
			return !hit
		})
		if !hit {
			for _, c := range callees(fd) {
				if readsStatus(c, depth+1, visiting) {
					hit = true
					break
				}
			}
		}
		readsMemo[name] = hit
		return hit
	}
	isRule := func(name string) bool {
		return strings.HasPrefix(name, "check") || strings.HasSuffix(name, "Problems")
	}
	found := map[string]bool{}
	if decls["run"] == nil {
		return nil
	}
	for _, c := range callees(decls["run"]) {
		if isRule(c) && readsStatus(c, 0, map[string]bool{}) {
			found[c] = true
		}
	}
	var out []string
	for n := range found {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// unregisteredStatusKeyedRules is the scan minus the registry and the declared
// not-a-row exemptions.
func unregisteredStatusKeyedRules(files []*ast.File) []string {
	known := map[string]bool{}
	for _, r := range statusKeyedLintRules {
		known[r.name] = true
	}
	for n := range statusKeyedNotRows {
		known[n] = true
	}
	var missing []string
	for _, n := range statusKeyedRuleScan(files) {
		if !known[n] {
			missing = append(missing, n)
		}
	}
	return missing
}

func parsePackageFiles(t *testing.T) []*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, p := range pkgs {
		for _, f := range p.Files {
			files = append(files, f)
		}
	}
	return files
}

// TestStatusKeyedLintRulesRegistered is the class guard for "a Status-cell
// writer moves a row into a state the lint rejects": a NEW lint rule run()
// calls that reads a row's Status must be registered with the --apply hold (or
// declared not-a-row), or this test fails naming it — so the hourly writer can
// never again be one rule behind the lint it is checked by.
func TestStatusKeyedLintRulesRegistered(t *testing.T) {
	files := parsePackageFiles(t)
	if missing := unregisteredStatusKeyedRules(files); len(missing) != 0 {
		t.Fatalf("run() calls Status-keyed lint rule(s) the reconcile --apply hold does not evaluate: %v — add each to statusKeyedLintRules (reconcilehold.go), or to statusKeyedNotRows with the reason it never reads a brief row", missing)
	}
	// Every registered name must still be a live run() callee — a stale entry
	// would read as coverage it no longer gives.
	live := map[string]bool{}
	for _, n := range statusKeyedRuleScan(files) {
		live[n] = true
	}
	for _, r := range statusKeyedLintRules {
		if !live[r.name] {
			t.Errorf("statusKeyedLintRules names %q, which run() no longer calls as a Status-keyed rule", r.name)
		}
	}
}

// TestStatusKeyedRuleScanCatchesPlantedRule plants a second instance of the
// class — a new run() callee that reads a row's Status through a helper and is
// not registered — and proves the guard names it. Without this, a scan that
// silently matched nothing would pass the test above forever.
func TestStatusKeyedRuleScanCatchesPlantedRule(t *testing.T) {
	const planted = `package main
func run() int { _ = plantedGateProblems(nil); _ = designGateProblems("", nil); return 0 }
func plantedGateProblems(s []*Stream) []string { return rowState(s) }
func rowState(s []*Stream) []string { return []string{s[0].Briefs[0].Status} }
func designGateProblems(root string, s []*Stream) []string { return []string{s[0].Briefs[0].Status} }
`
	f, err := parser.ParseFile(token.NewFileSet(), "planted.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := unregisteredStatusKeyedRules([]*ast.File{f})
	if len(got) != 1 || got[0] != "plantedGateProblems" {
		t.Fatalf("the guard must name exactly the planted unregistered rule (and not the registered designGateProblems); got %v", got)
	}
}
