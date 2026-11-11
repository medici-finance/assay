package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// The class guards for #4683's two review findings, over every non-test source
// file in this package. Each one is also run on planted files, so a second
// instance at a site the fix does not touch goes red.

// modSources is every non-test Go source in this package (packageSources).
func modSources(t *testing.T) map[string]string {
	t.Helper()
	files, err := packageSources()
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// callName is the called function's identifier for a plain call f(...).
func callName(call *ast.CallExpr) string {
	if id, ok := call.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// goModDownloadLiteral reports whether call is exec.Command or
// exec.CommandContext starting `go mod download`.
func goModDownloadLiteral(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "exec" {
		return false
	}
	var words []string
	for _, a := range call.Args {
		if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if w, err := strconv.Unquote(lit.Value); err == nil {
				words = append(words, w)
			}
		}
	}
	return len(words) >= 3 && words[0] == "go" && words[1] == "mod" && words[2] == "download"
}

// unscreenedDownloads returns "file: func" for every module download site that
// is not the one screened site: a call to goModDownload outside
// prepareModuleCache, a `go mod download` command outside goModDownload's own
// body, and a prepareModuleCache whose first download is not preceded by the
// egress screen (moduleEgressProblem).
func unscreenedDownloads(t *testing.T, files map[string]string) []string {
	t.Helper()
	var stray []string
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				// goModDownload's own body is the one place the command runs.
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, v := range vs.Values {
						owner := vs.Names[i].Name
						ast.Inspect(v, func(n ast.Node) bool {
							if call, ok := n.(*ast.CallExpr); ok && goModDownloadLiteral(call) && owner != "goModDownload" {
								stray = append(stray, name+": "+owner+" (go mod download)")
							}
							return true
						})
					}
				}
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				fn := d.Name.Name
				screen, download := token.NoPos, token.NoPos
				ast.Inspect(d.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch {
					case goModDownloadLiteral(call):
						stray = append(stray, name+": "+fn+" (go mod download)")
					case callName(call) == "goModDownload":
						if fn != "prepareModuleCache" {
							stray = append(stray, name+": "+fn+" (goModDownload)")
						} else if download == token.NoPos {
							download = call.Pos()
						}
					case callName(call) == "moduleEgressProblem" && screen == token.NoPos:
						screen = call.Pos()
					}
					return true
				})
				if fn == "prepareModuleCache" && download != token.NoPos && (screen == token.NoPos || screen > download) {
					stray = append(stray, name+": "+fn+" (download before the egress screen)")
				}
			}
		}
	}
	slices.Sort(stray)
	return stray
}

// regression: #4683
// TestModDownloadScreenedOnce is the class guard for egress-control parity: a
// module download that reaches the public proxy runs at exactly one site, and
// that site screens the module paths before its first request. The planted
// files add a second downloader, a raw `go mod download`, and a
// prepareModuleCache that downloads before it screens.
func TestModDownloadScreenedOnce(t *testing.T) {
	if stray := unscreenedDownloads(t, modSources(t)); len(stray) > 0 {
		t.Fatalf("a module download outside the one screened site: %v", stray)
	}
	planted := map[string]string{"planted.go": "package main\nimport \"os/exec\"\n" +
		"func refillCache(d string) { _, _ = goModDownload(d, nil) }\n" +
		"func rawDownload(d string) { c := exec.Command(\"go\", \"mod\", \"download\"); c.Dir = d; c.Env = nil; _ = c.Run() }\n" +
		"func prepareModuleCache(d string) { _, _ = goModDownload(d, nil); _ = moduleEgressProblem(d, nil, nil, nil) }\n"}
	want := []string{"planted.go: prepareModuleCache (download before the egress screen)", "planted.go: rawDownload (go mod download)", "planted.go: refillCache (goModDownload)"}
	if stray := unscreenedDownloads(t, planted); !slices.Equal(stray, want) {
		t.Fatalf("guard blind to a planted second download site:\n got %q\nwant %q", stray, want)
	}
}

// receiptDecoders returns "file: func" for every function outside
// readBracketReceipt that decodes JSON into a bracketReceipt, and for a
// readBracketReceipt that does not bind the module cache to the home
// (moduleCacheRequirementProblem).
func receiptDecoders(t *testing.T, files map[string]string) []string {
	t.Helper()
	var stray []string
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			names := receiptVars(fn)
			decodes, binds := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if callName(call) == "moduleCacheRequirementProblem" {
					binds = true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "Unmarshal" && sel.Sel.Name != "Decode") || len(call.Args) == 0 {
					return true
				}
				target := call.Args[len(call.Args)-1]
				if u, ok := target.(*ast.UnaryExpr); ok && u.Op == token.AND {
					target = u.X
				}
				if id, ok := target.(*ast.Ident); ok && names[id.Name] {
					decodes = true
				}
				return true
			})
			switch {
			case decodes && fn.Name.Name != "readBracketReceipt":
				stray = append(stray, name+": "+fn.Name.Name)
			case fn.Name.Name == "readBracketReceipt" && !binds:
				stray = append(stray, name+": readBracketReceipt (no moduleCacheRequirementProblem)")
			}
		}
	}
	slices.Sort(stray)
	return stray
}

// receiptVars names fn's variables and results of type bracketReceipt or
// *bracketReceipt.
func receiptVars(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	isReceipt := func(e ast.Expr) bool {
		if s, ok := e.(*ast.StarExpr); ok {
			e = s.X
		}
		if c, ok := e.(*ast.CompositeLit); ok {
			e = c.Type
		}
		if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
			if c, ok := u.X.(*ast.CompositeLit); ok {
				e = c.Type
			}
		}
		if call, ok := e.(*ast.CallExpr); ok && callName(call) == "new" && len(call.Args) == 1 {
			e = call.Args[0]
		}
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "bracketReceipt"
	}
	if fn.Type.Results != nil {
		for _, f := range fn.Type.Results.List {
			if isReceipt(f.Type) {
				for _, n := range f.Names {
					names[n.Name] = true
				}
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			if n.Type != nil && isReceipt(n.Type) {
				for _, id := range n.Names {
					names[id.Name] = true
				}
			}
			for i, v := range n.Values {
				if i < len(n.Names) && isReceipt(v) {
					names[n.Names[i].Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, v := range n.Rhs {
				if i < len(n.Lhs) && isReceipt(v) {
					if id, ok := n.Lhs[i].(*ast.Ident); ok {
						names[id.Name] = true
					}
				}
			}
		}
		return true
	})
	return names
}

// regression: #4683
// TestReceiptReadBindsModuleCache is the class guard for a requirement the
// open step enforces but a later stage does not re-derive: every stage reads
// the receipt through readBracketReceipt, and readBracketReceipt binds the
// receipt's module cache to the home's manifest. The planted file decodes a
// receipt in three other shapes and carries a readBracketReceipt without the
// binding.
func TestReceiptReadBindsModuleCache(t *testing.T) {
	if stray := receiptDecoders(t, modSources(t)); len(stray) > 0 {
		t.Fatalf("a receipt read that skips the module-cache binding: %v", stray)
	}
	planted := map[string]string{"planted.go": "package main\nimport (\"encoding/json\"; \"os\")\n" +
		"func closeAgain(b []byte) { var r bracketReceipt; _ = json.Unmarshal(b, &r) }\n" +
		"func admitAgain(b []byte) { r := &bracketReceipt{}; _ = json.Unmarshal(b, r) }\n" +
		"func streamReceipt(f *os.File) (r bracketReceipt) { _ = json.NewDecoder(f).Decode(&r); return }\n" +
		"func readBracketReceipt(b []byte) { var r bracketReceipt; _ = json.Unmarshal(b, &r) }\n"}
	want := []string{"planted.go: admitAgain", "planted.go: closeAgain", "planted.go: readBracketReceipt (no moduleCacheRequirementProblem)", "planted.go: streamReceipt"}
	if stray := receiptDecoders(t, planted); !slices.Equal(stray, want) {
		t.Fatalf("guard blind to a planted second receipt reader:\n got %q\nwant %q", stray, want)
	}
}
