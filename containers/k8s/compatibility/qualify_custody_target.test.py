"""Check that verifier-facing statuses distinguish observations from missing evidence."""
from contextlib import redirect_stderr
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import unittest
from unittest import mock


sys.dont_write_bytecode = True
path = Path(__file__).with_name('qualify_custody_target.py')
spec = importlib.util.spec_from_file_location('custody_suite', path)
suite = importlib.util.module_from_spec(spec)
spec.loader.exec_module(suite)


class ResultStatus(unittest.TestCase):
    def test_target_status(self):
        for label, events, code, expected in [
            ('observed child failure', [{'Action': 'fail', 'Test': suite.TEST + '/other-run'}],
             1, suite.check.Invalid),
            ('observed parent failure', [{'Action': 'fail', 'Test': suite.TEST}],
             1, suite.check.Invalid),
            ('build unavailable', [{'Action': 'fail'}], 1, suite.check.Unavailable),
            ('no named test', [{'Action': 'pass'}], 0, suite.check.Unavailable),
            ('missing named child', [{'Action': 'pass', 'Test': suite.TEST}], 0, suite.check.Unavailable),
        ]:
            with self.subTest(label=label):
                output = '\n'.join(json.dumps(event) for event in events)
                completed = subprocess.CompletedProcess([], code, stdout=output, stderr='')
                with mock.patch.object(suite.subprocess, 'run', return_value=completed):
                    with self.assertRaises(expected):
                        suite.check_targets()

    def test_exit_status(self):
        for error, expected_code, prefix in [
            (suite.check.Invalid('observed policy violation'), 1, 'checked-failed:'),
            (suite.check.Unavailable('missing evidence'), 2, 'could-not-check:'),
            (FileNotFoundError('missing external tool'), 2, 'could-not-check:'),
        ]:
            with self.subTest(error=type(error).__name__):
                stderr = io.StringIO()
                with mock.patch.object(suite, 'main', side_effect=error), redirect_stderr(stderr):
                    self.assertEqual(suite.run_suite(), expected_code)
                self.assertTrue(stderr.getvalue().startswith(prefix), stderr.getvalue())


if __name__ == '__main__':
    unittest.main()
