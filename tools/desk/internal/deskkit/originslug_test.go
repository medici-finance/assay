package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

// The transport pin a worktree repair writes: a reset of the inherited list, then one
// https URL for the same repository, for both the fetch and the push list.
var pinArgs = [][]string{
	{"config", "--worktree", "--replace-all", "remote.origin.url", ""},
	{"config", "--worktree", "--add", "remote.origin.url", "https://github.com:443/owner/repo.git"},
	{"config", "--worktree", "--replace-all", "remote.origin.pushurl", ""},
	{"config", "--worktree", "--add", "remote.origin.pushurl", "https://github.com:443/owner/repo.git"},
	{"config", "--worktree", "--replace-all", "credential.helper", ""},
}

// pinTo is the worktree pin shape with url in place of the repair's own URL.
func pinTo(url string) [][]string {
	return [][]string{
		{"config", "--worktree", "--replace-all", "remote.origin.url", ""},
		{"config", "--worktree", "--add", "remote.origin.url", url},
	}
}

func gitAll(t *testing.T, dir string, cmds [][]string) {
	t.Helper()
	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %s %v", args, out, err)
		}
	}
}

// Every slug reader built on SlugRemoteURL refuses when config it does not merge could
// change origin, including a worktree pin that names another repository.
//
// regression: #6606
func TestOriginSlugScopeRefused(t *testing.T) {
	const steer = "https://github.com/other/steer.git"
	wt := func(args ...string) []string { return append([]string{"config", "--worktree"}, args...) }
	cases := []struct {
		name   string
		ext    bool
		atRoot bool
		args   [][]string
	}{
		{"url appended", true, false, [][]string{wt("remote.origin.url", steer)}},
		{"url appended, main", true, true, [][]string{wt("remote.origin.url", steer)}},
		{"reset only", true, false, [][]string{wt("--replace-all", "remote.origin.url", "")}},
		{"pin other repo", true, false, [][]string{
			wt("--replace-all", "remote.origin.url", ""), wt("--add", "remote.origin.url", steer)}},
		{"pin two urls", true, false, [][]string{
			wt("--replace-all", "remote.origin.url", ""),
			wt("--add", "remote.origin.url", "https://github.com/owner/repo.git"),
			wt("--add", "remote.origin.url", steer)}},
		{"pin plus insteadOf", true, false, append(append([][]string{}, pinArgs...),
			wt("url.https://github.com/other/.insteadOf", "https://github.com:443/owner/"))},
		{"pin plus include", true, false, append(append([][]string{}, pinArgs...), wt("include.path", "steer.cfg"))},
		{"pin over two local urls", true, false, append([][]string{
			{"config", "--add", "remote.origin.url", "https://github.com/owner/repo.git"}}, pinArgs...)},
		{"pin other host", true, false, pinTo("https://steer.example.invalid:443/owner/repo.git")},
		{"pin longer path", true, false, pinTo("https://github.com:443/steer/owner/repo.git")},
		// Hosts that fold onto an ASCII host only under Unicode case mapping (U+0130, U+212A).
		{"pin lookalike host", true, false, pinTo("https://g\u0130thub.com:443/owner/repo.git")},
		{"pin lookalike, ssh origin", true, false, append([][]string{
			{"remote", "set-url", "origin", "git@github.com:owner/repo.git"}},
			pinTo("https://G\u0130THUB.com/owner/repo.git")...)},
		{"pin Kelvin-sign host", true, false, append([][]string{
			{"remote", "set-url", "origin", "https://bitbucket.org/owner/repo.git"}},
			pinTo("https://bitbuc\u212aet.org/owner/repo.git")...)},
		{"ssh origin, pin other host", true, false, append([][]string{
			{"remote", "set-url", "origin", "git@github.com:owner/repo.git"}},
			pinTo("https://steer.example.invalid/owner/repo.git")...)},
		{"pin lookalike scp host", true, false, pinTo("git@g\u0130thub.com:owner/repo")},
		// An ssh alias stands for a network host; a pin with no host is not that host.
		{"alias pin without host", true, false, append([][]string{
			{"remote", "set-url", "origin", "example-alias:owner/repo"}},
			pinTo("file:///owner/repo.git")...)},
		{"alias pin local path", true, false, append([][]string{
			{"remote", "set-url", "origin", "example-alias:owner/repo"}},
			pinTo("/owner/repo.git")...)},
		// A pin may move ssh to https; it may not move to a cleartext transport.
		{"pin cleartext http", true, false, pinTo("http://github.com/owner/repo.git")},
		{"pin git protocol", true, false, pinTo("git://github.com/owner/repo.git")},
		{"include", false, true, [][]string{{"config", "include.path", "steer.cfg"}}},
		{"includeIf", false, false, [][]string{{"config", "includeIf.gitdir:/.path", "steer.cfg"}}},
		{"activation grammar", false, true, [][]string{{"config", "extensions.worktreeConfig", "maybe"}}},
	}
	for _, tc := range cases {
		root, linked, _ := workSourceFixture(t, tc.ext)
		dir := linked
		if tc.atRoot {
			dir = root
		}
		gitAll(t, dir, tc.args)
		slug, err := OriginRepoSlug(dir)
		if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) {
			t.Errorf("%s: OriginRepoSlug must refuse, got %q %v", tc.name, slug, err)
			continue
		}
		if strings.Contains(err.Error(), "other/") || strings.Contains(err.Error(), "steer") {
			t.Errorf("%s: refusal echoes a configured value: %v", tc.name, err)
		}
		if got := RepoSlugForDir(dir); got != "" {
			t.Errorf("%s: RepoSlugForDir must not decide a slug, got %q", tc.name, got)
		}
	}
}

// Controls: the pin a worktree transport repair writes, from an https or an ssh shared
// origin, and worktree keys that cannot change the fetch list, all still decide the slug.
//
// regression: #6606
func TestOriginSlugPinAccepted(t *testing.T) {
	cases := []struct {
		name string
		args [][]string
	}{
		{"https origin pinned", pinArgs},
		{"ssh origin pinned", append([][]string{{"remote", "set-url", "origin", "git@github.com:owner/repo.git"}}, pinArgs...)},
		// An ssh alias only ssh resolves: the pin's host cannot be compared, its path still is.
		{"ssh alias pinned", append([][]string{{"remote", "set-url", "origin", "example-alias:owner/repo"}}, pinArgs...)},
		{"identity only", nil},
	}
	for _, tc := range cases {
		root, linked, _ := workSourceFixture(t, true)
		gitAll(t, linked, tc.args)
		for _, dir := range []string{linked, root} {
			slug, err := OriginRepoSlug(dir)
			if err != nil || slug != "owner/repo" {
				t.Errorf("%s: OriginRepoSlug = %q %v, want owner/repo", tc.name, slug, err)
			}
			if got := RepoSlugForDir(dir); got != "owner/repo" {
				t.Errorf("%s: RepoSlugForDir = %q, want owner/repo", tc.name, got)
			}
		}
	}
}

// Environment native git reads and no config file carries: command-priority config and
// repository-location variables. Each refuses with and without the worktree pin, and the
// work-source reader built on ScopedRemoteURLs refuses too.
//
// regression: #6606
func TestOriginSlugEnvRefused(t *testing.T) {
	for _, v := range []struct{ key, val string }{
		{"GIT_CONFIG_COUNT", "1"},
		{"GIT_CONFIG_PARAMETERS", "'remote.origin.url'=''"},
		{"GIT_CONFIG", "steer.cfg"},
		{"GIT_DIR", "steer"},
		{"GIT_COMMON_DIR", "steer"},
		{"GIT_WORK_TREE", "steer"},
	} {
		for _, pinned := range []bool{false, true} {
			_, linked, _ := workSourceFixture(t, true)
			if pinned {
				gitAll(t, linked, pinArgs)
			}
			t.Run(v.key, func(t *testing.T) {
				t.Setenv(v.key, v.val)
				slug, err := OriginRepoSlug(linked)
				if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || strings.Contains(err.Error(), "steer") {
					t.Errorf("pinned=%v: OriginRepoSlug must refuse, got %q %v", pinned, slug, err)
				}
				repo, err := gitcore.Open(linked)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := repo.ScopedRemoteURLs("origin"); !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) {
					t.Errorf("pinned=%v: ScopedRemoteURLs must refuse, got %v", pinned, err)
				}
			})
		}
	}
}

// Activation spellings outside git's own boolean grammar, and a worktree-scope file in a
// form git accepts but this reader does not parse, refuse with the scope reason, for the
// slug readers and the work-source reader alike. Git's own spellings still decide.
//
// regression: #6606
func TestOriginSlugGrammar(t *testing.T) {
	activation := func(v string) func(t *testing.T, root, linked string) {
		return func(t *testing.T, root, _ string) {
			gitAll(t, root, [][]string{{"config", "extensions.worktreeConfig", v}})
		}
	}
	unparsed := func(t *testing.T, _, linked string) {
		out, err := exec.Command("git", "-C", linked, "rev-parse", "--absolute-git-dir").Output()
		if err != nil {
			t.Fatal(err)
		}
		// git reads this as remote.origin.url; the decoder here rejects the header form.
		body := "[remote \"origin\"] url = https://github.com/other/steer.git\n"
		file := filepath.Join(strings.TrimSpace(string(out)), "config.worktree")
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name  string
		ext   bool
		setup func(t *testing.T, root, linked string)
	}{
		{"activation t", false, activation("t")},
		{"activation F", false, activation("F")},
		{"unparsed worktree file", true, unparsed},
	} {
		root, linked, _ := workSourceFixture(t, tc.ext)
		tc.setup(t, root, linked)
		for _, dir := range []string{linked, root} {
			if tc.name == "unparsed worktree file" && dir == root {
				continue // the file is the linked worktree's own scope
			}
			slug, err := OriginRepoSlug(dir)
			if !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || strings.Contains(err.Error(), "steer") {
				t.Errorf("%s: OriginRepoSlug must refuse with the scope reason, got %q %v", tc.name, slug, err)
			}
			_, err = LocalWorkCatalog(dir, "")
			if err == nil || !strings.Contains(err.Error(), "could be changed by worktree-scope or include config") ||
				strings.Contains(err.Error(), "steer") {
				t.Errorf("%s: LocalWorkCatalog must refuse with the scope message, got %v", tc.name, err)
			}
		}
	}
	for _, v := range []string{"TRUE", "On", "no", "0"} {
		root, linked, _ := workSourceFixture(t, false)
		gitAll(t, root, [][]string{{"config", "extensions.worktreeConfig", v}})
		for _, dir := range []string{linked, root} {
			if slug, err := OriginRepoSlug(dir); err != nil || slug != "owner/repo" {
				t.Errorf("activation %s: OriginRepoSlug = %q %v, want owner/repo", v, slug, err)
			}
		}
	}
}
