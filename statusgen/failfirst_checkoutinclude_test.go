package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #7833 — verify-gate-close runs in an actions/checkout clone whose
// .git/config persists the push credential through includeIf entries. The
// delivering commit of a risk-bearing row must stay readable there, so its
// fail-first base can be checked as the commit's parent.
func TestDeliveringParentInCheckoutWithCredentialInclude(t *testing.T) {
	dir := t.TempDir()
	initRepoAt(t, dir, fixedDate)
	base := strings.TrimSpace(string(checkoutIncludeGit(t, dir, "rev-parse", "HEAD")))
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("delivered"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "f.txt")
	runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + fixedDate, "GIT_COMMITTER_DATE=" + fixedDate}, "commit", "-q", "-m", "deliver")
	delivering := strings.TrimSpace(string(checkoutIncludeGit(t, dir, "rev-parse", "HEAD")))

	admin, err := filepath.EvalSymlinks(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	cred := filepath.Join(t.TempDir(), "git-credentials-6c1f0d2a-8b4e-4a7f-9e3d-1a2b3c4d5e6f.config")
	if err := os.WriteFile(cred, []byte("[http \"https://github.com/\"]\n\textraheader = AUTHORIZATION: basic ***\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	container := "/github/runner_temp/" + filepath.Base(cred)
	runGit(t, dir, "config", "--add", "includeIf.gitdir:"+admin+".path", cred)
	runGit(t, dir, "config", "--add", "includeIf.gitdir:"+admin+"/worktrees/*.path", cred)
	runGit(t, dir, "config", "--add", "includeIf.gitdir:/github/workspace/.git.path", container)
	runGit(t, dir, "config", "--add", "includeIf.gitdir:/github/workspace/.git/worktrees/*.path", container)
	if got := strings.TrimSpace(string(checkoutIncludeGit(t, dir, "config", "--get", "http.https://github.com/.extraheader"))); got != "AUTHORIZATION: basic ***" {
		t.Fatalf("native Git did not merge the checkout include: %q", got)
	}

	resetReadMemo()
	full, parent, why := deliveringParent(dir, delivering)
	if why != "" {
		t.Fatalf("delivering commit refused in a checkout clone: %s", why)
	}
	if full != delivering || parent != base {
		t.Fatalf("delivering=%s parent=%s, want %s and %s", full, parent, delivering, base)
	}

	// A second include that names this repository and carries any other key keeps
	// the commit off the library path, as before.
	other := filepath.Join(t.TempDir(), "other.config")
	if err := os.WriteFile(other, []byte("[http \"https://github.com/\"]\n\tproxy = http://example.invalid:3128\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "config", "--add", "includeIf.gitdir:"+admin+".path", other)
	resetReadMemo()
	if _, _, why := deliveringParent(dir, delivering); !strings.Contains(why, "native discovery config includes are not merged") {
		t.Fatalf("include carrying another key was read: %q", why)
	}
}

func checkoutIncludeGit(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := historyGit(dir, args...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}
