package evaluation

import "testing"

// regression: #6002
func TestOutcomeProtocolMatrix(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		want       VerificationOutcome
		held, wake bool
	}{
		{"verified", OutcomeVerified, false, false},
		{" Verified ", OutcomeVerified, false, false},
		{"verify-fail", OutcomeFailed, false, true},
		{"FAIL", OutcomeFailed, false, true},
		{"blocked", OutcomeBlocked, true, true},
		{"needs_context", OutcomeNeedsContext, true, true},
		{"needs-context", OutcomeNeedsContext, true, true},
		{"pass", OutcomeUnknown, false, false},
		{"could-not-run", OutcomeUnknown, false, false},
		{"", OutcomeUnknown, false, false},
		{"future-value", OutcomeUnknown, false, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got := ParseVerificationOutcome(tc.raw)
			if got != tc.want || got.Held() != tc.held || got.NeedsWake() != tc.wake {
				t.Errorf("outcome = %q held=%v wake=%v; want %q held=%v wake=%v",
					got, got.Held(), got.NeedsWake(), tc.want, tc.held, tc.wake)
			}
		})
	}
}
