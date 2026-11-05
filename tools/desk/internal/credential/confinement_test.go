package credential_test

// confinement_test.go — desktools-v2/58: the provider is reachable from exactly the two
// adapters the brief names, and the package itself stays acyclic.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePath     = "github.com/medici-finance/assay/tools/desk"
	credentialPath = modulePath + "/internal/credential"
	deskRoot       = "../.." // tools/desk, from internal/credential
)

// credentialImporters are the only shipped directories that may import the provider: deskkit
// (the in-process lookup and the shared snapshot/adapters) and the desktoken CLI. Any other
// verb reaches a credential through deskkit.RoleTokenForOwner, so the guard, the audit row
// and the memo are never bypassed.
var credentialImporters = map[string]bool{"internal/deskkit": true, "cmd/desktoken": true}

// operationSites are the only functions that may RUN a provider operation (credential.GitHub /
// credential.GitLab): "<dir>|<func>". A new call site is a new adapter and needs review.
var operationSites = map[string]bool{
	"internal/deskkit|providerMint":  true,
	"cmd/desktoken|cmdTokenMeasured": true,
	"cmd/desktoken|cmdGitLabRotate":  true,
}

type siteHit struct{ key, pos string }

func scanShipped(t *testing.T, root string) (importers map[string]string, ops []siteHit, credImports []string) {
	t.Helper()
	importers = map[string]string{}
	fset := token.NewFileSet()
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if n := d.Name(); path != root && (n == "testdata" || n == "vendor" || strings.HasPrefix(n, ".")) {
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
		files++
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))
		alias := ""
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if dir == "internal/credential" && strings.HasPrefix(p, modulePath+"/") {
				credImports = append(credImports, rel+" imports "+p)
			}
			if p == credentialPath {
				importers[dir] = rel
				alias = "credential"
				if im.Name != nil {
					alias = im.Name.Name
				}
			}
		}
		if alias == "" {
			return nil
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == alias && (sel.Sel.Name == "GitHub" || sel.Sel.Name == "GitLab") {
					ops = append(ops, siteHit{key: dir + "|" + fn.Name.Name, pos: fset.Position(sel.Pos()).String()})
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v — could-not-check, NOT clean", root, err)
	}
	if files == 0 {
		t.Fatal("scanned no files — a scanner that sees nothing certifies everything")
	}
	return importers, ops, credImports
}

// regression: desktools-v2/58 — the provider is a credential mint; an importer outside the two
// adapters would be a third, unreviewed path to a role token (no memo, possibly no audit row),
// and an import of deskkit from the provider would be the cycle the adapter design exists to
// avoid.
func TestCredentialImportConfinement(t *testing.T) {
	importers, ops, credImports := scanShipped(t, deskRoot)
	if len(credImports) > 0 {
		t.Fatalf("internal/credential must import nothing from the desk module (acyclic; adapters carry every control):\n  %s",
			strings.Join(credImports, "\n  "))
	}
	var bad []string
	for dir, file := range importers {
		if !credentialImporters[dir] {
			bad = append(bad, file)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("internal/credential imported outside deskkit and the desktoken CLI — reach a role token through "+
			"deskkit.RoleTokenForOwner instead:\n  %s", strings.Join(bad, "\n  "))
	}
	for dir := range credentialImporters {
		if importers[dir] == "" {
			t.Fatalf("%s no longer imports the provider — the confinement register is stale", dir)
		}
	}
	seen := map[string]bool{}
	for _, o := range ops {
		seen[o.key] = true
		if !operationSites[o.key] {
			t.Errorf("unreviewed provider operation call site %s at %s", o.key, o.pos)
		}
	}
	for k := range operationSites {
		if !seen[k] {
			t.Errorf("listed provider call site %s no longer present — remove it from operationSites", k)
		}
	}
}

// regression: desktools-v2/58 — a confinement scan that cannot see a planted importer passes
// vacuously; this positive control proves it flags one.
func TestCredentialImportConfinementSeesPlant(t *testing.T) {
	importers, ops, _ := scanShipped(t, filepath.Join("testdata", "confinement"))
	if importers["cmd/plantedverb"] == "" {
		t.Fatalf("the planted importer was not seen: %v", importers)
	}
	found := false
	for _, o := range ops {
		if o.key == "cmd/plantedverb|plantedMint" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the planted provider call was not seen: %v", ops)
	}
}
