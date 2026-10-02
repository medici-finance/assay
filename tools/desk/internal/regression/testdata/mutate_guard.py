#!/usr/bin/env python3
"""Fail-first controls for the regression register and its CI reachability."""
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[5]
floor_go = root / "tools/desk/internal/regression/floor-go.sh"
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
controls["directories"] = (
    "tools/desk/internal/deskkit/citrigger_test.go",
    "func ciReadCoverage(t *testing.T, root string, globs, reads []string) []string {",
    "^TestCIReadTreeCoverage$",
    "./internal/deskkit",
)
controls["deadline"] = (
    "tools/desk/internal/regression/shell_test.go",
    'exec.CommandContext(ctx, "bash", path)',
    "^TestShellDeadline$",
    "./internal/regression",
)
controls["gitenv"] = (
    "tools/desk/internal/regression/fixtureenv.go",
    "func FixtureEnv(extra ...string) []string {",
    "^TestShellGitIsolation$",
    "./internal/regression",
)
controls["runnerenv"] = (
    "tools/desk/internal/regression/floor-go.sh",
    "\nscrub\nexec ",
    "^TestFloorRunnerGitIsolation$",
    "./internal/regression",
)
controls["execenv"] = (
    "tools/desk/internal/regression/gitenv_test.go",
    "func execEnvFaults(name string, src []byte) (int, []string) {",
    "^TestFloorExecEnv$",
    "./internal/regression",
)
replacements = {
    "deadline": 'exec.Command("bash", path)',
    "gitenv": "func FixtureEnv(extra ...string) []string {\nreturn append(os.Environ(), extra...)",
    "execenv": "func execEnvFaults(name string, src []byte) (int, []string) {\nreturn 0, nil",
    "runnerenv": "\nexec ",
}
if mode not in controls:
    raise SystemExit("usage: mutate_guard.py manifest|ci|directories|deadline|gitenv|execenv|runnerenv")
# A run killed between the mutation and its restore leaves a backup beside the target;
# restore every such backup first, so a guard never stays disabled in the tree.
for other, *_ in controls.values():
    target = root / other
    backup = target.with_name(target.name + ".mutate-backup")
    if backup.exists():
        target.write_bytes(backup.read_bytes())
        backup.unlink()
        print(f"restored {other} from an interrupted control run", file=sys.stderr)
relative, marker, test, package = controls[mode]
path = root / relative
backup = path.with_name(path.name + ".mutate-backup")
original = path.read_bytes()
text = original.decode()
if text.count(marker) != 1:
    raise SystemExit("mutation target changed: expected one function")
backup.write_bytes(original)
try:
    replacement = replacements.get(mode, marker + "\nreturn nil")
    path.write_text(text.replace(marker, replacement, 1))
    # The go tool starts through floor-go.sh, the floor's one choke point, so the
    # caller's GIT_* variables and global git config never reach the guard's fixtures.
    result = subprocess.run(
        ["bash", str(floor_go), "test", "-run", test, "-count=1", "-timeout", "60s", package],
        cwd=root / "tools/desk",
        timeout=90,
        capture_output=True,
        text=True,
    )
    print(result.stdout, end="")
    print(result.stderr, end="", file=sys.stderr)
    if result.returncode != 1 or "--- FAIL:" not in result.stdout:
        raise SystemExit("control did not produce the expected test failure")
finally:
    path.write_bytes(original)
    backup.unlink()
