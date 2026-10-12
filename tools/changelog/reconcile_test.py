#!/usr/bin/env python3
"""Regression #2686: real Git objects through the actual changelog gate, offline."""
import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
CHECK = Path(os.environ.get('CHECK_IMPL', HERE / 'check.sh')).resolve()
README = '''---
stream: example
board: generated
---
# Example

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [One](brief-01-one.md) | 0 | S | todo | — | — |
| 02 | [Two \\| alternatives](brief-02-two.md) | 1 | M | in-progress | — | — |
<!-- statusgen:briefs:end -->

Unchanged prose.
'''
PATH = 'docs/streams/example/README.md'


class ReconcileGate(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.env = {**{k: v for k, v in os.environ.items() if not k.startswith('GIT_')},
                    'GIT_CONFIG_NOSYSTEM': '1',
                    'GIT_CONFIG_GLOBAL': os.devnull, 'GIT_AUTHOR_NAME': 'fixture',
                    'GIT_AUTHOR_EMAIL': 'fixture@example.invalid',
                    'GIT_COMMITTER_NAME': 'fixture',
                    'GIT_COMMITTER_EMAIL': 'fixture@example.invalid'}
        self.git('init', '-q')
        self.git('config', 'maintenance.auto', 'false')
        self.git('config', 'gc.auto', '0')
        self.write(PATH, README)
        self.write('CHANGELOG.md', '# Changelog\n\n## Unreleased\n\n## v1.0.0\n- Initial\n')
        self.base = self.commit()
        self.write(PATH, README.replace(' | todo | ', ' | implemented | '))
        self.head = self.commit()
        self.event = {'repository': {'full_name': 'o/r'}, 'pull_request': {
            'user': {'login': 'assay-board-writer[bot]', 'type': 'Bot', 'id': 319272446},
            'base': {'sha': self.base, 'ref': 'main', 'repo': {'full_name': 'o/r'}},
            'head': {'sha': self.head, 'ref': 'board/reconcile', 'repo': {'full_name': 'o/r'}}}}

    def git(self, *args):
        return subprocess.check_output(['git', *args], cwd=self.root, env=self.env,
                                       stderr=subprocess.STDOUT).decode().strip()

    def write(self, path, content):
        dest = self.root / path
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(content)

    def commit(self):
        self.git('add', '--', '.')
        self.git('commit', '-qm', 'fixture')
        return self.git('rev-parse', 'HEAD')

    def check(self, want, event=None, extra=None):
        self.event['pull_request']['head']['sha'] = self.head
        event_file = self.root / '.git' / 'event.json'
        event_file.write_text(json.dumps(self.event if event is None else event))
        env = {**self.env, 'SKIP': 'false', 'BASE_SHA': self.base, 'HEAD_SHA': self.head,
               'PR_NUMBER': '', 'BASE_REF': '', 'GITHUB_BASE_REF': '',
               'GITHUB_EVENT_NAME': 'pull_request', 'GITHUB_REPOSITORY': 'o/r',
               'GITHUB_EVENT_PATH': str(event_file), 'CHANGELOG_AGG': str(HERE / 'aggregate.py')}
        env.update(extra or {})
        run = subprocess.run(['bash', str(CHECK)], cwd=self.root, env=env,
                             stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        self.assertEqual(run.returncode, want, run.stdout)
        if want == 0 and env['SKIP'] == 'false':
            self.assertIn('PASS: board reconciliation', run.stdout)
        return run.stdout

    # regression: #2686
    def test_status_only_and_second_stream(self):
        self.check(0)
        self.git('reset', '--hard', self.base)
        self.write('docs/streams/second/README.md', README)
        self.base = self.commit()
        self.event['pull_request']['base']['sha'] = self.base
        for path in (PATH, 'docs/streams/second/README.md'):
            self.write(path, README.replace(' | todo | ', ' | implemented | '))
        self.head = self.commit()
        self.check(0)

    # regression: #2686
    def test_second_stream_does_not_hide_foreign_prose(self):
        self.git('reset', '--hard', self.base)
        self.write('docs/streams/second/README.md', README)
        self.base = self.commit()
        self.event['pull_request']['base']['sha'] = self.base
        self.write(PATH, README.replace(' | todo | ', ' | implemented | '))
        self.write('docs/streams/second/README.md', README + 'Planted unrelated prose.\n')
        self.head = self.commit()
        self.check(1)

    # regression: #2686
    def test_repository_module_cannot_replace_classifier_import(self):
        self.write('json.py', 'raise SystemExit(0)\n')
        self.head = self.commit()
        self.check(1)

    # regression: #2686
    def test_metadata_not_a_branch_or_author_waiver(self):
        for section, key, value in [('user', 'login', 'assay-worker-app[bot]'),
                                    ('user', 'type', 'User'), ('user', 'id', 1),
                                    ('head', 'ref', 'board/reconcile-copy'),
                                    ('base', 'ref', 'release'), ('base', 'sha', '0'*40),
                                    ('head', 'sha', '0'*40),
                                    ('head', 'repo', {'full_name': 'fork/r'}),
                                    ('base', 'repo', {'full_name': 'other/r'})]:
            with self.subTest(section=section, key=key):
                event = copy.deepcopy(self.event)
                event['pull_request'][section][key] = value
                self.check(1, event)
        for event in ({}, {'pull_request': None}, {'repository': [], 'pull_request': {}}, []):
            with self.subTest(event=event):
                self.check(1, event)
        for extra in ({'GITHUB_EVENT_NAME': 'push'}, {'GITHUB_EVENT_PATH': '/nonexistent'},
                      {'GITHUB_REPOSITORY': 'elsewhere/r'}, {'HEAD_SHA': '0'*40}):
            with self.subTest(extra=extra):
                self.check(1, extra=extra)

    # regression: #2686
    def test_readme_content_boundary(self):
        changed = README.replace(' | todo | ', ' | implemented | ')
        for bad in (changed + 'Unrelated prose.\n', changed.replace('[One]', '[Rewritten]'),
                    changed.replace(' | 0 | S | ', ' | 2 | S | '),
                    changed.replace(' | — | — |', ' | evidence | — |'),
                    changed.replace('implemented', 'unrecognized'),
                    changed.replace('<!-- statusgen:briefs:end -->', 'Injected text\n<!-- statusgen:briefs:end -->'),
                    changed.replace('<!-- statusgen:briefs:end -->', '<!-- statusgen:briefs:end -->\n<!-- statusgen:briefs:end -->'),
                    changed.replace('| 02 |', '| 03 |'), changed.replace('board: generated', 'board: authored')):
            with self.subTest(bad=bad):
                self.git('reset', '--hard', self.base)
                self.write(PATH, bad)
                self.head = self.commit()
                self.check(1)

    # regression: #2686
    def test_entire_diff_and_file_modes(self):
        for path in ('src/main.go', 'docs/streams/example/brief-01-one.md',
                     'docs/streams/example/nested/README.md', 'docs/streams/STATUS.md'):
            with self.subTest(path=path):
                self.git('reset', '--hard', self.base)
                self.write(PATH, README.replace(' | todo | ', ' | implemented | '))
                self.write(path, 'extra\n')
                self.head = self.commit()
                self.check(1)
        self.git('reset', '--hard', self.base)
        self.git('mv', PATH, 'docs/streams/example/OLD.md')
        self.head = self.commit()
        self.check(1)
        self.git('reset', '--hard', self.base)
        (self.root / PATH).chmod(0o755)
        self.head = self.commit()
        self.check(1)
        self.git('reset', '--hard', self.base)
        (self.root / PATH).unlink()
        (self.root / PATH).symlink_to('brief-01-one.md')
        self.head = self.commit()
        self.check(1)

    # regression: #2686
    def test_empty_diff_and_absolute_guard(self):
        self.head = self.base
        self.check(1)
        self.write('CHANGELOG.md', '# Changelog\n\n## Unreleased\n- Direct edit\n')
        self.head = self.commit()
        self.assertIn('Unreleased edit refused', self.check(1, extra={'SKIP': 'true'}))

    # regression: #2686
    def test_missing_or_non_generated_markers(self):
        for original in (README.replace('board: generated', 'board: authored'),
                         README.replace('<!-- statusgen:briefs:begin -->', ''),
                         README.replace('<!-- statusgen:briefs:end -->', ''),
                         README.replace('Status |', 'State |')):
            with self.subTest(original=original):
                self.git('reset', '--hard', self.base)
                self.write(PATH, original)
                self.base = self.commit()
                self.event['pull_request']['base']['sha'] = self.base
                self.write(PATH, original.replace(' | todo | ', ' | implemented | '))
                self.head = self.commit()
                self.check(1)

    # regression: #2686
    def test_merge_base_diff_and_repeat(self):
        original_head = self.head
        self.git('checkout', '--detach', self.base)
        self.write('main-only.txt', 'base advanced\n')
        self.base = self.commit()
        self.event['pull_request']['base']['sha'] = self.base
        self.head = original_head
        self.check(0)
        self.check(0)  # release/main movement does not require any fragment


if __name__ == '__main__':
    unittest.main(verbosity=2)
