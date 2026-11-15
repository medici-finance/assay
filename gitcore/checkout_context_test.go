package gitcore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #4823
func TestCheckoutOriginContext(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_CONFIG_PARAMETERS", "")
	for _, tc := range []struct{ name, origin, want string }{
		{"https-port", "https://github.com:443/example/project.git", "https://github.com:443/example/project.git"},
		{"ssh", "git@github.com:example/project.git", "ssh://github.com/example/project.git"},
		{"foreign", "https://gitlab.example/example/project.git", "https://gitlab.example/example/project.git"},
		{"credentials", "https://fixture-user:" + "fixture-password" + "@github.com/example/project.git?fixture-secret=yes#fixture-secret", "https://github.com/example/project.git"},
		{"ssh-query", "ssh://git@github.com/example/project.git?fixture-secret=yes#fixture-secret", "ssh://github.com/example/project.git"},
		{"scp-query", "git@github.com:example/project.git?fixture-secret=yes#fixture-secret", "ssh://github.com/example/project.git"},
		{"missing", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := gittest.NewFixture(t)
			sha := f.CommitFile(t, "check.txt", "check\n", "fixture")
			git := func(args ...string) string {
				t.Helper()
				out, err := f.Git(args...)
				if err != nil {
					t.Fatal(err)
				}
				return strings.TrimSpace(out)
			}
			git("update-ref", "refs/remotes/origin/main", sha)
			if tc.origin != "" {
				git("remote", "add", "origin", tc.origin)
			}
			git("config", "credential.helper", "fixture-secret-helper")
			git("config", "http.extraHeader", "fixture-secret-header")
			git("config", "core.hooksPath", "fixture-secret-hooks")
			before, err := os.ReadFile(filepath.Join(f.Dir, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			// This independent caller plants the same lost-context defect outside deskevidence.
			dest := t.TempDir()
			if err := CheckoutLocalCommit(f.Dir, dest, sha, "refs/remotes/origin/main"); err != nil {
				t.Fatal(err)
			}
			r, err := Open(dest)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.RemoteURL("origin")
			if tc.want == "" {
				if err == nil {
					t.Fatalf("invented origin %q", got)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("planted checkout lost repository authority: origin=%q err=%v want=%q", got, err, tc.want)
			}
			config, err := os.ReadFile(filepath.Join(dest, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(config), "fixture-") {
				t.Fatalf("credentials/config copied: %s", config)
			}
			after, err := os.ReadFile(filepath.Join(f.Dir, ".git", "config"))
			if err != nil || string(after) != string(before) {
				t.Fatal("source configuration changed")
			}
			head, err := r.Resolve("HEAD")
			if err != nil || head.String() != sha {
				t.Fatal("checkout revision changed")
			}
		})
	}
}

// regression: #4823 C1 C2 C3
func TestCheckoutEffectiveOrigin(t *testing.T) {
	const want = "https://github.com:443/example/project.git"
	for _, mode := range []string{"worktree-host", "worktree-other", "local-reset", "push-only", "multiple", "push-rewrite", "fetch-rewrite", "malformed", "unreadable", "include", "global", "global-rewrite", "injected", "bare-extension"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_COUNT", "0")
			t.Setenv("GIT_CONFIG_PARAMETERS", "")
			f := gittest.NewFixture(t)
			sha, err := f.Git("rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			git := func(args ...string) {
				t.Helper()
				if _, err := f.Git(args...); err != nil {
					t.Fatal(err)
				}
			}
			git("update-ref", "refs/remotes/origin/main", sha)
			git("remote", "add", "origin", want)
			source := f.Dir
			expected := want
			switch mode {
			case "worktree-host", "worktree-other", "unreadable":
				shared := "git@gitlab.com:example/project.git"
				if mode == "worktree-other" {
					shared = "git@github.com:other/project.git"
				}
				git("remote", "set-url", "origin", shared)
				git("config", "extensions.worktreeConfig", "true")
				source = filepath.Join(t.TempDir(), "linked")
				git("worktree", "add", "--detach", source, "HEAD")
				linked := &gittest.Fixture{Dir: source}
				for _, args := range [][]string{{"config", "--worktree", "--add", "remote.origin.url", ""}, {"config", "--worktree", "--add", "remote.origin.url", want}} {
					if _, err := linked.Git(args...); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "unreadable" {
					admin, err := resolveGitDirForTest(source)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(filepath.Join(admin, "config.worktree")); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(filepath.Join(admin, "config.worktree"), 0700); err != nil {
						t.Fatal(err)
					}
					expected = ""
				}
			case "local-reset":
				git("config", "--replace-all", "remote.origin.url", "")
				git("config", "--add", "remote.origin.url", want)
			case "push-only":
				git("config", "--unset-all", "remote.origin.url")
				git("config", "remote.origin.pushurl", want)
				expected = ""
			case "multiple":
				git("config", "--add", "remote.origin.url", "https://github.com/other/project.git")
				expected = ""
			case "push-rewrite":
				git("config", "url.https://push.invalid/.pushInsteadOf", "https://github.com/")
			case "fetch-rewrite":
				git("remote", "set-url", "origin", "gitlab.com:example/project.git")
				git("config", "url.https://github.com:443/.insteadOf", "gitlab.com:")
			case "malformed":
				git("remote", "set-url", "origin", "ssh://git@github.com:o/r.git")
				expected = ""
			case "include":
				// Git honours this match and silently skips the missing file.
				git("config", "includeIf.gitdir:**.path", filepath.Join(t.TempDir(), "missing"))
			case "global-rewrite":
				global := filepath.Join(t.TempDir(), "global")
				if err := os.WriteFile(global, []byte("[url \"https://github.com:443/\"]\n insteadOf = gitlab.com:\n"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_CONFIG_GLOBAL", global)
				git("remote", "set-url", "origin", "gitlab.com:example/project.git")
			case "injected":
				t.Setenv("GIT_CONFIG_COUNT", "1")
				t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
				t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/other/project.git")
				expected = ""
			case "bare-extension":
				git("config", "extensions.worktreeConfig", "")
				expected = ""
			case "global":
				global := filepath.Join(t.TempDir(), "global")
				if err := os.WriteFile(global, []byte("[remote \"origin\"]\n url = https://github.com/other/project.git\n"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_CONFIG_GLOBAL", global)
				expected = "" // multiple effective fetch URLs must not be narrowed
			}
			if expected != "" {
				cmd := exec.Command("git", "-C", source, "remote", "get-url", "--all", "origin")
				raw, err := cmd.Output()
				if err != nil || strings.TrimSpace(string(raw)) != want {
					t.Fatalf("source effective origin=%q err=%v", raw, err)
				}
			}
			dest := t.TempDir()
			if err := CheckoutLocalCommit(source, dest, sha, "refs/remotes/origin/main"); err != nil {
				t.Fatalf("optional origin blocked checkout: %v", err)
			}
			out := &gittest.Fixture{Dir: dest}
			got, err := out.Git("remote", "get-url", "--all", "origin")
			if expected == "" {
				// A global-only origin can still be inherited by native Git. The snapshot
				// must never manufacture a local origin when its authority is uncertain.
				r, openErr := Open(dest)
				if openErr != nil {
					t.Fatal(openErr)
				}
				if urls, e := r.RemoteURLs("origin"); e == nil {
					t.Fatalf("uncertain authority copied: %q", urls)
				}
				if mode != "injected" { // injected command config also overrides any consumer; it is outside the profile
					cmd := exec.Command("git", "-C", dest, "remote", "get-url", "--all", "origin")
					raw, err := cmd.Output()
					if err == nil && strings.TrimSpace(string(raw)) != "origin" {
						t.Fatalf("native consumer inherited authority: %q", raw)
					}
				}
			} else if err != nil || got != expected {
				t.Fatalf("effective origin=%q err=%v want=%q", got, err, expected)
			}
		})
	}
}

func resolveGitDirForTest(dir string) (string, error) {
	admin, _, err := resolveGitDir(dir)
	return admin, err
}

// regression: #4823 C3 C4
func TestOriginIncludeOracle(t *testing.T) {
	const want = "https://github.com/example/project.git"
	const foreign = "[remote \"origin\"]\n url = https://github.com/foreign/project.git\n"
	// available rows are the review's accepted-at-merge-base shapes: they must
	// reach authority. Every other row may only agree with native Git or refuse.
	for _, tc := range []struct {
		name, global, content string
		available             bool
	}{
		{"home-slash", "[includeIf \"gitdir:~/.assay-fixture-absent/\"]\n path = INC\n", foreign, true},
		{"abs-slash", "[includeIf \"gitdir:/example.invalid/work/\"]\n path = INC\n", foreign, true},
		{"onbranch-other", "[includeIf \"onbranch:release/\"]\n path = INC\n", foreign, true},
		{"identity", "[include]\n path = INC\n", "[user]\n name = fixture\n email = fixture@example.invalid\n", true},
		{"missing", "[include]\n path = MISSING\n", "", true},
		{"credential-rewrite", "[url \"https://fixture-user:" + "fixture-token" + "@github.com/\"]\n insteadOf = https://github.com/\n", "", true},
		{"global-extension", "[extensions]\n worktreeConfig = true\n", "", true},
		{"include-extension", "[include]\n path = INC\n", "[extensions]\n worktreeConfig = true\n", false},
		{"home-match", "[includeIf \"gitdir:~/\"]\n path = INC\n", foreign, false},
		{"onbranch-match", "[includeIf \"onbranch:ma*\"]\n path = INC\n", foreign, false},
		{"question-match", "[includeIf \"gitdir:**/.gi?\"]\n path = INC\n", foreign, false},
		{"relative-match", "[includeIf \"gitdir:.git\"]\n path = INC\n", foreign, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			f := gittest.NewFixture(t)
			if _, err := f.Git("remote", "add", "origin", want); err != nil {
				t.Fatal(err)
			}
			physical, err := filepath.EvalSymlinks(filepath.Dir(f.Dir))
			if err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(dir, "home")
			if err := os.Mkdir(home, 0700); err != nil {
				t.Fatal(err)
			}
			if tc.name == "home-match" {
				home = physical
			}
			inc := filepath.Join(dir, "included.config")
			if tc.content != "" {
				if err := os.WriteFile(inc, []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// A worktree-scoped origin that Git ignores unless the repository itself
			// enables the extension; global or included declarations never do.
			if err := os.WriteFile(filepath.Join(f.Dir, ".git", "config.worktree"), []byte("[remote \"origin\"]\n url =\n url = https://github.com/foreign/worktree.git\n"), 0600); err != nil {
				t.Fatal(err)
			}
			body := strings.NewReplacer("INC", filepath.ToSlash(inc), "MISSING", filepath.ToSlash(filepath.Join(dir, "absent.config"))).Replace(tc.global)
			global := filepath.Join(dir, "global")
			if err := os.WriteFile(global, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_COUNT", "0")
			t.Setenv("GIT_CONFIG_PARAMETERS", "")
			raw, nativeErr := exec.Command("git", "-C", f.Dir, "remote", "get-url", "--all", "origin").Output()
			native := strings.TrimSpace(string(raw))
			if tc.available && (nativeErr != nil || strings.Contains(native, "\n") || !strings.Contains(native, "github.com/example/project.git")) {
				t.Fatalf("native control=%q err=%v", native, nativeErr)
			}
			got, err := OriginAuthority(f.Dir)
			if tc.available && (err != nil || got != want) {
				t.Fatalf("accepted shape withheld authority=%q err=%v", got, err)
			}
			if err == nil && (nativeErr != nil || strings.Contains(native, "\n") || !strings.HasSuffix(native, "@github.com/example/project.git") && native != want) {
				t.Fatalf("authority %q disagrees with native Git %q", got, native)
			}
			if err != nil && (strings.Contains(err.Error(), "://") || strings.Contains(err.Error(), "fixture") || strings.Contains(err.Error(), dir)) {
				t.Fatalf("unavailable class leaked detail: %v", err)
			}
			// The disposable snapshot carries the same decision to native consumers.
			dest := t.TempDir()
			sha := mustOut(t, f, "rev-parse", "HEAD")
			mustOut(t, f, "update-ref", "refs/remotes/origin/main", sha)
			if err := CheckoutLocalCommit(f.Dir, dest, sha, "refs/remotes/origin/main"); err != nil {
				t.Fatal(err)
			}
			snapshot, err := OriginAuthority(dest)
			if (err == nil) != (got != "") || snapshot != got {
				t.Fatalf("snapshot authority=%q err=%v source=%q", snapshot, err, got)
			}
		})
	}
}

func mustOut(t *testing.T, f *gittest.Fixture, args ...string) string {
	t.Helper()
	out, err := f.Git(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
