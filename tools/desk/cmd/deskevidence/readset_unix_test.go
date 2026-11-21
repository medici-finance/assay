//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// regression: #2026
// TestReadStateFileFIFOSwapRefused — a FIFO swapped in at the name after any
// path check and before the open never blocks the landing: the open is
// O_NONBLOCK and the descriptor's own mode refuses it.
func TestReadStateFileFIFOSwapRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(p, []byte("regular\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := stateFileOpening
	t.Cleanup(func() { stateFileOpening = prev })
	stateFileOpening = func(name string) {
		if err := os.Remove(name); err != nil {
			t.Error(err)
		}
		if err := syscall.Mkfifo(name, 0o644); err != nil {
			t.Error(err)
		}
	}
	done := make(chan string, 1)
	go func() {
		_, why := readStateFile(p)
		done <- why
	}()
	select {
	case why := <-done:
		if why != "is not a regular file" {
			t.Fatalf("FIFO at the name: why %q; want \"is not a regular file\"", why)
		}
	case <-time.After(5 * time.Second):
		// Release the blocked open so the goroutine ends, then fail.
		go func() {
			if w, err := os.OpenFile(p, os.O_WRONLY, 0); err == nil {
				w.Close()
			}
		}()
		t.Fatal("readStateFile blocked opening a FIFO swapped in at the name")
	}
}

// regression: #2026
// TestReadStateFileLinkRefused — a symlink at the name is refused by the
// open (O_NOFOLLOW); the file it points at is never read.
func TestReadStateFileLinkRefused(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "state")
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	if b, why := readStateFile(p); why != "is not a regular file" || b != nil {
		t.Fatalf("link at the name: read %q, why %q; want refusal", b, why)
	}
}
