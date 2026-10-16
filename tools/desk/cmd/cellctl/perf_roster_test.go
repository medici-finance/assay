package main

import (
	"slices"
	"testing"
)

// regression: #3099
func TestPerfVerbRosterCoversTree(t *testing.T) {
	t.Setenv("ASSAY_TELEMETRY", "1")
	roster := perfVerbs()
	for _, c := range buildRoot().Commands() {
		if !slices.Contains(roster, c.Name()) {
			t.Errorf("command %q is missing from the perf verb roster %v", c.Name(), roster)
		}
	}
	for _, a := range []string{"--version", "--help"} {
		if !slices.Contains(roster, a) {
			t.Errorf("%q missing from the perf verb roster", a)
		}
	}
	t.Setenv("ASSAY_TELEMETRY", "")
	if got := perfVerbs(); got != nil {
		t.Errorf("perf off still builds a roster: %v", got)
	}
}
