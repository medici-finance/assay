package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file exercises the BUILT cellctl binary for the ASSAY_COCKPIT export: `desk` resolves the
// cell's cockpit exactly as `up` does and exports the RESOLVED value (never `auto`) into the
// window, where the worker-desk skill's worktree-create step reads it. An explicit cockpit that is
// not available is refused, never exported as something else.
//
// Every run is DRY_RUN=1 on a house fixture (repairFixture) with a PRIVATE PATH — a stub dir plus
// the system directories only — so a cockpit installed on the host cannot decide a case, and
// nothing is fetched, launched or contacted (C3).

// cockpitPath returns a PATH holding a stub dir (with the named cockpit stubs) ahead of the system
// directories, and nothing else.
func cockpitPath(t *testing.T, stubs ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, s := range stubs {
		if err := os.WriteFile(filepath.Join(dir, s), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return "PATH=" + dir + ":/usr/bin:/bin"
}

func TestBinaryAssayCockpitAutoExportsResolvedValue(t *testing.T) {
	f := newRepairFixture(t)
	r := f.run(t, []string{"DRY_RUN=1", cockpitPath(t)}, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("dry-run desk: exit %d: %s%s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "[dry-run] env ASSAY_COCKPIT=tmux (fallback: no herdr/orca on PATH)") {
		t.Errorf("auto with no herdr/orca must export the RESOLVED tmux:\n%s", r.stdout)
	}
	if strings.Contains(r.stdout, "ASSAY_COCKPIT=auto") {
		t.Errorf("the literal auto must never reach the window:\n%s", r.stdout)
	}
}

func TestBinaryAssayCockpitExplicitPresent(t *testing.T) {
	f := newRepairFixture(t)
	f.writeCellEnv(t, "CELL_COCKPIT=herdr\n")
	r := f.run(t, []string{"DRY_RUN=1", cockpitPath(t, "herdr")}, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("dry-run desk: exit %d: %s%s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "[dry-run] env ASSAY_COCKPIT=herdr (explicit: cell.env CELL_COCKPIT)") {
		t.Errorf("cell.env CELL_COCKPIT=herdr with herdr on PATH must export herdr:\n%s", r.stdout)
	}
	// --cockpit beats cell.env for the run.
	r = f.run(t, []string{"DRY_RUN=1", cockpitPath(t, "herdr", "tmux")}, "desk", "example", "worker-desk", "--cockpit", "tmux")
	if r.code != 0 {
		t.Fatalf("dry-run desk --cockpit tmux: exit %d: %s%s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "[dry-run] env ASSAY_COCKPIT=tmux (explicit: --cockpit)") {
		t.Errorf("--cockpit tmux must override cell.env herdr:\n%s", r.stdout)
	}
}

func TestBinaryAssayCockpitExplicitAbsentRefused(t *testing.T) {
	f := newRepairFixture(t)
	f.writeCellEnv(t, "CELL_COCKPIT=herdr\n")
	r := f.run(t, []string{"DRY_RUN=1", cockpitPath(t)}, "desk", "example", "worker-desk")
	if r.code == 0 {
		t.Fatalf("an explicit cockpit not on PATH must be refused, got exit 0:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, "cockpit herdr") || !strings.Contains(r.stderr, "not on PATH") {
		t.Errorf("the refusal must name herdr and PATH: %s", r.stderr)
	}
	if strings.Contains(r.stdout, "ASSAY_COCKPIT=") {
		t.Errorf("nothing may be exported in place of the refused choice:\n%s", r.stdout)
	}
}

// TestRoleCmdThreadsResolvedCockpit: `up` hands every window the cockpit it resolved, after
// --provider and before the config dir, so the order stays stable to grep against.
func TestRoleCmdThreadsResolvedCockpit(t *testing.T) {
	c := &Cell{Name: "example"}
	got := c.roleCmd("worker-desk", "/cfg", upOverrides{Provider: "glm", Cockpit: "herdr"})
	if !strings.HasSuffix(got, " desk 'example' 'worker-desk' --provider 'glm' --cockpit 'herdr' '/cfg'") {
		t.Errorf("roleCmd = %q, want the --cockpit pair after --provider and before the config dir", got)
	}
	if got := c.roleCmd("worker-desk", "/cfg", upOverrides{}); strings.Contains(got, "--cockpit") {
		t.Errorf("roleCmd with no resolved cockpit must carry no --cockpit: %q", got)
	}
}
