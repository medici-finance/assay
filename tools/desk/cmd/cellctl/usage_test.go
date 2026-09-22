package main

import (
	"os"
	"strings"
	"testing"
)

// TestUsageMatchesOracle is the anti-drift proof for the one piece of text the port has to keep
// a COPY of. The shell oracle's `usage()` prints its own header comment:
//
//	awk 'NR>1 && /^set -euo/ {exit} NR>1 {print}' "$SELF"
//
// A compiled binary cannot read its source, so usage.txt is embedded — and an embedded copy is a
// second source of truth unless something proves it equal. This re-derives the header from the
// oracle and requires the two to match exactly, so a help-text edit that lands on only one side
// is a red test rather than a silent divergence the parity matrix does not cover (`--help` is
// not one of its verbs).
func TestUsageMatchesOracle(t *testing.T) {
	const oracle = "../../../cellctl/cellctl"
	raw, err := os.ReadFile(oracle)
	if err != nil {
		t.Skipf("oracle not readable from this checkout (%v) — the parity harness covers the rest", err)
	}
	lines := strings.Split(string(raw), "\n")
	var header []string
	for i, l := range lines {
		if i == 0 {
			continue
		}
		if strings.HasPrefix(l, "set -euo") {
			break
		}
		header = append(header, l)
	}
	want := strings.Join(header, "\n") + "\n"
	if usageText != want {
		t.Errorf("embedded usage.txt has drifted from %s's header.\n"+
			"Regenerate it from the oracle (the header above its `set -euo` line) in the same PR "+
			"that changes the oracle.\nembedded %d bytes, oracle header %d bytes",
			oracle, len(usageText), len(want))
	}
}

func TestUsageIsNotEmpty(t *testing.T) {
	if !strings.Contains(usageText, "cellctl ls") {
		t.Error("embedded usage text does not describe the verbs")
	}
}
