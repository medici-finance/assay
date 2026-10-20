package reviewflow

import (
	"strings"
	"testing"
)

// TestSecurityReviewNotesNameTheMarker pins the security-pass marker's spelling at its
// owner and in both SECURITY-REVIEW-REQUIRED notes. The notes tell a reviewer which line
// to post, so a drifted constant (or a literal that diverged from it) would send them to
// write a line the board never accepts.
//
// regression: #3570
func TestSecurityReviewNotesNameTheMarker(t *testing.T) {
	const want = "Security-Review: pass"
	if SecurityPassMarker != want {
		t.Fatalf("SecurityPassMarker = %q, want %q", SecurityPassMarker, want)
	}
	arms := map[string]ClassifyInput{
		// approved at head, nothing failed or pending, no green: the CI-zero arm.
		"ci-zero arm": {Ever: true, AtHead: true, ApprovedAtHead: true, RiskClassed: true},
		// approved at head with CI green: the MERGE-NOW arm.
		"merge-now arm": {Ever: true, AtHead: true, ApprovedAtHead: true, CiGreen: true, Pass: 1, RiskClassed: true},
	}
	for name, in := range arms {
		action, note := Classify(in, "reviewer-app")
		if action != ActSecReview {
			t.Errorf("%s: action = %q, want %q (note: %s)", name, action, ActSecReview, note)
			continue
		}
		if !strings.Contains(note, "'"+want+"' from reviewer-app") {
			t.Errorf("%s: note does not name %q as the required line:\n%s", name, want, note)
		}
	}
}
