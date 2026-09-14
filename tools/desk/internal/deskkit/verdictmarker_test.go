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

// TestCorrectnessNoteState pins the correctness verdict-note reducer (#798) — the read half
// that lets a GitLab `Verdict: approve|request-changes` note carry the review STATE the
// reviewer-approved gate reads. It mirrors the security pass/fail asymmetry about fences:
// the GRANT (approve) skips a fenced marker, the BLOCK (request-changes) still reads one, and
// a body carrying both reduces to the block.
func TestCorrectnessNoteState(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"canonical approve", "## Review\n\nVerdict: approve\n", "APPROVED"},
		{"canonical request-changes", "## Review\n\nVerdict: request-changes\n", "CHANGES_REQUESTED"},
		{"uppercase value", "Verdict: APPROVE", "APPROVED"},
		{"case-varied key", "verdict: request-changes", "CHANGES_REQUESTED"},
		{"bold approve", "**Verdict: approve**", "APPROVED"},
		{"bold request-changes", "**Verdict: request-changes**", "CHANGES_REQUESTED"},
		{"both present reduces to the block", "Verdict: approve\nVerdict: request-changes\n", "CHANGES_REQUESTED"},
		{"approve in a fenced block is not a grant", "```\nVerdict: approve\n```\n", ""},
		{"request-changes in a fenced block still blocks", "```\nVerdict: request-changes\n```\n", "CHANGES_REQUESTED"},
		{"quoted approve (escape hatch) is not a grant", "> Verdict: approve", ""},
		{"security line is not a correctness verdict", "Security-Review: pass\n", ""},
		{"plain comment", "looks fine to me\n", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CorrectnessNoteState(c.body); got != c.want {
				t.Errorf("CorrectnessNoteState(%q) = %q, want %q", c.body, got, c.want)
			}
		})
	}
}
