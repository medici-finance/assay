package deskkit

import "testing"

// The verify-gate card carve-out (#868). `github-actions[bot]` is NOT a trusted
// author and must stay untrusted for every general-purpose predicate — the carve-out
// is a separate, narrower read that admits ONE verb (`comment`) on ONE shape of issue
// (label `verify-gate`). These tests pin both halves: what it admits, and the far
// larger set it does not.

const verifyGateFixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,intake-loop=example-intake-loop-app:300000002,issue-loop=example-issue-loop-app:300000003,reviewer=example-reviewer-app:300000004,verifier=example-verifier-app:300000005,worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`

func TestVerifyGateCardCommentAdmitted(t *testing.T) {
	plantRoster(t, verifyGateFixtureRoster)

	cases := []struct {
		name   string
		login  string
		labels []string
		want   bool
	}{
		// The one admitted shape: the forge's own Actions identity, on an issue
		// carrying the verify-gate label.
		{"card", "github-actions[bot]", []string{"verify-gate"}, true},
		{"card among other labels", "github-actions[bot]", []string{"gate:human", "verify-gate", "bug"}, true},
		// GitHub logins and label names are case-insensitively unique.
		{"card case-insensitive login", "GitHub-Actions[bot]", []string{"verify-gate"}, true},
		{"card case-insensitive label", "github-actions[bot]", []string{"Verify-Gate"}, true},

		// Label-scoped: the SAME author without the label stays refused. This is the
		// whole reason the carve-out is a label read and not an author read — an
		// Actions workflow can file anything, and only the sign-off card's body is
		// generated from the tree.
		{"no label", "github-actions[bot]", nil, false},
		{"wrong label", "github-actions[bot]", []string{"gate:human"}, false},
		{"label substring is not the label", "github-actions[bot]", []string{"verify-gate-close"}, false},

		// Author-scoped: the label admits nothing on its own. Any other author with
		// the label is still whatever the general trust gate says it is.
		{"other bot with the label", "dependabot[bot]", []string{"verify-gate"}, false},
		{"external user with the label", "external-user", []string{"verify-gate"}, false},
		{"trusted human with the label", "ada", []string{"verify-gate"}, false},
		{"empty login", "", []string{"verify-gate"}, false},

		// Spoofing-shaped renderings: only the exact `[bot]` form GitHub itself
		// produces counts. The bare slug is a registrable username namespace.
		{"bare slug", "github-actions", []string{"verify-gate"}, false},
		{"gh CLI rendering", "app/github-actions", []string{"verify-gate"}, false},
		{"lookalike", "github-actions[bot]x", []string{"verify-gate"}, false},
		{"lookalike prefix", "xgithub-actions[bot]", []string{"verify-gate"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := VerifyGateCardCommentAdmitted(c.login, c.labels); got != c.want {
				t.Fatalf("VerifyGateCardCommentAdmitted(%q, %v) = %v, want %v",
					c.login, c.labels, got, c.want)
			}
		})
	}
}

// TestVerifyGateCardCommentAdmittedUnconfiguredRosterRefuses — fail closed in the
// same direction every sibling predicate in trust.go does. An unconfigured roster
// means no deployment has told these tools anything; a carve-out that answered true
// there would be the one trust read that widens itself on a missing config.
func TestVerifyGateCardCommentAdmittedUnconfiguredRosterRefuses(t *testing.T) {
	plantRoster(t, "")
	if VerifyGateCardCommentAdmitted("github-actions[bot]", []string{"verify-gate"}) {
		t.Fatal("an unconfigured roster must admit nothing")
	}
}

// TestVerifyGateCardAuthorStaysUntrustedGenerally — the carve-out must not leak into
// the general predicates. `github-actions[bot]` is untrusted for TrustedAuthor,
// TrustedAuthorID, TrustedPublicAuthor and TrustedHumanAuthor, before and after this
// change; nothing about carrying a verify-gate label can reach them (they take no
// labels), and this pins that the author itself was not quietly added to the roster.
func TestVerifyGateCardAuthorStaysUntrustedGenerally(t *testing.T) {
	plantRoster(t, verifyGateFixtureRoster)
	const login = "github-actions[bot]"
	if TrustedAuthor(login) {
		t.Fatal("TrustedAuthor must stay false for the Actions identity")
	}
	if TrustedAuthorID(login, 41898282) {
		t.Fatal("TrustedAuthorID must stay false for the Actions identity")
	}
	if TrustedPublicAuthor(login) {
		t.Fatal("TrustedPublicAuthor must stay false for the Actions identity")
	}
	if TrustedHumanAuthor(login) {
		t.Fatal("TrustedHumanAuthor must stay false for the Actions identity")
	}
}
