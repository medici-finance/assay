package gitmaintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// regression: #2513
func TestDeadlineKillsChildTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based descendant fixture is Unix-specific; Windows jobs have native cellprocess witnesses")
	}
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	started, escaped := filepath.Join(dir, "started"), filepath.Join(dir, "escaped")
	script := "#!/bin/sh\ncase \" $* \" in\n*' pack-refs '*)\n printf started > '" + started + "'\n (sleep 1; printf escaped > '" + escaped + "') &\n wait\n ;;\n*) exec '" + realGit + "' \"$@\" ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := Run(context.Background(), repo, Options{Timeout: 400 * time.Millisecond}); err == nil {
		t.Fatal("timed-out task reported success")
	}
	if _, err := os.Stat(started); err != nil {
		t.Fatalf("positive control: child tree was never launched: %v", err)
	}
	// The planted descendant writes after the deadline unless the entire process
	// group is stopped before releasing the common-directory coordinator lock.
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatal("descendant escaped deadline and continued after coordinator exit")
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "assay-maintenance.json")); !os.IsNotExist(err) {
		t.Fatal("timed-out task wrote a success stamp")
	}
}
