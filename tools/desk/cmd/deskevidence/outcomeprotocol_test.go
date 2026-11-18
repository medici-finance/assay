package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6002
func TestOutcomeRecordRefusesUnrecognisedOverallResult(t *testing.T) {
	for _, value := range []string{"pass", "new-outcome", ""} {
		t.Run(value, func(t *testing.T) {
			f, admissions := joinFake(t)
			code, message := jland(t, jlegacy(t, map[string]any{"outcome": value}))
			wantRefused(t, value, code, message, "outcome", "verified", "verify-fail", "blocked", "needs-context")
			wantNothingWritten(t, value, f)
			if *admissions != 0 {
				t.Errorf("admission calls = %d, want zero for unknown outcome", *admissions)
			}
		})
	}
}

// The queue already reads case and surrounding whitespace compatibly.
func TestVerifiedSpellingRunsClosureGate(t *testing.T) {
	f, _ := setupFake(t)
	calls := 0
	verifiedClosureCheckFn = func(_, _ string, _ childOptions) (closureVerdict, string, error) {
		calls++
		return closureNotAccepted, "missing closure", nil
	}
	code, message := jland(t, jlegacy(t, map[string]any{"outcome": " Verified "}))
	if code != deskkit.ExitRefused || calls != 1 {
		t.Errorf("exit=%d closure checks=%d, want refusal after one check: %s", code, calls, message)
	}
	wantNothingWritten(t, "verified", f)
}
