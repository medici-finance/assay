package deskkit

// securitybodycr_test.go — the security lane's marked body-only clearance (#5092). The
// accepting fixture clears; every refusal starts from it and breaks one clause of the ruling.

import (
	"strconv"
	"strings"
	"testing"
)

const (
	scrHead       = "aaaaaaaabbbbbbbbccccccccdddddddd11111111"
	scrOtherHead  = "1111111122222222333333334444444455555555"
	scrBodyBefore = "Summary: the change enforces the egress allowlist."
	scrBodyAfter  = "Summary: the change documents the egress allowlist; enforcement is a follow-up."
	scrFinding    = "f-body-1"
	scrFailAt     = "2026-01-01T00:00:00Z"
	scrEditedAt   = "2026-01-01T00:10:00Z"
	scrPassAt     = "2026-01-01T00:20:00Z"
)

func scrFailBody(digestOf string) string {
	return "## Security review\n\nThe body claims a control the diff does not add.\n\nSecurity-Review: fail\n" +
		"Blocked-On-Body: " + scrFinding + " " + PRBodyDigest(digestOf)
}

func scrPass(body, commit, at string) SecurityVerdictReview {
	return SecurityVerdictReview{Body: body, CommitID: commit, State: "COMMENTED", SubmittedAt: at}
}

func scrPassBody(live string) string {
	return "## Security review\n\nRe-read the live body.\n\nSecurity-Review: pass\nBody-Reread-Digest: " + PRBodyDigest(live)
}

func scrInput() SecurityBodyInput {
	return SecurityBodyInput{
		FailBody:        scrFailBody(scrBodyBefore),
		FailSubmittedAt: scrFailAt,
		Head:            scrHead,
		Later:           []SecurityVerdictReview{scrPass(scrPassBody(scrBodyAfter), scrHead, scrPassAt)},
		LiveBody:        scrBodyAfter,
		BodyEditedAt:    scrEditedAt,
	}
}

func wantSCRRefused(t *testing.T, in SecurityBodyInput, clause string) {
	t.Helper()
	dec := EvaluateSecurityBodyClearance(in)
	if dec.Cleared {
		t.Fatalf("cleared, want a refusal naming %q (reason: %s)", clause, dec.Reason)
	}
	if !strings.Contains(dec.Reason, clause) {
		t.Fatalf("reason = %q, want it to name %q", dec.Reason, clause)
	}
}

// regression: #5092
func TestSecBodyClears(t *testing.T) {
	dec := EvaluateSecurityBodyClearance(scrInput())
	if !dec.Cleared || !dec.Declared {
		t.Fatalf("marked fail, edited body, later pass re-reading it: cleared=%v declared=%v (%s)", dec.Cleared, dec.Declared, dec.Reason)
	}
}

// regression: #5092
func TestSecBodyUnmarkedFail(t *testing.T) {
	in := scrInput()
	in.FailBody = "## Security review\n\nSecurity-Review: fail"
	wantSCRRefused(t, in, "no `Blocked-On-Body:` line")
}

// regression: #5092
func TestSecBodyNotASecurityFail(t *testing.T) {
	in := scrInput()
	in.FailBody = "Verdict: request-changes\nBlocked-On-Body: " + scrFinding + " " + PRBodyDigest(scrBodyBefore)
	wantSCRRefused(t, in, "no `Security-Review: fail` line")
}

// regression: #5092
func TestSecBodyNoLaterPass(t *testing.T) {
	in := scrInput()
	in.Later = nil
	wantSCRRefused(t, in, "no `Security-Review: pass`")
}

// regression: #5092
func TestSecBodyPassNotAtHead(t *testing.T) {
	in := scrInput()
	in.Later = []SecurityVerdictReview{scrPass(scrPassBody(scrBodyAfter), scrOtherHead, scrPassAt)}
	wantSCRRefused(t, in, "no `Security-Review: pass`")
}

// regression: #5092
func TestSecBodyPassBeforeFail(t *testing.T) {
	in := scrInput()
	in.Later = []SecurityVerdictReview{scrPass(scrPassBody(scrBodyAfter), scrHead, "2025-12-31T23:00:00Z")}
	wantSCRRefused(t, in, "no `Security-Review: pass`")
}

// regression: #5092
func TestSecBodyDigestMismatch(t *testing.T) {
	in := scrInput()
	in.LiveBody = scrBodyAfter + "\n\nEdited again after the re-read."
	wantSCRRefused(t, in, "not the digest of the live PR body")
	in = scrInput()
	in.Later = []SecurityVerdictReview{scrPass("## Security review\n\nSecurity-Review: pass", scrHead, scrPassAt)}
	wantSCRRefused(t, in, "does not document a live-API re-read")
}

// regression: #5092
func TestSecBodyNoEditAfterFail(t *testing.T) {
	in := scrInput()
	in.BodyEditedAt = ""
	wantSCRRefused(t, in, "reports no edit")
	in.BodyEditedAt = "2025-12-31T23:00:00Z"
	wantSCRRefused(t, in, "not after the security fail")
	in.BodyEditedAt = "yesterday"
	wantSCRRefused(t, in, "not readable as RFC3339")
}

// regression: #5092
func TestSecBodyLaterUnmarkedFail(t *testing.T) {
	unmarked := scrPass("## Security review\n\nSecurity-Review: fail", scrHead, "2026-01-01T00:05:00Z")
	in := scrInput()
	in.Later = []SecurityVerdictReview{unmarked, scrPass(scrPassBody(scrBodyAfter), scrHead, scrPassAt)}
	wantSCRRefused(t, in, "later unmarked security fail blocks")
}

// regression: #5092
func TestSecBodyFailAfterPass(t *testing.T) {
	in := scrInput()
	in.Later = append(in.Later, scrPass(scrFailBody(scrBodyAfter), scrHead, "2026-01-01T00:30:00Z"))
	wantSCRRefused(t, in, "no `Security-Review: pass`")
	in = scrInput()
	in.Later = append(in.Later, scrPass("Security-Review: fail", scrOtherHead, "2026-01-01T00:30:00Z"))
	wantSCRRefused(t, in, "not at head")
}

// regression: #5092
func TestSecBodyMarkedChain(t *testing.T) {
	second := scrPass(scrFailBody(scrBodyBefore+" (second read)"), scrHead, "2026-01-01T00:05:00Z")
	in := scrInput()
	in.Later = []SecurityVerdictReview{second, scrPass(scrPassBody(scrBodyAfter), scrHead, scrPassAt)}
	if dec := EvaluateSecurityBodyClearance(in); !dec.Cleared {
		t.Fatalf("two marked fails then a pass, body edited after both: %s", dec.Reason)
	}
	in.BodyEditedAt = "2026-01-01T00:03:00Z" // after the first fail, before the second
	wantSCRRefused(t, in, "not after the security fail")
}

// regression: #5092
func TestSecBodyRereadEqualsFail(t *testing.T) {
	in := scrInput()
	in.FailBody = scrFailBody(scrBodyAfter)
	wantSCRRefused(t, in, "equals the digest a security fail recorded")
}

// scrForgeStates is every review state the forge reports, plus the empty state.
var scrForgeStates = []string{"APPROVED", "COMMENTED", "CHANGES_REQUESTED", "DISMISSED", "PENDING", ""}

// The clearing pass counts only in a review state that carries a standing pass: APPROVED or
// COMMENTED, the list `deskpost ready` reads for the same grant. Every other state refuses
// with the clearance's own reason, naming the state.
//
// regression: #5092
func TestSecBodyPassStates(t *testing.T) {
	for _, st := range scrForgeStates {
		t.Run("state="+st, func(t *testing.T) {
			in := scrInput()
			in.Later[0].State = st
			if SecurityPassStateStands(st) {
				if dec := EvaluateSecurityBodyClearance(in); !dec.Cleared {
					t.Fatalf("a pass in state %q did not clear: %s", st, dec.Reason)
				}
				return
			}
			wantSCRRefused(t, in, "review state "+strconv.Quote(st)+" does not stand")
		})
	}
}

// The allow-list itself, exactly. `deskpost ready`, deskflip and deskboard all read it, so a
// change here changes every surface at once and must be deliberate.
//
// regression: #5092
func TestSecBodyPassStateList(t *testing.T) {
	want := map[string]bool{"APPROVED": true, "COMMENTED": true}
	for _, st := range append(scrForgeStates, "approved", " COMMENTED", "Commented") {
		if got := SecurityPassStateStands(st); got != want[st] {
			t.Errorf("SecurityPassStateStands(%q) = %v, want %v", st, got, want[st])
		}
	}
}

// A standing pass is not undone by a later pass in a state that does not stand.
//
// regression: #5092
func TestSecBodyLaterDismissedPass(t *testing.T) {
	in := scrInput()
	late := scrPass(scrPassBody(scrBodyAfter), scrHead, "2026-01-01T00:30:00Z")
	late.State = "DISMISSED"
	in.Later = append(in.Later, late)
	if dec := EvaluateSecurityBodyClearance(in); !dec.Cleared {
		t.Fatalf("a standing pass followed by a dismissed one: %s", dec.Reason)
	}
}
