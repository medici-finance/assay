package deskkit

import (
	"strings"
	"testing"
)

// TestRoleWorktreeCommitIdentityResolvesGitHubRoles proves the shared resolver returns each
// GitHub desk role's bot commit identity — the #638 bot-USER-id noreply address — under the
// fixture roster (installed by TestMain). This is the ONE resolver deskwt role-init, deskwt
// add --role and deskdispatch's worktree-create all stamp from, so a drift here is a drift in
// every worktree's attributed identity.
func TestRoleWorktreeCommitIdentityResolvesGitHubRoles(t *testing.T) {
	cases := []struct{ role, name, email string }{
		{"verifier", "assay-verifier-app[bot]", "300000005+assay-verifier-app[bot]@users.noreply.github.com"},
		{"worker", "assay-worker-app[bot]", "300000006+assay-worker-app[bot]@users.noreply.github.com"},
		{"reviewer", "assay-reviewer-app[bot]", "300000004+assay-reviewer-app[bot]@users.noreply.github.com"},
	}
	for _, c := range cases {
		name, email, err := RoleWorktreeCommitIdentity(c.role)
		if err != nil {
			t.Fatalf("role %q: unexpected error: %v", c.role, err)
		}
		if name != c.name || email != c.email {
			t.Fatalf("role %q resolved to (%q, %q), want (%q, %q)", c.role, name, email, c.name, c.email)
		}
	}
}

// TestRoleIdentityLabelRendersSlugAndBotUserID pins the OK-line provenance token the stamping
// tools print.
func TestRoleIdentityLabelRendersSlugAndBotUserID(t *testing.T) {
	if got := RoleIdentityLabel("verifier"); got != "assay-verifier-app 300000005" {
		t.Fatalf("RoleIdentityLabel(verifier) = %q, want %q", got, "assay-verifier-app 300000005")
	}
}

// TestRoleWorktreeCommitIdentityRefusesUnboundRole is the fail-closed branch: a role the
// roster does not bind to a bot commit identity yields a typed Refused (exit 5) naming the
// roster key, so a caller that must stamp an identity refuses rather than inheriting or
// stamping an account-unlinked one. FAIL-FIRST: a resolver that fell back to an empty or a
// guessed identity would return nil here and the assertion would flip.
func TestRoleWorktreeCommitIdentityRefusesUnboundRole(t *testing.T) {
	// A roster that binds worker but NOT verifier.
	withRoster(t, map[string]string{
		EnvTrustedBotSlugs:    "worker=assay-worker-app:300000006",
		"ASSAY_ALLOWED_REPOS": "medici-finance/assay:ci:private",
	})
	name, email, err := RoleWorktreeCommitIdentity("verifier")
	if err == nil {
		t.Fatalf("unbound verifier resolved to (%q, %q) with no error; want a refusal", name, email)
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("unbound-role error exit code = %d, want %d (refused)", ExitCodeOf(err), ExitRefused)
	}
	if !strings.Contains(err.Error(), EnvTrustedBotSlugs) {
		t.Fatalf("refusal must name the roster key %q to pin the identity; got: %v", EnvTrustedBotSlugs, err)
	}
}
