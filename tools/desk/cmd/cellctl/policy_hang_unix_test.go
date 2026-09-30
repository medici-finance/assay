//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// Tests for sec-1594-S1b: a model-policy hook that can be made to HANG is a fail-open, because
// Claude Code cancels a timed-out command hook and lets the action proceed. The hook refuses a
// policy source that is not a regular file (a FIFO would block its open), and bounds its whole
// run with an in-process deadline that exits with the blocking status. Unix-only: FIFOs and
// process groups.

// hangBound is how long a bounded hook run may take before the harness kills it. It is well
// ABOVE the hook's own deadline, so a hook whose deadline works exits 2 by itself, and a hook
// with no deadline is killed here and reported as hung rather than hanging the test.
const hangBound = 20 * time.Second

// runHookBounded runs a hook command line like runHookEnv, but in its own process group and
// killed after hangBound. hung reports that the harness had to kill it.
func runHookBounded(t *testing.T, f *policyFixture, command string, event any, extraEnv ...string) (code int, stderr string, hung bool) {
	t.Helper()
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = append([]string{"PATH=" + f.binDir + ":/usr/bin:/bin", "HOME=" + f.cellDir, "KUBECONFIG=/dev/null"}, extraEnv...)
	cmd.Stdin = bytes.NewReader(raw)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// The binary is a grandchild of sh and keeps the stderr pipe open; without WaitDelay a
	// killed shell would leave Wait blocked on the pipe.
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var killed atomic.Bool
	timer := time.AfterFunc(hangBound, func() {
		killed.Store(true)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	err = cmd.Wait()
	timer.Stop()
	hung = killed.Load()
	if err == nil {
		return 0, errb.String(), hung
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), errb.String(), hung
	}
	return -1, errb.String() + err.Error(), hung
}

func mkfifo(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}
}

// TestBinaryHookRefusesFifoSource: a policy-locating variable pointed at a FIFO must refuse at
// once — not hang in open() until Claude Code gives up on the hook.
func TestBinaryHookRefusesFifoSource(t *testing.T) {
	f := catalogFixture(t)
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	s, _ := launchPolicyDesk(t, f, "worker-desk", "--provider", "glm")
	command := s.Hooks["PreModelSwitch"][0].Hooks[0].Command
	outside := map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "claude-fable-5-1"}
	fifo := filepath.Join(t.TempDir(), "policy.fifo")
	mkfifo(t, fifo)
	for _, name := range []string{"CELL_PROVIDER_DEFAULTS", "CELL_PROVIDER_OVERRIDES", "CELL_MODEL_POLICY"} {
		code, stderr, hung := runHookBounded(t, f, command, outside, name+"="+fifo)
		if hung || code != wantBlock || !strings.Contains(stderr, "not a regular file") {
			t.Errorf("%s=<fifo>: exit %d (hung %v), want %d naming a non-regular file; stderr %s", name, code, hung, wantBlock, stderr)
		}
	}
}

// TestBinaryHookDeadlineBlocks: a hang the source check does not cover — the roster the
// binary echoes at start-up, or the cell's own cell.env, replaced by a FIFO — still ends in
// the blocking status, from the hook's own deadline. Both cases pay the full deadline.
func TestBinaryHookDeadlineBlocks(t *testing.T) {
	f := catalogFixture(t)
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	s, _ := launchPolicyDesk(t, f, "worker-desk", "--provider", "glm")
	command := s.Hooks["PreModelSwitch"][0].Hooks[0].Command
	inside := map[string]any{"hook_event_name": "PreModelSwitch", "to_model": "glm-5.3[1m]"}
	// The harness must not cancel the hook before its own 5 s deadline has refused.
	for event, entries := range s.Hooks {
		if got := entries[0].Hooks[0].Timeout; got <= 5 {
			t.Errorf("%s hook timeout is %d s; want one longer than the hook's 5 s deadline", event, got)
		}
	}
	if code, stderr, _ := runHookBounded(t, f, command, inside); code != 0 {
		t.Fatalf("baseline: an in-policy switch must be allowed: exit %d %s", code, stderr)
	}

	// HOME comes from the hook's inherited environment, and the roster echo reads
	// $HOME/.config/assay/roster.env before any verb runs.
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "assay"), 0o700); err != nil {
		t.Fatal(err)
	}
	mkfifo(t, filepath.Join(home, ".config", "assay", "roster.env"))
	code, stderr, hung := runHookBounded(t, f, command, inside, "HOME="+home)
	if hung || code != wantBlock || !strings.Contains(stderr, "did not finish") {
		t.Errorf("roster under HOME replaced by a FIFO: exit %d (hung %v), want %d; stderr %s", code, hung, wantBlock, stderr)
	}

	cellEnv := filepath.Join(f.cellDir, "cell.env")
	if err := os.Remove(cellEnv); err != nil {
		t.Fatal(err)
	}
	mkfifo(t, cellEnv)
	code, stderr, hung = runHookBounded(t, f, command, inside)
	if hung || code != wantBlock || !strings.Contains(stderr, "did not finish") {
		t.Errorf("cell.env replaced by a FIFO: exit %d (hung %v), want %d; stderr %s", code, hung, wantBlock, stderr)
	}
}
