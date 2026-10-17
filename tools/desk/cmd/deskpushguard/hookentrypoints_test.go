package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// regression: #2801
func TestHookInstallMakeEntrypoint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Make entrypoint")
	}
	makefile := hookContent(t, "../../../../Makefile")
	for _, force := range []bool{false, true} {
		dir := t.TempDir()
		hookFile(t, filepath.Join(dir, "Makefile"), makefile)
		for _, sub := range []string{"tools/desk/hooks", "dist", ".githooks"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
				t.Fatal(err)
			}
		}
		hookFile(t, filepath.Join(dir, "tools/desk/hooks/pre-push"), hookContent(t, "testdata/legacy-pre-push"))
		path := filepath.Join(dir, ".githooks/pre-push")
		old := hookContent(t, "testdata/legacy-pre-push")
		hookFile(t, path, old)
		args := []string{"--no-print-directory", "desk-hook-install", "DIST_DIR=dist"}
		if force {
			args = append(args, "FORCE=1")
		}
		cmd := exec.Command("make", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("missing built guard did not refuse")
		}
		if hookContent(t, path) != old {
			t.Fatal("missing guard changed a hook")
		}
		if !strings.Contains(string(out), "desk-build") {
			t.Fatalf("missing build guidance: %s", out)
		}
		// Recording stub: only the owned fixture executable can receive delegation.
		guard := filepath.Join(dir, "dist/deskpushguard")
		hookFile(t, guard, "#!/bin/sh\nprintf '%s\\n' \"$@\" > called\nexit 17\n")
		cmd = exec.Command("make", args...)
		cmd.Dir = dir
		if out, err = cmd.CombinedOutput(); err == nil {
			t.Fatalf("guard refusal not propagated: %s", out)
		}
		want := "hook-install\n--dir\n.githooks\n"
		if force {
			want += "--force\n"
		}
		if got := hookContent(t, filepath.Join(dir, "called")); got != want {
			t.Fatalf("delegation=%q want %q", got, want)
		}
		if hookContent(t, path) != old {
			t.Fatal("entrypoint changed hook outside guard")
		}
	}
}

func windowsHookEntrypoint(t *testing.T, raw string) string {
	t.Helper()
	start := strings.Index(raw, "function Target-DeskHookInstall {")
	end := strings.Index(raw, "\nfunction Target-DeskTest {")
	if start < 0 || end <= start {
		t.Fatal("cannot locate Windows hook entrypoint")
	}
	return raw[start:end]
}
func singleWindowsHookWriter(body string) bool {
	return !strings.Contains(body, "Copy-Item") && !strings.Contains(body, "$HookSrc") &&
		strings.Count(body, "& $guard @hookArgs") == 1 && strings.Contains(body, "if ($Force)") &&
		strings.Contains(body, "$LASTEXITCODE -ne 0") && strings.Contains(body, "throw") &&
		strings.Contains(body, "desk-build")
}

// regression: #2801
func TestHookInstallWindowsEntrypoint(t *testing.T) {
	body := windowsHookEntrypoint(t, hookContent(t, "../../../../scripts/build-windows.ps1"))
	if !singleWindowsHookWriter(body) {
		t.Fatal("Windows installer retains a second hook writer or lacks guarded delegation")
	}
	// A planted second copy path must be detected, even if delegation remains.
	if singleWindowsHookWriter(body + "\nCopy-Item -Force legacy-hook .githooks/pre-push\n") {
		t.Fatal("planted second writer escaped")
	}
}

// regression: #2801
func TestHookInstallStaticAsset(t *testing.T) {
	if got := hookContent(t, "../../hooks/pre-push"); got != unixShim {
		t.Fatal("published static shim differs from portable current shim")
	}
}
