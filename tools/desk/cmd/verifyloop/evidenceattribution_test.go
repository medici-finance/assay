package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"strings"
	"testing"
)

func TestEvidenceAttributionProducer(t *testing.T) {
	for _, verdict := range []string{loopengine.VerdictPass, loopengine.VerdictFail, loopengine.VerdictBlocked} {
		t.Run(string(verdict), func(t *testing.T) {
			evidence := renderEvidence(loopengine.Result{Verdict: verdict, RunnerID: "human:operator", Rows: []loopengine.EvidenceRow{{Command: "command | filter", Output: "could-not-run: closure refused"}}}, "2026-10-23")
			for _, want := range []string{"| Output | Date | Runner |", "| 2026-10-23 | human:operator |", "could-not-run: closure refused", `command \| filter`} {
				if !strings.Contains(evidence, want) {
					t.Fatalf("missing %q in %s", want, evidence)
				}
			}
		})
	}
	prompt := renderDispatchPrompt(loopengine.Item{ID: "x/1"}, loopengine.TierLocal)
	for _, want := range []string{"separate Date (YYYY-MM-DD) and Runner cells", "PASS, FAIL and BLOCKED", "including unrun rows and supplemental host observations", "never guess it or invent execution witnesses", "deskevidence --dry-run"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("missing %q in dispatch", want)
		}
	}
}
