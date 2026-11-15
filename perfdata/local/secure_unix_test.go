//go:build unix

package local

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// regression: a non-regular segment entry or a widened store directory was trusted.
func TestStoreRefusesUnsafeTree(t *testing.T) {
	t.Run("a FIFO named as a segment is refused without blocking", func(t *testing.T) {
		dir := t.TempDir()
		openOnce(t, dir)
		if err := syscall.Mkfifo(filepath.Join(Root(dir), "segments", segmentName(1, 1, "2026-10-01")), 0o600); err != nil {
			t.Skipf("mkfifo: %v", err)
		}
		done := make(chan error, 1)
		go func() {
			st, err := Open(dir, armed, testPolicy(t, ""), Options{Now: fixed(day1)})
			if err == nil {
				st.Close()
			}
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, ErrUnsafe) {
				t.Fatalf("want ErrUnsafe for a FIFO segment, got %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Open blocked on a FIFO segment")
		}
	})

	for _, sub := range []string{"", "segments", "run", "distributions"} {
		t.Run("group or other access on "+filepath.Join("store", sub)+" is refused", func(t *testing.T) {
			dir := t.TempDir()
			openOnce(t, dir)
			p := filepath.Join(Root(dir), sub)
			if err := os.Chmod(p, 0o750); err != nil {
				t.Fatal(err)
			}
			st, err := Open(dir, armed, testPolicy(t, ""), Options{Now: fixed(day1)})
			if err == nil {
				st.Close()
				t.Fatalf("Open accepted a 0750 %s", p)
			}
			if !errors.Is(err, ErrUnsafe) {
				t.Fatalf("want ErrUnsafe, got %v", err)
			}
			if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o750 {
				t.Fatalf("a refused tree must not be repaired silently: %v", fi.Mode().Perm())
			}
		})
	}
}

// regression: a state directory other users could write let them replace the store
// directory between checks; it is now refused by both the writer and the reader.
func TestStateDirChecked(t *testing.T) {
	dir := t.TempDir()
	openOnce(t, dir)
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	st, err := Open(dir, armed, testPolicy(t, ""), Options{Now: fixed(day1)})
	if err == nil {
		st.Close()
		t.Fatal("Open accepted a state directory every user can write")
	}
	if !errors.Is(err, ErrUnsafe) {
		t.Fatalf("Open: want ErrUnsafe, got %v", err)
	}
	if _, err := Read(dir); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("Read: want ErrUnsafe, got %v", err)
	}
	// With the sticky bit other users cannot rename or remove this user's entries.
	if err := os.Chmod(dir, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	mustOpen(t, dir, testPolicy(t, ""), Options{Now: fixed(day1)}).Close()
	if _, err := Read(dir); err != nil {
		t.Fatalf("Read refused a sticky shared state directory: %v", err)
	}
}
