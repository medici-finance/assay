//go:build darwin || linux

package cellstop

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// regression: #2880 (security A1) — the kernel start time recorded at listing is re-checked
// immediately before each signal; a mismatch is a different process and is never signalled.
func TestStopMarkedRechecksKernelStart(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	m := Marker{Cell: "fixture", Dir: dir}
	cmd := exec.Command(exe)
	cmd.Env = append(append(os.Environ(), "CELL_STOP_FIXTURE=1"), m.Env("worker-desk")...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	pid := cmd.Process.Pid
	var target Marked
	for deadline := time.Now().Add(2 * time.Second); ; {
		all, err := inventory()
		if err != nil {
			t.Fatal(err)
		}
		if p, ok := all[pid]; ok && m.carries(p) {
			target = Marked{PID: pid, Parent: p.parent, Identity: p.identity, Role: "worker-desk", Orphan: true, Start: "0.000001"}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture marker unreadable")
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := StopMarked(ctx, m, []Marked{target}, 200*time.Millisecond); err == nil {
		t.Fatal("a process whose kernel start time differs was reported stopped")
	}
	if syscall.Kill(pid, 0) != nil {
		t.Fatal("process with a different kernel start time was signalled")
	}
	target.Start = ""
	if err := StopMarked(ctx, m, []Marked{target}, 200*time.Millisecond); err == nil {
		t.Fatal("a target with no recorded kernel start time was accepted")
	}
}
