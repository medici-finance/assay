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
	git(t, root, "", "init", "-q", quiet)
	for _, kv := range Settings {
		if got := git(t, quiet, "", "config", "--get", kv[0]); got != kv[1] {
			t.Errorf("effective %s = %q, want %q", kv[0], got, kv[1])
		}
	}
	restore()
	if os.Getenv("GIT_TEMPLATE_DIR") != previous {
		t.Fatal("caller template was not restored")
	}
	plain := filepath.Join(root, "plain")
	git(t, root, "", "init", "-q", plain)
	if got := git(t, plain, "", "config", "--get", "maintenance.auto"); got != "true" {
		t.Fatalf("negative control did not inherit hostile config: %q", got)
	}
}
