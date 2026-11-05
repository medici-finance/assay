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
func TestOriginReviewBoundaries(t *testing.T) {
	const origin = "https://github.com/example/project.git"
	const foreign = "https://github.com/foreign/project.git"
	for _, mode := range []string{"symlink-home", "symlink-home-repo", "symlink-home-fold", "unresolvable-home", "head-no-space", "head-tab", "chained-rewrite"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_COUNT", "0")
			t.Setenv("GIT_CONFIG_PARAMETERS", "")
			t.Setenv("PWD", "/") // no repository alias can mask a missed HOME expansion
			f := gittest.NewFixture(t)
			mustOut(t, f, "remote", "add", "origin", origin)
			sha := strings.TrimSpace(mustOut(t, f, "rev-parse", "HEAD"))
			mustOut(t, f, "update-ref", "refs/remotes/origin/main", sha)
			dir := t.TempDir()
			inc := filepath.Join(dir, "included.config")
			if err := os.WriteFile(inc, []byte("[remote \"origin\"]\n url = "+foreign+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			condition := "gitdir:~/"
			nativeMatch := true
			switch mode {
			case "symlink-home", "symlink-home-repo", "symlink-home-fold":
				physical, err := filepath.EvalSymlinks(f.Dir)
				if err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(dir, "home-link")
				if err := os.Symlink(filepath.Dir(physical), link); err != nil {
					t.Fatal(err)
				}
				t.Setenv("HOME", link)
				if mode == "symlink-home-repo" {
					condition += filepath.Base(physical) + "/"
				}
				if mode == "symlink-home-fold" {
					condition = "gitdir/i:~/"
				}
			case "unresolvable-home":
				t.Setenv("HOME", filepath.Join(dir, "missing-home"))
				nativeMatch = false // owner must conservatively inspect the include anyway
			case "head-no-space", "head-tab":
				branch := strings.TrimSpace(mustOut(t, f, "symbolic-ref", "HEAD"))
				separator := ""
				if mode == "head-tab" {
					separator = "\t"
				}
				if err := os.WriteFile(filepath.Join(f.Dir, ".git", "HEAD"), []byte("ref:"+separator+branch+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				condition = "onbranch:" + strings.TrimPrefix(branch, "refs/heads/")
			}
			body := "[includeIf \"" + condition + "\"]\n path = " + filepath.ToSlash(inc) + "\n"
			if mode == "chained-rewrite" {
				body = "[url \"https://github.com/first-hop/\"]\n insteadOf = https://github.com/example/\n[url \"https://github.com/second-hop/\"]\n insteadOf = https://github.com/first-hop/\n"
			}
			global := filepath.Join(dir, "global")
			if err := os.WriteFile(global, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			native := exec.Command("git", "-C", f.Dir, "remote", "get-url", "--all", "origin")
			native.Dir = "/"
			raw, err := native.Output()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "chained-rewrite" {
				if strings.TrimSpace(string(raw)) != "https://github.com/first-hop/project.git" {
					t.Fatalf("native chain control=%q", raw)
				}
			} else if nativeMatch && !strings.Contains(string(raw), foreign) {
				t.Fatalf("native include control=%q", raw)
			}
			got, err := OriginAuthority(f.Dir)
			if err == nil || got != "" {
				t.Errorf("uncertain source authority accepted: %q err=%v", got, err)
			}
			snapshot := t.TempDir()
			if err := CheckoutLocalCommit(f.Dir, snapshot, sha, "refs/remotes/origin/main"); err != nil {
				t.Fatal(err)
			}
			if got, err := OriginAuthority(snapshot); err == nil || got != "" {
				t.Errorf("uncertain snapshot authority accepted: %q err=%v", got, err)
			}
		})
	}
}

// regression: #4823
func TestOriginCaseVariantOracle(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_CONFIG_PARAMETERS", "")
	t.Setenv("PWD", "/")
	f := gittest.NewFixture(t)
	const origin = "https://github.com/example/project.git"
	mustOut(t, f, "remote", "add", "origin", origin)
	physical, err := filepath.EvalSymlinks(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	alias := strings.ToUpper(physical)
	actualInfo, err := os.Stat(physical)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(alias)
	if err != nil || !os.SameFile(actualInfo, aliasInfo) {
		t.Skip("case-variant alias not supported by this filesystem")
	}
	dir := t.TempDir()
	inc := filepath.Join(dir, "included.config")
	if err := os.WriteFile(inc, []byte("[remote \"origin\"]\n url = https://github.com/foreign/project.git\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"gitdir:", "gitdir/i:"} {
		t.Run(prefix, func(t *testing.T) {
			global := filepath.Join(dir, "global")
			if err := os.WriteFile(global, []byte("[includeIf \""+prefix+filepath.ToSlash(physical)+"/\"]\n path = "+filepath.ToSlash(inc)+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			native := exec.Command("git", "-C", alias, "remote", "get-url", "--all", "origin")
			native.Dir = "/"
			raw, nativeErr := native.Output()
			got, err := OriginAuthority(alias)
			if err == nil && (got != origin || nativeErr != nil || strings.TrimSpace(string(raw)) != origin) {
				t.Fatalf("case-variant authority=%q native=%q nativeErr=%v", got, raw, nativeErr)
			}
		})
	}
}
