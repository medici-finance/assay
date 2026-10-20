package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3467
func TestAcceptanceChildPermission(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$TRANSITION_ARGV_LOG\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "statusgen"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TRANSITION_ARGV_LOG", log)
	old := acceptanceForge
	t.Cleanup(func() { acceptanceForge = old })
	calls := []struct {
		name string
		run  func()
	}{
		{"closure", func() { verifiedClosureCheckAt(dir, "vf/01") }},
		{"outcome", func() { checkVerifiedClosure(dir, "vf/01") }},
		{"lint", func() { statusgenLintAt(dir) }},
		{"flip", func() { statusgenVerifyFlip(dir, "vf/01", "abc", "verifier", true) }},
	}
	for _, enabled := range []bool{false, true} {
		for _, c := range calls {
			t.Run(c.name+map[bool]string{false: "-offline", true: "-opted-in"}[enabled], func(t *testing.T) {
				acceptanceForge = enabled
				if err := os.WriteFile(log, nil, 0600); err != nil {
					t.Fatal(err)
				}
				c.run()
				raw, err := os.ReadFile(log)
				if err != nil {
					t.Fatal(err)
				}
				got := strings.Count(string(raw), "--acceptance-forge\n")
				want := 0
				if enabled {
					want = 1
				}
				if got != want {
					t.Fatalf("child argv %q: opt-in count=%d want=%d", raw, got, want)
				}
			})
		}
	}
}
