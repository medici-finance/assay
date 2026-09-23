package deskkit

// roleworktreeidentity.go — the ONE resolver behind every tool that stamps a git commit
// identity onto a desk worktree.
//
// Three tools must answer the same question — "which committer identity does role R's
// worktree carry?" — and they must never answer it three ways:
//
//   - `deskwt role-init <role>` provisions a role window's own worktree;
//   - `deskwt add --role <role>` stamps a hand-created or dispatched worktree;
//   - `deskdispatch`'s worktree-create step stamps the DISPATCHED agent's worktree.
//
// Before this, the GitHub-vs-GitLab branch and its two fail-closed refusals lived INLINE in
// `cmdRoleInit`, so `deskwt add` set nothing and `deskdispatch` set nothing: a dispatched
// worktree simply INHERITED whatever `user.name`/`user.email` the shared checkout's config
// carried. A verifier dispatched from a desk checkout then committed — and reported its
// runner — under the desk App's identity, and `statusgen verifyrun` stamped that wrong
// identity into every Evidence witness Runner cell: silent misattribution. Extracting the
// resolver here makes the three tools resolve one role to one identity by construction.
//
// The GitHub identity is DERIVED (slug + bot USER id → the #638 noreply address). The GitLab
// identity is READ from the trusted session/implementer allowlist (#643) — a service
// account's noreply address embeds a group id and per-account suffix a roster entry does not
// carry (forgeidentity.go), so it is validated, never constructed, and NEVER allowed to fall
// back to the GitHub shape (the #677 bug). A role with no derivable identity is a typed
// Refused (exit 5) naming the role and the roster key to fix, so every caller that must
// stamp an identity refuses loudly rather than inheriting an unrelated one.

import (
	"fmt"
	"strings"
)

// RoleWorktreeCommitIdentity resolves the git commit identity (name, email) role R's
// worktree must be stamped with, choosing the GitHub or GitLab shape from the role's
// forge-qualified roster entry. The returned error is a typed Refused (exit 5): a caller
// that cannot resolve an identity must never proceed to stamp — or, worse, inherit — one.
//
// The role is accepted in the TOKEN-role vocabulary the roster binds (desk / worker /
// reviewer / verifier / issue-loop / intake-loop); a caller holding a loop name or a kit
// name folds it to its token role first (deskwt's resolveRoleKey, deskdispatch's kitRole).
func RoleWorktreeCommitIdentity(role string) (name, email string, err error) {
	r := strings.ToLower(strings.TrimSpace(role))
	// The forge is read from the role's own entry so a GitLab role never takes the GitHub
	// branch (and never the reverse). An unbound role has no entry; both arms below then
	// return the fail-closed refusal their resolver already computed.
	if ident, bound := EffectiveConfig().RoleBotIdentity(r); bound && ident.Forge == ForgeGitLab {
		n, e, ok := RoleGitLabCommitIdentity(r)
		if !ok {
			return "", "", Refused("refused: role " + r + " is a GitLab identity (" + ident.Slug + "); its " +
				"service-account commit email (service_account_group_<group-id>_<suffix>@noreply.<host>) embeds a " +
				"group id and per-account suffix the roster does not carry, so it cannot be constructed — and it " +
				"must NOT fall back to the GitHub noreply shape. Configure the trusted GitLab session / implementer " +
				"commit address in " + EnvGitLabSessionEmails + " (the two-identity mechanism; to commit AS " +
				"the service account, list its provisioned noreply address there), in " + ConfigHomePath())
		}
		return n, e, nil
	}
	n, e, ok := RoleBotCommitIdentity(r)
	if !ok {
		return "", "", Refused("refused: role " + r + " has no bot commit identity in the roster — " +
			"pin it with a " + EnvTrustedBotSlugs + " entry " + r +
			"=<app-slug>:<bot-user-id> (the bot USER id, from `gh api /users/<app-slug>[bot]`) in " +
			ConfigHomePath())
	}
	return n, e, nil
}

// RoleIdentityLabel renders the `<slug> <bot-user-id>` provenance token a stamped-identity
// OK line prints — the human-readable half of what RoleWorktreeCommitIdentity stamps, so an
// operator reading a dispatch transcript can see WHICH identity a worktree was given without
// decoding the noreply email. It is display only; the id is already in the roster the
// operator controls, so printing it leaks nothing a config read would not.
//
// An unbound role yields "" — a caller reaches this only after
// RoleWorktreeCommitIdentity has already succeeded, so the empty case is defensive.
func RoleIdentityLabel(role string) string {
	ident, ok := EffectiveConfig().RoleBotIdentity(strings.ToLower(strings.TrimSpace(role)))
	if !ok {
		return ""
	}
	if ident.ID == 0 {
		return ident.Slug
	}
	return fmt.Sprintf("%s %d", ident.Slug, ident.ID)
}
