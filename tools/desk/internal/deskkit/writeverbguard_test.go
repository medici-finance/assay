package deskkit

// writeverbguard_test.go — no desk WRITE verb can reach desktoken's rotation path (#5457).
//
// desktoken resolves `desktoken <role> --repo <slug>` with no --forge to its GitLab rotation
// path when the roster binds the repo to GitLab, and that path lives in the CLI's own package
// main: the only way into it is to fork the binary. So "no write verb reaches the rotate path"
// is a property of the source, checked here three ways over the real tree:
//
//  1. each enumerated write-verb site reads its credential through WriteVerbRoleCredential;
//  2. no non-test file in a write-verb package names the desktoken program at all, so no
//     other function in those packages can fork it either;
//  3. the cmd/ sites still allowed to name desktoken are a CLOSED set, so a tenth site cannot
//     join desktokenExecAllow without failing here.
//
// It also holds the enumeration honest the other way: every cmd/ caller of the helper must be
// on writeVerbSites, so the list stays the inventory of write-verb credential reads. The walk
// is AST-only and names the helper as a string, so it compiles — and runs red — on a tree
// that predates the helper.

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

// writeVerbSite is one write verb's credential read: the file and function that, before
// #5457, forked `desktoken <role> --repo <slug>` without --forge.
type writeVerbSite struct{ file, fn string }

func (s writeVerbSite) key() string { return s.file + ":" + s.fn }

// writeVerbSites is every write-verb credential read, as the issue enumerated them.
var writeVerbSites = []writeVerbSite{
	{"cmd/deskclose/forge.go", "mintSessionToken"},
	{"cmd/deskdisposition/forge.go", "mintSessionToken"},
	{"cmd/deskevidence/github.go", "mintVerifierToken"},
	{"cmd/deskfile/github.go", "mintSessionToken"},
	{"cmd/desklabel/forge.go", "mintSessionToken"},
	{"cmd/deskpathguard/forge.go", "mintSessionToken"},
	{"cmd/deskpr/exec.go", "mintWorkerToken"},
	{"cmd/deskreply/exec.go", "mintWorkerToken"},
	{"cmd/deskrestamp/forge.go", "mintSessionToken"},
}

// writeVerbHelper is the read-only entry point every site above must call, by name.
const writeVerbHelper = "WriteVerbRoleCredential"

// cmdExecSitesClosed is the closed set of cmd/ functions desktokenExecAllow may hold. Neither
// is a write verb's credential read: one probes the CLI itself, one pins the release path's
// binary. Adding a key to the allow-list under cmd/ fails this test until it is argued here.
var cmdExecSitesClosed = map[string]bool{
	"cmd/deskboot/boot.go:stepMint":                  true,
	"cmd/deskrelease/github.go:resolveDeskTokenPath": true,
}

// regression: #5457
func TestWriteVerbsReadCredential(t *testing.T) {
	root := deskTreeRoot
	pkgs := map[string]bool{}
	for _, s := range writeVerbSites {
		pkgs[filepath.ToSlash(filepath.Dir(s.file))+"/"] = true
		body, err := funcBody(filepath.Join(root, s.file), s.fn)
		if err != nil {
			t.Errorf("%s: %v — the site moved; re-grep and re-point writeVerbSites", s.key(), err)
			continue
		}
		if !callsPkgFunc(body, "deskkit", writeVerbHelper) {
			t.Errorf("%s does not read its credential through deskkit.%s — a write verb must never "+
				"fork desktoken, whose no---forge shape rotates a GitLab-bound role's PAT (#5457)",
				s.key(), writeVerbHelper)
		}
	}

	refs, err := scanDesktokenProgramRefs(root)
	if err != nil {
		t.Fatalf("could not walk %s: %v — this is could-not-check, not a clean tree", root, err)
	}
	for _, r := range refs {
		for p := range pkgs {
			if strings.HasPrefix(r.rel, p) {
				t.Errorf("write-verb package %s still names the desktoken program: %s", p, r)
			}
		}
	}

	var open []string
	for k := range desktokenExecAllow {
		if strings.HasPrefix(k, "cmd/") && !cmdExecSitesClosed[k] {
			open = append(open, k)
		}
	}
	sort.Strings(open)
	if len(open) > 0 {
		t.Errorf("cmd/ sites on desktokenExecAllow outside the closed set — a write verb reads its "+
			"credential through deskkit.%s, it does not fork desktoken:\n%s",
			writeVerbHelper, strings.Join(open, "\n"))
	}

	callers, err := pkgFuncCallers(filepath.Join(root, "cmd"), "deskkit", writeVerbHelper)
	if err != nil {
		t.Fatalf("could not walk cmd/: %v — this is could-not-check, not a clean tree", err)
	}
	want := make([]string, 0, len(writeVerbSites))
	for _, s := range writeVerbSites {
		want = append(want, s.key())
	}
	sort.Strings(want)
	if strings.Join(callers, "\n") != strings.Join(want, "\n") {
		t.Errorf("deskkit.%s callers under cmd/ differ from writeVerbSites (keep the inventory in "+
			"step):\n got: %q\nwant: %q", writeVerbHelper, callers, want)
	}
}

// funcBody parses path and returns the body of its top-level function fn.
func funcBody(path, fn string) (*ast.BlockStmt, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == fn && fd.Body != nil {
			return fd.Body, nil
		}
	}
	return nil, os.ErrNotExist
}

// callsPkgFunc reports whether n contains a call to pkg.name.
func callsPkgFunc(n ast.Node, pkg, name string) bool {
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		if c, ok := m.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkg {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// pkgFuncCallers returns, sorted, every "cmd/<file>:<func>" under dir whose body calls
// pkg.name, test files and testdata excluded. Keys are relative to dir's parent (the desk
// tree root), so they compare with writeVerbSite.key.
func pkgFuncCallers(dir, pkg, name string) ([]string, error) {
	base := filepath.Dir(dir)
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			if info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		rel, rerr := filepath.Rel(base, path)
		if rerr != nil {
			return rerr
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || !callsPkgFunc(fd.Body, pkg, name) {
				continue
			}
			fn := fd.Name.Name
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				fn = recvTypeName(fd.Recv.List[0].Type) + "." + fn
			}
			out = append(out, filepath.ToSlash(rel)+":"+fn)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}
