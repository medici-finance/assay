package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// regression: #2749
func TestEmittedLinkTargetUntouched(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "task.md")
			original := "caller task\n"
			if err := os.WriteFile(input, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			dest := emittedPathFor(input)
			link := os.Symlink
			if kind == "hardlink" {
				link = os.Link
			}
			if err := link(input, dest); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("link unavailable: %v", err)
				}
				t.Fatal(err)
			}
			var emitErr error
			out := captureStdout(t, func() { emitErr = emitPrompt(dispatchOpts{promptFile: input, quiet: true}, "assembled") })
			if emitErr != nil {
				t.Fatal(emitErr)
			}
			got, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != original {
				t.Errorf("%s target changed: got %q, want %q", kind, got, original)
			}
			if out != dest+"\n" {
				t.Errorf("stdout = %q, want emitted path", out)
			}
			info, err := os.Lstat(dest)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() {
				t.Errorf("emitted path is not a regular file: %v", info.Mode())
			}
			got, err = os.ReadFile(dest)
			if err != nil || string(got) != "assembled\n" {
				t.Errorf("emitted prompt = %q, err %v", got, err)
			}
		})
	}
}

// regression: #2749
func TestEmittedExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	input := filepath.Join(t.TempDir(), "task.md")
	dest := emittedPathFor(input)
	if err := os.WriteFile(dest, []byte("old prompt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dest, 0666); err != nil {
		t.Fatal(err)
	}
	var emitErr error
	captureStdout(t, func() { emitErr = emitPrompt(dispatchOpts{promptFile: input, quiet: true}, "replacement") })
	if emitErr != nil {
		t.Fatal(emitErr)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("emitted file mode = %04o, want 0600", info.Mode().Perm())
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "replacement\n" {
		t.Errorf("emitted prompt = %q, err %v", got, err)
	}
}
