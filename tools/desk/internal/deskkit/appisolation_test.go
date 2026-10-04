package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// isolateAppConfig owns both the override and default credential roots. Setting
// USERPROFILE also keeps os.UserHomeDir on the fixture on Windows.
func isolateAppConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".config", "assay")
	t.Setenv(EnvConfigHome, dir)
	return dir
}

// appIsolationGaps enumerates direct credential-resolver tests and deskpost mint wrappers.
// Even env-first tests must isolate file fallback: InstallID resolves the single
// installation from file before trying the per-owner environment value.
func appIsolationGaps(name string, src []byte) ([]string, error) {
	name = strings.ReplaceAll(filepath.ToSlash(name), `\`, "/")
	deskkit := strings.Contains(name, "/internal/deskkit/")
	deskpost := strings.Contains(name, "/cmd/deskpost/")
	mintTest := strings.Contains(name, "/cmd/desktoken/") || strings.Contains(name, "/cmd/deskapps/")
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	var gaps []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !(strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "setupFake" || fn.Name.Name == "setupTest" || fn.Name.Name == "isolateAppConfig") || fn.Body == nil {
			continue
		}
		resolver, helper, home, override := false, false, false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := ""
			if id, ok := call.Fun.(*ast.Ident); ok {
				callee = id.Name
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "deskkit" {
					callee = sel.Sel.Name
				}
			}
			switch callee {
			case "AppID", "InstallID", "AppBinding", "AppIDForApp", "resolveAppConfigValue", "installForOwner", "mintInstallationToken":
				resolver = true
			case "isolateAppConfig":
				helper = helper || deskkit
			case "setupFake":
				helper = helper || deskpost
			case "setupTest":
				helper = helper || mintTest
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Setenv" && len(call.Args) == 2 {
				if sel, ok := call.Args[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "EnvConfigHome" {
					override = true
				}
				if id, ok := call.Args[0].(*ast.Ident); ok && id.Name == "EnvConfigHome" {
					override = true
				}
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					key, _ := strconv.Unquote(lit.Value)
					home = home || key == "HOME"
					override = override || key == "ASSAY_CONFIG_HOME"
				}
			}
			return true
		})
		fixture := (fn.Name.Name == "setupFake" && deskpost) || (fn.Name.Name == "setupTest" && mintTest) || (fn.Name.Name == "isolateAppConfig" && deskkit)
		if (resolver || fixture) && !helper && !(home && override) {
			gaps = append(gaps, name+":"+fn.Name.Name)
		}
	}
	return gaps, nil
}

func TestAppConfigIsolation(t *testing.T) {
	var files []string
	err := filepath.WalkDir("../..", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test sources checked")
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		gaps, err := appIsolationGaps(file, src)
		if err != nil {
			t.Fatal(err)
		}
		for _, gap := range gaps {
			t.Errorf("credential test lacks both config roots: %s", gap)
		}
	}
}

func TestAppIsolationMatcher(t *testing.T) {
	// A planted second caller is a positive control, so a broken matcher cannot
	// silently declare the package clean. Comments and strings are not calls.
	src := []byte(`package deskkit
func TestSecondCaller(t *testing.T) { t.Setenv("HOME", t.TempDir()); InstallID("reader", "example-org") }
func TestSafeCaller(t *testing.T) { isolateAppConfig(t); AppID("reader") }
func TestExplicitRoots(t *testing.T) { t.Setenv("HOME", t.TempDir()); t.Setenv(EnvConfigHome, ""); AppBinding("reader") }
func TestNotACaller(t *testing.T) { _ = "AppID" /* InstallID("reader", "example-org") */ }
`)
	gaps, err := appIsolationGaps("../../internal/deskkit/planted_test.go", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 || gaps[0] != "../../internal/deskkit/planted_test.go:TestSecondCaller" {
		t.Fatalf("planted caller detection = %v", gaps)
	}
}

func TestAppFixtureMatcher(t *testing.T) {
	cases := []struct{ name, src string }{
		{`..\..\cmd\deskpost\harness_test.go`, `package main; func setupFake(t *testing.T) { t.Setenv("HOME", t.TempDir()) }`},
		{"../../cmd/desktoken/helpers_test.go", `package main; func setupTest(t *testing.T) { t.Setenv("HOME", t.TempDir()) }`},
		{"../../internal/deskkit/fixture_test.go", `package deskkit; func isolateAppConfig(t *testing.T) { t.Setenv("HOME", t.TempDir()) }`},
		{"../../cmd/unrelated/test_test.go", `package main; func TestCaller(t *testing.T) { setupTest(t); deskkit.AppID("reader") }`},
	}
	for _, c := range cases {
		gaps, err := appIsolationGaps(c.name, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		if len(gaps) != 1 {
			t.Errorf("fixture detection for %s = %v", c.name, gaps)
		}
	}
}
