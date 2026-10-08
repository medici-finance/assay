package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Interface lists as `--network none` presents them (issue #2350): `lo` up and a
// DOWN `tunl0` that is always there. The precheck must pass on exactly this.
func netnoneIfaces() ([]netIface, error) {
	return []netIface{
		{Name: "lo", Up: true, Loopback: true},
		{Name: "tunl0", Up: false},
	}, nil
}

func bridgedIfaces() ([]netIface, error) {
	return []netIface{
		{Name: "lo", Up: true, Loopback: true},
		{Name: "eth0", Up: true},
	}, nil
}

func stubIfaces(t *testing.T, fn func() ([]netIface, error)) {
	t.Helper()
	orig := listInterfacesFn
	listInterfacesFn = fn
	t.Cleanup(func() { listInterfacesFn = orig })
}

// TestSandboxPrecheckLoopback: loopback-only passes, a down tunl0 included.
func TestSandboxPrecheckLoopback(t *testing.T) {
	if ok, why := loopbackOnlyPrecheck(netnoneIfaces); !ok {
		t.Fatalf("loopback-only (lo up, tunl0 down) refused: %s", why)
	}
	// No interface up at all is also no route off the box.
	none := func() ([]netIface, error) { return []netIface{{Name: "lo", Loopback: true}}, nil }
	if ok, why := loopbackOnlyPrecheck(none); !ok {
		t.Fatalf("nothing up refused: %s", why)
	}
}

// TestSandboxPrecheckExtraIface: any non-loopback interface up refuses, naming
// it; an unreadable list refuses too (could-not-check is never a pass).
func TestSandboxPrecheckExtraIface(t *testing.T) {
	ok, why := loopbackOnlyPrecheck(bridgedIfaces)
	if ok {
		t.Fatal("eth0 up passed the loopback-only precheck")
	}
	if !strings.Contains(why, "eth0") {
		t.Errorf("refusal %q does not name the up interface eth0", why)
	}
	broken := func() ([]netIface, error) { return nil, errors.New("planted read failure") }
	if ok, _ := loopbackOnlyPrecheck(broken); ok {
		t.Fatal("an unreadable interface list passed the precheck")
	}
}

// TestSandboxRunRefusedNoExec: in container-netns mode a refused precheck runs
// NO row and records every one could-not-run, with the mode in the witness.
func TestSandboxRunRefusedNoExec(t *testing.T) {
	stubIfaces(t, bridgedIfaces)
	root := t.TempDir()
	marker := filepath.Join(root, "ran")
	rows := []verifyRow{
		{ID: "1", Command: "touch " + marker, Expect: "exit 0", Class: classCheckCI, Classed: true},
		{ID: "2", Command: "touch " + marker, Expect: "exit 0"},
	}
	ws := runWitnessesSandboxed(resolveShellPlan(), sandboxContainerNetns, root, rows, "human:tester", "", "0000", "2026-10-08", 30*time.Second, false)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a row executed although the loopback-only precheck refused the run")
	}
	for _, w := range ws {
		if w.State != stateCouldNotRun {
			t.Errorf("row %s: state %q, want %q", w.ID, w.State, stateCouldNotRun)
		}
		if !strings.Contains(w.Note, "eth0") {
			t.Errorf("row %s: note %q does not carry the precheck reason", w.ID, w.Note)
		}
		if got := witnessSandboxOf(w.row()); got != sandboxContainerNetns {
			t.Errorf("row %s: witness sandbox %q, want %q (row %s)", w.ID, got, sandboxContainerNetns, w.row())
		}
	}
}

// TestSandboxWitnessRecordsMode: the witness Result cell records which sandbox
// ran, readable back through witnessSandboxOf, in both modes; an unsandboxed
// row records none.
func TestSandboxWitnessRecordsMode(t *testing.T) {
	stubIfaces(t, netnoneIfaces)
	ci := verifyRow{ID: "1", Command: "true", Expect: "exit 0", Class: classCheckCI, Classed: true}
	ws := runWitnessesSandboxed(resolveShellPlan(), sandboxContainerNetns, t.TempDir(), []verifyRow{ci}, "human:tester", "", "0000", "2026-10-08", 30*time.Second, false)
	// The container mode runs the check:ci row WITHOUT the unshare wrapper, so it
	// passes on every host.
	if ws[0].State != statePass {
		t.Fatalf("container-netns check:ci row: state %q (%s), want pass", ws[0].State, ws[0].Note)
	}
	row := ws[0].row()
	if !strings.Contains(row, "pass exit=0 sandbox=container-netns") {
		t.Errorf("witness row %q does not record the container-netns mode", row)
	}
	if got := witnessSandboxOf(row); got != sandboxContainerNetns {
		t.Errorf("witnessSandboxOf = %q, want %q", got, sandboxContainerNetns)
	}
	if witnessStateOf(row) != statePass {
		t.Errorf("witnessStateOf(%q) = %q, want pass — the token must not disturb the state", row, witnessStateOf(row))
	}

	// The default mode records unshare on a check:ci row, whatever this host's
	// sandbox does: planted unavailable, the row is could-not-run, still stamped.
	defer func(orig func() ([]string, bool, string)) { networkOffWrapperFn = orig }(networkOffWrapperFn)
	networkOffWrapperFn = func() ([]string, bool, string) { return nil, false, "planted: no sandbox" }
	plain := verifyRow{ID: "2", Command: "true", Expect: "exit 0"}
	ws = runWitnessesSandboxed(resolveShellPlan(), sandboxUnshare, t.TempDir(), []verifyRow{ci, plain}, "human:tester", "", "0000", "2026-10-08", 30*time.Second, false)
	if got := witnessSandboxOf(ws[0].row()); got != sandboxUnshare {
		t.Errorf("unshare-mode check:ci witness sandbox %q, want %q (row %s)", got, sandboxUnshare, ws[0].row())
	}
	if got := witnessSandboxOf(ws[1].row()); got != "" {
		t.Errorf("an unsandboxed row recorded sandbox %q (row %s)", got, ws[1].row())
	}
	if strings.Contains(ws[1].row(), "sandbox=") {
		t.Errorf("an unsandboxed row changed format: %s", ws[1].row())
	}
}

// TestSandboxCILaneRefuses: the container mode is refused in the CI lane — by
// --ci and by GITHUB_ACTIONS=true — through the real sub-command entry point.
func TestSandboxCILaneRefuses(t *testing.T) {
	brief := filepath.Join(t.TempDir(), "missing-brief.md")

	t.Setenv("GITHUB_ACTIONS", "")
	res, stderr := captureVerifyrun(t, []string{"--sandbox=" + sandboxContainerNetns, "--ci", "--brief", brief})
	if res.code != verifyrunExitUsageError || !strings.Contains(stderr, "in the CI lane") {
		t.Errorf("--ci: exit %d stderr %q, want the CI-lane refusal", res.code, stderr)
	}

	t.Setenv("GITHUB_ACTIONS", "true")
	res, stderr = captureVerifyrun(t, []string{"--sandbox=" + sandboxContainerNetns, "--brief", brief})
	if res.code != verifyrunExitUsageError || !strings.Contains(stderr, "in the CI lane") {
		t.Errorf("GITHUB_ACTIONS=true: exit %d stderr %q, want the CI-lane refusal", res.code, stderr)
	}

	// Control: off the CI lane the mode is not refused for being in CI (the run
	// then fails later, on the missing brief — not on this check).
	t.Setenv("GITHUB_ACTIONS", "")
	_, stderr = captureVerifyrun(t, []string{"--sandbox=" + sandboxContainerNetns, "--brief", brief})
	if strings.Contains(stderr, "in the CI lane") {
		t.Errorf("off CI the mode was refused as CI: %q", stderr)
	}
	// The default mode is never refused in CI: CI keeps the unshare path.
	if why := sandboxRefusal(sandboxUnshare, true, false); why != "" {
		t.Errorf("the default unshare mode was refused in CI: %s", why)
	}
}

// TestSandboxFlagRefusals: an unknown mode and the container mode combined with
// --in-container are usage refusals.
func TestSandboxFlagRefusals(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	if why := sandboxRefusal("bogus", false, false); !strings.Contains(why, "unknown --sandbox") {
		t.Errorf("unknown mode: %q", why)
	}
	if why := sandboxRefusal(sandboxContainerNetns, false, true); !strings.Contains(why, "--in-container") {
		t.Errorf("container-netns with --in-container: %q", why)
	}
	if why := sandboxRefusal(sandboxContainerNetns, false, false); why != "" {
		t.Errorf("container-netns off CI refused: %q", why)
	}
}

// TestSandboxOfReadsResultOnly: the sandbox field is read positionally from the
// Result cell's head, never from a command that mentions it or a reason that
// quotes the flag.
func TestSandboxOfReadsResultOnly(t *testing.T) {
	cmdOnly := "| 1 | `echo sandbox=container-netns` | pass exit=0 | sha256:000000000000 | 2026-10-08 | human:t @ 0000 |"
	if got := witnessSandboxOf(cmdOnly); got != "" {
		t.Errorf("a command mentioning sandbox= was read as the field: %q", got)
	}
	reason := "| 1 | `true` | could-not-run exit=- — --sandbox=container-netns precheck refused | sha256:000000000000 | 2026-10-08 | human:t @ 0000 |"
	if got := witnessSandboxOf(reason); got != "" {
		t.Errorf("a reason quoting the flag was read as the field: %q", got)
	}
}
