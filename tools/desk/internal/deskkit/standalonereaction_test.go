package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// standaloneReactionWriteSites lists every reference to the two unexported
// reaction writes in shipped code, keyed by file and enclosing function. The
// writes sit outside the Forge interface, so the outbound-checked wrappers
// never see them; only the guarded standalone verb may reach them.
func standaloneReactionWriteSites(t *testing.T, root string) map[string]int {
	t.Helper()
	sites := map[string]int{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "testdata", "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, decl := range file.Decls {
			enclosing := "<package>"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				enclosing = fn.Name.Name
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "addChangeReaction", "addChangeAward":
					sites[rel+":"+enclosing+":"+sel.Sel.Name]++
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sites
}

func TestStandaloneReactionWritesReachedOnlyByAddReaction(t *testing.T) {
	want := map[string]int{
		"internal/deskkit/standalonereaction.go:AddReaction:addChangeReaction": 1,
		"internal/deskkit/standalonereaction.go:AddReaction:addChangeAward":    1,
	}
	got := standaloneReactionWriteSites(t, deskTreeRoot)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reaction writes must be reached only through the guarded AddReaction: got %v, want %v", got, want)
	}
}

func TestStandaloneReactionWriteScannerSeesPlantedCaller(t *testing.T) {
	root := t.TempDir()
	const source = `package deskkit
func bypass(g *GitHubForge, l *GitLabForge) {
	_ = g.addChangeReaction(ForgeRepo{}, 1, "eyes")
	_ = l.addChangeAward(ForgeRepo{}, 1, "eyes")
}
var later = (*GitHubForge).addChangeReaction
`
	if err := os.WriteFile(filepath.Join(root, "caller.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"caller.go:bypass:addChangeReaction":    1,
		"caller.go:bypass:addChangeAward":       1,
		"caller.go:<package>:addChangeReaction": 1,
	}
	if got := standaloneReactionWriteSites(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("scan missed planted reaction writes: got %v, want %v", got, want)
	}
}

// TestStandaloneReactionAllowlistExcludesApprovalShapes pins the admitted set:
// +1 and -1 read as approvals when an App account gives them.
func TestStandaloneReactionAllowlistExcludesApprovalShapes(t *testing.T) {
	var got []string
	for content := range standaloneReactionAwards {
		got = append(got, content)
	}
	sort.Strings(got)
	want := []string{"confused", "eyes", "heart", "hooray", "laugh", "rocket"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("admitted reactions %v, want %v", got, want)
	}
}
