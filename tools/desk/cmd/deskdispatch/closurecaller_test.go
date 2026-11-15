package main

import (
	"strings"
	"testing"
)

// regression: #4823
func TestOutcomeAuthorityRecipes(t *testing.T) {
	prompt := dispatchPrompt(t, "verifier", "verify-item-9")
	commands := 0
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "deskevidence ") || !strings.Contains(line, "--outcome-record ") {
			continue
		}
		commands++
		permitted := false
		for _, arg := range strings.Fields(line) {
			permitted = permitted || arg == "--acceptance-forge"
		}
		if !permitted {
			t.Errorf("outcome caller omits fresh approval permission: %s", line)
		}
	}
	if commands == 0 {
		t.Fatal("no executable outcome caller was emitted")
	}
}
