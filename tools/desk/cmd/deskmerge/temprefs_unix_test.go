//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// regression: #2525
func TestFetchNoHookListVerdict(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	w.install(t, defaultPR(), false)
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	// A Git without `git hook list`: every other operation is the real binary.
	shim := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = hook ]; then echo \"git: 'hook' is not a git command.\" >&2; exit 1; fi\ndone\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	assertCleanupKeepsVerdict(t, w)
}
