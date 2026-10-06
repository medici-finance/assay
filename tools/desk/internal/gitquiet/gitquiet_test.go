package gitquiet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Run(m)) }

// git runs git in dir with GIT_TRACE written to trace (when non-empty), failing the test
// on error. Identity is inline so the host's config is never needed.
func git(t *testing.T, dir, trace string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if trace != "" {
		cmd.Env = append(cmd.Env, "GIT_TRACE="+trace)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

var autoMaintenance = regexp.MustCompile(`maintenance run --auto|gc --auto`)

// TestNewReposForkNoMaintenance — the behaviour. Every way a fixture gets a repository
// (init, init --bare, clone) carries both settings in the repository's own config, and the
// commands that fork automatic maintenance (a commit in a repository with several packs,
// and receive-pack in a bare remote on a local push) fork none. Before the fix, the final
// commit's trace shows `maintenance run --auto --quiet --detach` and the detached repack is
// still writing under .git/objects when the directory is removed.
func TestNewReposForkNoMaintenance(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "origin.git")
	git(t, root, "", "init", "-q", "-b", "main", work)
	git(t, root, "", "init", "-q", "--bare", "-b", "main", bare)
	// Several packs: the state in which current git's automatic maintenance repacks in
	// the background after the next commit.
	for _, f := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(work, f), []byte(f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, work, "", "add", f)
		git(t, work, "", "commit", "-q", "-m", f)
		git(t, work, "", "repack", "-q")
	}
	git(t, work, "", "remote", "add", "origin", bare)
	git(t, root, "", "clone", "-q", bare, filepath.Join(root, "clone"))

	for _, repo := range []string{work, bare, filepath.Join(root, "clone")} {
		for _, kv := range Settings {
			if got := git(t, repo, "", "config", "--local", "--get", kv[0]); got != kv[1] {
				t.Errorf("%s: %s = %q in the repository's own config, want %q", filepath.Base(repo), kv[0], got, kv[1])
			}
		}
	}

	trace := filepath.Join(t.TempDir(), "trace")
	git(t, work, trace, "push", "-q", "origin", "main")
	if err := os.WriteFile(filepath.Join(work, "z"), []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "", "add", "z")
	git(t, work, trace, "commit", "-q", "-m", "z")
	body, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if hit := autoMaintenance.FindString(string(body)); hit != "" {
		t.Errorf("a fixture commit or push still forks automatic maintenance (%q):\n%s", hit, body)
	}
	// The removal t.TempDir's cleanup would make, made now while any detached child would
	// still be running.
	if err := os.RemoveAll(work); err != nil {
		t.Errorf("removing the fixture right after the commit: %v", err)
	}
}

// TestTemplateConfigParses — the rendered template is a config git reads back key by key.
func TestTemplateConfigParses(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(p, []byte(templateConfig()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, kv := range Settings {
		if got := git(t, filepath.Dir(p), "", "config", "--file", p, "--get", kv[0]); got != kv[1] {
			t.Errorf("template %s = %q, want %q", kv[0], got, kv[1])
		}
	}
}

// TestTemplateDirExplicit — a child whose environment has every inherited GIT_* variable
// removed (Run's GIT_TEMPLATE_DIR with them) still creates quiet repositories when the
// fixture passes TemplateDir's directory explicitly, and does not without it.
func TestTemplateDirExplicit(t *testing.T) {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			env = append(env, kv)
		}
	}
	env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	get := func(env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	root := t.TempDir()
	quiet, plain := filepath.Join(root, "quiet"), filepath.Join(root, "plain")
	explicit := append(append([]string{}, env...), "GIT_TEMPLATE_DIR="+TemplateDir(t))
	get(explicit, "init", "-q", "--bare", quiet)
	get(env, "init", "-q", "--bare", plain)
	for _, kv := range Settings {
		if got := get(env, "-C", quiet, "config", "--local", "--default", "", "--get", kv[0]); got != kv[1] {
			t.Errorf("with TemplateDir: %s = %q in the repository's own config, want %q", kv[0], got, kv[1])
		}
		if got := get(env, "-C", plain, "config", "--local", "--default", "", "--get", kv[0]); got != "" {
			t.Errorf("control: without TemplateDir %s = %q; the stripped env still reaches a template", kv[0], got)
		}
	}
}

// --- class guard ------------------------------------------------------------------

// namesGit reports whether a file names the git binary (a "git" string literal).
func namesGit(f *ast.File) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil && s == "git" {
				found = true
			}
		}
		return !found
	})
	return found
}

// gitTestPackage reports whether a package's test binary runs git. Either the code compiled
// only into the test binary (its _test.go files, and any non-test file that imports
// "testing") names the git binary or builds fixtures with the shared gittest harness, or
// the package has tests and its own non-test code names the git binary. The second arm is
// a package whose tests reach git through the package's own launcher (internal/gitexec's
// Run): the only "git" literal is outside the test files, so the first arm never sees it.
// It over-approximates (a launcher package whose tests never call the launcher is counted
// too); the cost of that is one TestMain line. A package whose tests reach git only through
// ANOTHER package's launcher is not matched: following imports would count most of the
// module, and that reach stays outside this static guard.
func gitTestPackage(files []*ast.File, names []string) bool {
	hasTests, launches := false, false
	for i, f := range files {
		isTest := strings.HasSuffix(names[i], "_test.go")
		hasTests = hasTests || isTest
		testOnly, harness := isTest, false
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == "testing" {
				testOnly = true
			}
			if strings.HasSuffix(p, "/internal/gittest") {
				harness = true
			}
		}
		if !testOnly {
			launches = launches || namesGit(f)
			continue
		}
		if harness || namesGit(f) {
			return true
		}
	}
	return hasTests && launches
}

// installsQuiet reports whether a _test.go file declares a TestMain that calls
// gitquiet.Run (or Run, inside this package), directly or through one package-level
// function that does (the runTests(m) shape, whose defers must run before os.Exit).
func installsQuiet(files []*ast.File, names []string) bool {
	callsRun := func(f *ast.File, body *ast.BlockStmt, via map[string]bool) bool {
		found := false
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return !found
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok && x.Name == "gitquiet" && fun.Sel.Name == "Run" {
					found = true
				}
			case *ast.Ident:
				if (f.Name.Name == "gitquiet" && fun.Name == "Run") || via[fun.Name] {
					found = true
				}
			}
			return !found
		})
		return found
	}
	type decl struct {
		f  *ast.File
		fn *ast.FuncDecl
	}
	var funcs []decl
	for i, f := range files {
		if !strings.HasSuffix(names[i], "_test.go") {
			continue
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				funcs = append(funcs, decl{f, fn})
			}
		}
	}
	via := map[string]bool{}
	for _, d := range funcs {
		if d.fn.Name.Name != "TestMain" && callsRun(d.f, d.fn.Body, nil) {
			via[d.fn.Name.Name] = true
		}
	}
	for _, d := range funcs {
		if d.fn.Name.Name == "TestMain" && callsRun(d.f, d.fn.Body, via) {
			return true
		}
	}
	return false
}

// uncovered walks root and returns, sorted and relative to root, every package directory
// whose test binary runs git but whose TestMain does not install the quiet template. It
// also returns how many git-using packages it examined, so a scan that matched nothing
// is visible as such. testdata, vendor and dot directories below root are not packages.
func uncovered(root string) (missing []string, examined int, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		entries, rerr := os.ReadDir(path)
		if rerr != nil {
			return rerr
		}
		fset := token.NewFileSet()
		var files []*ast.File
		var names []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			f, perr := parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			files = append(files, f)
			names = append(names, e.Name())
		}
		if !gitTestPackage(files, names) {
			return nil
		}
		examined++
		if !installsQuiet(files, names) {
			rel, _ := filepath.Rel(root, path)
			missing = append(missing, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(missing)
	return missing, examined, err
}

// TestGitPackagesInstallQuiet — the class guard. Any package under tools/desk
// whose tests run git must route its TestMain through gitquiet.Run, so a fixture
// repository in a NEW package cannot bring the cleanup race back. Fix a failure by adding
//
//	func TestMain(m *testing.M) { os.Exit(gitquiet.Run(m)) }
//
// to the named package, or by calling gitquiet.Run(m) in place of m.Run() in its existing
// TestMain.
func TestGitPackagesInstallQuiet(t *testing.T) {
	missing, examined, err := uncovered(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// Non-vacuous: the two packages the race was first reported in are git-test packages.
	if examined < 20 {
		t.Fatalf("the scan found only %d packages whose tests run git; the matcher has stopped matching", examined)
	}
	if len(missing) > 0 {
		t.Fatalf("%d package(s) run git in tests without gitquiet.Run in TestMain, so their fixture "+
			"repositories fork background maintenance that races t.TempDir cleanup:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestGuardControl — the guard's positive control. testdata/guard holds one package that
// runs git with no TestMain (must be flagged), one whose TestMain calls m.Run directly
// (must be flagged), one that installs the template (must pass), one that installs it
// through a runTests helper (must pass), one whose helper calls m.Run (must be flagged),
// one that uses the gittest harness without it (must be flagged), one whose tests reach
// git only through the package's own non-test launcher (must be flagged), one with that
// launcher but no tests (ignored), and one that never runs git (ignored).
func TestGuardControl(t *testing.T) {
	missing, examined, err := uncovered(filepath.Join("testdata", "guard"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"harness", "helpernorun", "plainmain", "uncovered", "wrapper"}
	if strings.Join(missing, ",") != strings.Join(want, ",") || examined != 7 {
		t.Fatalf("guard over testdata/guard flagged %v of %d git-test packages; want %v of 7", missing, examined, want)
	}
}
