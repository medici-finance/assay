package gitquiet

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Run(m)) }

// git runs git in dir with GIT_TRACE written to trace (when non-empty), failing the test
// on error. Identity is inline so the host's config is never needed, and the host's global
// and system git config are disabled so a developer's settings (a core.hooksPath, say) cannot
// reach the temporary repositories.
func git(t *testing.T, dir, trace string, args ...string) string {
	t.Helper()
	return gitWithGlobal(t, dir, trace, os.DevNull, args...)
}

// gitWithGlobal is git with an explicit global config file: TestHostileGitConfig points it at
// a deliberately hostile file; every other caller gets os.DevNull.
func gitWithGlobal(t *testing.T, dir, trace, global string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+global, "GIT_CONFIG_NOSYSTEM=1")
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

// regression: #1608
func TestHostileGitConfig(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "global")
	if err := os.WriteFile(cfg, []byte("[maintenance]\n auto = true\n[gc]\n auto = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())
	// Install must replace even a caller-supplied template; restoration preserves it.
	previous := os.Getenv("GIT_TEMPLATE_DIR")
	restore, err := Install()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	quiet := filepath.Join(root, "quiet")
	gitWithGlobal(t, root, "", cfg, "init", "-q", quiet)
	for _, kv := range Settings {
		if got := gitWithGlobal(t, quiet, "", cfg, "config", "--get", kv[0]); got != kv[1] {
			t.Errorf("effective %s = %q, want %q", kv[0], got, kv[1])
		}
	}
	restore()
	if os.Getenv("GIT_TEMPLATE_DIR") != previous {
		t.Fatal("caller template was not restored")
	}
	plain := filepath.Join(root, "plain")
	gitWithGlobal(t, root, "", cfg, "init", "-q", plain)
	if got := gitWithGlobal(t, plain, "", cfg, "config", "--get", "maintenance.auto"); got != "true" {
		t.Fatalf("negative control did not inherit hostile config: %q", got)
	}
}

// plantEnv is the switch that makes TestPlantedSetURL run: only the reexecuted child of
// TestRunRefusesTempInRepo sets it.
const plantEnv = "GITQUIET_PLANT_SET_URL"

// TestPlantedSetURL is the planted instance of the defect, run only as that child: a
// fixture that runs `git remote set-url` in an empty t.TempDir() with every inherited
// GIT_* variable removed, expecting no repository there. It writes to whatever
// repository discovery finds above the temporary root.
func TestPlantedSetURL(t *testing.T) {
	if os.Getenv(plantEnv) != "1" {
		t.Skip("run only as TestRunRefusesTempInRepo's child")
	}
	cmd := exec.Command("git", "-C", t.TempDir(), "remote", "set-url", "origin", "https://example.invalid/planted.git")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	out, err := cmd.CombinedOutput()
	t.Logf("planted set-url: err=%v %s", err, out)
}

// TestRunRefusesTempInRepo — the behaviour. A test binary whose temporary roots sit
// inside a repository refuses to run, so the planted fixture never writes the enclosing
// repository's origin URL; the same binary with its roots outside every repository runs
// the planted test (the control). Before the fix the child runs and the URL is rewritten.
//
// regression: F-temp-root-in-repo
func TestRunRefusesTempInRepo(t *testing.T) {
	const want = "https://example.invalid/o/r.git"
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	git(t, root, "", "init", "-q", repo)
	git(t, repo, "", "remote", "add", "origin", want)
	child := func(tmp, gotmp string) (string, error) {
		t.Helper()
		for _, d := range []string{tmp, gotmp} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestPlantedSetURL$", "-test.count=1", "-test.v")
		cmd.Env = append(os.Environ(), plantEnv+"=1", "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp, "GOTMPDIR="+gotmp)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	inside, outside := filepath.Join(repo, "tmp"), filepath.Join(root, "outside")
	// Each root alone: os.MkdirTemp("", …) creates under TMPDIR, t.TempDir() under GOTMPDIR.
	for _, c := range [][2]string{{inside, inside}, {inside, outside}, {outside, inside}} {
		out, err := child(c[0], c[1])
		if got := git(t, repo, "", "config", "--get", "remote.origin.url"); got != want {
			t.Errorf("TMPDIR=%s GOTMPDIR=%s: the planted fixture rewrote the enclosing repository's origin URL to %q", c[0], c[1], got)
			git(t, repo, "", "remote", "set-url", "origin", want)
		}
		if err == nil || !strings.Contains(out, "gitquiet: refusing to run the tests") || !strings.Contains(out, repo) {
			t.Errorf("TMPDIR=%s GOTMPDIR=%s: the binary ran instead of refusing (err %v):\n%s", c[0], c[1], err, out)
		}
	}
	out, err := child(outside, outside)
	if err != nil || !strings.Contains(out, "--- PASS: TestPlantedSetURL") {
		t.Errorf("control: roots outside every repository, the binary did not run the test (err %v):\n%s", err, out)
	}
}

// TestEnclosingRepo — the probe. A directory below a work tree or a bare repository is
// inside it, whatever GIT_CEILING_DIRECTORIES the caller set; a directory outside both is
// not; a directory git cannot enter is an error, even when its path contains git's
// "not a git repository" phrase.
//
// regression: F-temp-root-in-repo
func TestEnclosingRepo(t *testing.T) {
	root := t.TempDir()
	work, bare, out := filepath.Join(root, "work"), filepath.Join(root, "bare.git"), filepath.Join(root, "out")
	git(t, root, "", "init", "-q", work)
	git(t, root, "", "init", "-q", "--bare", bare)
	for _, d := range []string{filepath.Join(work, "a", "b"), filepath.Join(bare, "tmp"), out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A ceiling at each repository hides it from a probe that honours the caller's GIT_*
	// variables; an isolating child's environment drops the ceiling, so the probe must too.
	t.Setenv("GIT_CEILING_DIRECTORIES", work+string(os.PathListSeparator)+bare)
	for dir, inside := range map[string]bool{filepath.Join(work, "a", "b"): true, filepath.Join(bare, "tmp"): true, out: false} {
		got, err := enclosingRepo(dir)
		if err != nil || (got != "") != inside {
			t.Errorf("enclosingRepo(%s) = %q, %v; want inside=%v", dir, got, err, inside)
		}
	}
	// A root git cannot even enter is could-not-check, never "outside every repository".
	if got, err := enclosingRepo(filepath.Join(out, "absent")); err == nil || got != "" {
		t.Errorf("enclosingRepo(absent dir) = %q, %v; want a could-not-check error", got, err)
	}
	// git echoes the path in that error. A path that contains git's clean phrase must not
	// make the failure read as clean: only stderr that starts with it counts.
	if got, err := enclosingRepo(filepath.Join(out, "not a git repository")); err == nil || got != "" {
		t.Errorf("enclosingRepo(absent dir named for the clean phrase) = %q, %v; want a could-not-check error", got, err)
	}
}
