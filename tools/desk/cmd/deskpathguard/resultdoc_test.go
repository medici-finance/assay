package main

import (
	"strings"
	"testing"
)

// A labelled check (state=checked-failed) exits 0 once its label landed, so a caller gating on
// the exit code reads it as clean unless the help says the state line is the result — and a
// caller reading ONLY the state line believes the label is on the PR unless the help says a
// failed label write exits non-zero with no applied: line. The help must say both.
// regression: #2493
func TestUsageNamesStateLine(t *testing.T) {
	text := strings.Join(strings.Fields(usage), " ")
	for _, want := range []string{
		"the state= line is the result, never the exit code alone",
		"state=checked-failed",
		"exits 0 under --dry-run",
		"label write that fails after the state line exits 5 (refused) or 6 (could-not-check)",
		`prints no "applied:" line`,
		"the label is NOT on the PR",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("usage does not say %q — a caller cannot tell a labelled check from a clean one, or a landed label from a failed write", want)
		}
	}
	if strings.Contains(text, "Only state=could-not-check exits non-zero") {
		t.Error("usage still claims only could-not-check exits non-zero — a failed label write does too")
	}
}
