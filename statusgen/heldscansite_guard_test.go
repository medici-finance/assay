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
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "heldOrCouldNotCheckRe" {
					out = append(out, filepath.Base(f)+":"+fn.Name.Name)
				}
				return true
			})
		}
	}
	sort.Strings(out)
	return out
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
