package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// regression: #3327
func TestGitReadGitLab(t *testing.T) {
	installRoster(t, "ASSAY_ALLOWED_REPOS=example-team/project:ci:private\nASSAY_REPO_FORGES=example-team/project=gitlab\nASSAY_TRUSTED_BOT_SLUGS=worker=gitlab:example-worker:9001\n")
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("GITLAB_API_BASE", "https://gitlab.example.invalid:8443/api/v4")
	cfg := t.TempDir()
	t.Setenv(EnvConfigHome, cfg)
	if err := os.WriteFile(filepath.Join(cfg, "gitlab-worker.token"), []byte("gitlab-read-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git: %s %v", out, err)
		}
	}
	git("init", "-b", "main")
	git("remote", "add", "origin", "https://gitlab.example.invalid:8443/example-team/project.git")
	opts, err := CheckoutGitRead(root, "example-team/project", "worker")
	if err != nil {
		t.Fatal(err)
	}
	auth, ok := opts.Auth.(*githttp.BasicAuth)
	if !ok || auth.Username != "oauth2" || auth.Password != "gitlab-read-fixture" || opts.URL != "https://gitlab.example.invalid:8443/example-team/project.git" {
		t.Fatal("GitLab custody, destination or username changed")
	}
	for _, origin := range []string{"https://gitlab.example.invalid/example-team/project.git", "https://elsewhere.invalid:8443/example-team/project.git"} {
		git("remote", "set-url", "origin", origin)
		if _, err := CheckoutGitRead(root, "example-team/project", "worker"); err == nil {
			t.Fatal("GitLab host/port crossing admitted")
		}
	}
}
