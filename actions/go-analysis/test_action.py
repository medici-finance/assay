import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('action', Path(__file__).with_name('run.py'))
action = importlib.util.module_from_spec(spec)
spec.loader.exec_module(action)


class ActionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.home = Path(self.tmp.name)
        self.repo = self.home / 'repo'
        self.repo.mkdir()
        self.git('init', '-q')
        self.commit()
        self.cache = self.home / 'cache'
        self.cache.mkdir()
        self.output = self.home / 'outputs'
        self.summary = self.home / 'summary'
        self.env = patch.dict(os.environ, {
            'GITHUB_OUTPUT': str(self.output), 'GITHUB_STEP_SUMMARY': str(self.summary),
            'GITHUB_WORKSPACE': str(self.repo), 'RUNNER_TEMP': str(self.home),
            'INPUT_ROOT': '.', 'CACHE': str(self.cache), 'BOOTSTRAP_REV': '',
        })
        self.env.start()
        self.addCleanup(self.env.stop)

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], text=True,
                                       stderr=subprocess.DEVNULL).strip()

    def commit(self):
        self.git('add', '-A')
        self.git('-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
                 'commit', '-qm', 'fixture', '--allow-empty')
        return self.git('rev-parse', 'HEAD')

    def write(self, path, value='fixture'):
        p = self.repo / path
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(value)

    def outputs(self):
        return dict(x.split('=', 1) for x in self.output.read_text().splitlines())

    def test_prepare_external_cache(self):
        action.prepare()
        out = self.outputs()
        self.assertFalse(Path(out['cache']).is_relative_to(self.repo))
        self.assertEqual(out['source-sha'], self.git('rev-parse', 'HEAD'))
        self.assertEqual(Path(out['code-root']), Path(__file__).resolve().parents[2])

    # regression: #7421
    def test_bootstrap_requires_sha(self):
        # Bootstrap is retired: any value, even an immutable SHA, is refused, never ignored.
        for value in ['main', 'a' * 40]:
            with patch.dict(os.environ, BOOTSTRAP_REV=value):
                with self.assertRaisesRegex(ValueError, 'retired'):
                    action.prepare()

    # regression: #7421
    def test_mode_validated(self):
        with patch.dict(os.environ, INPUT_MODE='permissive'):
            with self.assertRaisesRegex(ValueError, 'mode'):
                action.prepare()

    def test_output_injection_refused(self):
        with self.assertRaises(ValueError):
            action.output('path', 'valid\nforged=success')

    # regression: #7421
    def test_trust_lifecycle(self):
        target = self.home / 'analyzed'
        subprocess.run(['git', 'clone', '-q', str(self.repo), str(target)], check=True)
        with patch.dict(os.environ, ANALYZED=str(target), BASE_DIR=str(self.repo)):
            with self.assertRaisesRegex(ValueError, 'policy'):
                action.trust()
            self.write('tools/go-analysis/pins.env')
            self.commit()
            with self.assertRaisesRegex(ValueError, 'policy'):
                action.trust()
            # No baseline file is needed: the comparison is against the merge base.
            self.write('.golangci.yml')
            sha = self.commit()
            action.trust()
            self.assertEqual(self.outputs()['base-sha'], sha)
            self.assertNotIn('bootstrap', self.outputs())
            # The trusted base commit is now reachable from the analyzed checkout.
            subprocess.run(['git', '-C', str(target), 'cat-file', '-e', sha + '^{commit}'], check=True)

    def test_scan_preserves_exit(self):
        self.write('scripts/go-analysis.sh', '#!/bin/bash\nexit 1\n')
        with patch.dict(os.environ, CODE_ROOT=str(self.repo), ANALYZED=str(self.repo),
                        BASE_SHA=self.git('rev-parse', 'HEAD'), BOOTSTRAP='false'):
            self.assertEqual(action.scan(), 1)
            self.write('scripts/go-analysis.sh', '#!/bin/bash\nexit 2\n')
            self.assertEqual(action.scan(), 2)
            self.write('scripts/go-analysis.sh', '#!/bin/bash\nexit 0\n')
            self.assertEqual(action.scan(), 0)

    def test_missing_report_is_unknown(self):
        self.assertEqual(action.summary(), 2)
        self.assertEqual(self.outputs()['state'], 'could-not-check')
        self.assertNotIn('existing', self.outputs())
        self.assertIn('unavailable', self.summary.read_text())

    def test_failed_partial_no_zero_counts(self):
        (self.cache / 'report.json').write_text(json.dumps({
            'schema': 'go-analysis/v1', 'state': 'could-not-check', 'gate': 'fail', 'exit_code': 2,
            'totals': {'existing': 0, 'new': 0, 'worsened': 0, 'resolved': 0}}))
        self.assertEqual(action.summary(), 0)
        self.assertEqual(self.outputs()['gate'], 'fail')
        self.assertNotIn('resolved', self.outputs())
        self.assertIn('unknown', self.summary.read_text())

    def test_complete_red_is_reported(self):
        report = {'schema': 'go-analysis/v1', 'state': 'complete', 'gate': 'fail', 'exit_code': 1,
                  'totals': {'existing': 7, 'new': 1, 'worsened': 0, 'resolved': 2}}
        (self.cache / 'report.json').write_text(json.dumps(report))
        self.assertEqual(action.summary(), 0)
        self.assertEqual(self.outputs()['existing'], '7')
        self.assertEqual(self.outputs()['new'], '1')
        report['totals']['new'] = -1
        (self.cache / 'report.json').write_text(json.dumps(report))
        with self.assertRaises(ValueError):
            action.summary()

    def test_inconsistent_outcomes_rejected(self):
        for state, gate, code in [('could-not-check', 'pass', 0),
                                  ('complete', 'pass', 1),
                                  ('complete', 'pass', 0)]:
            report = {'schema': 'go-analysis/v1', 'state': state,
                      'gate': gate, 'exit_code': code,
                      'totals': {'existing': 1, 'new': 1, 'worsened': 0, 'resolved': 0}}
            (self.cache / 'report.json').write_text(json.dumps(report))
            with self.assertRaises(ValueError):
                action.summary()

    # regression: #7421
    def test_scan_mode_arguments(self):
        self.write('scripts/go-analysis.sh', '#!/bin/bash\necho "$@" > "$GO_ARGS"\n')
        record = self.home / 'args'
        base = self.git('rev-parse', 'HEAD')
        with patch.dict(os.environ, CODE_ROOT=str(self.repo), ANALYZED=str(self.repo),
                        BASE_SHA=base, GO_ARGS=str(record)):
            self.assertEqual(action.scan(), 0)
            self.assertTrue(record.read_text().startswith('lint '))
            self.assertIn('--base ' + base, record.read_text())
            with patch.dict(os.environ, INPUT_MODE='report'):
                self.assertEqual(action.scan(), 0)
                self.assertTrue(record.read_text().startswith('report '))
                self.assertNotIn('--base', record.read_text())
            with patch.dict(os.environ, INPUT_MODE='permissive'):
                with self.assertRaises(ValueError):
                    action.scan()

    # regression: #7421
    def test_debt_report_summary(self):
        report = {'schema': 'go-analysis/v1', 'mode': 'debt-report', 'state': 'complete', 'gate': 'pass',
                  'exit_code': 0, 'totals': {'existing': 9, 'new': 0, 'worsened': 0, 'resolved': 0}}
        (self.cache / 'report.json').write_text(json.dumps(report))
        self.assertEqual(action.summary(), 0)
        self.assertEqual(self.outputs()['existing'], '9')
        self.assertIn('never gates', self.summary.read_text())
        report['mode'] = 'lenient'
        (self.cache / 'report.json').write_text(json.dumps(report))
        with self.assertRaises(ValueError):
            action.summary()

    def test_cache_in_analyzed_tree_rejected(self):
        with patch.dict(os.environ, RUNNER_TEMP=str(self.repo)):
            with self.assertRaises(ValueError):
                action.prepare()

    def test_bad_json_rejected(self):
        (self.cache / 'report.json').write_text('{bad json')
        with self.assertRaises(ValueError):
            action.summary()


if __name__ == '__main__':
    unittest.main()
