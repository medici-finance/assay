package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Existing foreign-marker fixtures proved refusal, but never observed the
// legacy coordinator establishing custody around its own mutating children.
// regression: #2513
func TestLegacyMarkerBeforeChild(t *testing.T) {
	work, _ := maintenanceFixture(t)
	marker := filepath.Join(work, ".git", "assay-maintenance-active.json")
	base := maintenanceCommand
	writes := 0
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "pack-refs") || strings.Contains(strings.Join(args, " "), "commit-graph write") {
			writes++
			if _, err := os.Stat(marker); err != nil {
				t.Errorf("mutating child starts without durable custody: %v", err)
			}
		}
		return base(ctx, name, args...)
	}
	if _, err := maintenanceRun(t, "--repo", work, "--apply"); err != nil {
		t.Fatal(err)
	}
	if writes != 2 {
		t.Fatalf("writes=%d, want two fixed index tasks", writes)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("confirmed cleanup retained marker: %v", err)
	}
}
