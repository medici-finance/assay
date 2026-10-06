// Package gitquiet stops git's automatic background maintenance in every repository a
// test binary creates. It is test support: a package's TestMain calls Run in place of
// m.Run, and no desk tool imports it.
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
// keys itself, as `-c` flags or written into the repository's config.
package gitquiet

import (
	"fmt"
	"os"
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

// Install writes the template into a fresh temporary directory and points
// GIT_TEMPLATE_DIR at it for this process and every child that inherits its
// environment. The template keeps the hooks/ and info/exclude a default template
// provides (without the sample hooks), so a fixture that writes a hook or an exclude
// finds the directory it expects. The returned func restores the previous GIT_TEMPLATE_DIR and removes the template.
func Install() (restore func(), err error) {
	dir, err := os.MkdirTemp("", "gitquiet-template-")
	if err != nil {
		return nil, err
	}
	fail := func(e error) (func(), error) {
		_ = os.RemoveAll(dir)
		return nil, e
	}
	for _, sub := range []string{"hooks", "info"} {
		if e := os.Mkdir(filepath.Join(dir, sub), 0o755); e != nil {
			return fail(e)
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "info", "exclude"), []byte("# git ls-files --others --exclude-from=.git/info/exclude\n"), 0o644); e != nil {
		return fail(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "config"), []byte(templateConfig()), 0o644); e != nil {
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

// Run installs the template, runs the package's tests, and restores the environment.
// A package's TestMain calls it in place of m.Run and hands the result to os.Exit.
func Run(m *testing.M) int {
	restore, err := Install()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gitquiet: cannot install the git template: %v\n", err)
		return 1
	}
	defer restore()
	return m.Run()
}
