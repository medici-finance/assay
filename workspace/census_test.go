package workspace

// census_test.go — the caller census of docs/workspace-callers.md, dereferenced against the
// real source tree. The scan finds every production site that creates, removes, prunes,
// recursively copies or deletes a checkout, delegates that to the worktree tool, or emits a
// recipe telling an agent to do it; the census document must route each one (exact site,
// symbol and effect) and list nothing the scan cannot find. A new caller therefore fails this
// test until it is routed, and a retired one fails it until its row goes.

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// censusDoc is the census, relative to the repository root.
const censusDoc = "docs/workspace-callers.md"

// unit is one routed caller: a file, the enclosing function (or "-" for a non-Go file) and the
// effect it has on a checkout.
type unit struct{ Site, Symbol, Effect string }

func (u unit) String() string { return u.Site + " " + u.Symbol + " " + u.Effect }

// Effects a census row may name.
var censusEffects = map[string]string{
	"create":   "creates a checkout (git worktree add, git clone)",
	"remove":   "removes a linked worktree (git worktree remove)",
	"prune":    "prunes worktree administration (git worktree prune)",
	"rmtree":   "recursively deletes a directory in a function that manages worktrees",
	"copy":     "recursively copies a source tree into a throwaway directory",
	"delegate": "runs the worktree tool's lifecycle verb",
	"recipe":   "a shipped script, workflow or document that runs or tells an agent to run one of the above",
	"prompt":   "a Go string emitted to an agent or operator naming one of the above",
}

// Dispositions a census row may name (spec/cell-workspaces-v1.md section 9).
var censusDispositions = map[string]bool{"managed": true, "snapshot": true, "temporary-compatibility": true}

// Owner steps a census row may name.
var ownerStep = regexp.MustCompile(`^WS-(0[2-9]|1[0-3])$`)

var (
	gitVerbRe     = regexp.MustCompile(`\bgit\s+(?:-C\s+\S+\s+)?(?:worktree\s+(?:add|remove|prune)|clone)\b`)
	harnessRe     = regexp.MustCompile(`\bEnterWorktree\b|\bcodex\b[^\n]*\s--worktree\b`)
	deskwtVerbs   = map[string]bool{"add": true, "remove": true, "prune": true, "maintain": true, "role-init": true, "role-clean": true}
	worktreeVerbs = map[string]string{"add": "create", "remove": "remove", "prune": "prune"}
	recipeExts    = map[string]bool{".md": true, ".txt": true, ".tmpl": true, ".sh": true, ".bash": true, ".ps1": true, ".py": true, ".yml": true, ".yaml": true, ".mjs": true}
)

// skipDir reports whether a directory holds no production caller: history, tests, fixtures,
// generated or design documents, and this module itself (its tests plant callers on purpose).
func skipDir(p string) bool {
	base := path.Base(p)
	switch base {
	case ".git", "testdata", "tests", "node_modules", "vendor":
		return true
	}
	switch p {
	case "docs", "changelog", "spec", "schemas", "workspace":
		return true
	}
	return false
}

func skipFile(p string) bool {
	return strings.HasSuffix(p, "_test.go") || strings.HasSuffix(p, ".test.sh") || p == "CHANGELOG.md"
}

// scanTree walks fsys (a repository root) and returns every caller unit it finds.
func scanTree(fsys fs.FS) ([]unit, error) {
	found := map[unit]bool{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && skipDir(p) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || skipFile(p) {
			return nil
		}
		ext := path.Ext(p)
		if ext != ".go" && !recipeExts[ext] {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		var us []unit
		if ext == ".go" {
			us, err = scanGo(p, b)
			if err != nil {
				return err
			}
		} else {
			us = scanText(p, b)
		}
		for _, u := range us {
			found[u] = true
		}
		return nil
	})
	out := make([]unit, 0, len(found))
	for u := range found {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out, err
}

// scanText finds recipe lines in a non-Go file. Comment-only shell lines still count: a recipe
// is a recipe whether or not this repository runs it.
func scanText(p string, b []byte) []unit {
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if gitVerbRe.MatchString(sc.Text()) || harnessRe.MatchString(sc.Text()) {
			return []unit{{p, "-", "recipe"}}
		}
	}
	return nil
}

// scanGo reads one Go file's syntax tree. Comments never count: only string literals and calls.
func scanGo(p string, b []byte) ([]unit, error) {
	if bytes.Contains(b, []byte("\n//go:build ignore")) {
		return nil, nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), p, b, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	ownTool := strings.Contains(p, "/cmd/deskwt/")
	var out []unit
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			sym := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) == 1 {
				sym = recvName(d.Recv.List[0].Type) + "." + sym
			}
			for e := range effectsOf(d.Body, d.Name.Name, ownTool) {
				out = append(out, unit{p, sym, e})
			}
		case *ast.GenDecl:
			// Package-level constants and variables: a usage text or prompt template is a
			// caller as much as a function is. The symbol is the declared name.
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Values) == 0 {
					continue
				}
				names := make([]string, len(vs.Names))
				for i, n := range vs.Names {
					names[i] = n.Name
				}
				for _, v := range vs.Values {
					for e := range effectsOf(v, "", ownTool) {
						out = append(out, unit{p, strings.Join(names, ","), e})
					}
				}
			}
		}
	}
	return out, nil
}

// effectsOf reads one function body or package-level value.
func effectsOf(body ast.Node, funcName string, ownTool bool) map[string]bool {
	var lits []string
	calls := map[string]bool{}
	gitClone, deskwtCall := false, false
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if s, err := strconv.Unquote(x.Value); err == nil {
					lits = append(lits, s)
				}
			}
		case *ast.CallExpr:
			name := ""
			switch f := x.Fun.(type) {
			case *ast.SelectorExpr:
				name = f.Sel.Name
				if id, ok := f.X.(*ast.Ident); ok {
					calls[id.Name+"."+f.Sel.Name] = true
				}
			case *ast.Ident:
				name = f.Name
			}
			args := callLits(x)
			gitClone = gitClone || (args["clone"] && (args["git"] || strings.Contains(strings.ToLower(name), "git")))
			deskwtCall = deskwtCall || args["deskwt"]
		}
		return true
	})
	effects := map[string]bool{}
	verbs := false
	for _, s := range lits {
		verbs = verbs || deskwtVerbs[s]
	}
	if gitClone {
		effects["create"] = true
	}
	if deskwtCall && verbs && !ownTool {
		effects["delegate"] = true
	}
	for i, s := range lits {
		if s == "worktree" && i+1 < len(lits) && worktreeVerbs[lits[i+1]] != "" {
			effects[worktreeVerbs[lits[i+1]]] = true
		}
		if gitVerbRe.MatchString(s) || harnessRe.MatchString(s) {
			effects["prompt"] = true
		}
	}
	managesTree := effects["create"] || effects["remove"] || effects["prune"] ||
		strings.Contains(strings.ToLower(funcName), "worktree")
	if calls["os.RemoveAll"] && managesTree {
		effects["rmtree"] = true
	}
	if calls["filepath.WalkDir"] && calls["os.MkdirAll"] && (calls["os.WriteFile"] || calls["os.Symlink"]) {
		effects["copy"] = true
	}
	return effects
}

// callLits returns the string-literal arguments of one call.
func callLits(c *ast.CallExpr) map[string]bool {
	out := map[string]bool{}
	for _, a := range c.Args {
		if bl, ok := a.(*ast.BasicLit); ok && bl.Kind == token.STRING {
			if s, err := strconv.Unquote(bl.Value); err == nil {
				out[s] = true
			}
		}
	}
	return out
}

func recvName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return recvName(x.X)
	case *ast.IndexExpr:
		return recvName(x.X)
	case *ast.IndexListExpr:
		return recvName(x.X)
	case *ast.Ident:
		return x.Name
	}
	return "?"
}

// censusRow is one routed row of the census document.
type censusRow struct {
	unit
	Disposition, Owner, Retirement string
	Line                           int
}

var revisionRe = regexp.MustCompile("(?m)^Source revision: `([0-9a-f]{40})`")

// parseCensus reads the census table: | site | symbol | effect | disposition | owner step |
// retirement condition |, with the site and symbol in backticks.
func parseCensus(b []byte) (rev string, rows []censusRow, err error) {
	m := revisionRe.FindSubmatch(b)
	if m == nil {
		return "", nil, fmt.Errorf("%s: no `Source revision:` line naming a full commit", censusDoc)
	}
	rev = string(m[1])
	inTable := false
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}
		if len(cells) == 6 && cells[0] == "Site" {
			inTable = true
			continue
		}
		if !inTable || strings.HasPrefix(cells[0], "---") {
			continue
		}
		if len(cells) != 6 {
			return rev, nil, fmt.Errorf("%s:%d: want 6 cells, got %d", censusDoc, i+1, len(cells))
		}
		r := censusRow{unit: unit{strings.Trim(cells[0], "`"), strings.Trim(cells[1], "`"), cells[2]},
			Disposition: cells[3], Owner: cells[4], Retirement: cells[5], Line: i + 1}
		if censusEffects[r.Effect] == "" {
			return rev, nil, fmt.Errorf("%s:%d: unknown effect %q", censusDoc, r.Line, r.Effect)
		}
		if !censusDispositions[r.Disposition] {
			return rev, nil, fmt.Errorf("%s:%d: unknown disposition %q", censusDoc, r.Line, r.Disposition)
		}
		if !ownerStep.MatchString(r.Owner) {
			return rev, nil, fmt.Errorf("%s:%d: owner step %q is not WS-02..WS-13", censusDoc, r.Line, r.Owner)
		}
		if len(r.Retirement) < 12 {
			return rev, nil, fmt.Errorf("%s:%d: no retirement condition", censusDoc, r.Line)
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return rev, nil, fmt.Errorf("%s: no census rows", censusDoc)
	}
	return rev, rows, nil
}

// reconcile compares the scan with the census. Every discovered unit must be routed by exactly
// one row; every row must name a unit the scan found.
func reconcile(found []unit, rows []censusRow) []string {
	var problems []string
	routed := map[unit]int{}
	for _, r := range rows {
		if prev, dup := routed[r.unit]; dup {
			problems = append(problems, fmt.Sprintf("%s:%d: %s routed twice (also line %d)", censusDoc, r.Line, r.unit, prev))
		}
		routed[r.unit] = r.Line
	}
	seen := map[unit]bool{}
	for _, u := range found {
		seen[u] = true
		if _, ok := routed[u]; !ok {
			problems = append(problems, "unrouted caller: "+u.String())
		}
	}
	for _, r := range rows {
		if !seen[r.unit] {
			problems = append(problems, fmt.Sprintf("%s:%d: stale row, no such caller in source: %s", censusDoc, r.Line, r.unit))
		}
	}
	return problems
}

// overlay is the real tree with planted files on top.
type overlay struct {
	base   fs.FS
	plants fstest.MapFS
}

func (o overlay) Open(name string) (fs.File, error) {
	if _, ok := o.plants[name]; ok {
		return o.plants.Open(name)
	}
	return o.base.Open(name)
}

func (o overlay) ReadDir(name string) ([]fs.DirEntry, error) {
	extra, perr := fs.ReadDir(o.plants, name)
	entries, err := fs.ReadDir(o.base, name)
	if err != nil {
		if perr == nil {
			return extra, nil // a directory only the plants create
		}
		return nil, err
	}
	have := map[string]bool{}
	for _, e := range entries {
		have[e.Name()] = true
	}
	for _, e := range extra {
		if !have[e.Name()] {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// realTree returns the repository root and its census, or skips (could-not-check) when this
// module is not inside a repository checkout carrying the census and its sources.
func realTree(t *testing.T) (fs.FS, []byte) {
	t.Helper()
	root := os.DirFS("..")
	doc, err := fs.ReadFile(root, censusDoc)
	if err != nil {
		t.Skipf("could-not-check: census %s unreadable from the module's parent: %v", censusDoc, err)
	}
	if _, err := fs.Stat(root, "tools/desk/cmd/deskwt/deskwt.go"); err != nil {
		t.Skipf("could-not-check: worktree tool source absent: %v", err)
	}
	return root, doc
}

// regression: F-cell-workspace-census
func TestWorkspaceCallerCensus(t *testing.T) {
	root, doc := realTree(t)
	_, rows, err := parseCensus(doc)
	if err != nil {
		t.Fatal(err)
	}
	found, err := scanTree(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("scan found no caller at all: the scope or the scanner is broken, not the tree clean")
	}
	for _, p := range reconcile(found, rows) {
		t.Error(p)
	}

	// The known S-worktree creation duplicates are dereferenced against source, not counted from
	// prose: each must be a discovered create unit, and routed.
	known := []unit{
		{"tools/desk/cmd/cellctl/launch.go", "Cell.deskLaunch", "create"},
		{"tools/desk/cmd/commsloop/dispatch_native.go", "gitDetachedWorktreeComms", "create"},
		{"tools/desk/cmd/deskmerge/currency.go", "newWorktree", "create"},
		{"tools/desk/cmd/deskreconcile/reconcile.go", "Run", "create"},
		{"tools/desk/cmd/scanloop/lane.go", "scanCarrierPRLane.carry", "create"},
		{"tools/desk/cmd/verifyloop/dispatch_native.go", "gitDetachedWorktree", "create"},
		{"statusgen/lintaudit.go", "productionWorktree", "create"},
		{"tools/desk/cmd/deskwt/deskwt.go", "cmdAdd", "create"},
	}
	have := map[unit]bool{}
	for _, u := range found {
		have[u] = true
	}
	for _, k := range known {
		if !have[k] {
			t.Errorf("known caller not found in source: %s", k)
		}
	}

	// Planted callers: each kind of unrouted site must fail the census until routed.
	plants := map[string]struct {
		file, body string
		want       unit
	}{
		"creator": {"tools/desk/cmd/zzplant/plant.go",
			"package main\n\nfunc plantAdd(dir string) []string {\n\treturn []string{\"git\", \"-C\", dir, \"worktree\", \"add\", \"x\"}\n}\n",
			unit{"tools/desk/cmd/zzplant/plant.go", "plantAdd", "create"}},
		"remover": {"tools/desk/cmd/zzplant/plant.go",
			"package main\n\ntype p struct{}\n\nfunc (*p) drop(dir string) []string {\n\treturn []string{\"worktree\", \"remove\", \"--force\", dir}\n}\n",
			unit{"tools/desk/cmd/zzplant/plant.go", "p.drop", "remove"}},
		"prompt": {"plugins/assay/references/zzplant.md",
			"# plant\n\nIsolate first: `git worktree add ../x -b x origin/main`.\n",
			unit{"plugins/assay/references/zzplant.md", "-", "recipe"}},
	}
	for name, pl := range plants {
		t.Run("planted-"+name, func(t *testing.T) {
			tree := overlay{root, fstest.MapFS{pl.file: {Data: []byte(pl.body)}}}
			got, err := scanTree(tree)
			if err != nil {
				t.Fatal(err)
			}
			problems := reconcile(got, rows)
			want := "unrouted caller: " + pl.want.String()
			hit := false
			for _, p := range problems {
				hit = hit || p == want
			}
			if !hit {
				t.Fatalf("planted %s was not reported; problems: %v", name, problems)
			}
			routedRows := append(append([]censusRow(nil), rows...), censusRow{unit: pl.want,
				Disposition: "managed", Owner: "WS-03", Retirement: "routed by the plant"})
			if left := reconcile(got, routedRows); len(left) != 0 {
				t.Fatalf("routing the planted %s did not clear the census: %v", name, left)
			}
		})
	}

	t.Run("stale-row", func(t *testing.T) {
		gone := censusRow{unit: unit{"tools/desk/cmd/zzgone/gone.go", "gone", "create"},
			Disposition: "managed", Owner: "WS-03", Retirement: "a row whose caller was deleted"}
		if len(reconcile(found, append(append([]censusRow(nil), rows...), gone))) == 0 {
			t.Fatal("a row naming no caller was not reported")
		}
	})
}
