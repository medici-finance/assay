package deskkit

// citoken_guard_test.go — forge-neutral brief 34: the CI workflow-token constructor is confined.
//
// ReadOnlyForgeForCIToken builds a backend from a token its CALLER hands it, so a second caller
// would be a second route from "some token in the environment" to a forge client. The
// constructor's own checks (installation-token shape, same repository, GitHub only) and the
// read-only decorator still bind such a caller, but the kind allowlist and the CI gate live in
// deskread. This guard keeps the caller set at exactly one reviewed function, the same pattern as
// githubMinterAllow in roletokenguard_test.go: widening it is a diff someone argues for here.
//
// It also fails any use of SetCITokenAPIBaseForTest from a non-test file: that seam repoints the
// backend (and so the token) at another host, and exists only for tests. The scan reads
// non-_test.go files only; the constructor's own tests call it directly and are not callers in this
// sense.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ciTokenConstructorAllow is the allow-list, keyed "<path relative to the scanned root>:<enclosing
// func>". Exactly one entry: the deskread function on the CI path.
var ciTokenConstructorAllow = map[string]string{
	"cmd/deskread/forge.go:ciForgeFor": "the one function on deskread's CI path: reached only after ciGate " +
		"(flag, CI job, not pull_request_target, installation-token shape, listed kind) and the same-repository " +
		"check, and it returns a read-only backend",
}

// scanCITokenConstructorUse walks the non-test Go files under root and returns the "<rel>:<func>"
// key of every reference to ReadOnlyForgeForCIToken outside its own definition, and a position
// string for every reference to SetCITokenAPIBaseForTest outside its own definition.
func scanCITokenConstructorUse(root string) (callers, setterUses []string, err error) {
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
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
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		visit := func(fn string, n ast.Node) {
			ast.Inspect(n, func(n ast.Node) bool {
				// A pkg.Name selector's Sel is itself an *ast.Ident, so idents cover both forms.
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				name, pos := id.Name, id.Pos()
				switch name {
				case "ReadOnlyForgeForCIToken":
					callers = append(callers, rel+":"+fn)
				case "SetCITokenAPIBaseForTest":
					setterUses = append(setterUses, fmt.Sprintf("%s (in %s)", fset.Position(pos), fn))
				}
				return true
			})
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name == "ReadOnlyForgeForCIToken" || d.Name.Name == "SetCITokenAPIBaseForTest" {
					// The definition itself: only its body and signature are skipped, not a
					// same-named method elsewhere (there is none).
					continue
				}
				visit(d.Name.Name, d)
			default:
				visit("", decl)
			}
		}
		return nil
	})
	sort.Strings(callers)
	sort.Strings(setterUses)
	return callers, setterUses, err
}

func TestCITokenConstructorConfined(t *testing.T) {
	callers, setterUses, err := scanCITokenConstructorUse(deskTreeRoot)
	if err != nil {
		t.Fatalf("the CI-token confinement guard could not scan the desk tree: %v — could-not-check, NOT clean", err)
	}
	if len(callers) == 0 {
		t.Fatal("the scan found no caller of ReadOnlyForgeForCIToken, not even deskread's — a scanner that " +
			"sees nothing certifies everything; re-point deskTreeRoot")
	}
	seen := map[string]bool{}
	for _, key := range callers {
		seen[key] = true
		if _, ok := ciTokenConstructorAllow[key]; !ok {
			t.Errorf("%s references ReadOnlyForgeForCIToken. The CI workflow-token backend is built from a "+
				"caller-supplied token and is reachable from deskread's CI path only; widening the set is a "+
				"reviewed entry in ciTokenConstructorAllow, never a way to go green.", key)
		}
	}
	for key := range ciTokenConstructorAllow {
		if !seen[key] {
			t.Errorf("stale allow-list entry %s: that function no longer calls ReadOnlyForgeForCIToken", key)
		}
	}
	for _, u := range setterUses {
		t.Errorf("%s uses SetCITokenAPIBaseForTest from a non-test file; the seam repoints the CI token at "+
			"another host and may be called from _test.go files only", u)
	}
	// Negative path: the same scanner over a fixture tree must report a planted second caller and a
	// planted non-test use of the seam.
	ciTokenConfinementNegative(t)
}

// ciTokenConfinementNegative runs the scanner over a fixture tree with a planted second
// caller in another cmd package and a planted non-test use of the test seam, and asserts both are
// reported while the allowed caller is not an offender.
func ciTokenConfinementNegative(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("cmd/deskread/forge.go", "package main\nfunc ciForgeFor() { deskkit.ReadOnlyForgeForCIToken(r, j, t) }\n")
	write("cmd/other/main.go", "package main\nfunc sneak() { _, _, _ = deskkit.ReadOnlyForgeForCIToken(r, j, t) }\n")
	write("cmd/other/seam.go", "package main\nfunc repoint() { deskkit.SetCITokenAPIBaseForTest(\"http://x\") }\n")
	write("cmd/other/seam_test.go", "package main\nfunc TestX() { deskkit.SetCITokenAPIBaseForTest(\"http://x\") }\n")

	callers, setterUses, err := scanCITokenConstructorUse(root)
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	for _, c := range callers {
		if _, ok := ciTokenConstructorAllow[c]; !ok {
			offenders = append(offenders, c)
		}
	}
	if len(offenders) != 1 || offenders[0] != "cmd/other/main.go:sneak" {
		t.Errorf("planted second caller not reported as the sole offender: callers=%v offenders=%v", callers, offenders)
	}
	if len(setterUses) != 1 || !strings.Contains(setterUses[0], "seam.go") || strings.Contains(setterUses[0], "_test.go") {
		t.Errorf("planted non-test use of the test seam not reported exactly once (and the _test.go use ignored): %v", setterUses)
	}
}

// ciConstructorEnvAllow names the package functions ReadOnlyForgeForCIToken may call although
// they read the environment themselves, each with the reason the read cannot steer the token's
// host. It is empty today: the constructor's only resolution step goes through EffectiveConfig,
// whose reads are not direct os calls in the callee. An entry is a reviewed diff.
var ciConstructorEnvAllow = map[string]string{}

// envReadCalls are the os functions that read the process environment.
var envReadCalls = map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true}

// scanCIConstructorEnvReads parses the non-test Go files of the package in dir and returns every
// environment read reachable from ReadOnlyForgeForCIToken's own body: a direct os.Getenv-family
// call, or a call to a package-level function of the same package whose body makes one. It is the
// class guard for "the CI token's API host comes from the environment" — the host has one source,
// the test seam, and a read of the environment in the constructor (or one hop under it) is how a
// hostile GITHUB_API_URL would carry the job's token elsewhere.
func scanCIConstructorEnvReads(dir string) (reads []string, found bool, err error) {
	fset := token.NewFileSet()
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, false, err
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, p := range matches {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil, false, perr
		}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
				funcs[fd.Name.Name] = fd
			}
		}
	}
	directReads := func(fd *ast.FuncDecl) []string {
		var out []string
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && envReadCalls[sel.Sel.Name] {
				out = append(out, fmt.Sprintf("os.%s at %s", sel.Sel.Name, fset.Position(sel.Pos())))
			}
			return true
		})
		return out
	}
	ctor, ok := funcs["ReadOnlyForgeForCIToken"]
	if !ok {
		return nil, false, nil
	}
	reads = append(reads, directReads(ctor)...)
	seen := map[string]bool{}
	ast.Inspect(ctor.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || seen[id.Name] {
			return true
		}
		seen[id.Name] = true
		callee, ok := funcs[id.Name]
		if !ok {
			return true
		}
		if _, allowed := ciConstructorEnvAllow[id.Name]; allowed {
			return true
		}
		for _, r := range directReads(callee) {
			reads = append(reads, fmt.Sprintf("%s (via %s)", r, id.Name))
		}
		return true
	})
	sort.Strings(reads)
	return reads, true, nil
}

func TestCICtorReadsNoEnv(t *testing.T) {
	reads, found, err := scanCIConstructorEnvReads(".")
	if err != nil {
		t.Fatalf("could-not-check: scanning the deskkit package: %v", err)
	}
	if !found {
		t.Fatal("could-not-check: ReadOnlyForgeForCIToken not found in the package — a scanner that sees nothing certifies everything")
	}
	for _, r := range reads {
		t.Errorf("ReadOnlyForgeForCIToken reads the environment: %s. The CI token's API host has one source, the test "+
			"seam; an environment read here is how GITHUB_API_URL would carry the token to another host.", r)
	}

	// Positive control: a planted fixture package with a direct read and a one-hop read must report both.
	dir := t.TempDir()
	const planted = `package deskkit
import "os"
func ReadOnlyForgeForCIToken(repo ForgeRepo, jobRepo, token string) (Forge, ForgeResolution, error) {
	base := os.Getenv("GITHUB_API_URL")
	_ = apiHostFromEnv()
	_ = pure()
	return nil, ForgeResolution{}, nil
}
func apiHostFromEnv() string { v, _ := os.LookupEnv("GITHUB_SERVER_URL"); return v }
func pure() string { return "" }
`
	if err := os.WriteFile(filepath.Join(dir, "planted.go"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	got, found, err := scanCIConstructorEnvReads(dir)
	if err != nil || !found {
		t.Fatalf("planted fixture: found=%v err=%v", found, err)
	}
	if len(got) != 2 || !strings.Contains(got[0]+got[1], "os.Getenv") || !strings.Contains(got[0]+got[1], "os.LookupEnv") ||
		!strings.Contains(got[0]+got[1], "via apiHostFromEnv") {
		t.Errorf("the guard over the planted fixture reported %v, want the direct os.Getenv and the os.LookupEnv via apiHostFromEnv", got)
	}
}
