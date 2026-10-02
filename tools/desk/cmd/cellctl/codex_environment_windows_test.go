package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsCommandEnvironmentNativeSemantics(t *testing.T) {
	env := envSet([]string{"Path=old", "PATH=duplicate", "UserProfile=old"}, "PATH", "new")
	if strings.Join(env, "|") != "UserProfile=old|PATH=new" {
		t.Fatal(env)
	}
	if got := envUnset(env, "USERPROFILE"); len(got) != 1 || got[0] != "PATH=new" {
		t.Fatal(got)
	}
	home := t.TempDir()
	before, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	restore := withHome(home)
	if got, err := os.UserHomeDir(); err != nil || got != home {
		t.Fatalf("home %q: %v", got, err)
	}
	restore()
	if got, _ := os.UserHomeDir(); got != before {
		t.Fatal("home not restored")
	}
	exe := filepath.Join(t.TempDir(), "deskboot.exe")
	if err := os.WriteFile(exe, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if !isExecFile(strings.TrimSuffix(exe, ".exe")) {
		t.Fatal("native .exe without Unix execute bits not discovered")
	}
}
