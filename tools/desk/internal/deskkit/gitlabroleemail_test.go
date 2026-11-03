package deskkit

// Tests for the per-role GitLab commit address (#4978): RoleGitLabCommitIdentity used to
// return the FIRST ASSAY_GITLAB_SESSION_EMAILS entry for every GitLab role, so a cell that
// binds one service account per role committed every role's work under one account.

import (
	"strings"
	"testing"
)

const (
	glVerifierSA = "service_account_group_77_aa11@noreply.gitlab.example.com"
	glWorkerSA   = "service_account_group_77_bb22@noreply.gitlab.example.com"
	glTwoRoles   = "verifier=gitlab:example-verifier-sa:5001,worker=gitlab:example-worker-sa:5002"
)

// glRoster plants a roster binding verifier and worker to two distinct GitLab accounts.
func glRoster(t *testing.T, extra map[string]string) {
	t.Helper()
	withRoster(t, forgeRoster(glTwoRoles, extra))
	if !EffectiveConfig().Configured() {
		t.Fatalf("fixture roster not configured: %v", EffectiveConfig().Problems)
	}
}

func mustRefuse(t *testing.T, role string, wants ...string) {
	t.Helper()
	n, e, err := RoleGitLabCommitIdentity(role)
	if err == nil {
		t.Fatalf("role %s resolved to (%q, %q); want a refusal", role, n, e)
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("role %s: exit %d, want %d (refused): %v", role, ExitCodeOf(err), ExitRefused, err)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("role %s: refusal must name %q; got: %v", role, w, err)
		}
	}
}

// TestGLRoleEmailEachRoleOwn — with a per-role binding each GitLab role stamps its OWN
// address, even when the allowlist lists another role's address first.
// regression: #4978
func TestGLRoleEmailEachRoleOwn(t *testing.T) {
	glRoster(t, map[string]string{
		EnvGitLabSessionEmails: glWorkerSA + "," + glVerifierSA,
		EnvGitLabRoleEmails:    "verifier=" + glVerifierSA + ",worker=" + glWorkerSA,
	})
	for role, want := range map[string]string{"verifier": glVerifierSA, "worker": glWorkerSA} {
		_, got, err := RoleWorktreeCommitIdentity(role)
		if err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
		if got != want {
			t.Fatalf("role %s stamped %q, want its own address %q", role, got, want)
		}
	}
}

// TestGLRoleEmailAmbiguousRefuses — a multi-entry allowlist with no per-role binding
// names no address for the role; taking the first entry is the defect, so it refuses.
// regression: #4978
func TestGLRoleEmailAmbiguousRefuses(t *testing.T) {
	glRoster(t, map[string]string{EnvGitLabSessionEmails: glWorkerSA + "," + glVerifierSA})
	mustRefuse(t, "verifier", EnvGitLabRoleEmails, EnvGitLabSessionEmails)
}

// TestGLRoleEmailOneSAManyRoles — a single service-account address belongs to ONE account;
// with two GitLab accounts bound to roles it cannot be attributed to either, so it refuses.
// regression: #4978
func TestGLRoleEmailOneSAManyRoles(t *testing.T) {
	glRoster(t, map[string]string{EnvGitLabSessionEmails: glWorkerSA})
	mustRefuse(t, "verifier", EnvGitLabRoleEmails)
	mustRefuse(t, "worker", EnvGitLabRoleEmails)
}

// TestGLRoleEmailSessionUserShared — the two-identity model unchanged: a single ordinary
// session-user address is every role's commit author, by design.
func TestGLRoleEmailSessionUserShared(t *testing.T) {
	const user = "implementer@example.com"
	glRoster(t, map[string]string{EnvGitLabSessionEmails: user})
	for _, role := range []string{"verifier", "worker"} {
		if _, got, err := RoleGitLabCommitIdentity(role); err != nil || got != user {
			t.Fatalf("role %s = (%q, %v), want the session user %q", role, got, err, user)
		}
	}
}

// TestGLRoleEmailOneRoleOneSA — a deployment with ONE GitLab account and one service-account
// address keeps stamping it (the address can only be that account's).
func TestGLRoleEmailOneRoleOneSA(t *testing.T) {
	withRoster(t, forgeRoster("verifier=gitlab:example-verifier-sa:5001",
		map[string]string{EnvGitLabSessionEmails: glVerifierSA}))
	if _, got, err := RoleGitLabCommitIdentity("verifier"); err != nil || got != glVerifierSA {
		t.Fatalf("verifier = (%q, %v), want %q", got, err, glVerifierSA)
	}
}

// TestGLRoleEmailSharedAccount — two role NAMES bound to one account share its address,
// both through the allowlist fallback and through a per-role binding.
func TestGLRoleEmailSharedAccount(t *testing.T) {
	both := "intake-loop=gitlab:example-intake-sa:5003,issue-loop=gitlab:example-intake-sa:5003"
	withRoster(t, forgeRoster(both, map[string]string{EnvGitLabSessionEmails: glVerifierSA}))
	if _, got, err := RoleGitLabCommitIdentity("issue-loop"); err != nil || got != glVerifierSA {
		t.Fatalf("issue-loop = (%q, %v), want %q", got, err, glVerifierSA)
	}
	withRoster(t, forgeRoster(both, map[string]string{
		EnvGitLabRoleEmails: "intake-loop=" + glVerifierSA + ",issue-loop=" + glVerifierSA,
	}))
	if p := EffectiveConfig().GitLabRoleEmailsProblem; p != "" {
		t.Fatalf("one account's address on its two role names was rejected: %s", p)
	}
}

// TestGLRoleEmailInvalidFailsClosed — every broken binding disables the key (the roster
// stays configured) and every GitLab role then refuses, naming the key and the reason.
// regression: #4978
func TestGLRoleEmailInvalidFailsClosed(t *testing.T) {
	cases := []struct{ name, val, reason string }{
		{"malformed", "verifier" + glVerifierSA, "expected role=email"},
		{"not-email", "verifier=nobody", "not an email address"},
		{"github-role", "reviewer=" + glVerifierSA, "not bound to a GitLab identity"},
		{"unbound-role", "verifeir=" + glVerifierSA, "not bound to a GitLab identity"},
		{"unacceptable", "verifier=someone@example.com", "neither the GitLab service-account"},
		{"role-twice", "verifier=" + glVerifierSA + ",verifier=" + glWorkerSA, "bound more than once"},
		{"addr-two-roles", "verifier=" + glVerifierSA + ",worker=" + glVerifierSA, "bound to both role"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withRoster(t, forgeRoster(glTwoRoles+",reviewer=github:example-reviewer-app:3004",
				map[string]string{EnvGitLabSessionEmails: glVerifierSA, EnvGitLabRoleEmails: c.val}))
			cfg := EffectiveConfig()
			if !cfg.Configured() {
				t.Fatalf("a bad %s refused the whole roster: %v", EnvGitLabRoleEmails, cfg.Problems)
			}
			if !strings.Contains(cfg.GitLabRoleEmailsProblem, c.reason) {
				t.Fatalf("problem = %q, want it to contain %q", cfg.GitLabRoleEmailsProblem, c.reason)
			}
			if cfg.GitLabRoleEmails != nil {
				t.Fatalf("a rejected value was partly applied: %v", cfg.GitLabRoleEmails)
			}
			mustRefuse(t, "verifier", EnvGitLabRoleEmails, c.reason)
			mustRefuse(t, "worker", EnvGitLabRoleEmails)
			// A GitHub role is untouched by a GitLab-only key.
			if _, _, err := RoleWorktreeCommitIdentity("reviewer"); err != nil {
				t.Fatalf("GitHub role refused over a GitLab-only key: %v", err)
			}
		})
	}
}

// TestGLRoleEmailEcho — the effective-config echo shows the bindings sorted, or the reason
// the value was not applied.
func TestGLRoleEmailEcho(t *testing.T) {
	glRoster(t, map[string]string{
		EnvGitLabRoleEmails: "worker=" + glWorkerSA + ",verifier=" + glVerifierSA,
	})
	want := "verifier=" + glVerifierSA + ",worker=" + glWorkerSA
	if got := EffectiveConfig().gitLabRoleEmailsEcho(); got != want {
		t.Fatalf("echo = %q, want %q", got, want)
	}
	glRoster(t, map[string]string{EnvGitLabRoleEmails: "verifier=nobody"})
	if got := EffectiveConfig().gitLabRoleEmailsEcho(); !strings.HasPrefix(got, "(INVALID") {
		t.Fatalf("echo of an invalid value = %q, want an INVALID marker", got)
	}
}
