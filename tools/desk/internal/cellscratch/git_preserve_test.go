package cellscratch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScratchBareGit(t *testing.T) {
	seed, revision := gitFixture(t)
	for _, location := range []string{"mirror", "nested/project.git", "outside-work"} {
		for _, code := range []int{0, 23} {
			for _, apply := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/exit-%d/apply-%t", location, code, apply), func(t *testing.T) {
					s := scratchStore(t)
					r := scratchRun(t, s)
					target := filepath.Join(r.Work(), location)
					if location == "outside-work" {
						target = filepath.Join(s.Path, r.Record.ID, "mirror")
					}
					if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
						t.Fatal(err)
					}
					sourceGit(t, seed, "clone", "--bare", "--no-hardlinks", seed, target)
					acknowledge(t, r)
					finish(t, r, code)
					for _, policy := range []Policy{DefaultPolicy(), {MaxAge: 0, MaxBytes: 0}} {
						got := sweep(t, s, policy, apply)
						if len(got.Entries) != 1 || got.Entries[0].Action != "keep" || got.Reclaimed != 0 {
							t.Fatalf("bare Git data not preserved: %+v; HEAD exists=%v", got, exists(t, filepath.Join(target, "HEAD")))
						}
						if gotRevision := sourceGit(t, target, "rev-parse", "HEAD"); gotRevision != revision {
							t.Fatal("Git revision changed")
						}
					}
				})
			}
		}
	}
}

func TestScratchGitMarkers(t *testing.T) {
	for _, marker := range []string{".git", ".GIT", "objects", "refs", "packed-refs", "reftable", "commondir", "none"} {
		t.Run(marker, func(t *testing.T) {
			s := scratchStore(t)
			r := scratchRun(t, s)
			// Partial or damaged metadata is protected without asking Git to parse it.
			if err := os.WriteFile(filepath.Join(r.Work(), "HEAD"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if marker != "none" {
				if err := os.WriteFile(filepath.Join(r.Work(), marker), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			acknowledge(t, r)
			finish(t, r, 0)
			got := sweep(t, s, DefaultPolicy(), true)
			if marker == "none" {
				if got.Entries[0].Action != "remove" {
					t.Fatal("ordinary HEAD output not disposable", got)
				}
			} else if got.Entries[0].Action != "keep" || got.Reclaimed != 0 || !exists(t, r.Work()) {
				t.Fatal("Git marker shape not preserved", got)
			}
		})
	}
}

func scratchKeepSites(src string) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, 0)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		owner := fn.Name.Name
		if fn.Recv != nil {
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if ok {
				if typ, ok := star.X.(*ast.Ident); ok {
					owner = typ.Name + "." + owner
				}
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && (sel.Sel.Name == "remove" || sel.Sel.Name == "gitMarker") {
				sites = append(sites, owner+":"+sel.Sel.Name)
			}
			return true
		})
	}
	return sites, nil
}

func TestScratchGitKeepSites(t *testing.T) {
	// A new deletion caller must not bypass the classifier that Sweep owns.
	plant, err := scratchKeepSites(`package fixture; func planted(s *Store) { s.remove("work") }`)
	if err != nil || !reflect.DeepEqual(plant, []string{"planted:remove"}) {
		t.Fatal("guard missed second cleanup caller", plant, err)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		found, err := scratchKeepSites(string(body))
		if err != nil {
			t.Fatal(err)
		}
		for _, site := range found {
			sites = append(sites, entry.Name()+":"+site)
		}
	}
	want := []string{"store.go:Open:remove", "store.go:Store.Sweep:remove", "store.go:Store.measure:gitMarker"}
	if !reflect.DeepEqual(sites, want) {
		t.Fatal("unclassified Git cleanup site", sites)
	}
}
