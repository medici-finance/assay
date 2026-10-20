package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2972
func TestPlanPartialClaims(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	plantFixtureRoster(t, home)
	dir := filepath.Join(home, ".config", "assay", "claims")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.claim"), []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err := cmdPlan([]string{"--root", t.TempDir(), "--scan-target", "medici-finance/assay", "--state-dir", armedStateDir(t), "--inbound", writeTemp(t, "poll.txt", planPoll), "--now", "2026-08-24T12:00:00Z"}, &out)
	if !deskkit.IsUnverifiable(err) || !strings.Contains(err.Error(), "bad.claim") {
		t.Fatalf("partial claims gave a successful plan: %v", err)
	}
	if !strings.Contains(out.String(), "unknown (") {
		t.Fatalf("claim row was shown as free: %q", out.String())
	}
}
