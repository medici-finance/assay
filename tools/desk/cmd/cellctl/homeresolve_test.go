package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
		profile = `C:\Profiles\example`
		appdata = `C:\Profiles\example\AppData\Roaming`
		bash    = `D:\home\example-bash`
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
		// A set home must be an absolute local path that is not the bare root; a bad primary
		// refuses rather than silently falling back to the other variable.
		{"windows", "relative_profile_refuses", map[string]string{"USERPROFILE": `Profiles\example`}, nil},
		{"windows", "bad_profile_no_fallback", map[string]string{"USERPROFILE": `Profiles\example`, "HOME": bash}, nil},
		{"windows", "drive_root_refuses", map[string]string{"USERPROFILE": `C:\`}, nil},
		{"windows", "unc_profile_refuses", map[string]string{"USERPROFILE": `\` + `\fileserver\profiles\example`}, nil},
		{"windows", "posix_style_home_refuses", map[string]string{"HOME": "/c/profiles/example-bash"}, nil},
		{"linux", "home_set", map[string]string{"HOME": "/srv/example-home"},
			ptr(homeWant("/srv/example-home", under("/srv/example-home", ".config", "gh")))},
		{"linux", "both_set", map[string]string{"HOME": "/srv/example-home", "USERPROFILE": "/srv/other-home"},
			ptr(homeWant("/srv/example-home", under("/srv/example-home", ".config", "gh")))},
		{"linux", "neither_set_refuses", map[string]string{"APPDATA": appdata}, nil},
		// No USERPROFILE fallback off windows: os.UserHomeDir reads HOME alone there, so every
		// tool cellctl launches would fail to find a home cellctl resolved this way.
		{"linux", "userprofile_only_refuses", map[string]string{"USERPROFILE": "/srv/other-home"}, nil},
		{"linux", "relative_home_refuses", map[string]string{"HOME": "."}, nil},
		{"linux", "blank_home_refuses", map[string]string{"HOME": " "}, nil},
		{"linux", "root_home_refuses", map[string]string{"HOME": "/"}, nil},
		{"linux", "root_home_dotted_refuses", map[string]string{"HOME": "//./"}, nil},
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
// functions that may read HOME or USERPROFILE (or the platform home) in any form. Everything
// else derives its paths through homeresolve.go.
var homeReadAllowed = map[string]string{
	"homeresolve.go:operatorHomeFor": "the one resolver",
	"cell.go:newEnvFromProcess":      "fills HOME from USERPROFILE on a Windows host before any resolution",
	"container.go:containerRun": "forwards the invoking environment's HOME (process env overlaid with cell.env) " +
		"to the container launcher as-is — it derives no path from it",
	"roster.go:withHome": "saves and restores the process HOME/USERPROFILE around an in-process deskkit call — " +
		"it derives no path from the saved value",
}

// homeVarNames are the variables the class covers.
var homeVarNames = map[string]bool{"HOME": true, "USERPROFILE": true}

// homeReadFuncs are the call names that read a variable by key: the Env accessors and the os
// (or syscall) environment reads.
var homeReadFuncs = map[string]bool{"Get": true, "GetOr": true, "GetOrSet": true, "Getenv": true, "LookupEnv": true}

// homeKeyConsts collects every constant, package-level or local, whose value is a home
// variable name — so a read spelled through a named constant is still a read.
func homeKeyConsts(files map[string]*ast.File) map[string]bool {
	consts := map[string]bool{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			g, ok := n.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				return true
			}
			for _, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, v := range vs.Values {
					if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING && i < len(vs.Names) {
						if u, err := strconv.Unquote(lit.Value); err == nil && homeVarNames[u] {
							consts[vs.Names[i].Name] = true
						}
					}
				}
			}
			return true
		})
	}
	return consts
}

// homeReads lists every home read in files as "<file>:<enclosing func or var>": a by-key read
// (`<x>.Get`/`GetOr`/`GetOrSet`, `os.Getenv`/`os.LookupEnv`) or a map index whose key is a home
// variable name — spelled as a literal or a named constant — and any `os.UserHomeDir()` call.
func homeReads(files map[string]*ast.File) []string {
	consts := homeKeyConsts(files)
	isHomeKey := func(x ast.Expr) bool {
		switch k := x.(type) {
		case *ast.BasicLit:
			u, err := strconv.Unquote(k.Value)
			return k.Kind == token.STRING && err == nil && homeVarNames[u]
		case *ast.Ident:
			return consts[k.Name]
		}
		return false
	}
	var sites []string
	scan := func(file, owner string, root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			hit := false
			switch x := n.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					hit = sel.Sel.Name == "UserHomeDir" ||
						(homeReadFuncs[sel.Sel.Name] && len(x.Args) > 0 && isHomeKey(x.Args[0]))
				}
			case *ast.IndexExpr:
				hit = isHomeKey(x.Index)
			}
			if hit {
				sites = append(sites, file+":"+owner)
			}
			return true
		})
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, d := range files[name].Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Body != nil {
					scan(name, d.Name.Name, d.Body)
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok && d.Tok == token.VAR && len(vs.Names) > 0 {
						for _, v := range vs.Values {
							scan(name, vs.Names[0].Name, v)
						}
					}
				}
			}
		}
	}
	return sites
}

func parseGoSources(t *testing.T, srcs map[string]string) map[string]*ast.File {
	t.Helper()
	out := map[string]*ast.File{}
	for name, src := range srcs {
		var in any
		if src != "" {
			in = src
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, in, 0)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(name)] = f
	}
	return out
}

// TestHomeReadClassGuard closes the defect class behind the neither-set gap: a cellctl site that
// derives a path from the raw HOME (or USERPROFILE) value, so an unset home silently becomes a
// relative or root-anchored path. Every such read outside the allow-list fails here.
func TestHomeReadClassGuard(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	srcs := map[string]string{}
	for _, f := range entries {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		srcs[f.Name()] = ""
	}
	if len(srcs) == 0 {
		t.Fatal("no source scanned")
	}
	seen := map[string]bool{}
	for _, s := range homeReads(parseGoSources(t, srcs)) {
		seen[s] = true
		if _, ok := homeReadAllowed[s]; !ok {
			t.Errorf("raw home read outside the resolver: %s — derive the path through homeresolve.go", s)
		}
	}
	// A stale allow-list entry is a guard that no longer describes the code.
	for s := range homeReadAllowed {
		if !seen[s] {
			t.Errorf("allow-list entry %s matches no home read; remove it", s)
		}
	}
	// Positive controls: one planted raw read per spelling the class can take must be reported,
	// or the matcher has gone blind to that spelling.
	for want, src := range map[string]string{
		"planted.go:plantedGet":        `func plantedGet(e *Env) string { return e.Get("HOME") + "/.config" }`,
		"planted.go:plantedGetOr":      `func plantedGetOr(e *Env) string { return e.GetOr("USERPROFILE", "x") }`,
		"planted.go:plantedGetOrSet":   `func plantedGetOrSet(e *Env) string { return e.GetOrSet("HOME", "") }`,
		"planted.go:plantedGetenv":     `func plantedGetenv() string { return os.Getenv("HOME") }`,
		"planted.go:plantedLookup":     `func plantedLookup() string { v, _ := os.LookupEnv("USERPROFILE"); return v }`,
		"planted.go:plantedUserHome":   `func plantedUserHome() string { h, _ := os.UserHomeDir(); return h }`,
		"planted.go:plantedVals":       `func plantedVals(e *Env) string { return e.vals["HOME"] }`,
		"planted.go:plantedConst":      `const homeVar = "HOME"` + "\n" + `func plantedConst(e *Env) string { return e.Get(homeVar) }`,
		"planted.go:plantedLocalConst": `func plantedLocalConst() string { const k = "USERPROFILE"; return os.Getenv(k) }`,
		"planted.go:plantedHome":       `var plantedHome = os.Getenv("HOME")`,
	} {
		t.Run("planted/"+strings.TrimPrefix(want, "planted.go:"), func(t *testing.T) {
			sites := homeReads(parseGoSources(t, map[string]string{"planted.go": "package main\n" + src}))
			if len(sites) != 1 || sites[0] != want {
				t.Fatalf("positive control: got %v, want [%s]", sites, want)
			}
		})
	}
	// Negative control: a write and an unrelated key are not reads.
	if sites := homeReads(parseGoSources(t, map[string]string{"clean.go": `package main
func clean(e *Env) string { e.Put("HOME", "/x"); return e.Get("PATH") + os.Getenv("HOMEBREW_PREFIX") }`})); len(sites) != 0 {
		t.Fatalf("negative control reported %v", sites)
	}
}

// TestNewRefusesBeforeMkdir pins where `cellctl new` refuses an unresolvable home: before the
// first cell directory exists, so the refusal leaves nothing a re-run would then refuse to
// overwrite. Both scaffold paths that link from the operator home are covered.
func TestNewRefusesBeforeMkdir(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	t.Run("house", func(t *testing.T) {
		root := t.TempDir()
		e := envWith(map[string]string{"ASSAY_CONFIG_HOME": t.TempDir()})
		assertDies(t, "house new with no home", func() {
			newHouse(e, root, "h1", repo, "example/repo="+repo, rolesDefault, "8787")
		})
		if exists(filepath.Join(root, "h1")) {
			t.Fatal("house new refused after creating the cell directory")
		}
	})
	t.Run("k8s", func(t *testing.T) {
		root := t.TempDir()
		yaml := filepath.Join(t.TempDir(), "cells.yaml")
		if err := os.WriteFile(yaml, []byte("cells: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", "")
		t.Setenv("USERPROFILE", "")
		t.Setenv("CELLS_ROOT", root)
		t.Setenv("ASSAY_CONFIG_HOME", t.TempDir())
		assertDies(t, "k8s new with no home", func() {
			cmdNew([]string{"k1", "--repo", repo, "--cells-yaml", yaml, "--orgs", "example", "--deskd-app-pem", yaml})
		})
		if exists(filepath.Join(root, "k1")) {
			t.Fatal("k8s new refused after creating the cell directory")
		}
	})
}
