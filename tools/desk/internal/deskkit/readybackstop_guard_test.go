package deskkit

// readybackstop_guard_test.go — the #6242 defect class: a ready flip whose caller reports
// success without the forge's own merge verdict agreeing. Ready means mergeable per the forge,
// so every function in this module that performs a ready flip must also run the post-flip
// backstop (a call to `readyBackstop`, the per-verb adapter over ReadyBackstop). The ready
// primitives themselves (a function NAMED like one, which only delegates or implements the
// mutation) are exempt. A committed positive control (testdata/readyflip_fixture) keeps the
// matcher honest: a guard that stopped matching would also report the fixture clean.

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

// readyFlipPrimitives are the calls that flip a change ready: the seam op, deskpost's
// postBackend op, and deskkit's identity-gated wrapper.
var readyFlipPrimitives = map[string]bool{
	"MarkReadyForReview": true, "markReadyForReview": true, "ReadyFlip": true,
}

// readyFlipsWithoutBackstop returns "file:func" for every non-test function under root that
// calls a ready primitive and never calls readyBackstop.
func readyFlipsWithoutBackstop(root string) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if perr != nil {
			return perr
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || readyFlipPrimitives[fn.Name.Name] {
				continue
			}
			flips, backstop := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					name = fun.Sel.Name
				case *ast.Ident:
					name = fun.Name
				}
				flips = flips || readyFlipPrimitives[name]
				backstop = backstop || name == "readyBackstop"
				return true
			})
			if flips && !backstop {
				rel, _ := filepath.Rel(root, p)
				hits = append(hits, filepath.ToSlash(rel)+":"+fn.Name.Name)
			}
		}
		return nil
	})
	sort.Strings(hits)
	return hits, err
}

// regression: #6242
func TestReadyFlipBackstopGuard(t *testing.T) {
	moduleRoot, err := filepath.Abs("../..") // internal/deskkit -> tools/desk
	if err != nil {
		t.Fatal(err)
	}
	hits, err := readyFlipsWithoutBackstop(moduleRoot)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(hits) > 0 {
		t.Fatalf("ready flip with no post-flip backstop (#6242: ready means mergeable per the forge; "+
			"call readyBackstop right after the flip): %s", strings.Join(hits, ", "))
	}

	fixture, err := filepath.Abs("testdata/readyflip_fixture")
	if err != nil {
		t.Fatal(err)
	}
	planted, err := readyFlipsWithoutBackstop(fixture)
	if err != nil {
		t.Fatalf("fixture scan: %v", err)
	}
	if len(planted) != 1 || planted[0] != "planted_flip.go:flipWithoutBackstop" {
		t.Fatalf("positive control: the planted flip was not flagged (got %q) — the matcher is not live", planted)
	}
}
