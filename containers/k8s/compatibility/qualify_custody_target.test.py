"""Check that verifier-facing statuses distinguish observations from missing evidence."""
from contextlib import redirect_stderr
import importlib.util
import io
import json
import os
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
    def test_standalone_child_environment(self):
        retained = {'PATH': '/fixture/bin', 'HOME': '/fixture/home', 'TMPDIR': '/fixture/tmp',
                    'GOCACHE': '/fixture/build', 'GOMODCACHE': '/fixture/modules',
                    'GOTMPDIR': '/fixture/go-tmp', 'LC_ALL': 'C'}
        removed = {name: 'inert-fixture-marker' for name in
                   ('GH_TOKEN', 'AWS_SESSION_TOKEN', 'SSH_AUTH_SOCK', 'MODEL_API_KEY',
                    'UNRELATED_FIXTURE_VALUE', 'HTTPS_PROXY', 'ASSAY_TEST_DOCKER_HOST',
                    'ASSAY_TEST_FIXTURE_IMAGE')}
        pinned = {'KUBECONFIG': os.devnull, 'GOPROXY': 'off', 'GOSUMDB': 'off',
                  'GOTOOLCHAIN': 'local', 'GOWORK': 'off', 'GOFLAGS': '',
                  'GONOPROXY': 'none', 'GOVCS': '*:off', 'GOENV': 'off'}
        caller = {**retained, **removed, **dict.fromkeys(pinned, 'overridden-marker')}
        names = [suite.TEST, *(f'{suite.TEST}/{case}' for case in suite.TARGET_CASES)]
        output = '\n'.join(json.dumps({'Test': name, 'Action': 'pass'}) for name in names)
        with mock.patch.dict(os.environ, caller, clear=True):
            with mock.patch.object(suite.subprocess, 'run',
                                   return_value=subprocess.CompletedProcess([], 0, output, '')) as child:
                suite.check_targets()
                go_env = child.call_args.kwargs['env']
            # The render helper uses the same boundary as its git, YAML and Kustomize tools.
            probe = suite.check.run([sys.executable, '-I', '-c',
                                     'import json, os; print(json.dumps(dict(os.environ)))'])
        self.assertEqual(probe.returncode, 0, probe.stderr)
        for env in [go_env, json.loads(probe.stdout)]:
            unexpected = sorted(removed.keys() & env.keys())
            self.assertEqual(unexpected, [], 'ambient names reached child')
            for name, value in {**retained, **pinned}.items():
                self.assertEqual(env.get(name), value, name)

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
