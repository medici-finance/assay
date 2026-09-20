package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The whole contract of the mkdir lock: a dead pid must never wedge every future boot, and a
// live one must never be stolen.
func TestSessionLockTakesOverAStaleLock(t *testing.T) {
	dir := t.TempDir()
	c := &Cell{Env: envWith(map[string]string{}), Name: "demo", Dir: dir}
	lockdir := filepath.Join(dir, "run", "lock.d")
	if err := os.MkdirAll(filepath.Dir(lockdir), 0o700); err != nil {
		t.Fatal(err)
	}
	// A lock held by a pid that cannot be alive.
	if err := os.Mkdir(lockdir, 0o700); err != nil {
		t.Fatal(err)
	}
	writePid(lockdir, "999999999")
	c.takeSessionLock(lockdir, "demo-cell")
	if got := readPid(lockdir); got != strconv.Itoa(os.Getpid()) {
		t.Errorf("a stale lock was not taken over: pid=%q", got)
	}
}

func TestSessionLockRefusesALiveHolder(t *testing.T) {
	dir := t.TempDir()
	c := &Cell{Env: envWith(map[string]string{}), Name: "demo", Dir: dir}
	lockdir := filepath.Join(dir, "run", "lock.d")
	if err := os.MkdirAll(filepath.Dir(lockdir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(lockdir, 0o700); err != nil {
		t.Fatal(err)
	}
	// This test process is unambiguously alive.
	writePid(lockdir, strconv.Itoa(os.Getpid()))
	assertDies(t, "live lock holder", func() { c.takeSessionLock(lockdir, "demo-cell") })
}

func TestSessionLockIsFreshWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	c := &Cell{Env: envWith(map[string]string{}), Name: "demo", Dir: dir}
	lockdir := filepath.Join(dir, "run", "lock.d")
	if err := os.MkdirAll(filepath.Dir(lockdir), 0o700); err != nil {
		t.Fatal(err)
	}
	c.takeSessionLock(lockdir, "demo-cell")
	if _, err := os.Stat(lockdir); err != nil {
		t.Fatalf("lock directory not created: %v", err)
	}
}

// TestStatusLineReadsTheLock covers status.go: `stopped` / `running` / `stale-lock`, the three
// answers `cellctl status` is allowed to give.
func TestStatusLineReadsTheLock(t *testing.T) {
	dir := t.TempDir()
	c := &Cell{Env: envWith(map[string]string{}), Name: "demo", Dir: dir}
	if got := c.statusLine(); got != "stopped" {
		t.Errorf("no lock ⇒ %q, want stopped", got)
	}
	lockdir := filepath.Join(dir, "run", "lock.d")
	if err := os.MkdirAll(lockdir, 0o700); err != nil {
		t.Fatal(err)
	}
	writePid(lockdir, strconv.Itoa(os.Getpid()))
	if got := c.statusLine(); got != "running demo-cell" {
		t.Errorf("live lock ⇒ %q, want 'running demo-cell'", got)
	}
	writePid(lockdir, "999999999")
	if got := c.statusLine(); got != "stale-lock 999999999" {
		t.Errorf("dead pid ⇒ %q, want 'stale-lock 999999999'", got)
	}
}

func TestLastNonBlankLine(t *testing.T) {
	if got := lastNonBlankLine("noise\nREADY\n\n   \n"); got != "READY" {
		t.Errorf("lastNonBlankLine = %q, want READY", got)
	}
	if got := lastNonBlankLine("\n \n"); got != "" {
		t.Errorf("lastNonBlankLine of blanks = %q, want empty", got)
	}
}
