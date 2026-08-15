package deskkit

import "testing"

// #943: on a PUBLIC repo the review-desk auto-reviews a PR only if its author is
// a role App (ASSAY_TRUSTED_BOT_SLUGS) or a mapped, accountable human
// (ASSAY_HUMAN_LOGIN_MAP). A shared machine / CI account (here `shared-agent`)
// admitted as a human via ASSAY_TRUSTED_LOGINS — so TrustedAuthor accepts it —
// must NOT clear the public bar, and neither must a fork author. The load-bearing
// case is the pair (shared-agent: TrustedAuthor true, TrustedPublicAuthor false):
// a naive gate that reused TrustedAuthor on public repos would let the shared
// account through, which is exactly the hole #943 closes.
func TestTrustedPublicAuthor(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001,shared-agent:2099", // shared-agent: shared acct, trusted-login ONLY
		EnvTrustedBotSlugs: "desk=assay-desk-app:300000001,worker=assay-worker-app:300000006",
		EnvHumanLoginMap:   "Maintainer:mapped-maintainer", // the accountable mapped human
	})

	cases := []struct {
		login string
		want  bool
		why   string
	}{
		{"assay-desk-app[bot]", true, "role App, [bot] rendering"},
		{"app/assay-worker-app", true, "role App, app/ rendering"},
		{"ASSAY-DESK-APP[bot]", true, "App match is case-insensitive"},
		{"mapped-maintainer", true, "mapped human (ASSAY_HUMAN_LOGIN_MAP)"},
		{"MAPPED-MAINTAINER", true, "mapped human, case-insensitive"},
		{"shared-agent", false, "shared machine account — trusted-login only, NOT a role App or mapped human"},
		{"assay-desk-app", false, "bare slug is never trusted (squatting fail-close)"},
		{"randofork", false, "arbitrary fork author"},
		{"", false, "empty login"},
	}
	for _, tc := range cases {
		if got := TrustedPublicAuthor(tc.login); got != tc.want {
			t.Errorf("TrustedPublicAuthor(%q) = %v, want %v — %s", tc.login, got, tc.want, tc.why)
		}
	}

	// The distinction is real, not vacuous: TrustedAuthor DOES admit shared-agent
	// (it is a configured trusted login), so the two predicates genuinely diverge
	// on the shared account #943 is about. If this ever fails, shared-agent dropped
	// out of the trusted-login set and the pairing above no longer proves anything.
	if !TrustedAuthor("shared-agent") {
		t.Fatal("precondition: TrustedAuthor(shared-agent) must be true here, else the " +
			"TrustedPublicAuthor(shared-agent)=false case proves nothing about the narrowing")
	}
}

// Fail-closed: an unconfigured roster trusts nobody for public review either.
func TestTrustedPublicAuthorUnconfiguredFailsClosed(t *testing.T) {
	withNoRoster(t)
	for _, login := range []string{"assay-desk-app[bot]", "app/assay-desk-app", "mapped-maintainer", "shared-agent", ""} {
		if TrustedPublicAuthor(login) {
			t.Errorf("TrustedPublicAuthor(%q) = true with NO roster — unset must trust nobody", login)
		}
	}
}
