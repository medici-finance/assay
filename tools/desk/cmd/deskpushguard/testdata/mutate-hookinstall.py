#!/usr/bin/env python3
"""Offline #2801 fail-first controls; restore every planted source change.

Run from any directory with the Go toolchain available. No hooks are
installed outside Go test's disposable fixture directories. Mutation execution
requires an otherwise idle checkout because it temporarily edits source files.
"""
from pathlib import Path
import subprocess

repo = Path(__file__).resolve().parents[5]
module = repo / "tools/desk"
source = module / "cmd/deskpushguard/hookinstall.go"
windows = repo / "scripts/build-windows.ps1"
asset = module / "hooks/pre-push"
makefile = repo / "Makefile"
# Each unique source anchor must match exactly once: stale mutations fail closed.
cases = [
    ("legacy-skip", source, "if normalized == content {", "if normalized == content || normalized == anchoredUnixShim {", "TestHookInstallLegacyMigration"),
    ("marker-spoof", source, "if normalized == content {", 'if strings.Contains(normalized, "deskpushguard") {', "TestHookInstallForeignAndPartialPair"),
    ("pair-preflight", source, "pending = append(pending, sh)", "pending = append(pending, sh)\nif err := os.WriteFile(sh.path, []byte(sh.content), sh.mode); err != nil { return nil, err }", "TestHookInstallForeignAndPartialPair"),
    ("anchor-downgrade", source, 'return targetOS == "windows", nil', "return true, nil", "TestHookInstallLegacyMigration"),
    ("windows-pair", source, 'if targetOS == "windows" {', 'if targetOS == "never" {', "TestHookInstallLegacyMigration"),
    ("guard-status", source, 'exec "$(command -v deskpushguard)" "$@"', '"$(command -v deskpushguard)" "$@" || exit 0', "TestHookInstallGuardInvocation"),
    ("second-windows-writer", windows, "function Target-DeskHookInstall {", "function Target-DeskHookInstall {\n    Copy-Item -Force legacy-hook .githooks/pre-push", "TestHookInstallWindowsEntrypoint"),
    ("published-path-shim", asset, 'exec /opt/desk-tools/bin/deskpushguard "$@"', 'exec "$(command -v deskpushguard)" "$@"', "TestHookInstallStaticAsset"),
    ("make-missing-guard", makefile, "first\" >&2; \\\n\t\texit 1;", "first\" >&2; \\\n\t\texit 0;", "TestHookInstallMakeEntrypoint"),
]
for name, path, old, new, test in cases:
    original = path.read_bytes()
    raw = original.decode()
    if raw.count(old) != 1:
        raise SystemExit(f"{name}: expected exactly one source anchor")
    try:
        path.write_bytes(raw.replace(old, new).encode())
        run = subprocess.run(["go", "test", "./cmd/deskpushguard", "-run", f"^{test}$", "-count=1", "-timeout=30s"], cwd=module, capture_output=True, text=True, timeout=60)
        if run.returncode == 0 or f"--- FAIL: {test}" not in run.stdout:
            raise SystemExit(f"{name}: expected named assertion failure, got:\n{run.stdout}{run.stderr}")
        print(f"{name}: observed {test} FAIL", flush=True)
    finally:
        path.write_bytes(original)
