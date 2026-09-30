package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHookInstallUnix pins the "works" verdict brief-12 claims for the pre-push shim
// on a non-windows target: writeHooks must produce ONLY .githooks/pre-push, marked
// executable, carrying #!/bin/sh, and — the property that actually makes this a fix
// rather than a re-commit of the old shim — resolving the guard binary via
// `command -v deskpushguard` rather than any /opt literal.
func TestHookInstallUnix(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".githooks")

	installed, err := writeHooks(hooksDir, "linux", false)
	if err != nil {
		t.Fatalf("writeHooks: %v", err)
	}
	if len(installed) != 1 || installed[0] != filepath.Join(hooksDir, "pre-push") {
		t.Fatalf("writeHooks(unix): expected exactly [.githooks/pre-push], got %v", installed)
	}

	prePush := filepath.Join(hooksDir, "pre-push")
	info, statErr := os.Stat(prePush)
	if statErr != nil {
		t.Fatalf("stat pre-push: %v", statErr)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("pre-push is not executable: mode %v", info.Mode())
	}

	raw, rerr := os.ReadFile(prePush)
	if rerr != nil {
		t.Fatalf("read pre-push: %v", rerr)
	}
	content := string(raw)
	if !strings.HasPrefix(content, "#!/bin/sh") {
		t.Fatalf("pre-push does not start with #!/bin/sh:\n%s", content)
	}
	if strings.Contains(content, "/opt/") {
		t.Fatalf("pre-push still carries an /opt literal (the exact defect this brief closes):\n%s", content)
	}
	if !strings.Contains(content, "command -v deskpushguard") {
		t.Fatalf("pre-push does not resolve deskpushguard via `command -v` (PATH resolution):\n%s", content)
	}

	// A windows pair must NOT be written for a unix target.
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-push.cmd")); err == nil {
		t.Fatalf("pre-push.cmd was written for a unix target — it should only exist for windows")
	}
}

// TestHookInstallWindows pins the "works" verdict for the windows target: writeHooks
// must produce BOTH .githooks/pre-push (for Git for Windows' bundled sh.exe) and
// .githooks/pre-push.cmd (the native pair, calling deskpushguard.exe from PATH).
func TestHookInstallWindows(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".githooks")

	installed, err := writeHooks(hooksDir, "windows", false)
	if err != nil {
		t.Fatalf("writeHooks: %v", err)
	}
	wantPrePush := filepath.Join(hooksDir, "pre-push")
	wantCmd := filepath.Join(hooksDir, "pre-push.cmd")
	if len(installed) != 2 || installed[0] != wantPrePush || installed[1] != wantCmd {
		t.Fatalf("writeHooks(windows): expected [pre-push, pre-push.cmd], got %v", installed)
	}

	cmdRaw, cerr := os.ReadFile(wantCmd)
	if cerr != nil {
		t.Fatalf("read pre-push.cmd: %v", cerr)
	}
	cmdContent := string(cmdRaw)
	if !strings.Contains(cmdContent, "@echo off") {
		t.Fatalf("pre-push.cmd is not a .cmd script:\n%s", cmdContent)
	}
	if !strings.Contains(cmdContent, "deskpushguard.exe") {
		t.Fatalf("pre-push.cmd does not invoke deskpushguard.exe:\n%s", cmdContent)
	}
	if strings.Contains(cmdContent, "/opt/") {
		t.Fatalf("pre-push.cmd carries an /opt literal:\n%s", cmdContent)
	}

	prePushRaw, perr := os.ReadFile(wantPrePush)
	if perr != nil {
		t.Fatalf("read pre-push: %v", perr)
	}
	if !strings.Contains(string(prePushRaw), "command -v deskpushguard") {
		t.Fatalf("pre-push (the sh pair) does not resolve deskpushguard via PATH:\n%s", string(prePushRaw))
	}
}

// TestHookInstallIdempotentAndForeignRefusal pins the two behaviours the Makefile's
// existing desk-hook-install target already has, so `hook-install` matches it: a
// re-run over a hook this installer already wrote is a silent no-op (no error, no
// rewrite claimed as installed), and a FOREIGN pre-push hook (one lacking the
// "deskpushguard" marker) is refused unless --force is set.
func TestHookInstallIdempotentAndForeignRefusal(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".githooks")

	if _, err := writeHooks(hooksDir, "linux", false); err != nil {
		t.Fatalf("first writeHooks: %v", err)
	}
	installedAgain, err := writeHooks(hooksDir, "linux", false)
	if err != nil {
		t.Fatalf("second writeHooks (idempotent re-run): %v", err)
	}
	if len(installedAgain) != 0 {
		t.Fatalf("re-run over an already-installed hook should report nothing newly installed, got %v", installedAgain)
	}

	foreignDir := filepath.Join(dir, "foreign", ".githooks")
	if err := os.MkdirAll(foreignDir, 0o755); err != nil {
		t.Fatalf("mkdir foreign hooks dir: %v", err)
	}
	foreignHook := filepath.Join(foreignDir, "pre-push")
	if err := os.WriteFile(foreignHook, []byte("#!/bin/sh\necho some-other-teams-hook\n"), 0o755); err != nil {
		t.Fatalf("write foreign hook: %v", err)
	}
	if _, err := writeHooks(foreignDir, "linux", false); err == nil {
		t.Fatalf("writeHooks over a foreign hook without --force should refuse, got no error")
	}
	if _, err := writeHooks(foreignDir, "linux", true); err != nil {
		t.Fatalf("writeHooks over a foreign hook WITH --force should overwrite, got error: %v", err)
	}
	raw, rerr := os.ReadFile(foreignHook)
	if rerr != nil {
		t.Fatalf("read overwritten hook: %v", rerr)
	}
	if !strings.Contains(string(raw), "deskpushguard") {
		t.Fatalf("--force did not overwrite the foreign hook:\n%s", string(raw))
	}
}
