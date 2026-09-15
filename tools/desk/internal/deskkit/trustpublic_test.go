package deskkit

import "testing"

// #808 (ruling recorded 2026-09-06): the public-repo trust bar is
// retired. `TrustedAuthor` is now the ONE bar on every repo, private or public — the
// same predicate `deskpost`'s trustGate applies. This file used to pin the narrower
// public-only bar (#943 — a role App or mapped human only, on its own retired
// predicate); that predicate is gone and these tests pin its replacement: a login in the configured
// trusted-logins set is a trusted author on ANY repo, an unlisted login is not, and
// an unconfigured roster trusts nobody.
func TestTrustedAuthorIsOneTrustBarOnAnyRepo(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001,shared-agent:2099,mapped-maintainer:2050",
		EnvTrustedBotSlugs: "desk=assay-desk-app:300000001,worker=assay-worker-app:300000006",
		EnvHumanLoginMap:   "Maintainer:mapped-maintainer", // the accountable mapped human — a DIFFERENT axis
	})

	cases := []struct {
		login string
		want  bool
		why   string
	}{
		{"assay-desk-app[bot]", true, "role App, [bot] rendering"},
		{"app/assay-worker-app", true, "role App, app/ rendering"},
		{"ASSAY-DESK-APP[bot]", true, "App match is case-insensitive"},
		{"mapped-maintainer", true, "in the trusted-logins set (the map alone is a different axis, see below)"},
		{"MAPPED-MAINTAINER", true, "same, case-insensitive"},
		{"shared-agent", true, "a configured trusted login is reviewable on ANY repo now — the #943 narrowing this brief removes"},
		{"ada", true, "the bless authority is also a plain configured trusted login"},
		{"assay-desk-app", false, "bare slug is never trusted (squatting fail-close)"},
		{"randofork", false, "an unlisted login is not trusted on any repo"},
		{"", false, "empty login"},
	}
	for _, tc := range cases {
		if got := TrustedAuthor(tc.login); got != tc.want {
			t.Errorf("TrustedAuthor(%q) = %v, want %v — %s", tc.login, got, tc.want, tc.why)
		}
	}

	// ASSAY_HUMAN_LOGIN_MAP is a DIFFERENT axis (TrustedHumanAuthor's accountable-human
	// set) and never widens TrustedAuthor on its own: a login present ONLY in the map,
	// never in ASSAY_TRUSTED_LOGINS or ASSAY_TRUSTED_BOT_SLUGS, is not in the
	// trusted-logins set and TrustedAuthor must refuse it. This is the boundary the
	// retired public-only predicate blurred (it read the map directly) and
	// TrustedAuthor does not.
	if TrustedAuthor("only-in-human-map") {
		t.Fatal(`TrustedAuthor("only-in-human-map") = true — a login present ONLY in ` +
			"ASSAY_HUMAN_LOGIN_MAP, not in the trusted-logins set, must stay untrusted")
	}
}

// TestTrustedAuthorUnconfiguredFailsClosed — Verify row 5 (brief 17): an
// unconfigured roster trusts nobody, on any repo. This fail-closed property is the
// one this brief must not disturb while widening who counts as trusted when the
// roster IS configured.
func TestTrustedAuthorUnconfiguredFailsClosed(t *testing.T) {
	withNoRoster(t)
	for _, login := range []string{"assay-desk-app[bot]", "app/assay-desk-app", "mapped-maintainer", "shared-agent", "ada", ""} {
		if TrustedAuthor(login) {
			t.Errorf("TrustedAuthor(%q) = true with NO roster — unset must trust nobody", login)
		}
	}
}
