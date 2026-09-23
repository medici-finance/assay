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
// THE PRIMITIVE ITSELF. roletoken.go defines both minters and RoleTokenForRepo delegates to
// RoleTokenForOwner. That self-reference inside the minter's own definition is the primitive,
// not a caller, and is the one thing the walk skips besides the allow-list.

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

// githubMinterNames are the GitHub-App-only minters the guard confines.
var githubMinterNames = map[string]bool{"RoleTokenForOwner": true, "RoleTokenForRepo": true}

// githubMinterAllow is the allow-list, keyed "<path relative to tools/desk>:<enclosing func>".
// Each entry is a site that has ALREADY resolved the forge before it reaches the minter. It is
// deliberately two entries long; widening it is a reviewed decision, never a way to go green.
var githubMinterAllow = map[string]string{
	"internal/deskkit/forgeresolve.go:githubAppRoleToken": "the GitHub arm of the forge-aware " +
		"resolver: reached only after the forge resolved to GitHub (ResolveRoleCredential, " +
		"GitHubRoleTokenForRemote, and ForgeFor's default GitHub custody)",
	"cmd/cellctl/deskd.go:Cell.deskdMintGitHub": "forge-switched: cmdDeskd calls it only on the " +
		"cell's GitHub arm; the GitLab arm provisions separately (deskdProvisionGitLab)",
}

// githubMinterRef is one reference to a minter: where it is, and the function it sits in
// ("" for a package-level declaration such as `var mint = deskkit.RoleTokenForRepo`).
type githubMinterRef struct {
	rel, fn, name string
	pos           token.Position
}

func (r githubMinterRef) key() string { return r.rel + ":" + r.fn }

func (r githubMinterRef) String() string {
	fn := r.fn
	if fn == "" {
		fn = "<package level>"
	}
	return fmt.Sprintf("%s:%d in %s references %s", r.rel, r.pos.Line, fn, r.name)
}

// scanGitHubMinterRefs walks root and returns every reference to a GitHub App minter in a
// non-test Go file, testdata/.git/vendor excluded. The minter's own self-reference inside
// roletoken.go's definitions is skipped (see the file header).
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
			// The primitive's own definition (RoleTokenForRepo delegating to RoleTokenForOwner).
			if rel == "internal/deskkit/roletoken.go" && githubMinterNames[fn] {
				continue
			}
			visit(fn, decl.Body)
		case *ast.GenDecl:
			visit("", decl)
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
		t.Errorf("the GitHub App minter (RoleTokenForOwner/RoleTokenForRepo) is reached outside the forge-aware "+
			"arms — resolve the credential through deskkit.ResolveRoleCredential (or GitHubRoleToken for a "+
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
// package var, an aliased import, a dot-import, and a same-named method that must NOT count —
// is scanned, and every planted reference must be reported at its own line and function.
func TestGitHubMinterGuardCatchesPlantedCallers(t *testing.T) {
	refs, err := scanGitHubMinterRefs(filepath.Join("testdata", "githubminterguard"))
	if err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.key()+":"+r.name)
	}
	sort.Strings(got)
	want := []string{
		"aliased.go:mintViaAlias:RoleTokenForOwner",
		"direct.go::RoleTokenForRepo",
		"direct.go:forgeBlindRoleInit:RoleTokenForOwner",
		"dotimport.go:mintViaDot:RoleTokenForRepo",
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
