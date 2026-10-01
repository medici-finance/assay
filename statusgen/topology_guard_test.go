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

// Class guard for the alias-dropping reducer (#1239, item (d)).
//
// DEFECT CLASS: a code path reduces an aliased brief ref to a local <stream>/<NN> by DISCARDING
// the alias instead of resolving it through the registry. The instance was closeVerify calling
// normalizeBriefKey, so a brief-v2 id naming another repo flipped this tree's same-numbered
// brief. The fix routes the close through resolveLocalBriefRef; this guard pins that no other
// site can reach the discard:
//
//   - normalizeBriefKey (which drops the alias by design, for the marker identity) may be called
//     only from the two read-side sites that want exactly that: verifyMarker and
//     selectConsumerBriefs.
//   - parseBriefV2ID's alias (its 2nd result) may be thrown away with `_` only inside
//     normalizeBriefKey itself; every other caller must bind and use it.
//
// The scan covers every non-test .go file in the package; a violation names file and line.

// aliasDropCallers are the ONLY functions allowed to call normalizeBriefKey.
var aliasDropCallers = map[string]bool{"verifyMarker": true, "selectConsumerBriefs": true}

// aliasDiscarders are the ONLY functions allowed to discard parseBriefV2ID's alias.
var aliasDiscarders = map[string]bool{"normalizeBriefKey": true}

func calledName(call *ast.CallExpr) string {
	if id, ok := call.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// aliasDropViolations parses one Go source and returns every site outside the allow-lists that
// calls normalizeBriefKey or discards parseBriefV2ID's alias result.
func aliasDropViolations(fset *token.FileSet, name string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, decl := range f.Decls {
		owner := ""
		if fn, ok := decl.(*ast.FuncDecl); ok {
			owner = fn.Name.Name
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if calledName(x) == "normalizeBriefKey" && !aliasDropCallers[owner] {
					out = append(out, fset.Position(x.Pos()).String()+": "+owner+" calls normalizeBriefKey, which drops the repo alias unresolved — resolve it with resolveLocalBriefRef")
				}
			case *ast.AssignStmt:
				if len(x.Rhs) != 1 || len(x.Lhs) < 2 {
					return true
				}
				call, ok := x.Rhs[0].(*ast.CallExpr)
				if !ok || calledName(call) != "parseBriefV2ID" {
					return true
				}
				if id, ok := x.Lhs[1].(*ast.Ident); ok && id.Name == "_" && !aliasDiscarders[owner] {
					out = append(out, fset.Position(x.Pos()).String()+": "+owner+" discards parseBriefV2ID's repo alias — bind it and resolve it through the registry")
				}
			}
			return true
		})
	}
	return out, nil
}

// TestAliasDropGuard is the class guard: no non-test file in the package drops a ref's repo
// alias outside the allow-listed read-side sites.
func TestAliasDropGuard(t *testing.T) {
	// Positive control: planted second instances — the pre-fix close (normalizeBriefKey on a
	// write path) and an inline discard of the alias — MUST both be flagged, so a matcher that
	// silently stopped matching fails here instead of reporting clean.
	planted := []byte(`package main
func closeVerify(root, briefID string) string {
	return normalizeBriefKey(briefID)
}
func flipByID(id string) string {
	_, _, stream, num, ok := parseBriefV2ID(id)
	if !ok {
		return id
	}
	return stream + "/" + num
}
`)
	got, err := aliasDropViolations(token.NewFileSet(), "planted.go", planted)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("positive control: the guard must flag both planted sites, got %v", got)
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
		v, err := aliasDropViolations(fset, name, src)
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
		t.Fatalf("a repo alias may only be dropped at the allow-listed read-side sites — every other path resolves it through the registry:\n%s",
			strings.Join(violations, "\n"))
	}
}
