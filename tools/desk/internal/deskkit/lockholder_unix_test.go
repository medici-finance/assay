//go:build linux || darwin

package deskkit

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Process fixtures for lock-holder identification. Every process signalled here
// is one this test spawned; nothing is found or killed by name or pattern.

const holderHelperEnv = "DESKKIT_LOCK_HOLDER_HELPER"

// TestLockHolderHelper is the holder process: it takes dir/owner.lock as cellctl
// would, records a role child, prints "ready" and then waits to be ended.
// mode "wedged" lets the recorded child exit; mode "current" keeps it alive.
func TestLockHolderHelper(t *testing.T) {
	mode := os.Getenv(holderHelperEnv)
	if mode == "" {
		t.Skip("helper process only")
	}
	dir, cell := os.Getenv("HOLDER_DIR"), os.Getenv("HOLDER_CELL")
	r, err := os.OpenRoot(dir)
	if err != nil {
		os.Exit(2)
	}
	f, err := r.OpenFile("owner.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		os.Exit(2)
	}
	h, err := AcquireHeld(r, "owner.lock", f, LockSpec{Name: "role lease", Cell: cell, Role: "worker-desk"})
	if err != nil {
		os.Exit(3)
	}
	// The role child reads stdin until this process goes away.
	child := exec.Command("/bin/sh", "-c", "read x")
	in, err := child.StdinPipe()
	if err != nil || child.Start() != nil {
		os.Exit(4)
	}
	if h.RecordChild(child.Process.Pid) != nil {
		os.Exit(5)
	}
	if mode == "wedged" {
		in.Close()
		child.Wait()
	}
	os.Stdout.WriteString("ready\n")
	for { // until SIGTERM or the test ends it; a sleeping timer is not a deadlock
		time.Sleep(time.Hour)
	}
}

type holderProc struct {
	cmd  *exec.Cmd
	done chan error
}

func (p *holderProc) alive() bool {
	select {
	case err := <-p.done:
		p.done <- err
		return false
	default:
		return true
	}
}

// startHolder runs the helper under the executable name "cellctl".
func startHolder(t *testing.T, mode, dir, cell string) *holderProc {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "cellctl")
	if err := os.Link(os.Args[0], exe); err != nil {
		src, err := os.ReadFile(os.Args[0])
		if err != nil || os.WriteFile(exe, src, 0o700) != nil {
			t.Fatalf("cannot stage helper binary: %v", err)
		}
	}
	cmd := exec.Command(exe, "-test.run=^TestLockHolderHelper$")
	cmd.Env = append(os.Environ(), holderHelperEnv+"="+mode, "HOLDER_DIR="+dir, "HOLDER_CELL="+cell)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &holderProc{cmd: cmd, done: make(chan error, 1)}
	ready := make(chan bool, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		ready <- line == "ready\n"
		io.Copy(io.Discard, out)
		p.done <- cmd.Wait()
	}()
	t.Cleanup(func() {
		if p.alive() {
			cmd.Process.Kill() // our own spawned pid only
			<-p.done
		}
	})
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("helper did not take the lock")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("helper never became ready")
	}
	return p
}

func fixtureSpec(cell string, endStale bool) LockSpec {
	return LockSpec{Name: "role lease", Cell: cell, Role: "worker-desk", Wait: 100 * time.Millisecond,
		EndStale: endStale, OptIn: "--end-stale-lock-holder", Grace: 50 * time.Millisecond, TermWait: 10 * time.Second}
}

func busyFrom(t *testing.T, err error) *BusyError {
	t.Helper()
	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("want *BusyError, got %v", err)
	}
	return busy
}

// A wedged holder (verified cellctl, role child gone) is stale. Without opt-in it
// is reported with the flag that would end it, and left running.
//
// regression: #2709
func TestStaleHolderReported(t *testing.T) {
	dir := t.TempDir()
	p := startHolder(t, "wedged", dir, "fixture")
	r, f := openLockFixture(t, dir)
	_, err := AcquireHeld(r, "owner.lock", f, fixtureSpec("fixture", false))
	busy := busyFrom(t, err)
	if busy.Diagnosis.State != HolderStale || !busy.Diagnosis.Eligible {
		t.Fatalf("wedged own-cell cellctl not diagnosed stale+eligible: %v", err)
	}
	for _, want := range []string{"holder current: no (stale)", "--end-stale-lock-holder", "has exited", "pid "} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("stale report lacks %q: %v", want, err)
		}
	}
	if !p.alive() {
		t.Fatal("a stale holder was ended without opt-in")
	}
}

// With opt-in, a wedged own-cell holder gets SIGTERM, exits, and the lock is taken.
//
// regression: #2709
func TestStaleHolderEnded(t *testing.T) {
	dir := t.TempDir()
	p := startHolder(t, "wedged", dir, "fixture")
	r, f := openLockFixture(t, dir)
	h, err := AcquireHeld(r, "owner.lock", f, fixtureSpec("fixture", true))
	if err != nil {
		t.Fatalf("opted-in recovery of a stale holder failed: %v", err)
	}
	defer h.Release()
	select {
	case werr := <-p.done:
		p.done <- werr
		if werr == nil {
			t.Fatal("holder exited cleanly; it should have been ended by the SIGTERM")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("holder still running after recovery took its lock")
	}
}

// A holder whose role child is alive is current: a correct refusal, even with
// opt-in, and it is never signalled.
func TestCurrentHolderRefused(t *testing.T) {
	dir := t.TempDir()
	p := startHolder(t, "current", dir, "fixture")
	r, f := openLockFixture(t, dir)
	_, err := AcquireHeld(r, "owner.lock", f, fixtureSpec("fixture", true))
	busy := busyFrom(t, err)
	if busy.Diagnosis.State != HolderCurrent || busy.Diagnosis.Eligible || !strings.Contains(err.Error(), "correct refusal") {
		t.Fatalf("live role session not reported current: %v", err)
	}
	if !p.alive() {
		t.Fatal("a current holder was signalled")
	}
}

// A stale holder that is another cell's cellctl is reported, never signalled.
func TestOtherCellStaleHolder(t *testing.T) {
	dir := t.TempDir()
	p := startHolder(t, "wedged", dir, "other-cell")
	r, f := openLockFixture(t, dir)
	_, err := AcquireHeld(r, "owner.lock", f, fixtureSpec("fixture", true))
	busy := busyFrom(t, err)
	if busy.Diagnosis.State != HolderStale || busy.Diagnosis.Eligible || !strings.Contains(err.Error(), "never signalled") {
		t.Fatalf("another cell's stale holder was eligible: %v", err)
	}
	if !p.alive() {
		t.Fatal("another cell's holder was signalled")
	}
}

// A record naming a live pid with the wrong start time is pid reuse: the real
// holder is unknown and the recorded pid is never acted on. A record whose pid
// is alive and matches but does not have the file open is unknown too, and so is
// a record naming a pid that has exited.
//
// regression: #2709
func TestPidReuseNotActed(t *testing.T) {
	dir := t.TempDir()
	r, holder := openLockFixture(t, dir)
	if err := TryLockExclusive(holder); err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	bystander := exec.Command("sleep", "120")
	if err := bystander.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { bystander.Process.Kill(); bystander.Wait() }() // our own spawned pid
	pid := bystander.Process.Pid
	start, err := procStart(pid)
	if err != nil {
		t.Fatal(err)
	}
	comm, err := procCommand(pid)
	if err != nil {
		t.Fatal(err)
	}
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err = gone.Run(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		rec        LockHolder
	}{
		{"reused", "pid reuse", LockHolder{PID: pid, Start: start + "1", Command: comm}},
		{"not-open", "does not have the lock file open", LockHolder{PID: pid, Start: start, Command: comm, ChildPID: gone.Process.Pid}},
		{"dead", "is not running", LockHolder{PID: gone.Process.Pid, Start: "0", Command: "cellctl", ChildPID: gone.Process.Pid}},
	} {
		tc.rec.Schema, tc.rec.Lock, tc.rec.Cell, tc.rec.Role = lockHolderSchema, "role lease", "fixture", "worker-desk"
		if err = writeLockHolder(r, "owner.lock", tc.rec); err != nil {
			t.Fatal(err)
		}
		_, f := openLockFixture(t, dir)
		_, err = AcquireHeld(r, "owner.lock", f, fixtureSpec("fixture", true))
		busy := busyFrom(t, err)
		if busy.Diagnosis.State != HolderUnknown || busy.Diagnosis.Eligible || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "holder: unknown") {
			t.Fatalf("%s record was trusted: %v", tc.name, err)
		}
		if bystander.ProcessState != nil {
			t.Fatalf("%s: the recorded pid was signalled", tc.name)
		}
		if err = syscall.Kill(pid, 0); err != nil {
			t.Fatalf("%s: the recorded pid is gone: %v", tc.name, err)
		}
	}
}
