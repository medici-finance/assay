#!/usr/bin/env python3
"""Fail-first controls for the regression register and its CI reachability."""
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[5]
mode = sys.argv[1] if len(sys.argv) == 2 else ""
controls = {
    "manifest": (
        "tools/desk/internal/regression/manifest_test.go",
        "func validateManifest(body, root string) []string {",
        "^Test(RegressionManifest|ManifestRules)$",
        "./internal/regression",
    ),
    "ci": (
        "tools/desk/internal/deskkit/citrigger_test.go",
        "func ciCrossModuleRegistry() []ciEntry {",
        "^TestRegressionCIEntrypoints$",
        "./internal/deskkit",
    ),
}
if mode not in controls:
    raise SystemExit("usage: mutate_guard.py manifest|ci")
relative, marker, test, package = controls[mode]
path = root / relative
original = path.read_bytes()
text = original.decode()
if text.count(marker) != 1:
    raise SystemExit("mutation target changed: expected one function")
try:
    path.write_text(text.replace(marker, marker + "\nreturn nil", 1))
    result = subprocess.run(
        ["go", "test", "-run", test, "-count=1", "-timeout", "30s", package],
        cwd=root / "tools/desk",
        timeout=45,
        capture_output=True,
        text=True,
    )
    print(result.stdout, end="")
    print(result.stderr, end="", file=sys.stderr)
    if result.returncode != 1 or "--- FAIL:" not in result.stdout:
        raise SystemExit("control did not produce the expected test failure")
finally:
    path.write_bytes(original)
