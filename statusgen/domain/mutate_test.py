"""Run with python3 -B -I statusgen/domain/mutate_test.py."""
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("domain_mutate", Path(__file__).with_name("mutate.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class SnapshotTests(unittest.TestCase):
    # regression: #4116
    def test_snapshot_closure(self):
        with tempfile.TemporaryDirectory(prefix="domain-closure-") as scratch:
            source, dest = Path(scratch) / "source", Path(scratch) / "dest"
            runner.snapshot(source)
            # A second, unrelated package expansion must never enter the module.
            (source / "statusgen/contexthandoff/unrelated.go").write_text(
                'package contexthandoff\nimport _ "example.invalid/unrelated"\n'
            )
            with patch.object(runner, "ROOT", source):
                runner.snapshot(dest)
            copied = dest / "statusgen/contexthandoff"
            self.assertEqual([f.name for f in copied.glob("*.go")], ["ids.go"])
            self.assertEqual((copied / "ids.go").read_bytes(),
                             (runner.ROOT / "statusgen/contexthandoff/ids.go").read_bytes())
            result = subprocess.run(
                ["go", "test", "-count=1", "-timeout=30s", "./domain"],
                cwd=dest / "statusgen", env=runner.offline_env(), text=True,
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60,
            )
            self.assertEqual(result.returncode, 0, result.stdout)

    # regression: #4116
    def test_build_is_not_kill(self):
        name = "TestDomainMissingFactHolds"
        for output in ("[build failed]", "[setup failed]", "no tests to run",
                       "--- FAIL: " + name + "\nFAIL [setup failed]",
                       "--- FAIL: " + name + "\nFAIL [build failed]"):
            result = subprocess.CompletedProcess([], 1, output)
            self.assertFalse(runner.is_kill(result, name), output)
        self.assertFalse(runner.is_kill(subprocess.CompletedProcess([], 0, "--- FAIL: " + name), name))
        self.assertTrue(runner.is_kill(subprocess.CompletedProcess([], 1, "--- FAIL: " + name), name))


if __name__ == "__main__":
    unittest.main()
