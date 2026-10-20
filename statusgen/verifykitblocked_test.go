package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// regression: #3461
func TestVflipKitBlockedSequence(t *testing.T) {
	// Read the actual dispatched kit, not a hand-written verdict fixture: the
	// writer's marker must both supersede and bound runs in the existing reader.
	raw, err := os.ReadFile(filepath.Join("..", "tools", "desk", "cmd", "deskdispatch", "references", "verifier-prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	marker := regexp.MustCompile(`\*\*(?:VERIFY: )?BLOCKED\*\*`).FindString(string(raw))
	if marker == "" {
		t.Fatal("verifier kit carries no blocked marker")
	}
	t.Run("blocks_stale_pass", func(t *testing.T) {
		o := vfDefaults()
		o.evidence += "\n\n" + marker + " — row 2 could-not-run: required tool unavailable."
		vfRefuses(t, o, "verdict mismatch")
	})
	t.Run("bounds_fresh_pass", func(t *testing.T) {
		o := vfDefaults()
		old := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab")
		o.evidence = old + "\n\n" + marker + " — row 2 could-not-run.\n\n" + o.evidence
		root, _ := vfFixture(t, o)
		if code, out, errOut := runVF(t, root, "--dry-run"); code != verifyflipExitOK {
			t.Fatalf("exit = %d, want 0; out=%q err=%q", code, out, errOut)
		}
	})
}
