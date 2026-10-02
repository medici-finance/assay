package main

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

// TestEnvResolution pins the operator-home, config-home, gh-config and harness-home resolution
// under each host's semantics with an injected environment, so the Windows rules are proven on
// any runner. The fixture values are path STRINGS — nothing here touches the filesystem.
//
// There is no glab row: cellctl resolves no glab config. A gitlab cell's custody is the role
// token store, and `check` reports the CLI config link as not applicable on that arm.
func TestEnvResolution(t *testing.T) {
	const (
		profile = `C:\Users\example`
		appdata = `C:\Users\example\AppData\Roaming`
		bash    = "/c/Users/example-bash"
	)
	type want struct{ home, config, gh, claude, codex string }
	under := func(h string, elem ...string) string { return filepath.Join(append([]string{h}, elem...)...) }
	homeWant := func(h, gh string) want {
		return want{h, under(h, ".config", "assay"), gh, under(h, ".claude"), under(h, ".codex")}
	}
	cases := []struct {
		goos, name string
		env        map[string]string
		want       *want // nil: every home-derived resolution must refuse
	}{
		{"windows", "home_unset_userprofile_set", map[string]string{"USERPROFILE": profile},
			ptr(homeWant(profile, under(profile, ".config", "gh")))},
		{"windows", "home_unset_userprofile_and_appdata_set", map[string]string{"USERPROFILE": profile, "APPDATA": appdata},
			ptr(homeWant(profile, under(appdata, "GitHub CLI")))},
		// Both set: USERPROFILE wins, as it does for os.UserHomeDir in every desk tool cellctl
		// launches — a Git Bash HOME must not pick a different roster than those tools read.
		{"windows", "both_set", map[string]string{"USERPROFILE": profile, "HOME": bash},
			ptr(homeWant(profile, under(profile, ".config", "gh")))},
		{"windows", "home_only", map[string]string{"HOME": bash},
			ptr(homeWant(bash, under(bash, ".config", "gh")))},
		{"windows", "neither_set_refuses", map[string]string{}, nil},
		{"linux", "home_set", map[string]string{"HOME": "/home/example"},
			ptr(homeWant("/home/example", under("/home/example", ".config", "gh")))},
		{"linux", "both_set", map[string]string{"HOME": "/home/example", "USERPROFILE": "/home/other"},
			ptr(homeWant("/home/example", under("/home/example", ".config", "gh")))},
		{"linux", "neither_set_refuses", map[string]string{"APPDATA": appdata}, nil},
		{"darwin", "neither_set_refuses", map[string]string{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.name, func(t *testing.T) {
			e := envWith(tc.env)
			got := map[string]func(string, *Env) (string, error){
				"home": operatorHomeFor, "config": configHomeFor, "gh": ghConfigDirFor,
				"claude": claudeConfigDirFor, "codex": codexHomeFor,
			}
			for _, key := range []string{"home", "config", "gh", "claude", "codex"} {
				p, err := got[key](tc.goos, e)
				if tc.want == nil {
					if err == nil {
						t.Fatalf("%s resolved %q with no home set; it must refuse", key, p)
					}
					if !strings.Contains(err.Error(), "cannot resolve the operator home") {
						t.Fatalf("%s refusal does not name the cause: %v", key, err)
					}
					if p != "" {
						t.Fatalf("%s refused but still returned a path %q", key, p)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v", key, err)
				}
				exp := map[string]string{"home": tc.want.home, "config": tc.want.config, "gh": tc.want.gh,
					"claude": tc.want.claude, "codex": tc.want.codex}[key]
				if p != exp {
					t.Fatalf("%s = %q, want %q", key, p, exp)
				}
			}
		})
	}

	// The refusal is for a home that is NEEDED and absent. An explicit override needs no home,
	// so it still resolves on a host with neither variable set.
	t.Run("windows/overrides_need_no_home", func(t *testing.T) {
		e := envWith(map[string]string{
			"ASSAY_CONFIG_HOME": `D:\cfg\assay`, "GH_CONFIG_DIR": `D:\cfg\gh`,
			"CLAUDE_CONFIG_DIR": `D:\cfg\claude`, "CODEX_HOME": `D:\cfg\codex`,
		})
		for _, c := range []struct {
			f    func(string, *Env) (string, error)
			want string
		}{{configHomeFor, `D:\cfg\assay`}, {ghConfigDirFor, `D:\cfg\gh`}, {claudeConfigDirFor, `D:\cfg\claude`}, {codexHomeFor, `D:\cfg\codex`}} {
			if p, err := c.f("windows", e); err != nil || p != c.want {
				t.Fatalf("override: got %q, %v; want %q", p, err, c.want)
			}
		}
		if _, err := operatorHomeFor("windows", e); err == nil {
			t.Fatal("overrides must not invent an operator home")
		}
	})
	// APPDATA alone resolves the gh config (gh's own Windows default) without any home.
	t.Run("windows/appdata_only_resolves_gh", func(t *testing.T) {
		e := envWith(map[string]string{"APPDATA": appdata})
		if p, err := ghConfigDirFor("windows", e); err != nil || p != under(appdata, "GitHub CLI") {
			t.Fatalf("gh config = %q, %v", p, err)
		}
		if _, err := configHomeFor("windows", e); err == nil {
			t.Fatal("APPDATA must not stand in for the operator home")
		}
	})
}

func ptr[T any](v T) *T { return &v }

// homeReadAllowed is the committed allow-list for TestHomeReadClassGuard: the only non-test
// functions that may read HOME or USERPROFILE out of an Env. Everything else derives its paths
// through homeresolve.go.
var homeReadAllowed = map[string]string{
	"homeresolve.go:operatorHomeFor": "the one resolver",
	"cell.go:newEnvFromProcess":      "fills HOME from USERPROFILE on a Windows host before any resolution",
	"container.go:containerRun":      "passes the cell's own HOME through to the launcher, unchanged — no path is derived",
}

// homeReads lists every `<x>.Get("HOME"|"USERPROFILE")` / `<x>.GetOr(...)` call in src, as
// "<file>:<enclosing func>".
func homeReads(name string, src any) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	var sites []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) == 0 {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Get" && sel.Sel.Name != "GetOr") {
				return true
			}
			lit, ok := c.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && (v == "HOME" || v == "USERPROFILE") {
				sites = append(sites, filepath.Base(name)+":"+fn.Name.Name)
			}
			return true
		})
	}
	return sites, nil
}

// TestHomeReadClassGuard closes the defect class behind the neither-set gap: a cellctl site that
// derives a path from the raw HOME (or USERPROFILE) value, so an unset home silently becomes a
// relative or root-anchored path. Every such read outside the allow-list fails here.
func TestHomeReadClassGuard(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	scanned, seen := 0, map[string]bool{}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		scanned++
		sites, err := homeReads(f.Name(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range sites {
			seen[s] = true
			if _, ok := homeReadAllowed[s]; !ok {
				t.Errorf("raw home read outside the resolver: %s — derive the path through homeresolve.go", s)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no source scanned")
	}
	// A stale allow-list entry is a guard that no longer describes the code.
	for s := range homeReadAllowed {
		if !seen[s] {
			t.Errorf("allow-list entry %s matches no home read; remove it", s)
		}
	}
	// Positive control: a planted raw read must be reported, or the matcher has gone blind.
	sites, err := homeReads("planted.go", `package main
func plantedDir(e *Env) string { return e.Get("HOME") + "/.config" }`)
	if err != nil || len(sites) != 1 || sites[0] != "planted.go:plantedDir" {
		t.Fatalf("positive control: %v %v", sites, err)
	}
}
