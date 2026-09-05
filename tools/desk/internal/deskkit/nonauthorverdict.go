package deskkit

// nonauthorverdict.go — the run-time non-author verdict assertion (sdlc/10).
//
// THE SEPARATION THIS BACKS UP. The load-bearing property the whole methodology
// rests on is that the identity that WRITES a change and the identity that
// CERTIFIES it (a review verdict, a merged-work verification) are not the same
// actor. On the full-Apps install path a first layer already enforces it: the
// implementer and the reviewer are different GitHub App identities, and the forge
// itself refuses to let a PR author approve their own PR (adopting-assay.md §1a).
//
// THE GAP THIS CLOSES. That first layer is a SINGLE control, and it fails silent:
// collapse the two identities and the board stays green while the property is
// gone (the sdlc/10 single-point-of-failure note). Worse, the forge's own refusal
// is keyed on PR AUTHORSHIP, so a supported path that reduces the identity set —
// the question sdlc/10's human gate is deciding — could put the poster and the
// author of the code being certified into one actor on a route the forge does not
// refuse. This assertion is the SECOND, independent layer: at verdict time, in the
// desk tool (a different component from the forge), for a different reason
// (identity equality against the certified head), it refuses to post a verdict
// whose poster is the same actor that authored the head commit.
//
// The two layers are independent by construction: the forge's refusal fires on a
// server-side authorship check at approve time; this fires on a client-side actor
// comparison at post time, and it fires on the collapsed paths where the forge's
// refusal may not. Removing this one and the collapsed path has no verdict-time
// protection at all — which is exactly what the sdlc/10 Verify mutation row pins.
//
// THREE-STATE, NOT TWO. Equality is a definite refusal. Inequality is a definite
// permit. An UNKNOWN head author (an empty login the caller could not read) is
// neither: SameActor already answers false for an empty operand, so this function
// PERMITS on an unreadable author rather than refusing every verdict on a transient
// read failure — but that "permit on unknown" is a could-not-check the CALLER must
// surface, never a silent pass. NonAuthorVerdictState makes the three states
// explicit for callers that need to warn on the unknown; AssertNonAuthorVerdict is
// the error-returning form for the common wiring, and its doc comment states the
// unknown behaviour so a caller cannot mistake a permit for a cleared check.

import "fmt"

// NonAuthorVerdictState is the three-state result of the non-author check.
type NonAuthorVerdictState int

const (
	// NonAuthorOK — the poster and the head-commit author are DIFFERENT actors: the
	// separation holds and the verdict may be posted.
	NonAuthorOK NonAuthorVerdictState = iota
	// NonAuthorRefused — the poster IS the head-commit author: posting is refused.
	NonAuthorRefused
	// NonAuthorUnknown — the head-commit author could not be determined (an empty
	// login). NOT a pass and NOT a refusal: the caller must decide, loudly. A caller
	// that treats this as OK has rounded a could-not-check up to a clear, which is the
	// exact failure the three-state instrument rule forbids.
	NonAuthorUnknown
)

// NonAuthorVerdict classifies a would-be verdict post by comparing the posting
// identity against the head commit's author, folding the two GitHub renderings of a
// single App identity through SameActor (so "app/<slug>" and "<slug>[bot]" of one
// App are recognised as one actor). An empty postingLogin is a caller bug — a post
// path always knows who it is posting as — so it is reported as Unknown rather than
// silently permitted.
func NonAuthorVerdict(postingLogin, headAuthorLogin string) NonAuthorVerdictState {
	if trimEmpty(postingLogin) || trimEmpty(headAuthorLogin) {
		return NonAuthorUnknown
	}
	if SameActor(postingLogin, headAuthorLogin) {
		return NonAuthorRefused
	}
	return NonAuthorOK
}

// AssertNonAuthorVerdict is the error-returning form of the non-author check for
// the verdict-posting wiring. It returns:
//
//   - a Refused error (exit 5) when postingLogin and headAuthorLogin name the SAME
//     actor — naming BOTH identities so the operator sees which collapse tripped it;
//   - nil when they are different actors (the verdict may be posted);
//   - nil when the head author is UNKNOWN (an empty login) — a transient read
//     failure must not brick the reviewer loop — BUT this is a could-not-check, not
//     a cleared check: a wired caller MUST warn on NonAuthorUnknown (call
//     NonAuthorVerdict to distinguish it) rather than read this nil as a pass.
//
// The Refused message states why the check exists (the second layer behind the
// forge's own author-cannot-approve refusal) so a reader who hits it on a collapsed
// path understands it is the designed control, not a bug.
func AssertNonAuthorVerdict(postingLogin, headAuthorLogin string) error {
	if NonAuthorVerdict(postingLogin, headAuthorLogin) == NonAuthorRefused {
		return Refused(fmt.Sprintf(
			"refused: the identity posting this verdict (%q) is the SAME actor that authored the head "+
				"commit (%q). A verdict must be posted by an identity distinct from the one that wrote the "+
				"code it certifies — that separation is the property the whole review gate rests on. On the "+
				"full-Apps path the forge's own \"an author cannot approve their own PR\" refusal is the first "+
				"layer; this is the SECOND layer, applied at verdict time in the desk tool, for a collapsed "+
				"identity path where the forge's refusal may not fire. If you meant to certify someone else's "+
				"work, post from the reviewer identity, not the implementer's.",
			postingLogin, headAuthorLogin))
	}
	return nil
}

// trimEmpty reports whether s is empty or only whitespace, without importing
// strings for one call site beyond what this file needs.
func trimEmpty(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
