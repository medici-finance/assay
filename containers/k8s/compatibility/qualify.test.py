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
            root = Path(scratch)
            (root / "tools/desk").mkdir(parents=True)
            runner = root / "containers/k8s/compatibility"
            runner.mkdir(parents=True)
            for name in ("qualify.py", "qualify.sh"):
                shutil.copyfile(HERE / name, runner / name)
            candidate = Path("components/compatibility-cellctl/candidate.json")
            target = runner.parent / candidate
            target.parent.mkdir(parents=True)
            record = json.loads((HERE.parent / candidate).read_text())
            roles = sorted(record["images"])
            target.write_text(json.dumps(record))
            bin_dir = root / "bin"
            bin_dir.mkdir()
            go = bin_dir / "go"
            go.write_text('#!/bin/sh\ncat "$FIXTURE_EVENTS" || exit 1\nexit "$FIXTURE_EXIT"\n')
            go.chmod(0o755)
            events = root / "events.jsonl"
            env = dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"], FIXTURE_EVENTS=str(events))
            parent = {"Action": "pass", "Test": TEST}
            children = [{"Action": "pass", "Test": TEST + "/" + role} for role in roles]
            complete = [parent, *children]

            def check(rows, status, want, message, *args):
                events.write_text("".join(json.dumps(row) + "\n" for row in rows))
                run = subprocess.run(["sh", str(runner / "qualify.sh"), *args],
                                     env=dict(env, FIXTURE_EXIT=str(status)), capture_output=True, text=True)
                self.assertEqual(run.returncode, want, run.stderr)
                self.assertIn(message, run.stderr if want else run.stdout)
                print(f"exit {run.returncode}: {message}")

            check(complete, 0, 0, '"result": "pass"')
            check(complete[:-1], 0, 1, f"missing named test passes: {TEST}/{roles[-1]}")
            check([{"Action": "skip", "Test": TEST}, *children], 0, 1, f"missing named test passes: {TEST}")
            check([*complete, {"Action": "output", "Output": "synthetic package failure\n"}],
                  7, 7, "synthetic package failure")
            check(complete, 0, 2, "unrecognized arguments: --allow-cluster-queries", "--allow-cluster-queries")
            record["images"].pop(roles[-1])
            target.write_text(json.dumps(record))
            check(complete, 0, 1, "qualification requires the five-role candidate inventory")


if __name__ == "__main__":
    unittest.main()
