package deskkit

// roleapp_forge_test.go — the FORGE dimension of the role→login rendering.
//
// The bug these tests pin: RoleAppLogin rendered `<slug>[bot]` unconditionally — the
// GitHub App rendering — for every role on every forge. A GitLab service account is
// attributed by its BARE username (BotIdentity.AcceptedLogins; there is no decorated
// form on that forge), so the expected login never matched the actual one, and because
// SameActor carries an isApp flag, `SameActor("gl-act", "gl-act[bot]")` is false even
// though the slug is identical. Every surface keyed on that comparison — the ready-flip
// gate's reviewer-approved lane and the board's UNREVIEWED/NEEDS-REVIEW classification
// alike — therefore read "no verdict" for an approval that was really there.
//
// The renderings themselves already existed (AcceptedLogins) and the trusted-login set
// already registered the bare GitLab username (rosterconfig.go) and already resolved its
// pinned id (expectedID). Only the ROLE rendering bypassed that layer. So these tests
// assert BOTH directions: the GitLab rendering is the bare username, AND the GitHub
// rendering is untouched — including the property that a GitHub App's BARE slug is still
// not the role's login, which is the username-squatting fail-close.

import "testing"

const (
	glReviewerRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=gitlab:gl-reviewer:41987965,worker=x-act:1
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`
)

// TestRoleAppLoginRendersPerForge is the direct unit measurement: one roster, two roles,
// two forges, two renderings. The worker arm is the no-regression half — a GitHub entry
// must keep rendering exactly `<slug>[bot]`.
func TestRoleAppLoginRendersPerForge(t *testing.T) {
	plantRoster(t, glReviewerRoster)

	got, ok := RoleAppLogin("reviewer")
	if !ok {
		t.Fatal("RoleAppLogin(reviewer) returned ok=false for a roster that binds the role — " +
			"a bound GitLab role must resolve to a login, not refuse")
	}
	if got != "gl-reviewer" {
		t.Fatalf("RoleAppLogin(reviewer) = %q, want %q — a GitLab service account is attributed "+
			"by its BARE username; the [bot] suffix is the GitHub App rendering and never appears "+
			"on a GitLab review author", got, "gl-reviewer")
	}

	got, ok = RoleAppLogin("worker")
	if !ok || got != "x-act[bot]" {
		t.Fatalf("RoleAppLogin(worker) = (%q,%v), want (x-act[bot],true) — the GitHub rendering "+
			"must be byte-identical to before this fix", got, ok)
	}
}

// TestSameActorMatchesGitLabReviewerLogin is the symptom the issue reports, at the exact
// comparison that produced it: the actual review author login as GitLab attributes it,
// against the expected login the role resolves to.
func TestSameActorMatchesGitLabReviewerLogin(t *testing.T) {
	plantRoster(t, glReviewerRoster)

	want, ok := RoleAppLogin("reviewer")
	if !ok {
		t.Fatal("reviewer role did not resolve to a login")
	}
	// The author string is what a GitLab approval carries: the bare username.
	if !SameActor("gl-reviewer", want) {
		t.Fatalf("SameActor(%q, %q) = false — the GitLab reviewer's own approval did not match "+
			"the login its role resolves to, which is precisely the comparison that made every "+
			"GitLab approval invisible to the flip gate and the board", "gl-reviewer", want)
	}
}

// TestReduceAppVerdictApprovesGitLabReviewAtHead exercises the whole reduction the flip
// gate and the board's classification both run — not just the string compare — so the
// test fails for the reported REASON (a real APPROVED at head reduced to "none") rather
// than merely on a rendering.
func TestReduceAppVerdictApprovesGitLabReviewAtHead(t *testing.T) {
	plantRoster(t, glReviewerRoster)

	want, ok := RoleAppLogin("reviewer")
	if !ok {
		t.Fatal("reviewer role did not resolve to a login")
	}
	const head = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"
	reviews := []AppReview{
		{AuthorLogin: "gl-reviewer", State: "APPROVED", CommitID: head, SubmittedAt: "2026-09-10T10:00:00Z"},
	}
	if v := ReduceAppVerdict(want, reviews, head); v != AppVerdictApproved {
		t.Fatalf("ReduceAppVerdict = %q, want %q — an at-head APPROVED by the rostered GitLab "+
			"reviewer must reduce to approved; reducing it to none is what refused the ready-flip "+
			"and kept the board's UNREVIEWED alarm firing forever", v, AppVerdictApproved)
	}
}

// TestGitHubBareSlugIsNotTheRoleLogin is the fail-close guard that keeps this fix from
// being a widening. On GitHub the App slug and a user login are different namespaces, so
// a plain user named after the slug must never satisfy the role comparison. The GitLab
// arm is safe from the same shape for a different reason: a GitLab username is unique
// instance-wide, so the bare username IS the account — which is why it is the accepted
// rendering there and the bare slug is not one here.
func TestGitHubBareSlugIsNotTheRoleLogin(t *testing.T) {
	plantRoster(t, glReviewerRoster)

	want, ok := RoleAppLogin("worker")
	if !ok {
		t.Fatal("worker role did not resolve to a login")
	}
	if SameActor("x-act", want) {
		t.Fatal("SameActor(\"x-act\", worker-login) = true — a GitHub App's BARE slug matched the " +
			"role login. A user may be named after an App slug on GitHub, so accepting the bare " +
			"form would let a stranger's review count as the desk's App")
	}
}

// TestUnboundRoleStillRefusesOnEitherForge keeps RoleAppLogin's ok-contract intact through
// the forge change: an unbound role resolves to NO login, so no caller can turn it into an
// empty-identity comparison (the shape RoleAppLogin's doc comment exists to prevent).
func TestUnboundRoleStillRefusesOnEitherForge(t *testing.T) {
	plantRoster(t, glReviewerRoster)

	if login, ok := RoleAppLogin("verifier"); ok || login != "" {
		t.Fatalf("RoleAppLogin(verifier) = (%q,%v), want (\"\",false) — the roster binds no "+
			"verifier, and an unbound role must refuse rather than render an identity", login, ok)
	}
}
