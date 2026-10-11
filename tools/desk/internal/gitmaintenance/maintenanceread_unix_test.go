//go:build unix

package gitmaintenance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// regression: #2513
func TestStateSpecialReadsBounded(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "missing"), link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fifo, link} {
		done := make(chan error, 1)
		go func() {
			f, err := openMaintenanceRegular(path)
			if f != nil {
				_ = f.Close()
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("special state read clean")
			}
		case <-time.After(time.Second):
			t.Fatal("special state open exceeded bound")
		}
	}
	// Even a link to an ordinary existing file must not confer receipt ownership.
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("record"), 0o600); err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(dir, "existing-link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if f, err := openMaintenanceRegular(link); err == nil {
		_ = f.Close()
		t.Fatal("regular symlink admitted as state")
	}
}
