"""Exercise the qualification command with recorded go test events."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


HERE = Path(__file__).resolve().parent
TEST = "TestKubeCompatCellctlLifecycle"


class RunnerControls(unittest.TestCase):
    def test_role_coverage_and_process_result(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch).resolve()
            go_cwd = root / "tools/desk"
            go_cwd.mkdir(parents=True)
            runner = root / "containers/k8s/compatibility"
            runner.mkdir(parents=True)
            for name in ("qualify.py", "qualify.sh"):
                shutil.copyfile(HERE / name, runner / name)
            shutil.copyfile(HERE.parent / "check.py", runner.parent / "check.py")
            candidate = Path("components/compatibility-cellctl/candidate.json")
            target = runner.parent / candidate
            target.parent.mkdir(parents=True)
            record = json.loads((HERE.parent / candidate).read_text())
            roles = sorted(record["images"])
            target.write_text(json.dumps(record))
            bin_dir = root / "bin"
            bin_dir.mkdir()
            go = bin_dir / "go"
            go.write_text("""#!/bin/sh
set -eu
test -z "${QUALIFY_TEST_UNRELATED+x}"
test "$KUBECONFIG" = /dev/null
test "$GOENV" = off
test "$GOMODCACHE" = "$PWD/modules"
if [ "${ASSAY_QUALIFY_INTERACTION:-}" = 1 ]; then
    test "$ASSAY_TEST_DOCKER_HOST" = unix:///fixture.sock
    test "$ASSAY_TEST_FIXTURE_IMAGE" = fixture-id
else
    test -z "${ASSAY_TEST_DOCKER_HOST+x}${ASSAY_TEST_FIXTURE_IMAGE+x}"
fi
cat events.jsonl
exit "$(cat status)"
""")
            go.chmod(0o755)
            events = go_cwd / "events.jsonl"
            status_file = go_cwd / "status"
            env = {
                "HOME": str(root),
                "PATH": str(bin_dir) + os.pathsep + os.environ["PATH"],
                "GOMODCACHE": str(go_cwd / "modules"),
                "ASSAY_TEST_DOCKER_HOST": "unix:///fixture.sock",
                "ASSAY_TEST_FIXTURE_IMAGE": "fixture-id",
                "QUALIFY_TEST_UNRELATED": "inert",
            }
            parent = {"Action": "pass", "Test": TEST}
            children = [{"Action": "pass", "Test": TEST + "/" + role} for role in roles]
            complete = [parent, *children]

            def check(rows, status, want, message, *args):
                events.write_text("".join(json.dumps(row) + "\n" for row in rows))
                status_file.write_text(str(status))
                run = subprocess.run(["sh", str(runner / "qualify.sh"), *args],
                                     env=env, capture_output=True, text=True)
                self.assertEqual(run.returncode, want, run.stderr)
                self.assertIn(message, run.stderr if want else run.stdout)
                print(f"exit {run.returncode}: {message}")

            check(complete, 0, 0, '"result": "pass"')
            check(complete[:-1], 0, 1, f"missing named test passes: {TEST}/{roles[-1]}")
            check([{"Action": "skip", "Test": TEST}, *children], 0, 1, f"missing named test passes: {TEST}")
            check([*complete, {"Action": "output", "Output": "synthetic package failure\n"}],
                  7, 7, "synthetic package failure")
            check(complete, 0, 2, "unrecognized arguments: --allow-cluster-queries", "--allow-cluster-queries")
            interaction = "TestKubeCompatCellctlInteraction"
            interact_rows = [{"Action": "pass", "Test": row["Test"].replace(TEST, interaction)} for row in complete]
            logs = []
            for role in roles:
                named = {"Action": "output", "Test": f"{interaction}/{role}"}
                logs.extend([dict(named, Output='fixture.go:1: INTERACTION_RECORD {"role": '),
                             dict(named, Output=json.dumps(role) + "}\n")])
            check([*interact_rows, *logs], 0, 0, '"suite": "interaction"', "--offline", "--suite", "interaction")
            check([*interact_rows, *logs[:-1]], 0, 1, "incomplete five-role", "--suite", "interaction")
            check([*interact_rows, *logs, *logs[:2]], 0, 1, "duplicate or inconsistent", "--suite", "interaction")
            check(complete, 0, 1, "missing named test passes", "--suite", "interaction")
            check(interact_rows[:-1], 0, 1, f"{interaction}/{roles[-1]}", "--suite", "interaction")
            check(complete, 0, 2, "requires a suite", "--self-test")
            check([], 0, 2, "invalid choice", "--suite", "real-profile")
            check([], 0, 2, "invalid choice", "--suite", "../controlled-restart")
            for name in ("controlled-restart", "custody-target"):
                check([], 0, 2, "invalid choice", "--suite", name)
                companion = runner / ("qualify_" + name.replace("-", "_") + ".py")
                uses_docker = name == "controlled-restart"
                expected_host = env["ASSAY_TEST_DOCKER_HOST"] if uses_docker else None
                expected_image = env["ASSAY_TEST_FIXTURE_IMAGE"] if uses_docker else None
                companion.write_text(
                    "import os, sys\nassert sys.flags.isolated\n"
                    "assert os.environ['KUBECONFIG'] == os.devnull\n"
                    "assert os.environ['GOENV'] == 'off'\n"
                    f"assert os.environ.get('ASSAY_TEST_DOCKER_HOST') == {expected_host!r}\n"
                    f"assert os.environ.get('ASSAY_TEST_FIXTURE_IMAGE') == {expected_image!r}\n"
                    "assert 'QUALIFY_TEST_UNRELATED' not in os.environ\n"
                    "print(sys.argv[1:])\n")
                check([], 0, 0, "['--offline']", "--suite", name)
                check([], 0, 0, "['--offline', '--self-test']", "--offline", "--suite", name, "--self-test")
                check([], 0, 2, "unrecognized arguments", "--suite", name, "--context", "other")
                companion.write_text("raise SystemExit(6)\n")
                check([], 0, 6, "", "--suite", name)
            record["images"].pop(roles[-1])
            target.write_text(json.dumps(record))
            check(complete, 0, 1, "qualification requires the five-role candidate inventory")


if __name__ == "__main__":
    unittest.main()
