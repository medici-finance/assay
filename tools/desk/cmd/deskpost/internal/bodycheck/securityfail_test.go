package bodycheck

import "testing"

// TestHasSecurityReviewFail pins the retraction parser (#216). Before it
// existed the ready gate could not see a withdrawn security pass at all.
func TestHasSecurityReviewFail(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"## Security review\n\nBlocker.\n\nSecurity-Review: fail\n", true},
		{"security-review:   FAIL", true},         // key case-insensitive, value token case-insensitive
		{"  \tSecurity-Review: fail  \t\r", true}, // leading/trailing whitespace and CRLF
		{"Security-Review: pass", false},
		{"", false},
		{"Security-Review: failed", false},                       // not the exact token
		{"the review did not Security-Review: fail here", false}, // must be the whole line
		{"## Review\n\nVerdict: request-changes\n", false},       // a correctness verdict is not a security one
	}
	for _, c := range cases {
		if got := HasSecurityReviewFail(c.body); got != c.want {
			t.Errorf("HasSecurityReviewFail(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}

// TestSecurityPassAndFailAreIndependent — a body carrying both lines reports BOTH, so
// callers can detect the ambiguity and fail closed rather than reading it as a pass.
func TestSecurityPassAndFailAreIndependent(t *testing.T) {
	both := "Security-Review: pass\nSecurity-Review: fail\n"
	if !HasSecurityReviewPass(both) || !HasSecurityReviewFail(both) {
		t.Fatalf("both markers must be reported: pass=%v fail=%v",
			HasSecurityReviewPass(both), HasSecurityReviewFail(both))
	}
}
