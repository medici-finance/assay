package main

import (
	"strings"
	"testing"
)

// regression: #3461
func TestVerifierOutcomeContract(t *testing.T) {
	prompt := dispatchPrompt(t, "verifier", "verify-item-9")
	kit, err := kitText("verifier")
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"assignment": assignmentSection(t, prompt), "embedded kit": kit} {
		t.Run(name, func(t *testing.T) {
			for _, marker := range []string{"**VERIFY: PASS**", "**VERIFY: FAIL**", "**BLOCKED**"} {
				if !strings.Contains(text, marker) {
					t.Errorf("%s missing outcome %s", name, marker)
				}
			}
		})
	}
	for _, obligation := range []string{"Preflight the exact", "Zero rows", "could-not-run", "no obligation failed", "never an implementation bug"} {
		if !strings.Contains(kit, obligation) {
			t.Errorf("embedded verifier kit missing %q", obligation)
		}
	}
}
