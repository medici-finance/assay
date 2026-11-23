// Package gitquiet stops git's automatic background maintenance in every repository a
// test binary creates. It is test support: a package's TestMain calls Run in place of
// m.Run, and production tools do not import it.
//
// THE DEFECT IT CLOSES. `git commit` (and fetch, merge, am, and receive-pack on the far
// side of a local push) ends by forking `git maintenance run --auto --detach`. The
// detached child outlives the git command the test waited on and keeps writing inside
// `.git` (a lock file, a repacked pack, a multi-pack-index). A fixture repository built
// under t.TempDir() is removed when the test returns, so that child races the removal,
// and when it wins the test fails with
//
//	TempDir RemoveAll cleanup: unlinkat …/.git/objects: directory not empty
//
// after every assertion held. It is a race, so it reddens whichever test lost on a
// loaded runner, never the one that is actually broken.
//
// THE FIX. `maintenance.auto=false` and `gc.auto=0` stop the fork happening at all. They
// must sit in each repository's OWN config: git clears GIT_CONFIG_COUNT and
// GIT_CONFIG_PARAMETERS when it runs a command in a different repository (a push to a
// local bare remote starts receive-pack there), and many fixtures point
// GIT_CONFIG_GLOBAL at an empty file and set GIT_CONFIG_NOSYSTEM. So Run points
// GIT_TEMPLATE_DIR at a template whose `config` carries both keys: `git init` and
// `git clone` copy it into every new repository, bare remotes included, whoever runs
// them — a fixture helper or the tool under test.
//
// A child started with an environment that drops GIT_TEMPLATE_DIR (a literal env list,
// or one with every GIT_* key removed) is outside this cover; such a fixture sets the
// keys itself, as `-c` flags or written into the repository's config, or passes its own
// template explicitly with TemplateDir.
//
// THE SECOND DEFECT IT CLOSES: a temporary root inside a repository. A test that runs git
// in a bare t.TempDir() and expects "not a git repository" relies on there being no
// repository above the temporary root. When TMPDIR or GOTMPDIR (t.TempDir's root when
// set) points inside a work tree, git's discovery walks up from the empty directory and
// finds it, and a write meant to fail lands there instead. In a linked worktree that is
// the shared checkout's own config: a `git remote set-url origin` fixture once rewrote
// the shared checkout's fetch URL this way. GIT_CEILING_DIRECTORIES does not close it,
// because a child whose environment drops every inherited GIT_* variable (a deliberate
// isolation rule) never sees the ceiling. So Run asks git, with every inherited GIT_*
// variable removed, whether each temporary root is inside a repository, and refuses to
// run the tests when one is: the fault is the caller's environment, and it is reported
// before any test can write through it.
package gitquiet

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Settings are the config keys, in order, that the template writes into every new
// repository. Each one alone stops the automatic fork on current git; both are set so a
// git that still reaches `gc --auto` directly is covered too.
var Settings = [][2]string{
	{"maintenance.auto", "false"},
	{"gc.auto", "0"},
}

// templateConfig renders Settings as a git config file.
func templateConfig() string {
	var b strings.Builder
	for _, kv := range Settings {
		i := strings.LastIndex(kv[0], ".")
		fmt.Fprintf(&b, "[%s]\n\t%s = %s\n", kv[0][:i], kv[0][i+1:], kv[1])
	}
	return b.String()
}

// writeTemplate writes the template into the existing directory dir. It keeps the
// hooks/ and info/exclude a default template provides (without the sample hooks), so a
// fixture that writes a hook or an exclude finds the directory it expects.
func writeTemplate(dir string) error {
	for _, sub := range []string{"hooks", "info"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "info", "exclude"), []byte("# git ls-files --others --exclude-from=.git/info/exclude\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config"), []byte(templateConfig()), 0o644)
}

// Install writes the template into a fresh temporary directory and points
// GIT_TEMPLATE_DIR at it for this process and every child that inherits its
// environment. The returned func restores the previous GIT_TEMPLATE_DIR and removes the
// template.
func Install() (restore func(), err error) {
	dir, err := os.MkdirTemp("", "gitquiet-template-")
	if err != nil {
		return nil, err
	}
	fail := func(e error) (func(), error) {
		_ = os.RemoveAll(dir)
		return nil, e
	}
	if e := writeTemplate(dir); e != nil {
		return fail(e)
	}
	prev, had := os.LookupEnv("GIT_TEMPLATE_DIR")
	if e := os.Setenv("GIT_TEMPLATE_DIR", dir); e != nil {
		return fail(e)
	}
	return func() {
		if had {
			_ = os.Setenv("GIT_TEMPLATE_DIR", prev)
		} else {
			_ = os.Unsetenv("GIT_TEMPLATE_DIR")
		}
		_ = os.RemoveAll(dir)
	}, nil
}

// tempRoots names the directories a test binary creates its temporary files under:
// os.TempDir() (TMPDIR, or TMP/TEMP on Windows: os.MkdirTemp("", …)) and GOTMPDIR when
// it is set, which is where t.TempDir() creates its directories.
func tempRoots() [][2]string {
	roots := [][2]string{{"the temporary directory (TMPDIR)", os.TempDir()}}
	if d := os.Getenv("GOTMPDIR"); d != "" {
		roots = append(roots, [2]string{"GOTMPDIR", d})
	}
	return roots
}

// enclosingRepo returns the git directory git discovers from dir, or "" when dir is in
// no repository. It asks git itself, in the environment most likely to find one: every
// inherited GIT_* variable removed (GIT_CEILING_DIRECTORIES included, as an isolating
// child's environment removes it), and discovery allowed across filesystems. Ownership
// checks stay on: git never parses the config of a repository owned by another account,
// and its dubious-ownership error is not "not a git repository", so that root is refused
// as could-not-check. Only stderr that starts with git's own "fatal: not a git
// repository" reads as clean, so an error that merely echoes a path containing that
// phrase does not. With no git on PATH there is nothing that could discover a
// repository, and it returns "".
func enclosingRepo(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--absolute-git-dir")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_DISCOVERY_ACROSS_FILESYSTEM=1")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	switch {
	case err == nil:
		return strings.TrimSpace(string(out)), nil
	case errors.Is(err, exec.ErrNotFound):
		return "", nil
	case strings.HasPrefix(strings.TrimSpace(stderr.String()), "fatal: not a git repository"):
		return "", nil
	}
	return "", fmt.Errorf("could-not-check whether %s is inside a git repository: git rev-parse: %v: %s",
		dir, err, strings.TrimSpace(stderr.String()))
}

// checkTempRoots refuses a temporary root that is inside a git repository, and one it
// could not check.
func checkTempRoots() error {
	for _, r := range tempRoots() {
		gitDir, err := enclosingRepo(r[1])
		if err != nil {
			return fmt.Errorf("%s: %w", r[0], err)
		}
		if gitDir != "" {
			return fmt.Errorf("%s %s is inside the git repository %s. A test that runs git in an "+
				"empty temporary directory would discover that repository and could write to it "+
				"(for a linked worktree, to the shared checkout's config). Point TMPDIR and GOTMPDIR "+
				"at a directory outside every git repository", r[0], r[1], gitDir)
		}
	}
	return nil
}

// Run refuses a temporary root inside a git repository (checkTempRoots), then installs
// the template, runs the package's tests, and restores the environment. A package's
// TestMain calls it in place of m.Run and hands the result to os.Exit.
func Run(m *testing.M) int {
	if err := checkTempRoots(); err != nil {
		fmt.Fprintf(os.Stderr, "gitquiet: refusing to run the tests: %v\n", err)
		return 1
	}
	restore, err := Install()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gitquiet: cannot install the git template: %v\n", err)
		return 1
	}
	defer restore()
	return m.Run()
}

// TemplateDir writes the template into a new directory under t.TempDir() and returns
// it. It is for a fixture whose child environment is built WITHOUT the inherited
// GIT_* variables (an isolation env that drops every one of them, so Run's
// GIT_TEMPLATE_DIR never reaches the child): the fixture appends
// "GIT_TEMPLATE_DIR="+TemplateDir(t) to that env explicitly. The value is the
// fixture's own directory, never one read from the caller's environment, so the
// isolation rule that drops inherited GIT_* variables is unchanged.
func TemplateDir(t testing.TB) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "gitquiet-template")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeTemplate(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}
