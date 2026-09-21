package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file exercises the BUILT cellctl binary for the ASSAY_REPAIR_ADMISSION rollout wiring:
// the opt-in that lets a cell durably turn the dispatch-boundary repair-admission gate ON via
// its cell.env, instead of only through a one-off shell `export` in an interactive session.
//
// Every fixture cell is a "house" kind with NO model policy: DRY_RUN=1 never fetches, never
// opens a real worktree and never contacts a forge or a model endpoint, so no live
// infrastructure is touched (C3). The gate itself lives in deskdispatch and enables ONLY on the
// literal value "on" (deskkit.RepairAdmissionEnabled), so composing exactly that string is what
// these tests assert cellctl now does.

// repairFixture is a minimal house cell — complete enough for set/show/desk DRY_RUN=1 but never
// for a real (non-dry-run) launch, which needs a real git checkout and worktree creation.
type repairFixture struct {
	cellsRoot string
	cellDir   string
	cfgDir    string
	repoDir   string
}

func newRepairFixture(t *testing.T) *repairFixture {
	t.Helper()
	root := t.TempDir()
	f := &repairFixture{
		cellsRoot: filepath.Join(root, "cells"),
		cfgDir:    filepath.Join(root, "claude-config"),
		repoDir:   filepath.Join(root, "repo"),
	}
	f.cellDir = filepath.Join(f.cellsRoot, "example")
	for _, d := range []string{
		filepath.Join(f.cellDir, "home", ".config", "assay"),
		f.cfgDir, f.repoDir,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.cellDir, "home", ".config", "assay", "roster.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f.writeCellEnv(t, "")
	return f
}

// writeCellEnv rewrites the fixture cell.env: a house cell with the given extra lines appended.
func (f *repairFixture) writeCellEnv(t *testing.T, extra string) {
	t.Helper()
	env := "CELL=example\nCELL_KIND=house\n" +
		"CELL_ROOTS=example-org/example-repo=" + f.repoDir + "\n" +
		"CELL_REPO=" + f.repoDir + "\n" + extra
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *repairFixture) run(t *testing.T, extraEnv []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(cellctlBinary(t), args...)
	base := []string{
		"CELLS_ROOT=" + f.cellsRoot,
		"CLAUDE_CONFIG_DIR=" + f.cfgDir,
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + f.cellDir,
		"KUBECONFIG=/dev/null",
	}
	cmd.Env = append(base, extraEnv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running cellctl %v: %v", args, err)
		}
	}
	return runResult{out.String(), errb.String(), code}
}

// TestBinaryRepairAdmissionSetShowDryRunRoundTrip is the whole point of the wiring: the key can
// be set into cell.env WITHOUT --force, `show` reflects it, and a DRY_RUN=1 desk boot shows it
// composed into the launch — proving a worker-desk booted from this cell would carry
// ASSAY_REPAIR_ADMISSION=on in its environment, where the deskdispatch gate reads it.
func TestBinaryRepairAdmissionSetShowDryRunRoundTrip(t *testing.T) {
	f := newRepairFixture(t)
	if r := f.run(t, nil, "set", "example", "ASSAY_REPAIR_ADMISSION=on"); r.code != 0 {
		t.Fatalf("set ASSAY_REPAIR_ADMISSION=on should be accepted without --force, got exit %d: %s", r.code, r.stderr)
	}
	rs := f.run(t, nil, "show", "example")
	if rs.code != 0 {
		t.Fatalf("show: exit %d: %s", rs.code, rs.stderr)
	}
	if !strings.Contains(rs.stdout, "ASSAY_REPAIR_ADMISSION=on") {
		t.Errorf("show does not reflect ASSAY_REPAIR_ADMISSION=on:\n%s", rs.stdout)
	}
	rd := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
	if rd.code != 0 {
		t.Fatalf("dry-run desk worker-desk: exit %d: %s%s", rd.code, rd.stdout, rd.stderr)
	}
	if !strings.Contains(rd.stdout, "ASSAY_REPAIR_ADMISSION=on") {
		t.Errorf("dry-run does not show ASSAY_REPAIR_ADMISSION composed into the launch:\n%s", rd.stdout)
	}
}

// TestBinaryRepairAdmissionOffAndUnsetAbsentFromComposedEnv proves off and unset behave
// identically — the key is simply ABSENT from the composed launch, exactly as the deskdispatch
// consumer treats "unset" and any non-"on" value as the default-off state.
func TestBinaryRepairAdmissionOffAndUnsetAbsentFromComposedEnv(t *testing.T) {
	f := newRepairFixture(t)
	// Unset: never composed.
	rd := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
	if rd.code != 0 {
		t.Fatalf("dry-run (unset): exit %d: %s%s", rd.code, rd.stdout, rd.stderr)
	}
	if strings.Contains(rd.stdout, "ASSAY_REPAIR_ADMISSION") {
		t.Errorf("an unset key must not be composed into the launch:\n%s", rd.stdout)
	}
	// Off: identical to unset from the launch's perspective — absent, never an explicit
	// ASSAY_REPAIR_ADMISSION=off.
	f.writeCellEnv(t, "ASSAY_REPAIR_ADMISSION=off\n")
	rd2 := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
	if rd2.code != 0 {
		t.Fatalf("dry-run (off): exit %d: %s%s", rd2.code, rd2.stdout, rd2.stderr)
	}
	if strings.Contains(rd2.stdout, "ASSAY_REPAIR_ADMISSION") {
		t.Errorf("an off value must not be composed into the launch:\n%s", rd2.stdout)
	}
}

// TestBinaryRepairAdmissionInvalidValueRefused: only the literal on/off are accepted, and the
// value rule is NON-bypassable — --force widens the unknown-KEY allowlist, never the value rule
// for a known key (the CELL_HARNESS/CELL_KIND/CELL_COCKPIT precedent).
func TestBinaryRepairAdmissionInvalidValueRefused(t *testing.T) {
	f := newRepairFixture(t)
	if r := f.run(t, nil, "set", "example", "ASSAY_REPAIR_ADMISSION=maybe"); r.code == 0 {
		t.Fatalf("an invalid ASSAY_REPAIR_ADMISSION value must be refused, got exit 0: %s", r.stdout)
	}
	r := f.run(t, nil, "set", "example", "ASSAY_REPAIR_ADMISSION=maybe", "--force")
	if r.code == 0 {
		t.Fatalf("--force must NOT bypass the on/off value rule for a known key, got exit 0: %s", r.stdout)
	}
	if !strings.Contains(r.stderr, "on") || !strings.Contains(r.stderr, "off") {
		t.Errorf("refusal should name the accepted values (on/off): %s", r.stderr)
	}
}
