package deskkit

import "testing"

// TestUnwrapNeverManufacturesAKeyword is the general property behind unwrapEmphasis
// (moved here from cmd/deskpost/internal/bodycheck by #408): unwrapping is
// only ever allowed to REMOVE punctuation that is wrapping something, never to fuse two
// alphanumerics together. If that ever breaks, a line that does not contain the marker's
// words in order could acquire them — including turning a `fail` into a `pass`.
func TestUnwrapNeverManufacturesAKeyword(t *testing.T) {
	cases := []string{
		"Security-Rev*iew: pass", "Security_Review: pass", "Security-Review: pa`ss",
		"Security-Review: pa*ss", "Security-Re**view: pass", "S*e*c*u*r*i*t*y-Review: pass",
		"Security-Review: f*ail", "Security-Review: p_ass",
	}
	for _, c := range cases {
		got := unwrapEmphasis(c)
		if secReviewPass.MatchString(got) || secReviewFail.MatchString(got) {
			t.Errorf("unwrapEmphasis(%q) = %q — that is a MANUFACTURED marker: emphasis removal "+
				"joined two alphanumerics", c, got)
		}
	}
}

// TestHasSecurityReviewPassFail is a compact end-to-end pass over the exported readers
// (#408) — the same shapes cmd/deskpost/internal/bodycheck's
// emphasis_test.go exercises through its own delegating wrappers, pinned here directly
// against the canonical implementation so deskboard and deskpost cannot drift again.
func TestHasSecurityReviewPassFail(t *testing.T) {
	cases := []struct {
		name             string
		body             string
		wantPass, wantFa bool
	}{
		{"canonical pass", "## Security review\n\nSecurity-Review: pass\n", true, false},
		{"canonical fail", "Security-Review: fail\n", false, true},
		{"bold pass", "**Security-Review: pass**", true, false},
		{"bold fail", "**Security-Review: fail**", false, true},
		{"case-varied key and value", "security-review: PASS", true, false},
		{"quoted (escape hatch)", "> Security-Review: pass", false, false},
		{"star bullet", "* Security-Review: pass", false, false},
		{"pass inside a fenced block (GRANT path skips)", "```\nSecurity-Review: pass\n```\n", false, false},
		{"fail inside a fenced block (BLOCK path reads)", "```\nSecurity-Review: fail\n```\n", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HasSecurityReviewPass(c.body); got != c.wantPass {
				t.Errorf("HasSecurityReviewPass(%q) = %v, want %v", c.body, got, c.wantPass)
			}
			if got := HasSecurityReviewFail(c.body); got != c.wantFa {
				t.Errorf("HasSecurityReviewFail(%q) = %v, want %v", c.body, got, c.wantFa)
			}
		})
	}
}
