package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// regression: #5759
func TestVerifiedIsOutcomeOnly(t *testing.T) {
	root := t.TempDir()
	rows := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | b01 | 0 | S | implemented | — | — |\n" +
		"| 02 | b02 | 0 | S | verified | 2026-08-13 human:alex | — |\n"
	brief := planBrief("model", "no", "no", "no", "no")
	writeFixtureStream(t, root, "example-stream", rows, map[string]string{"01": brief, "02": brief})
	items, err := scanAwaiting(root, "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, it := range items {
		prompt := renderDispatchPrompt(it, loopengine.TierLocal)
		hasClause := strings.Contains(prompt, "--outcome-only") && strings.Contains(prompt, "open NO Evidence PR")
		switch it.ID {
		case "example-stream/01":
			seen[it.ID] = true
			if payloadValue(it, nonDecisiveKey) != "" || hasClause {
				t.Fatalf("an implemented brief is not outcome-only: payload %q", it.Payload[nonDecisiveKey])
			}
		case "example-stream/02":
			seen[it.ID] = true
			if payloadValue(it, nonDecisiveKey) != nonDecisiveOutcomeOnly || !hasClause {
				t.Fatalf("a verified brief's BLOCKED result must be outcome-only: payload %q, prompt:\n%s", it.Payload[nonDecisiveKey], prompt)
			}
			if !strings.Contains(prompt, "already verified") {
				t.Fatalf("prompt does not name the status:\n%s", prompt)
			}
		}
	}
	if !seen["example-stream/01"] || !seen["example-stream/02"] {
		t.Fatalf("scan did not list both briefs: %v", seen)
	}
}
