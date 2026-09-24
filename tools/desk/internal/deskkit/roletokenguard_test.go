package deskkit

// roletokenguard_test.go — the CLASS guard for #1573 (and #676 before it).
//
// RoleTokenForOwner / RoleTokenForRepo are the GitHub App installation-token minter. Their
// names are forge-neutral; what they do is not: they shell `desktoken <role> --repo <owner>`,
// which mints against a GitHub App. A caller that reaches for them without first asking which
// forge serves the repo asks a GitLab-served repo for a GitHub App that does not exist. That
// mistake shipped twice at two different call sites (deskboot's mint step, then deskwt
// role-init), each fixed locally, because nothing stopped the next caller from repeating it.
//
// This guard stops the next caller. It walks every non-test Go file under tools/desk and
// fails on ANY reference to either minter — a call, and equally a function VALUE bound to a
// variable (`var mint = deskkit.RoleTokenForRepo`), which is how most of the pre-#1573 sites
// reached it and which a call-only check would miss — outside the allow-list below. A new
// caller either resolves its credential through the forge-aware entry points in
// forgeresolve.go (ResolveRoleCredential; GitHubRoleToken for a GitHub-only transport) or
// argues its way onto the list in review, where the argument is visible.
//
// It is structural (go/ast over the real tree), not a mock: it reads the code that ships.
//
// THE WHOLE CHAIN IS CONFINED, NOT JUST ITS EXPORTED TOP. Confining only the two exported
// names leaves every name above and below them open inside package deskkit: a new helper could
// call the GitHub arm (githubAppRoleToken) or the primitive underneath (mintRoleToken, the
// tokenMinter seam) and reach the same mint with no forge question in front of it, and the
// guard would stay green. So every link is a guarded name, and each link's callers are
// allow-listed by (file, function, name) — the arm by the three resolver functions that have
// resolved the forge before calling it, the exported minter by the arm alone.
//
// THE PRIMITIVE ITSELF. roletoken.go defines the chain RoleTokenForRepo → RoleTokenForOwner →
// mintRoleToken → tokenMinter. Those references inside the chain's own definitions are the
// primitive, not callers, and are the one thing the walk skips besides the allow-list. A
// declaration is not a reference either: the identifier being DECLARED (`var tokenMinter =`)
// is never counted, only the names its type and value use.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// deskkitImportPath is the import path a file outside this package reaches the minter through.
const deskkitImportPath = "github.com/medici-finance/assay/tools/desk/internal/deskkit"

// githubMinterNames are the links of the GitHub-App-only mint chain the guard confines: the
// exported minters, the forge-aware resolver's GitHub arm above them, and the primitive and
// its seam below them. The unexported names can only be reached from inside package deskkit,
// which is exactly where an alias of the arm would be planted.
var githubMinterNames = map[string]bool{
	"RoleTokenForOwner":  true,
	"RoleTokenForRepo":   true,
	"githubAppRoleToken": true,
	"mintRoleToken":      true,
	"tokenMinter":        true,
}

// githubMinterPrimitive is the chain's own definition in roletoken.go (see the file header):
// the references between these functions ARE the minter, not callers of it.
var githubMinterPrimitive = map[string]bool{"RoleTokenForOwner": true, "RoleTokenForRepo": true, "mintRoleToken": true}

// githubMinterAllow is the allow-list, keyed "<path relative to tools/desk>:<enclosing func>:<name
// referenced>". Each entry is a site that has ALREADY resolved the forge before it reaches the
// link it names. Widening it is a reviewed decision, never a way to go green; an entry names
// ONE link, so allowing a function to call the arm never also allows it the primitive.
var githubMinterAllow = map[string]string{
	"internal/deskkit/forgeresolve.go:ResolveRoleCredential:githubAppRoleToken": "the forge-aware " +
		"resolver's GitHub arm: reached only after roleCredentialForge resolved GitHub and bound the " +
		"origin host to github.com",
	"internal/deskkit/forgeresolve.go:GitHubRoleTokenForRemote:githubAppRoleToken": "the GitHub-only " +
		"transport entry point: reached only after roleCredentialForge resolved GitHub and bound the " +
		"origin host to github.com",
	"internal/deskkit/forgeresolve.go:githubCustody:githubAppRoleToken": "ForgeFor's default GitHub " +
		"custody: ForgeFor calls it only after resolving the forge to GitHub",
	"internal/deskkit/forgeresolve.go:githubAppRoleToken:RoleTokenForRepo": "the GitHub arm itself, " +
		"whose own callers are confined by the three entries above",
	"internal/deskkit/roletoken.go:SetRoleTokenMinter:tokenMinter": "the test seam: it swaps the " +
		"minter and restores it, and never calls it",
	"cmd/cellctl/deskd.go:Cell.deskdMintGitHub:RoleTokenForRepo": "forge-switched: cmdDeskd calls " +
		"it only on the cell's GitHub arm; the GitLab arm provisions separately (deskdProvisionGitLab)",
}

// githubMinterRef is one reference to a minter: where it is, and the function it sits in
// ("" for a package-level declaration such as `var mint = deskkit.RoleTokenForRepo`).
type githubMinterRef struct {
	rel, fn, name string
	pos           token.Position
}

func (r githubMinterRef) key() string { return r.rel + ":" + r.fn + ":" + r.name }

func (r githubMinterRef) String() string {
	fn := r.fn
	if fn == "" {
		fn = "<package level>"
	}
	return fmt.Sprintf("%s:%d in %s references %s", r.rel, r.pos.Line, fn, r.name)
}

// scanGitHubMinterRefs walks root and returns every reference to a link of the GitHub App mint
// chain in a non-test Go file, testdata/.git/vendor excluded. The chain's own references inside
// roletoken.go's definitions are skipped (see the file header).
func scanGitHubMinterRefs(root string) ([]githubMinterRef, error) {
	var refs []githubMinterRef
	err := filepath.Walk(root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			switch info.Name() {
			case "testdata", ".git", "vendor":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", path, perr)
		}
		refs = append(refs, githubMinterRefsInFile(fset, f, rel)...)
		return nil
	})
	return refs, err
}

// githubMinterRefsInFile finds the minter references in one parsed file. Inside package
// deskkit (or through a dot-import) a minter is a bare identifier; anywhere else it is a
// selector on whatever local name the file imported deskkit under.
func githubMinterRefsInFile(fset *token.FileSet, f *ast.File, rel string) []githubMinterRef {
	bare := f.Name.Name == "deskkit"
	qualifiers := map[string]bool{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != deskkitImportPath {
			continue
		}
		switch {
		case imp.Name == nil:
			qualifiers["deskkit"] = true
		case imp.Name.Name == ".":
			bare = true
		case imp.Name.Name != "_":
			qualifiers[imp.Name.Name] = true
		}
	}
	if !bare && len(qualifiers) == 0 {
		return nil
	}
	var refs []githubMinterRef
	visit := func(fn string, root ast.Node) {
		if root == nil {
			return
		}
		ast.Inspect(root, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && qualifiers[id.Name] && githubMinterNames[x.Sel.Name] {
					refs = append(refs, githubMinterRef{rel: rel, fn: fn, name: x.Sel.Name, pos: fset.Position(x.Pos())})
					return false
				}
				// A field or method that happens to share the name is not the package minter:
				// look only at the receiver expression, never at the selected name.
				ast.Inspect(x.X, func(m ast.Node) bool { return inspectBare(m, bare, rel, fn, fset, &refs) })
				return false
			case *ast.Field:
				// A struct field's or parameter's NAME is a declaration, never a reference to
				// the package function it happens to share a spelling with; its type still is.
				ast.Inspect(x.Type, func(m ast.Node) bool { return inspectBare(m, bare, rel, fn, fset, &refs) })
				return false
			case *ast.Ident:
				inspectBare(x, bare, rel, fn, fset, &refs)
			}
			return true
		})
	}
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			fn := decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				fn = recvTypeName(decl.Recv.List[0].Type) + "." + fn
			}
			// The primitive's own definition (RoleTokenForRepo → RoleTokenForOwner →
			// mintRoleToken → tokenMinter).
			if rel == "internal/deskkit/roletoken.go" && githubMinterPrimitive[fn] {
				continue
			}
			visit(fn, decl.Body)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					visit("", spec)
					continue
				}
				// The names a var/const DECLARES are not references; its type and values are.
				visit("", vs.Type)
				for _, v := range vs.Values {
					visit("", v)
				}
			}
		}
	}
	return refs
}

func inspectBare(n ast.Node, bare bool, rel, fn string, fset *token.FileSet, refs *[]githubMinterRef) bool {
	id, ok := n.(*ast.Ident)
	if !ok || !bare || !githubMinterNames[id.Name] {
		return true
	}
	*refs = append(*refs, githubMinterRef{rel: rel, fn: fn, name: id.Name, pos: fset.Position(id.Pos())})
	return true
}

func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	}
	return "?"
}

// TestGitHubMinterReachedOnlyFromForgeArms is the class guard over the real tree.
func TestGitHubMinterReachedOnlyFromForgeArms(t *testing.T) {
	refs, err := scanGitHubMinterRefs(deskTreeRoot)
	if err != nil {
		t.Fatalf("could not walk %s: %v — this is could-not-check, not a clean tree", deskTreeRoot, err)
	}
	seen := map[string]bool{}
	var offenders []string
	for _, r := range refs {
		if _, ok := githubMinterAllow[r.key()]; ok {
			seen[r.key()] = true
			continue
		}
		offenders = append(offenders, r.String())
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("the GitHub App mint chain (RoleTokenForOwner/RoleTokenForRepo, the githubAppRoleToken arm, "+
			"mintRoleToken/tokenMinter) is reached outside the forge-aware arms — resolve the credential through deskkit.ResolveRoleCredential (or GitHubRoleToken for a "+
			"GitHub-only transport) instead, so a GitLab-served repo never asks for a GitHub App (#1573):\n%s",
			strings.Join(offenders, "\n"))
	}
	// A stale entry is reported, never silently tolerated: an allow-list that names a site which
	// no longer reaches the minter is a list nobody is reading.
	var stale []string
	for k := range githubMinterAllow {
		if !seen[k] {
			stale = append(stale, k)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("allow-list entries that no longer reach the minter (remove them; do not re-point them):\n%s",
			strings.Join(stale, "\n"))
	}
}

// TestGitHubMinterGuardCatchesPlantedCallers proves the walk is not vacuous: a fixture tree
// planting the shapes a forge-blind caller takes — a direct call, a function value bound to a
// package var, an aliased import, a dot-import, an in-package pass-through through the GitHub
// arm, an in-package call to the primitive or its seam, and same-named methods, fields and
// declarations that must NOT count — is scanned, and every planted reference must be reported
// at its own line and function.
func TestGitHubMinterGuardCatchesPlantedCallers(t *testing.T) {
	refs, err := scanGitHubMinterRefs(filepath.Join("testdata", "githubminterguard"))
	if err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.key())
	}
	sort.Strings(got)
	want := []string{
		"aliased.go:mintViaAlias:RoleTokenForOwner",
		"direct.go::RoleTokenForRepo",
		"direct.go:forgeBlindRoleInit:RoleTokenForOwner",
		"dotimport.go:mintViaDot:RoleTokenForRepo",
		"inpackage.go::tokenMinter",
		"inpackage.go:PlantedForgeBlind:githubAppRoleToken",
		"inpackage.go:plantedPrimitive:mintRoleToken",
		"inpackage.go:plantedSeamCall:tokenMinter",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("planted forge-blind references not reported exactly:\n got: %q\nwant: %q", got, want)
	}
	for _, r := range refs {
		if _, ok := githubMinterAllow[r.key()]; ok {
			t.Fatalf("a planted fixture reference matched the real allow-list: %s", r)
		}
	}
}
