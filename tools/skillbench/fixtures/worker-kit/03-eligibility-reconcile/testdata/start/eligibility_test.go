package eligibility

import "testing"

func TestIneligibleWhenMerged(t *testing.T) {
	if IsEligible("merged") {
		t.Fatal("a merged PR must end eligibility")
	}
}

func TestIneligibleWhenClosed(t *testing.T) {
	if IsEligible("closed") {
		t.Fatal("a closed PR must end eligibility")
	}
}

func TestEligibleWhenOpen(t *testing.T) {
	if !IsEligible("open") {
		t.Fatal("an open PR keeps eligibility")
	}
}

func TestEligibleWhenDraft(t *testing.T) {
	if !IsEligible("draft") {
		t.Fatal("a draft PR keeps eligibility")
	}
}
