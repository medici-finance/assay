package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewObservationPrivateAtomicOutput(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	out := filepath.Join(dir, "observation.json")
	if err := os.WriteFile(target, []byte("untouched"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, out); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewObservation(out, map[string]string{"body": "private"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("artifact not a private regular file", info.Mode())
	}
	b, _ := os.ReadFile(target)
	if string(b) != "untouched" {
		t.Fatal("followed destination symlink")
	}
	before, _ := os.ReadFile(out)
	if err := writeReviewObservation(out, make(chan int)); err == nil {
		t.Fatal("encoding error ignored")
	}
	after, _ := os.ReadFile(out)
	if string(after) != string(before) {
		t.Fatal("failed write replaced prior complete artifact")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".review-observation-") {
			t.Fatal("sensitive partial left behind")
		}
	}
	if err := writeReviewObservation(filepath.Join(dir, "missing", "file"), nil); err == nil {
		t.Fatal("missing output parent ignored")
	}
}
