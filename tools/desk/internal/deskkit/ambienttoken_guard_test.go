package deskkit

// ambienttoken_guard_test.go — desktools-v2/03 class guard: no NEW function in the desk tree
// may read a forge token out of the process environment.
//
// The defect class the read-path migration retires is "a desk tool resolves WHO it acts as
// from whatever the environment holds" — an inherited GH_TOKEN deciding the installation
// (#628), an ambient credential standing in for a minted one. Migrating one verb fixes one
// instance; this test is what stops the next one from being written. It reads the AST of
// every shipped (non-test) Go file under tools/desk and flags any FUNCTION that both
//
//   - names a forge-token environment variable (a string literal, or a constant whose value
//     is one — including a []string{...} it ranges over), and
//   - reads the environment (os.Getenv / os.LookupEnv / syscall.Getenv).
//
// Function granularity is deliberate: it catches the literal form (os.Getenv("GH_TOKEN")),
// the constant form, and the loop form (range over a list of names, Getenv(name)) with one
// rule, and it does not fire on a function that only UNSETS one (deskmonitor drops an
// inherited token from its own process — the opposite of reading it).
//
// Each existing instance is a named, reasoned permit. The register is a RATCHET on its exact
// length, like internal/forgeban's: a new reader cannot land without a reviewed diff here, and
// a migrated one cannot stay listed (a stale permit fails).

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

// ambientTokenVars are the environment variables a forge client or CLI takes an identity from.
var ambientTokenVars = map[string]bool{
	"GH_TOKEN": true, "GITHUB_TOKEN": true, "GH_ENTERPRISE_TOKEN": true,
	"GITHUB_ENTERPRISE_TOKEN": true, "GITLAB_TOKEN": true,
}

// ambientTokenReadPermits are the functions that read a forge token from the environment today,
// keyed `<path under tools/desk>::<func>`. TARGET: 0. None of them is a migrated read verb:
// desktools-v2/03 moved deskmerge's reads onto a minted token, and deskmerge has no entry.
var ambientTokenReadPermits = map[string]string{
	"cmd/deskadvisory/advisory.go::ghToken": "the advisory/TPF read authenticates as the operator's " +
		"ambient credential by design; inventory row 27 (`gh auth token`), left in place by " +
		"desktools-v2/03 — retiring it means giving deskadvisory a minted token of its own.",
	"cmd/deskdispatch/dispatch.go::resolveClaimAuth": "reads an INHERITED GH_TOKEN only to verify it is " +
		"the dispatching role's own credential, and unsets it when it is not (#1631) — a refusal path, " +
		"not an identity source.",
	"cmd/deskdispatch/dispatch.go::stepDecision": "checks that a GH_TOKEN was deliberately exported before " +
		"running a forge-writing script, and refuses on ambient auth — a presence check on the WRITE path.",
	"cmd/deskclaim-ref/gogit.go::resolveToken": "the pure-Go claim helper's explicitly HANDED token " +
		"(--token-file first, then the named variables) for adopters without the desk's minter; a claim " +
		"ref write, outside the desk read seam.",
}

// ambientTokenReadCeiling is the ratchet: the exact number of permits above.
const ambientTokenReadCeiling = 4

func TestNoAmbientEnvTokenRead(t *testing.T) {
	found, err := scanAmbientTokenReads(deskTreeRoot)
	if err != nil {
		t.Fatalf("the ambient-token guard could not scan the desk tree: %v — could-not-check, NOT clean", err)
	}
	if len(found) == 0 {
		t.Fatal("the scan found no ambient-token reader at all, not even the permitted ones — a scanner " +
			"that sees nothing certifies everything; re-point deskTreeRoot")
	}
	if len(ambientTokenReadPermits) != ambientTokenReadCeiling {
		t.Fatalf("the permit register holds %d entries but the ratchet ceiling is %d — raising or lowering "+
			"it is a reviewed diff, never an accident", len(ambientTokenReadPermits), ambientTokenReadCeiling)
	}
	seen := map[string]bool{}
	for _, key := range found {
		seen[key] = true
		if _, ok := ambientTokenReadPermits[key]; !ok {
			t.Errorf("%s reads a forge token from the process environment. A desk read authenticates as a "+
				"MINTED App token (minted for the repository's installation and valid for all of it) through deskkit.ForgeFor (cmd/deskread/forge.go is the shape) — "+
				"never as whatever GH_TOKEN/GITHUB_TOKEN the shell inherited (#628, desktools-v2/03).", key)
		}
	}
	for key := range ambientTokenReadPermits {
		if !seen[key] {
			t.Errorf("stale permit %s: that function no longer reads a forge token from the environment — "+
				"remove the permit and lower ambientTokenReadCeiling so the gain is locked in", key)
		}
	}
}

// scanAmbientTokenReads returns the sorted `<rel path>::<func>` keys of every shipped function
// under root that both names a forge-token variable and reads the environment.
func scanAmbientTokenReads(root string) ([]string, error) {
	type pkgFiles struct {
		files map[string]*ast.File
	}
	pkgs := map[string]*pkgFiles{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		dir := filepath.Dir(path)
		if pkgs[dir] == nil {
			pkgs[dir] = &pkgFiles{files: map[string]*ast.File{}}
		}
		pkgs[dir].files[path] = f
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range pkgs {
		// Package-level string constants, so `os.Getenv(tokenEnv)` resolves the same as the literal.
		consts := map[string]string{}
		for _, f := range p.files {
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if s, ok := stringLit(vs.Values[i]); ok {
								consts[name.Name] = s
							}
						}
					}
				}
			}
		}
		for path, f := range p.files {
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return nil, rerr
			}
			rel = filepath.ToSlash(rel)
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if funcReadsAmbientToken(fd.Body, consts) {
					out = append(out, rel+"::"+fd.Name.Name)
				}
			}
			// Package-level var initialisers (a func literal bound to a var) are functions too.
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, spec := range gd.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, v := range vs.Values {
						if funcReadsAmbientToken(v, consts) && i < len(vs.Names) {
							out = append(out, rel+"::"+vs.Names[i].Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// funcReadsAmbientToken reports whether n both names a forge-token variable and calls an
// environment read.
func funcReadsAmbientToken(n ast.Node, consts map[string]string) bool {
	names, reads := false, false
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.BasicLit:
			if s, ok := stringLit(v); ok && ambientTokenVars[s] {
				names = true
			}
		case *ast.Ident:
			if s, ok := consts[v.Name]; ok && ambientTokenVars[s] {
				names = true
			}
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok {
					switch pkg.Name + "." + sel.Sel.Name {
					case "os.Getenv", "os.LookupEnv", "syscall.Getenv":
						reads = true
					}
				}
			}
		}
		return true
	})
	return names && reads
}
