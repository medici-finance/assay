package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The retired per-item reaction gate must not come back into deskpost.
//
// The allowed-repos write-gate change replaced the public-repo write gate's per-item human +1
// reaction check with the repository-scoped allowed-repos `:public` tag: deskkit.PublicRepoGate
// now reads ONLY the live repo visibility (deskkit.RepoInfoFetcher.RepoVisibility) and the
// configured entry. That change narrowed the shared interface and deleted the HTTP reaction
// probe, but deskpost's forge-backed adapter kept a reaction-read method with no caller and a
// comment still naming it as the retired gate's surface — the residue that failed the
// change's Verify row 7.
//
// DEFECT CLASS: a reaction/award read reachable from deskpost — any declaration, call or
// interface method carrying the reaction-read name, any reference to deskkit's Reaction /
// ReactionUser types, or a REST path ending in /reactions — in deskpost's non-test source.
// Nothing in deskpost consumes reactions any more, so the allow-list is EMPTY: every site is
// a finding. If a future verb genuinely needs a reaction read, it lands with that caller and
// an explicit, reviewed change to this guard — never as a silent re-add on the gate's path.
//
// The method name is assembled rather than written out so this guard is not itself a hit for
// the brief's residual grep (Verify row 7 greps tools/ for the joined name).
// ---------------------------------------------------------------------------

var reactionReadMethod = "Issue" + "Reactions"

// retiredReactionSites reports every site of the defect class in one parsed file, as
// "<file>:<line>: <what>".
func retiredReactionSites(fset *token.FileSet, f *ast.File) []string {
	var sites []string
	add := func(n ast.Node, what string) {
		p := fset.Position(n.Pos())
		sites = append(sites, p.Filename+":"+strconv.Itoa(p.Line)+": "+what)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			// deskkit.Reaction / deskkit.ReactionUser — the reaction payload types.
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "deskkit" &&
				(x.Sel.Name == "Reaction" || x.Sel.Name == "ReactionUser") {
				add(x, "reference to deskkit."+x.Sel.Name)
			}
		case *ast.Ident:
			// A method/func declaration, an interface method, or a call selector carrying
			// the reaction-read name — every one of those spellings is an *ast.Ident.
			if x.Name == reactionReadMethod {
				add(x, "identifier "+x.Name)
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if s, err := strconv.Unquote(x.Value); err == nil && strings.Contains(s, "/reactions") {
					add(x, "REST reactions path "+x.Value)
				}
			}
		}
		return true
	})
	return sites
}

// TestNoRetiredReactionGateInDeskpost — the class guard. It walks EVERY non-test Go file under
// the deskpost package directory (subpackages included, testdata excluded) and fails naming
// any site, so a re-add anywhere in the verb is red, not only at the forgeclient.go site that
// was reported. The reflection half pins the same absence on the two postBackend
// implementations and the interface itself, independent of the source parser.
func TestNoRetiredReactionGateInDeskpost(t *testing.T) {
	fset := token.NewFileSet()
	var sites []string
	scanned := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		scanned++
		sites = append(sites, retiredReactionSites(fset, f)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking deskpost source: %v", err)
	}
	if scanned == 0 {
		// could-not-check is never a pass: a walk that saw no source cleared nothing.
		t.Fatalf("scanned 0 non-test Go files — the guard looked at nothing")
	}
	sort.Strings(sites)
	for _, s := range sites {
		t.Errorf("retired reaction-gate surface reappeared in deskpost: %s", s)
	}

	for _, typ := range []reflect.Type{
		reflect.TypeOf((*forgeBackend)(nil)),
		reflect.TypeOf((*ghClient)(nil)),
		reflect.TypeOf((*postBackend)(nil)).Elem(),
	} {
		if _, ok := typ.MethodByName(reactionReadMethod); ok {
			t.Errorf("%v has method %s — the retired per-item reaction gate's read is back on a deskpost backend",
				typ, reactionReadMethod)
		}
	}
}

// TestRetiredReactionGuardPositiveControl — the guard's matcher could silently stop matching
// (a renamed AST case, a typo in the name), after which the class guard above would report
// clean over anything. This planted fixture holds one instance of every shape the class
// covers; the matcher must flag each, so a broken guard fails here instead.
func TestRetiredReactionGuardPositiveControl(t *testing.T) {
	src := strings.ReplaceAll(`package main

import "example.invalid/deskkit"

type planted struct{ fg deskkit.Forge }

type plantedIface interface {
	RRR(owner, repo string, n int) ([]deskkit.Reaction, error)
}

func (b *planted) RRR(owner, repo string, n int) ([]deskkit.Reaction, error) {
	return b.fg.RRR(deskkit.ForgeRepo{Owner: owner, Name: repo}, n)
}

func plantedPath(n int) string { return "/repos/o/r/issues/1/reactions" }

var _ deskkit.ReactionUser
`, "RRR", reactionReadMethod)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse planted fixture: %v", err)
	}
	got := strings.Join(retiredReactionSites(fset, f), "\n")
	for _, want := range []string{
		"planted.go:8: identifier " + reactionReadMethod,  // interface method
		"planted.go:8: reference to deskkit.Reaction",     // payload type in the interface
		"planted.go:11: identifier " + reactionReadMethod, // method declaration
		"planted.go:12: identifier " + reactionReadMethod, // call through the Forge
		"planted.go:15: REST reactions path",              // raw REST path
		"planted.go:17: reference to deskkit.ReactionUser",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("positive control: guard did not flag %q\nsites:\n%s", want, got)
		}
	}
}
