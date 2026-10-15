//go:build darwin || linux

package cellstop

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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
	same := func() ([]Marked, error) { return []Marked{target}, nil }
	if err := StopMarked(ctx, m, []Marked{target}, 200*time.Millisecond, same); err == nil {
		t.Fatal("a process whose kernel start time differs was reported stopped")
	}
	if syscall.Kill(pid, 0) != nil {
		t.Fatal("process with a different kernel start time was signalled")
	}
	target.Start = ""
	if err := StopMarked(ctx, m, []Marked{target}, 200*time.Millisecond, same); err == nil {
		t.Fatal("a target with no recorded kernel start time was accepted")
	}
}

// regression: #2880 (security round 2, S2) — the shield is re-proved immediately before each
// signal: a live target that a fresh listing no longer names as the same orphan (a window opened
// beneath it while the prompt waited) is never signalled, and the run stops.
func TestStopMarkedReprovesShield(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := Marker{Cell: "fixture", Dir: t.TempDir()}
	cmd := exec.Command(exe)
	cmd.Env = append(append(os.Environ(), "CELL_STOP_FIXTURE=1"), m.Env("worker-desk")...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	pid := cmd.Process.Pid
	var target Marked
	for deadline := time.Now().Add(2 * time.Second); ; {
		all, _ := inventory()
		if p, ok := all[pid]; ok && m.carries(p) {
			start, _, err := deskkit.ProcessStart(pid)
			if err != nil {
				t.Fatal(err)
			}
			target = Marked{PID: pid, Parent: p.parent, Identity: p.identity, Role: "worker-desk", Orphan: true, Start: start}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture marker unreadable")
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	nowShielded := func() ([]Marked, error) { return nil, nil }
	if err := StopMarked(ctx, m, []Marked{target}, 200*time.Millisecond, nowShielded); err == nil {
		t.Fatal("signalled a target the fresh listing no longer names as an orphan")
	}
	if syscall.Kill(pid, 0) != nil {
		t.Fatal("a now-shielded target was signalled")
	}
	if err := StopMarked(ctx, m, []Marked{target}, 200*time.Millisecond, nil); err == nil {
		t.Fatal("ran without a way to re-prove the shield")
	}
	notOrphan := target
	notOrphan.Orphan = false
	if err := StopMarked(ctx, m, []Marked{notOrphan}, 200*time.Millisecond, func() ([]Marked, error) { return []Marked{notOrphan}, nil }); err == nil {
		t.Fatal("a target not listed as an orphan was accepted") // A3
	}
	if syscall.Kill(pid, 0) != nil {
		t.Fatal("target signalled")
	}
}
