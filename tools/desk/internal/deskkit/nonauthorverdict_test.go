package deskkit

import "testing"

// TestNonAuthorVerdict pins the run-time non-author verdict assertion (sdlc/10 Verify
// rows 2-5). It is the layer-independence proof the brief's SPOF note calls for: the
// POSITIVE path (poster == head author) must REFUSE and name both identities, the
// NEGATIVE path (poster != head author) must PERMIT, and the MUTATION row — deleting
// the identity comparison from AssertNonAuthorVerdict — must redden the positive path
// while the negative path stays green. A check that refuses in BOTH directions has
// replaced a control with an outage; a check that permits in both is no control at all.
func TestNonAuthorVerdict(t *testing.T) {
	// POSITIVE PATH (row 3): the posting identity equals the head commit's author.
	// The two GitHub renderings of one App identity must fold to one actor, so the
	// collapse is caught whichever rendering each side carries.
	t.Run("positive/refused_when_same_actor", func(t *testing.T) {
		cases := []struct{ posting, head string }{
			{"assay-implementer-app[bot]", "assay-implementer-app[bot]"},
			// gh-CLI rendering on one side, REST rendering on the other — SameActor folds them.
			{"app/assay-implementer-app", "assay-implementer-app[bot]"},
			{"assay-implementer-app[bot]", "app/assay-implementer-app"},
			// case-insensitive, per NormalizeActorLogin.
			{"Assay-Implementer-App[bot]", "assay-implementer-app[bot]"},
		}
		for _, c := range cases {
			if got := NonAuthorVerdict(c.posting, c.head); got != NonAuthorRefused {
				t.Fatalf("NonAuthorVerdict(%q,%q) = %v, want NonAuthorRefused", c.posting, c.head, got)
			}
			err := AssertNonAuthorVerdict(c.posting, c.head)
			if err == nil {
				t.Fatalf("AssertNonAuthorVerdict(%q,%q) = nil, want a Refused error", c.posting, c.head)
			}
			if !IsRefused(err) {
				t.Fatalf("AssertNonAuthorVerdict(%q,%q) error is not Refused (exit 5): %v", c.posting, c.head, err)
			}
			// The message must NAME BOTH identities (row 3), so the operator sees which
			// collapse tripped it. Both raw operands appear in the refusal.
			msg := err.Error()
			if !contains(msg, c.posting) || !contains(msg, c.head) {
				t.Fatalf("refusal message must name both identities; got %q (want %q and %q)", msg, c.posting, c.head)
			}
		}
	})

	// NEGATIVE PATH (row 4, layer independence): a DIFFERENT posting identity is
	// PERMITTED. If this refuses too, the control has become an outage.
	t.Run("negative/permitted_when_different_actor", func(t *testing.T) {
		cases := []struct{ posting, head string }{
			{"assay-reviewer-app[bot]", "assay-implementer-app[bot]"},
			{"app/assay-reviewer-app", "assay-implementer-app[bot]"},
			{"assay-verifier-app[bot]", "app/assay-implementer-app"},
			// A human implementer and an App reviewer are different actors too.
			{"assay-reviewer-app[bot]", "some-human-login"},
		}
		for _, c := range cases {
			if got := NonAuthorVerdict(c.posting, c.head); got != NonAuthorOK {
				t.Fatalf("NonAuthorVerdict(%q,%q) = %v, want NonAuthorOK", c.posting, c.head, got)
			}
			if err := AssertNonAuthorVerdict(c.posting, c.head); err != nil {
				t.Fatalf("AssertNonAuthorVerdict(%q,%q) = %v, want nil (permit)", c.posting, c.head, err)
			}
		}
	})

	// UNKNOWN (three-state): an empty head author is could-not-check, not a pass and
	// not a refusal. Assert PERMITS (a transient read failure must not brick the loop)
	// but the state is Unknown so a wired caller can warn — the nil from Assert here is
	// explicitly NOT a cleared check.
	t.Run("unknown/empty_head_author_is_could_not_check", func(t *testing.T) {
		for _, head := range []string{"", "   ", "\t\n"} {
			if got := NonAuthorVerdict("assay-reviewer-app[bot]", head); got != NonAuthorUnknown {
				t.Fatalf("NonAuthorVerdict(reviewer, %q) = %v, want NonAuthorUnknown", head, got)
			}
			if err := AssertNonAuthorVerdict("assay-reviewer-app[bot]", head); err != nil {
				t.Fatalf("AssertNonAuthorVerdict(reviewer, %q) = %v, want nil (permit-with-warning)", head, err)
			}
		}
		// An empty POSTER is a caller bug, reported as Unknown, never a silent permit-clean.
		if got := NonAuthorVerdict("", "assay-implementer-app[bot]"); got != NonAuthorUnknown {
			t.Fatalf("NonAuthorVerdict(empty poster) = %v, want NonAuthorUnknown", got)
		}
	})
}
