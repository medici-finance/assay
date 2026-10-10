package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Enumerate every home-derived resource resolver, so additions cannot silently
// escape the replay matrix when the launcher changes HOME.
func homeDerivedNames(t *testing.T, source any) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "homeresolve.go", source, 0)
	must(t, err)
	var names []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		found := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "underHome" {
					found = true
				}
			}
			return true
		})
		if found {
			names = append(names, fn.Name.Name)
		}
	}
	return names
}

func TestConfigReplayClass(t *testing.T) {
	resolvers := map[string]func(string, *Env) (string, error){"configHomeFor": configHomeFor, "ghConfigDirFor": ghConfigDirFor, "claudeConfigDirFor": claudeConfigDirFor, "codexHomeFor": codexHomeFor}
	names := homeDerivedNames(t, nil)
	if len(names) != len(resolvers) {
		t.Fatalf("home resolver inventory changed: %v", names)
	}
	for _, name := range names {
		if resolvers[name] == nil {
			t.Fatalf("uncovered home resolver: %s", name)
		}
	}
	// Positive control: a second path derivation must enter the inventory.
	planted := homeDerivedNames(t, `package main
 func planted(e *Env)(string,error){return underHome("linux",e,".new-config")}`)
	if len(planted) != 1 || planted[0] != "planted" {
		t.Fatalf("inventory missed planted resolver: %v", planted)
	}
	for _, mode := range []string{"defaults", "overrides", "alias"} {
		t.Run(mode, func(t *testing.T) {
			c := codexEnvironmentCell(t)
			if mode != "defaults" {
				for _, key := range []string{"ASSAY_CONFIG_HOME", "GH_CONFIG_DIR", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
					c.Env.Put(key, filepath.Join(t.TempDir(), key))
				}
			}
			before := map[string]string{}
			for name, resolve := range resolvers {
				p, err := resolve(runtime.GOOS, c.Env)
				must(t, err)
				must(t, os.MkdirAll(p, 0700))
				before[name] = p
			}
			if mode == "alias" {
				alias := filepath.Join(t.TempDir(), "operator-config")
				must(t, os.Symlink(before["configHomeFor"], alias))
				c.Env.Put("ASSAY_CONFIG_HOME", alias)
			}
			must(t, os.MkdirAll(filepath.Dir(c.Config), 0700))
			must(t, os.Symlink(before["configHomeFor"], c.Config))
			values, err := c.codexCommandEnvironment(nil)
			must(t, err)
			if values["HOME"] != c.Home || values["USERPROFILE"] != c.Home || values["ASSAY_CONFIG_HOME"] != c.Config {
				t.Fatal("cell isolation changed")
			}
			captured := values["CELLCTL_OPERATOR_CONFIG_HOME"]
			if captured == "" || captured == c.Config {
				t.Fatalf("operator target not captured independently: %q", captured)
			}
			wantConfig, err := os.Stat(before["configHomeFor"])
			must(t, err)
			gotConfig, err := os.Stat(realConfigHome(envWith(values)))
			must(t, err)
			if !os.SameFile(wantConfig, gotConfig) {
				t.Fatal("operator resolver changed the captured resource")
			}
			nested := *c
			nested.Env = envWith(values)
			nested.Home = t.TempDir()
			nested.Config = filepath.Join(nested.Home, ".config", "assay")
			next, err := nested.codexCommandEnvironment(nil)
			must(t, err)
			if next["CELLCTL_OPERATOR_CONFIG_HOME"] != captured || next["ASSAY_CONFIG_HOME"] != nested.Config {
				t.Fatal("nested launch replaced operator context or lost cell isolation")
			}
			after := envWith(values)
			for name, resolve := range resolvers {
				p, err := resolve(runtime.GOOS, after)
				must(t, err)
				want, err := os.Stat(before[name])
				must(t, err)
				got, err := os.Stat(p)
				if err != nil || !os.SameFile(want, got) {
					t.Errorf("%s changed resource after launch", name)
				}
			}
		})
	}
}
