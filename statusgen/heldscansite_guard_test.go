package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Class guard for #2100.
//
// DEFECT CLASS: a HELD/could-not-check scan over Evidence text that reads the
// marker without the full quotation hygiene — fenced code, blockquotes, struck
// spans and (since #2100) inline code spans. The instance was the one scan
// site, unroutedHeldLine, missing inline code. The class is any OTHER site
// that applies heldOrCouldNotCheckRe to Evidence: it would read quoted output
// as a disposition again, and drift from the one wording the verify-gate card,
// the model autoflip and the human done close share.
//
// The guard: heldOrCouldNotCheckRe may be referenced only inside
// heldScanSites. A new HELD read reaches the marker through unroutedHeldLine
// (or verifyPassHeldContradiction / closeVerifyHeldRefusal on top of it),
// never by matching the regexp itself.
var heldScanSites = map[string]bool{
	"unroutedHeldLine": true,
}

// heldRegexRefs returns "file:func" for every reference to
// heldOrCouldNotCheckRe in the non-test .go files of dir, outside its own
// declaration.
func heldRegexRefs(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, heldRegexRefsIn(filepath.Base(f), file)...)
	}
	sort.Strings(out)
	return out
}

// heldRegexRefsIn returns "name:site" for every reference to
// heldOrCouldNotCheckRe in file. A site is the function whose body holds the
// reference, or "var <names>" for a package-level declaration whose value
// does: an alias there (var x = heldOrCouldNotCheckRe) would let any function
// read the regexp without naming it. The regexp's own declaring name is not a
// reference.
func heldRegexRefsIn(name string, file *ast.File) []string {
	var out []string
	collect := func(site string, n ast.Node) {
		ast.Inspect(n, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "heldOrCouldNotCheckRe" {
				out = append(out, name+":"+site)
			}
			return true
		})
	}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				collect(d.Name.Name, d.Body)
			}
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				names := make([]string, len(vs.Names))
				for i, n := range vs.Names {
					names[i] = n.Name
				}
				site := "var " + strings.Join(names, ",")
				if vs.Type != nil {
					collect(site, vs.Type)
				}
				for _, v := range vs.Values {
					collect(site, v)
				}
			}
		}
	}
	return out
}

// TestHeldRegexGuardSeesAlias is the guard's positive control for the
// package-level path: a planted alias, and a function reading it, must both be
// reported, so a matcher that stops walking value specs fails here instead of
// reporting the tree clean.
func TestHeldRegexGuardSeesAlias(t *testing.T) {
	const src = `package main

var heldOrCouldNotCheckRe = mustRe()

var plantedAlias = heldOrCouldNotCheckRe

func plantedDirect(s string) bool { return heldOrCouldNotCheckRe.MatchString(s) }
`
	file, err := parser.ParseFile(token.NewFileSet(), "plant.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(heldRegexRefsIn("plant.go", file), " ")
	want := "plant.go:var plantedAlias plant.go:plantedDirect"
	if got != want {
		t.Fatalf("guard walker reported %q, want %q", got, want)
	}
}

func TestHeldRegexSingleScanSite(t *testing.T) {
	refs := heldRegexRefs(t, ".")
	// Positive control: the guard must see the one sanctioned site, or its
	// matcher has stopped matching and an empty result means nothing.
	sawSite := false
	for _, r := range refs {
		fn := r[strings.Index(r, ":")+1:]
		if heldScanSites[fn] {
			sawSite = true
			continue
		}
		t.Errorf("%s reads heldOrCouldNotCheckRe directly — a HELD/could-not-check scan outside unroutedHeldLine skips its quotation hygiene (fences, blockquotes, struck and inline code); call unroutedHeldLine instead", r)
	}
	if !sawSite {
		t.Fatalf("guard matched no reference in unroutedHeldLine (refs=%v) — the matcher is broken, not the tree clean", refs)
	}
}

// Class guard for #1894.
//
// DEFECT CLASS: a HELD/could-not-check scan that reads Evidence text outside
// heldScanScope's read scope — an Expect or Command cell, exit-code prose, or
// an Evidence row a later strict-PASS entry re-ran — and so refuses a clean
// pass on text that is not a live row disposition; or, the other way, a scan
// site that calls heldScanScope and ignores its decision. The instance was
// the one scan site, unroutedHeldLine, reading the whole Evidence section.
// The class is any scan site (heldScanSites) that does not take its scope
// from heldScanScope AND read it. Together with TestHeldRegexSingleScanSite
// (one site may read the marker), this pins every HELD read to the one scope
// decision.
//
// heldScopeCallersIn returns each function in file that binds the result of
// a heldScanScope call to a named variable and then reads that variable
// through a selector or an index (scope.lines, scope.supersedes): a call
// whose result is discarded, or bound and never consulted, does not count.
func heldScopeCallersIn(file *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		bound := map[string]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			c, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := c.Fun.(*ast.Ident)
			lhs, ok2 := as.Lhs[0].(*ast.Ident)
			if ok && ok2 && fn.Name == "heldScanScope" && lhs.Name != "_" {
				bound[lhs.Name] = true
			}
			return true
		})
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			var x ast.Expr
			switch n := n.(type) {
			case *ast.SelectorExpr:
				x = n.X
			case *ast.IndexExpr:
				x = n.X
			}
			if id, ok := x.(*ast.Ident); ok && bound[id.Name] {
				out[fd.Name.Name] = true
			}
			return true
		})
	}
	return out
}

// heldScopeMissing returns each heldScanSites function defined in the
// non-test .go files of dir that never binds and reads heldScanScope, and how many
// sites it found defined at all.
func heldScopeMissing(t *testing.T, dir string) (missing []string, defined int) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		callers := heldScopeCallersIn(file)
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && heldScanSites[fd.Name.Name] {
				defined++
				if !callers[fd.Name.Name] {
					missing = append(missing, filepath.Base(f)+":"+fd.Name.Name)
				}
			}
		}
	}
	sort.Strings(missing)
	return missing, defined
}

// TestHeldScopeGuardSeesPlant is the scope guard's positive control: a
// planted site that reads without heldScanScope, one that discards its
// result and one that binds it but never reads it are reported; the twin
// that consults the result is not.
func TestHeldScopeGuardSeesPlant(t *testing.T) {
	const src = `package main

func plantedBare(ev string) bool { return heldOrCouldNotCheckRe.MatchString(ev) }

func plantedDiscard(ev string) bool { _ = heldScanScope(ev); return false }

func plantedUnread(ev string) bool { s := heldScanScope(ev); _ = s; return false }

func plantedScoped(ev string) bool { s := heldScanScope(ev); return s.supersedes(nil) }
`
	file, err := parser.ParseFile(token.NewFileSet(), "plant.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := heldScopeCallersIn(file)
	if got["plantedBare"] || got["plantedDiscard"] || got["plantedUnread"] || !got["plantedScoped"] {
		t.Fatalf("scope guard walker reported %v, want only plantedScoped", got)
	}
}

func TestHeldScanSitesUseScope(t *testing.T) {
	missing, defined := heldScopeMissing(t, ".")
	if defined != len(heldScanSites) {
		t.Fatalf("guard found %d of the %d heldScanSites defined — the matcher is broken, not the tree clean", defined, len(heldScanSites))
	}
	for _, m := range missing {
		t.Errorf("%s reads HELD/could-not-check without consulting heldScanScope — it would read Expect cells, exit-code prose and re-run Evidence rows as live dispositions again (#1894); bind heldScanScope's result and read it", m)
	}
}
