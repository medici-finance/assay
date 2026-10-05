package forgeban

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backendtype_test.go — the structural layer of desktools-v2/10: with one construction site
// (deskkit's TestForgeSingleConstructionSite), no package outside internal/deskkit may name a
// backend type, so none can build a Forge the outbound-write check does not wrap, nor unwrap
// the one it was handed.

// deskTree is tools/desk, two levels above this package.
var deskTree = filepath.Join("..", "..")

// TestNoBackendTypeOutsideDeskkit is the ban on the real tree, preceded by the planted reds
// that prove the scanner sees every spelling — a scan that could not see a literal would
// pass the tree for the wrong reason.
func TestNoBackendTypeOutsideDeskkit(t *testing.T) {
	const imp = "import \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\n"
	reds := map[string]string{
		"composite literal":     imp + "var _ = deskkit.GitHubForge{}\n",
		"pointer literal":       imp + "var _ = &deskkit.GitLabForge{Token: \"x\"}\n",
		"new":                   imp + "var _ = new(deskkit.GitHubForge)\n",
		"conversion":            imp + "func f(p any) any { return (*deskkit.GitHubForge)(nil) }\n",
		"type assertion":        imp + "func f(fg deskkit.Forge) bool { _, ok := fg.(*deskkit.GitHubForge); return ok }\n",
		"type switch case":      imp + "func f(fg deskkit.Forge) { switch fg.(type) { case *deskkit.GitLabForge: } }\n",
		"declared type":         imp + "type mine deskkit.GitHubForge\n",
		"alias":                 imp + "type mine = deskkit.GitLabForge\n",
		"field of a struct":     imp + "type s struct{ b *deskkit.GitHubForge }\n",
		"inside a call's X":     imp + "func g(any) struct{ N int } { return struct{ N int }{} }\nvar _ = g(deskkit.GitHubForge{}).N\n",
		"aliased import":        "import dk \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\nvar _ = dk.GitHubForge{}\n",
		"dot import":            "import . \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\nvar _ = GitLabForge{}\n",
		"dot import, embedded":  "import . \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\ntype s struct{ *GitHubForge }\n",
		"dot import, assertion": "import . \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\nfunc f(fg Forge) { _ = fg.(*GitHubForge) }\n",
	}
	for name, body := range reds {
		t.Run("planted-red/"+name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, "cmd/x/x.go", "package x\n\n"+body)
			uses, err := ScanBackendTypes(root)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if len(uses) == 0 {
				t.Fatalf("planted backend use NOT caught:\n%s", body)
			}
		})
	}

	greens := map[string]struct{ path, body string }{
		"comment and string": {"cmd/x/x.go", "package x\n\n// deskkit.GitHubForge is built only in deskkit.\nvar s = \"deskkit.GitLabForge{}\"\n"},
		"deskkit itself":     {"internal/deskkit/f.go", "package deskkit\n\ntype GitHubForge struct{}\n\nvar _ = GitHubForge{}\n"},
		"test file":          {"cmd/x/x_test.go", "package x\n\nimport \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\n\nvar _ = deskkit.GitHubForge{}\n"},
		"no deskkit import":  {"cmd/x/x.go", "package x\n\ntype GitHubForge struct{}\n\nvar _ = GitHubForge{}\n"},
		"dot import, field":  {"cmd/x/x.go", "package x\n\nimport . \"github.com/medici-finance/assay/tools/desk/internal/deskkit\"\n\nvar _ ForgeRepo\n\nfunc f(v struct{ GitHubForge int }) int { return v.GitHubForge }\n"},
	}
	for name, g := range greens {
		t.Run("clean/"+name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, "cmd/y/keep.go", "package y\n")
			writeTree(t, root, g.path, g.body)
			uses, err := ScanBackendTypes(root)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if len(uses) != 0 {
				t.Fatalf("false positive %v on:\n%s", uses, g.body)
			}
		})
	}

	if _, err := ScanBackendTypes(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a scan of a missing tree returned no error — could-not-check must never read as clean")
	}

	uses, err := ScanBackendTypes(deskTree)
	if err != nil {
		t.Fatalf("scanning %s: %v", deskTree, err)
	}
	if len(uses) != 0 {
		var b strings.Builder
		for _, u := range uses {
			b.WriteString("\n  " + u.String())
		}
		t.Fatalf("a backend type is named outside internal/deskkit — build the Forge with "+
			"deskkit.ResolveForge (which wraps it in the outbound-write check), never around it:%s", b.String())
	}
}

// writeTree writes one file under root, creating its directories.
func writeTree(t *testing.T, root, rel, src string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
}
