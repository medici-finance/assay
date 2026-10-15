package main

import (
	"strings"
	"testing"
)

// A labelled check (state=checked-failed) exits 0, so a caller gating on the exit code reads
// it as clean unless the help says the state line is the result. The help must say so.
// regression: #2493
func TestUsageNamesStateLine(t *testing.T) {
	text := strings.Join(strings.Fields(usage), " ")
	for _, want := range []string{
		"the state= line is the result, never the exit code",
		"state=checked-failed",
		"still exits 0",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("usage does not say %q — a caller cannot tell a labelled check from a clean one", want)
		}
	}
}
