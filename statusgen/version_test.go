package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionFlag pins the pin-checkability contract:
// a built binary answers `--version` with the tag stamped at link time, and an
// UNSTAMPED build answers "dev" rather than inventing a release number. Consumers
// pin statusgen by tag; a binary that cannot say which release it is makes a stale
// install indistinguishable from a current one.
func TestVersionFlag(t *testing.T) {
	if statusgenVersion != "dev" {
		t.Fatalf("in-package default statusgenVersion = %q, want \"dev\" — an unstamped build must not claim a release", statusgenVersion)
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	bin := filepath.Join(t.TempDir(), "statusgen")

	t.Run("unstamped build reports dev", func(t *testing.T) {
		build := exec.Command("go", "build", "-o", bin, ".")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		out, err := exec.Command(bin, "--version").Output()
		if err != nil {
			t.Fatalf("--version: %v", err)
		}
		if got := strings.TrimSpace(string(out)); got != "dev" {
			t.Errorf("--version = %q, want dev", got)
		}
	})

	t.Run("stamped build reports the tag", func(t *testing.T) {
		const tag = "statusgen/v9.9.9"
		build := exec.Command("go", "build", "-ldflags", "-X main.statusgenVersion="+tag, "-o", bin, ".")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		out, err := exec.Command(bin, "--version").Output()
		if err != nil {
			t.Fatalf("--version: %v", err)
		}
		if got := strings.TrimSpace(string(out)); got != tag {
			t.Errorf("--version = %q, want %q — the release workflow's -X stamp is the pin-check input", got, tag)
		}
		// `version` is accepted too, matching the sibling desk tools.
		out, err = exec.Command(bin, "version").Output()
		if err != nil {
			t.Fatalf("version: %v", err)
		}
		if got := strings.TrimSpace(string(out)); got != tag {
			t.Errorf("version = %q, want %q", got, tag)
		}
	})

	t.Run("release workflow stamps the tag", func(t *testing.T) {
		// The -X flag is the whole mechanism; a release built without it ships a
		// binary that answers "dev" and silently defeats every pin check. This
		// repo's release workflow is release.yml; a missing/unreadable workflow
		// is a hard failure, not a skip — a skipped guard passes silently, which
		// is the very fail-open this test exists to prevent.
		raw, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "release.yml"))
		if err != nil {
			t.Fatalf("release workflow not readable: %v", err)
		}
		if !strings.Contains(string(raw), "-X main.statusgenVersion=") {
			t.Error("release.yml does not stamp main.statusgenVersion — released binaries would report \"dev\"")
		}
	})
}
