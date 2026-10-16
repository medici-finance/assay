//go:build unix

package cellcadence

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// regression: #2880 (security A2) — a FIFO at the lock path is refused at once, never waited on.
func TestProbeRefusesFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "owner.lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := Probe(dir); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO lock path accepted")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("probe hung on a FIFO")
	}
}
