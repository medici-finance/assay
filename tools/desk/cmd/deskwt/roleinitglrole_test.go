package main

// #4978: every stamping site (role-init, add --role) must stamp each GitLab role with its
// OWN address, never the first ASSAY_GITLAB_SESSION_EMAILS entry for every role.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	glVerifierAddr = "service_account_group_9619193_ve11@noreply.gitlab.example.com"
	glWorkerAddr   = "service_account_group_9619193_wo22@noreply.gitlab.example.com"
)

// gitLabTwoRoleRoster binds BOTH verifier and worker to distinct GitLab service accounts,
// lists the WORKER's address first in the session allowlist, and (when roleEmails != "")
// adds ASSAY_GITLAB_ROLE_EMAILS.
func gitLabTwoRoleRoster(t *testing.T, roleEmails string) {
	t.Helper()
	r := strings.ReplaceAll(fixtureRoster,
		"verifier=assay-verifier-app:300000005",
		"verifier=gitlab:assay-verifier-sa:41987965")
	r = strings.ReplaceAll(r,
		"worker=assay-worker-app:300000006",
		"worker=gitlab:assay-worker-sa:41987966")
	r += "ASSAY_GITLAB_SESSION_EMAILS=" + glWorkerAddr + "," + glVerifierAddr + "\n"
	if roleEmails != "" {
		r += "ASSAY_GITLAB_ROLE_EMAILS=" + roleEmails + "\n"
	}
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(r), 0o600); err != nil {
		t.Fatalf("re-plant GitLab roster: %v", err)
	}
	deskkit.ReloadConfig()
	if !deskkit.EffectiveConfig().Configured() {
		t.Fatalf("two-role GitLab roster not configured: %v", deskkit.EffectiveConfig().Problems)
	}
}

// TestRoleInitGLStampsOwnAddr — role-init stamps the verifier's OWN address, not the
// allowlist's first entry (which is the worker's).
// regression: #4978
func TestRoleInitGLStampsOwnAddr(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	gitLabTwoRoleRoster(t, "verifier="+glVerifierAddr+",worker="+glWorkerAddr)

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "glown"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init rc = %d, want 0; stderr: %s", rc, stderr)
	}
	got := mustGit(t, filepath.Join(tmpBaseDir, "tracker-verify-desk-glown"), "config", "--worktree", "--get", "user.email")
	if got != glVerifierAddr {
		t.Fatalf("verifier worktree stamped %q, want its own address %q (the first allowlist entry is the worker's)", got, glVerifierAddr)
	}
}

// TestRoleInitGLAmbiguousRefuses — with no per-role binding the two-entry allowlist does not
// say which address is the verifier's; role-init refuses (exit 5) naming the key, and creates
// nothing. Pre-fix it stamped the first entry — the worker's account.
// regression: #4978
func TestRoleInitGLAmbiguousRefuses(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	gitLabTwoRoleRoster(t, "")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "glamb"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("role-init rc = %d, want 5 (refused); stderr: %s", rc, stderr)
	}
	if !contains(stderr, deskkit.EnvGitLabRoleEmails) {
		t.Fatalf("refusal must name %s; stderr: %s", deskkit.EnvGitLabRoleEmails, stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("a worktree was created despite the refusal; git calls: %v", gitCalls(*calls))
	}
}

// TestAddRoleGLStampsOwnAddr — the second stamping site, `deskwt add --role`, resolves through
// the same choke point: each role's worktree carries its own address, and the ambiguous
// allowlist refuses there too.
// regression: #4978
func TestAddRoleGLStampsOwnAddr(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	gitLabTwoRoleRoster(t, "verifier="+glVerifierAddr+",worker="+glWorkerAddr)

	for _, c := range []struct{ name, role, want string }{
		{"glver", "verifier", glVerifierAddr},
		{"glwrk", "worker", glWorkerAddr},
	} {
		if rc := run([]string{"add", c.name, "--role", c.role}); rc != deskkit.ExitOK {
			t.Fatalf("add --role %s rc = %d, want 0", c.role, rc)
		}
		got := mustGit(t, filepath.Join(tmpBaseDir, "tracker-"+c.name), "config", "--worktree", "--get", "user.email")
		if got != c.want {
			t.Fatalf("add --role %s stamped %q, want %q", c.role, got, c.want)
		}
	}
}

// TestAddRoleGLAmbiguousRefuses — the planted second instance of the defect: `deskwt add
// --role` on the ambiguous allowlist refuses too. Pre-fix it stamped the worker's address
// on the verifier's worktree.
// regression: #4978
func TestAddRoleGLAmbiguousRefuses(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	gitLabTwoRoleRoster(t, "")

	target := filepath.Join(tmpBaseDir, "tracker-glamb")
	if rc := run([]string{"add", "glamb", "--role", "verifier"}); rc != deskkit.ExitRefused {
		stamped := ""
		if _, err := os.Stat(target); err == nil {
			stamped = mustGit(t, target, "config", "--worktree", "--get", "user.email")
		}
		t.Fatalf("add --role verifier on an ambiguous allowlist rc = %d, want 5 (refused); stamped %q", rc, stamped)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target dir exists after a refusal: %v", err)
	}
}
