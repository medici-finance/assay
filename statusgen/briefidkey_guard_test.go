package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Class guard for #1960 (statusgen/05 row R).
//
// DEFECT CLASS: a BriefFile's `brief:` field (bf.Brief) used as a KEY — a map
// index, an equality operand, or an argument to a function that matches it against
// refs. Refs (depends:,
// unblocks:, follow-up targets, prose refs) are written <stream>/<NN>, but a
// brief-v2 file's bf.Brief is the hierarchical <cell>:<repo>:<stream>:<NN> id, so
// the raw value silently never matches on a v2 tree. The instance was the
// reciprocity lint (the checkRef self id and the depEdgeIndex keys in
// checkBriefFiles); the siblings were followUpReferencesBack's referrer
// (consumers.go) and the ordering-gate adjacency (orderinggate.go). The fix keys
// on the filename-derived <stream>/<NN> id (expectedBriefID) or resolves the id
// with resolveLocalBriefRef — never normalizeBriefKey, which drops the repo alias
// and whose callers TestAliasDropGuard (topology_guard_test.go) allow-lists.
//
// The scan covers every non-test .go file. A read of `<x>.Brief`, where <x> is a
// BriefFile-bound identifier, is a violation when it is a map index, an ==/!=
// operand (other than against ""), or a call argument — unless the callee only
// formats it for display or reduces it itself (briefIDSafeCallees), or the line
// carries a `briefid:raw <reason>` comment saying why the raw form is right there
// (a self-consistent identity key, the v1 filename match, a display label).

// briefIDSafeCallees take bf.Brief only to print it, or reduce it to the
// <stream>/<NN> form themselves. Listing a reducer here is NOT a remedy for a new
// key use: the remedy is the filename-derived id (expectedBriefID) or
// resolveLocalBriefRef.
var briefIDSafeCallees = map[string]bool{
	// display sinks
	"add": true, "notice": true, "addBad": true, "warn": true,
	"Sprintf": true, "Errorf": true, "Fprintf": true, "Printf": true,
	"Fprintln": true, "Println": true, "issueTitle": true,
	// existing v2-aware reducers, listed so their current call sites are not
	// double-reported. normalizeBriefKey drops the repo alias: its callers are
	// bounded by TestAliasDropGuard's aliasDropCallers, which binds over this list.
	"normalizeBriefKey": true, "parseBriefV2ID": true, "verifyMarker": true,
	"canonicalBriefKey": true, "briefStreamNum": true,
	"followUpReferencesBack": true, // also takes the referrer's path-derived <stream>/<NN> id
}

// briefIDRawMarker waives one line; it must be followed by a reason.
const briefIDRawMarker = "briefid:raw"

// briefFileIdents returns the identifiers inside fn bound to a BriefFile: params
// and results of type BriefFile/*BriefFile, the first LHS of an assignment from
// parseBriefFile/parseBriefFileBytes, `var x *BriefFile`, and the house name bf.
func briefFileIdents(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{"bf": true}
	isBF := func(e ast.Expr) bool {
		if st, ok := e.(*ast.StarExpr); ok {
			e = st.X
		}
		id, ok := e.(*ast.Ident)
		return ok && id.Name == "BriefFile"
	}
	addFields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			if isBF(f.Type) {
				for _, n := range f.Names {
					out[n.Name] = true
				}
			}
		}
	}
	addFields(fn.Type.Params)
	addFields(fn.Type.Results)
	ast.Inspect(fn, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Rhs) == 1 && len(x.Lhs) > 0 {
				if call, ok := x.Rhs[0].(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "parseBriefFile" || id.Name == "parseBriefFileBytes") {
						if l, ok := x.Lhs[0].(*ast.Ident); ok && l.Name != "_" {
							out[l.Name] = true
						}
					}
				}
			}
		case *ast.ValueSpec:
			if x.Type != nil && isBF(x.Type) {
				for _, n := range x.Names {
					out[n.Name] = true
				}
			}
		}
		return true
	})
	return out
}

func calleeName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// briefIDKeyViolations parses one Go source and returns every raw bf.Brief key use
// not waived on its line.
func briefIDKeyViolations(fset *token.FileSet, name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []string
	// Lines waived with a reasoned `briefid:raw` comment. A bare marker (no
	// reason) waives nothing and is itself reported.
	waived := map[int]bool{}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			i := strings.Index(c.Text, briefIDRawMarker)
			if i < 0 {
				continue
			}
			reason := strings.Trim(strings.TrimSpace(c.Text[i+len(briefIDRawMarker):]), "—-: ")
			if reason == "" {
				out = append(out, fset.Position(c.Pos()).String()+": bare "+briefIDRawMarker+" waiver states no reason")
				continue
			}
			waived[fset.Position(c.Pos()).Line] = true
		}
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		idents := briefFileIdents(fn)
		isRawID := func(e ast.Expr) bool {
			sel, ok := e.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Brief" {
				return false
			}
			id, ok := sel.X.(*ast.Ident)
			return ok && idents[id.Name]
		}
		isEmptyLit := func(e ast.Expr) bool {
			b, ok := e.(*ast.BasicLit)
			return ok && b.Kind == token.STRING && (b.Value == `""` || b.Value == "``")
		}
		flag := func(e ast.Expr, why string) {
			if waived[fset.Position(e.Pos()).Line] {
				return
			}
			out = append(out, fset.Position(e.Pos()).String()+": "+fn.Name.Name+": raw brief id "+why+" — key on the filename-derived <stream>/<NN> id (expectedBriefID) or resolve it with resolveLocalBriefRef")
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.IndexExpr:
				if isRawID(x.Index) {
					flag(x.Index, "used as a map index")
				}
			case *ast.BinaryExpr:
				if x.Op == token.EQL || x.Op == token.NEQ {
					if isRawID(x.X) && !isEmptyLit(x.Y) {
						flag(x.X, "compared with "+x.Op.String())
					}
					if isRawID(x.Y) && !isEmptyLit(x.X) {
						flag(x.Y, "compared with "+x.Op.String())
					}
				}
			case *ast.CallExpr:
				if briefIDSafeCallees[calleeName(x)] {
					return true
				}
				for _, a := range x.Args {
					if isRawID(a) {
						flag(a, "passed to "+calleeName(x))
					}
				}
			}
			return true
		})
	}
	return out, nil
}

// TestBriefIDKeyGuard is the class guard: no non-test file uses a raw bf.Brief as
// a key unless the line says why the raw form is right.
func TestBriefIDKeyGuard(t *testing.T) {
	// Positive control: planted repeats of the defect — the pre-fix reciprocity
	// shape (a map index and a checkRef self id), a raw comparison, the pre-fix
	// ordering-gate adjacency link, and a bare waiver — MUST each be flagged; the
	// filename-keyed, allow-listed reducer, display, empty-check and
	// reasoned-waiver forms must not be.
	planted := []byte(`package main
func plantedRecip(bf *BriefFile, idx *depEdgeIndex, byName map[string]*Stream) {
	idx.dependsOf[bf.Brief] = bf.Depends
	checkRef(nil, "p", "depends", "x/01", bf.Brief, byName)
	if bf.Brief == "x/01" {
	}
	id, _, _ := expectedBriefID("docs/streams/x/brief-01.md")
	idx.knownV1[id] = true
	_ = verifyMarker(bf.Brief)
	_ = fmt.Sprintf("%s", bf.Brief)
	if bf.Brief == "" {
	}
	seen[bf.Brief] = true // briefid:raw self-consistent identity key
	seen[bf.Brief] = true // briefid:raw
}
func plantedParsed(path string) {
	p, _, _ := parseBriefFile(path)
	for _, d := range p.Depends {
		link(p.Brief, d)
	}
}
`)
	got, err := briefIDKeyViolations(token.NewFileSet(), "planted.go", planted)
	if err != nil {
		t.Fatal(err)
	}
	// 4 raw-key sites + the bare waiver comment + the key it failed to waive.
	if len(got) != 6 {
		t.Fatalf("positive control: the guard must flag exactly the 6 planted sites, got %d:\n%s", len(got), strings.Join(got, "\n"))
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	var violations []string
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		v, err := briefIDKeyViolations(fset, name, src)
		if err != nil {
			t.Fatal(err)
		}
		violations = append(violations, v...)
		scanned++
	}
	if scanned == 0 {
		t.Fatal("could-not-check: no non-test source files were scanned")
	}
	if len(violations) > 0 {
		t.Fatalf("a brief-v2 bf.Brief is <cell>:<repo>:<stream>:<NN>, never the <stream>/<NN> form refs use — key on the filename-derived id (expectedBriefID) or resolve it with resolveLocalBriefRef, or waive the line with a reasoned `briefid:raw` comment (#1960):\n%s",
			strings.Join(violations, "\n"))
	}
}
