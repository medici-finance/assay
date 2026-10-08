//go:build unix

package cellscratch

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestScratchCrashFixture(t *testing.T) {
	root := os.Getenv("SCRATCH_CRASH_FIXTURE")
	if root == "" {
		return
	}
	s, err := Open(root)
	if err != nil {
		os.Exit(91)
	}
	r, err := s.Begin("session", "verifier", "revision")
	if err != nil {
		os.Exit(92)
	}
	if err = r.Started(os.Getpid()); err != nil {
		os.Exit(93)
	}
	if err = s.Acknowledge(r.Record.ID, "canonical-evidence", false); err != nil {
		os.Exit(94)
	}
	if err = os.WriteFile(filepath.Join(root, "ready"), []byte(r.Record.ID), 0600); err != nil {
		os.Exit(95)
	}
	for {
		time.Sleep(time.Hour)
	}
}
func TestScratchCrashRecovery(t *testing.T) {
	s := scratchStore(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestScratchCrashFixture$")
	cmd.Env = append(os.Environ(), "SCRATCH_CRASH_FIXTURE="+s.Path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	var id string
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, err := os.ReadFile(filepath.Join(s.Path, "ready"))
		if err == nil {
			id = string(b)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture never acquired lease")
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Remove(filepath.Join(s.Path, "ready"))
	got := sweep(t, s, DefaultPolicy(), true)
	if len(got.Entries) != 1 || got.Entries[0].Action != "keep" {
		t.Fatal("live owner removed", got)
	}
	// SIGKILL bypasses Finish/defers; the kernel releases the lease and reaping proves
	// the process group empty. The persisted identity + both proofs permit recovery.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	got = sweep(t, s, DefaultPolicy(), true)
	if got.Entries[0].Action != "compact" {
		t.Fatal("abandoned run not recovered", got)
	}
	r, err := s.record(id)
	if err != nil || r.State != "recovered" || !r.Reaped || r.ExitCode != -1 {
		t.Fatal("recovery fabricated success", r, err)
	}
}
func TestScratchChildStillLive(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	cmd := exec.Command(os.Args[0], "-test.run=^TestScratchCrashFixture$")
	// Another process group remains alive even after the owner lease is released.
	cmd = exec.Command("/bin/sleep", "10")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	r.Started(cmd.Process.Pid)
	r.Close()
	got := sweep(t, s, Policy{}, true)
	if got.Entries[0].Action != "keep" || !exists(t, r.Work()) {
		t.Fatal("live child ignored", got)
	}
}
