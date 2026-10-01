package deskkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Only the shared multi-family reader may reduce a single family's listing. A
// new adapter using the old primitive would recreate alias-dependent age-out.
func singleFamilyRefs(path string, src any) ([]string, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, path, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if ok && (id.Name == "ReviewClaimFamilyRefPrefix" || id.Name == "ReviewClaimLivenessFromMatchingRefs") {
			found = append(found, fmt.Sprint(fs.Position(id.Pos())))
		}
		return true
	})
	return found, nil
}

func TestReviewFamilyGuard(t *testing.T) {
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") && d.Name() != ".." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Clean(path) == filepath.Clean("../../internal/deskkit/claimref.go") {
			return nil
		}
		found, err := singleFamilyRefs(path, nil)
		if err != nil {
			return err
		}
		for _, at := range found {
			t.Errorf("single-family authority reader at %s; use ReadReviewClaims", at)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewGuardControl(t *testing.T) {
	plant := `package example; var alias = deskkit.ReviewClaimFamilyRefPrefix; func other(){ deskkit.ReviewClaimLivenessFromMatchingRefs(nil,"",nil) }`
	got, err := singleFamilyRefs("planted.go", plant)
	if err != nil || len(got) != 2 {
		t.Fatalf("planted direct reference and alias: %v %v", got, err)
	}
}
