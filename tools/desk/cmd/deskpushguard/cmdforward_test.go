package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushCmdForwardsArgs(t *testing.T) {
	dir := t.TempDir()
	files, err := writeHooks(dir, "windows", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("hook pair: %v", files)
	}
	body, err := os.ReadFile(filepath.Join(dir, "pre-push.cmd"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "deskpushguard.exe %*") {
		t.Fatal("remote arguments not forwarded to guard")
	}
}
