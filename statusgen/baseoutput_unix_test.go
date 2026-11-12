//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestBaseOutputOwnerReadableUnderRestrictiveUmask(t *testing.T) {
	home := canonicalPath(t.TempDir())
	dir := filepath.Join(canonicalPath(t.TempDir()), "primary")
	log, err := newBaseOutputLog(home, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer log.close()
	prior := syscall.Umask(0o777)
	defer syscall.Umask(prior)
	if err := log.record("1", []byte("retained\n")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, log.rows["1"].name)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("info=%v err=%v", info, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "retained\n" {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}
