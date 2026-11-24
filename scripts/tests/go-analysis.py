#!/usr/bin/env python3
"""Observed negative controls, including a real admitted upstream analyzer."""
import copy
import importlib.util
import http.server
import threading
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

OWNER = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('runner', Path(os.getenv('GO_ANALYSIS_RUNNER', str(OWNER / 'tools/go-analysis/runner.py'))))
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)


def record(name='one'):
    identity = dict(analyzer='errcheck', rule='errcheck', module='module', configuration='linux/amd64',
                    path=name + '.go', anchor=r.digest(name.encode()))
    return dict(identity, id=r.digest(r.encoded(identity)), diagnostic=r.digest(b'ignored error'),
                first_seen='2026-10-11', owner='maintainers', work='https://github.com/medici-finance/assay/issues/2553',
                reason='measured legacy debt')


def baseline(*entries):
    return dict(schema='go-analysis-baseline/v1', contract='policy', findings=list(entries))


class Coverage(unittest.TestCase):
    # regression: #2395
    def test_lifted_owner_tuples(self):
        expected = {('linux', 'amd64', ''), ('windows', 'amd64', ''),
                    ('darwin', 'arm64', ''), ('linux', 'amd64', 'parity')}
        plan = r.coverage(['statusgen', 'tools/desk', 'gitcore'])
        for owner in ['tools/desk', 'gitcore']:
            self.assertEqual({row[1:] for row in plan if row[0] == owner}, expected)
        self.assertEqual(len(plan), len(set(plan)))
        self.assertEqual([row for row in plan if row[0] == 'statusgen'],
                         [('statusgen', 'linux', 'amd64', '')])
        self.assertEqual(r.coverage(['statusgen']),
                         [('statusgen', 'linux', 'amd64', '')])


class Ratchet(unittest.TestCase):
    # regression: F-go-analysis-ratchet
    def test_legacy(self):
        old = record()
        out = r.compare([copy.deepcopy(old)], [old], True)
        self.assertEqual(out[0]['disposition'], 'existing')

    # regression: F-go-analysis-ratchet
    def test_new_and_worse(self):
        old = record()
        new = record('two')
        out = r.compare([new], [old], True)
        self.assertEqual([f['disposition'] for f in out], ['new', 'resolved'])
        worse = dict(old, diagnostic=r.digest(b'worse'))
        self.assertEqual(r.compare([worse], [old], True)[0]['disposition'], 'worsened')
        self.assertEqual(r.compare([copy.deepcopy(old)], [], True)[0]['disposition'], 'new')

    # regression: #7421
    def test_resolved_allowance(self):
        # A finding the head repairs resolves with no allowance edit; an incomplete scan
        # never resolves merge-base debt.
        old = record()
        self.assertEqual(r.compare([], [old], False), [])
        out = r.compare([], [old], True)
        self.assertEqual(out[0]['disposition'], 'resolved')
        self.assertIsNone(out[0]['exception'])

    # regression: F-go-analysis-ratchet
    def test_ambiguous(self):
        old = record()
        with self.assertRaisesRegex(ValueError, 'ambiguous.*at head'):
            r.compare([dict(old), dict(old)], [old], True)
        with self.assertRaisesRegex(ValueError, 'ambiguous.*at merge base'):
            r.compare([dict(old)], [old, dict(old)], True)
        with self.assertRaisesRegex(ValueError, 'malformed'):
            r.records({})
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            r.records(baseline(old, old))

    # regression: F-go-analysis-ratchet
    def test_shifts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            source = 'package p\nfunc Work() {\n    risky()\n}\n'
            (root / 'a.go').write_text(source)
            issue = dict(FromLinter='errcheck', Text='error ignored', Pos=dict(Filename='a.go', Line=3))
            before = r.normalize(root, '.', 'linux', issue)
            (root / 'a.go').write_text('\n\n' + source)
            issue['Pos']['Line'] += 2
            self.assertEqual(before['id'], r.normalize(root, '.', 'linux', issue)['id'])
            # Two calls on one line and repeated calls in separate branches do not share allowances.
            (root / 'a.go').write_text('package p\nfunc Work() {\n a(); b()\n if first {\n  risky()\n }\n if second {\n  risky()\n }\n}\n')
            issue['Pos'].update(Line=3, Column=2)
            first = r.normalize(root, '.', 'linux', issue)
            issue['Pos']['Column'] = 7
            self.assertNotEqual(first['id'], r.normalize(root, '.', 'linux', issue)['id'])
            issue['Pos'].update(Line=5, Column=3)
            first = r.normalize(root, '.', 'linux', issue)
            issue['Pos']['Line'] = 8
            self.assertNotEqual(first['id'], r.normalize(root, '.', 'linux', issue)['id'])

    # regression: F-go-analysis-sibling-identity
    def test_sibling_independence(self):
        # A repair or addition elsewhere in the file must not re-identify an untouched finding.
        close = '\tdefer resp.Close()'
        fixed = '\tdefer func() { _ = resp.Close() }()'
        head = 'package p\n\n'

        def func(name, line):
            return 'func ' + name + '(resp R) {\n' + line + '\n}\n\n'

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()

            def ident(text, function):
                # Identity of the Close finding inside the named function.
                (root / 'a.go').write_text(text)
                rows = text.splitlines()
                start = next(i for i, row in enumerate(rows) if row.startswith('func ' + function))
                line = next(i for i in range(start, len(rows)) if rows[i] == close) + 1
                issue = dict(FromLinter='errcheck', Text='error ignored', Pos=dict(Filename='a.go', Line=line, Column=2))
                return r.normalize(root, '.', 'linux', issue)['id']

            both = head + func('Alpha', close) + func('Beta', close)
            alpha, beta = ident(both, 'Alpha'), ident(both, 'Beta')
            self.assertNotEqual(alpha, beta)
            self.assertEqual(alpha, ident(head + func('Alpha', close) + func('Beta', fixed), 'Alpha'))
            self.assertEqual(beta, ident(head + func('Alpha', fixed) + func('Beta', close), 'Beta'))
            self.assertEqual(alpha, ident(head + func('Alpha', close) + func('Gamma', close) + func('Beta', close), 'Alpha'))
            self.assertEqual(beta, ident(head + func('Gamma', close) + func('Alpha', close) + func('Beta', close), 'Beta'))

    # regression: F-go-analysis-preamble-anchor
    def test_preamble_finding(self):
        # A finding on the package clause or a comment/directive above the first declaration
        # is a normal finding with a stable identity, never an ambiguous anchor.
        cases = [
            ('//nolint:errcheck // legacy whole-file allowance\npackage p\n\nfunc F() int { return 1 }\n', 1),
            ('//go:build ignore || !ignore\n// +build linux\npackage p\n\nfunc F() int { return 1 }\n', 2),
            ('package p\n\nfunc F() int { return 1 }\n\nfunc G() int { return 2 }\n', 1),
        ]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            for text, line in cases:
                (root / 'a.go').write_text(text)
                issue = dict(FromLinter='nolintlint', Text='unused directive', Pos=dict(Filename='a.go', Line=line, Column=1))
                before = r.normalize(root, '.', 'linux', issue)
                (root / 'a.go').write_text(text + '\nfunc H() int { return 3 }\n')
                self.assertEqual(before['id'], r.normalize(root, '.', 'linux', issue)['id'])


FAKE_FINDING = 'risky()'


class MergeBase(unittest.TestCase):
    """The gate compares the head with its merge base; no committed baseline is read."""

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve() / 'repo'
        self.cache = Path(self.temp.name).resolve() / 'cache'
        self.cache.mkdir()
        subprocess.run(['git', 'init', '-q', '-b', 'main', str(self.root)], check=True)
        for filename in r.POLICY:
            path = self.root / filename
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(OWNER / filename, path)
        self.write('mod/go.mod', 'module example.test/control\n\ngo 1.25.0\n')
        self.write('mod/a.go', 'package a\n\nfunc A() {\n\t' + FAKE_FINDING + '\n}\n')
        self.fork = self.commit('main carries one finding')
        self.fail_scans_under = None

    def tearDown(self):
        self.temp.cleanup()

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def git(self, *args):
        return r.git(self.root, *args).decode().strip()

    def commit(self, message):
        self.git('add', '-A')
        self.git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', message)
        return self.git('rev-parse', 'HEAD')

    def run_gate(self, base):
        # A stand-in analyzer: one errcheck finding per line carrying the planted call.
        real_run = subprocess.run
        fail_under = self.fail_scans_under

        def analyzer(cmd, **kwargs):
            if cmd[0] == 'git':
                return real_run(cmd, **kwargs)
            cwd = Path(kwargs['cwd'])
            output = Path(cmd[cmd.index('--output.json.path') + 1])
            if fail_under and cwd.is_relative_to(self.cache / fail_under):
                return subprocess.CompletedProcess(cmd, 3)
            issues = []
            for source in sorted(cwd.glob('*.go')):
                for number, text in enumerate(source.read_text().splitlines(), 1):
                    if FAKE_FINDING in text:
                        issues.append(dict(FromLinter='errcheck', Text='Error return value is not checked',
                                           Pos=dict(Filename=source.name, Line=number, Column=text.index(FAKE_FINDING) + 1)))
            output.write_text(json.dumps({'Issues': issues, 'Report': {}}))
            return subprocess.CompletedProcess(cmd, 1 if issues else 0)
        with mock.patch.object(r, 'verify_tool', return_value=(Path('/fixture/analyzer'), {}, {})), \
                mock.patch.object(r.subprocess, 'run', side_effect=analyzer):
            code = r.lint(self.root, self.cache, OWNER, base)
        report = json.loads((self.cache / 'report.json').read_text())
        self.assertFalse((self.cache / 'merge-base').exists(), 'merge-base checkout left behind')
        self.assertNotIn('merge-base', self.git('worktree', 'list'))
        return code, report

    def branch(self, name):
        self.git('checkout', '-q', '-b', name, self.fork)

    # regression: #7421
    def test_adds_finding(self):
        # (a) A PR that adds a finding goes red.
        self.branch('adds')
        self.write('mod/b.go', 'package a\n\nfunc B() {\n\t' + FAKE_FINDING + '\n}\n')
        self.commit('add a finding')
        code, report = self.run_gate(self.fork)
        self.assertEqual(code, 1)
        self.assertEqual(report['gate'], 'fail')
        self.assertEqual(report['totals'], dict(existing=1, new=1, worsened=0, resolved=0))
        self.assertEqual([f['path'] for f in report['findings'] if f['disposition'] == 'new'], ['mod/b.go'])
        self.assertEqual(report['merge_base_sha'], self.fork)

    # regression: #7421
    def test_untouched_debt_stays_green(self):
        # (b) A PR touching no finding stays green while main carries findings.
        self.branch('quiet')
        self.write('mod/c.go', 'package a\n\nfunc C() int { return 1 }\n')
        self.commit('touch no finding')
        code, report = self.run_gate(self.fork)
        self.assertEqual(code, 0)
        self.assertEqual(report['state'], 'complete')
        self.assertEqual(report['totals'], dict(existing=1, new=0, worsened=0, resolved=0))

    # regression: #7421
    def test_fix_needs_no_allowance_edit(self):
        # (c) A PR that fixes a pre-existing finding stays green; no baseline file is involved.
        self.branch('fixes')
        self.write('mod/a.go', 'package a\n\nfunc A() {\n\t_ = 1\n}\n')
        self.commit('repair the finding')
        self.assertFalse((self.root / r.BASELINE).exists())
        code, report = self.run_gate(self.fork)
        self.assertEqual(code, 0)
        self.assertEqual(report['totals'], dict(existing=0, new=0, worsened=0, resolved=1))

    # regression: #7421
    def test_base_moved_after_fork(self):
        # Main gains a finding after the PR forked: the PR compares with its own merge base,
        # so it stays green before and after it merges main in.
        self.git('checkout', '-q', 'main')
        self.write('mod/z.go', 'package a\n\nfunc Z() {\n\t' + FAKE_FINDING + '\n}\n')
        tip = self.commit('main adds a finding after the fork')
        self.branch('stale')
        self.write('mod/c.go', 'package a\n\nfunc C() int { return 1 }\n')
        self.commit('touch no finding')
        code, report = self.run_gate(tip)
        self.assertEqual((code, report['merge_base_sha']), (0, self.fork))
        self.assertEqual(report['totals'], dict(existing=1, new=0, worsened=0, resolved=0))
        self.git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                 'merge', '-q', '--no-edit', 'main')
        code, report = self.run_gate(tip)
        self.assertEqual((code, report['merge_base_sha']), (0, tip))
        self.assertEqual(report['totals'], dict(existing=2, new=0, worsened=0, resolved=0))

    # regression: #7421
    def test_policy_at_merge_base(self):
        # The merge base is analyzed under the same reviewed policy, or not at all.
        self.git('checkout', '-q', 'main')
        self.write('.golangci.yml', 'version: "2"\n')
        fork = self.commit('main under another policy')
        self.git('checkout', '-q', '-b', 'restores', fork)
        shutil.copyfile(OWNER / '.golangci.yml', self.root / '.golangci.yml')
        self.commit('head under the runner policy')
        code, report = self.run_gate(fork)
        self.assertEqual((code, report['state']), (2, 'could-not-check'))
        self.assertEqual(report['totals'], dict(existing=0, new=0, worsened=0, resolved=0))

    # regression: #7421
    def test_merge_base_scan_failure(self):
        # An incomplete merge-base scan cannot pass the head or resolve anything.
        self.branch('fixes')
        self.write('mod/a.go', 'package a\n\nfunc A() {\n\t_ = 1\n}\n')
        self.commit('repair the finding')
        self.fail_scans_under = 'merge-base'
        code, report = self.run_gate(self.fork)
        self.assertEqual((code, report['state'], report['gate']), (2, 'could-not-check', 'fail'))
        self.assertEqual(report['coverage'][0]['state'], 'complete')
        self.assertEqual(report['base_coverage'][0]['state'], 'could-not-check')
        self.assertFalse(any(f['disposition'] == 'resolved' for f in report['findings']))

    # regression: #7421
    def test_base_required(self):
        self.branch('quiet')
        for base in ['', 'main', 'f' * 40]:
            code, report = self.run_gate(base)
            self.assertEqual((code, report['state']), (2, 'could-not-check'), base)
        orphan = self.git('commit-tree', self.git('rev-parse', 'HEAD^{tree}'), '-m', 'unrelated')
        code, _ = self.run_gate(orphan)
        self.assertEqual(code, 2)

    # regression: #7421
    def test_debt_report(self):
        # The periodic report lists main's debt, annotates it from the retained register,
        # and never fails on findings; only an incomplete scan fails it.
        real_run = subprocess.run
        verdict = {'exit': 1}

        def analyzer(cmd, **kwargs):
            if cmd[0] == 'git':
                return real_run(cmd, **kwargs)
            output = Path(cmd[cmd.index('--output.json.path') + 1])
            issue = dict(FromLinter='errcheck', Text='Error return value is not checked',
                         Pos=dict(Filename='a.go', Line=4, Column=2))
            output.write_text(json.dumps({'Issues': [issue], 'Report': {}}))
            return subprocess.CompletedProcess(cmd, verdict['exit'])
        with mock.patch.object(r, 'verify_tool', return_value=(Path('/fixture/analyzer'), {}, {})), \
                mock.patch.object(r.subprocess, 'run', side_effect=analyzer):
            code = r.debt(self.root, self.cache, OWNER)
            report = json.loads((self.cache / 'report.json').read_text())
            self.assertEqual((code, report['mode'], report['gate']), (0, 'debt-report', 'pass'))
            self.assertEqual(report['totals']['existing'], 1)
            self.assertEqual(report['debt'], dict(recorded=0, unrecorded=1))
            found = report['findings'][0]
            entry = dict({k: found[k] for k in ['id', 'diagnostic', 'analyzer', 'rule', 'module',
                                                 'configuration', 'path', 'anchor']},
                         first_seen='2026-11-23', owner='maintainers', work='https://example.invalid/w', reason='debt')
            self.write(r.BASELINE, json.dumps(dict(schema='go-analysis-baseline/v1', findings=[entry])))
            code = r.debt(self.root, self.cache, OWNER)
            report = json.loads((self.cache / 'report.json').read_text())
            self.assertEqual(report['debt'], dict(recorded=1, unrecorded=0))
            self.assertEqual(report['findings'][0]['exception']['owner'], 'maintainers')
            verdict['exit'] = 2
            code = r.debt(self.root, self.cache, OWNER)
            report = json.loads((self.cache / 'report.json').read_text())
            self.assertEqual((code, report['state'], report['gate']), (2, 'could-not-check', 'fail'))


class Harness(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)

    def tearDown(self):
        self.temp.cleanup()

    def module(self, name):
        target = self.root / name
        target.mkdir(parents=True)
        (target / 'go.mod').write_text('module example.test/control\n\ngo 1.25.0\n')
        subprocess.run(['git', '-C', str(self.root), 'add', '--', name + '/go.mod'], check=True)
        return target

    # regression: F-go-analysis-discovery
    def test_discovery(self):
        with self.assertRaisesRegex(ValueError, 'no tracked'):
            r.modules(self.root)
        self.module('a module')
        self.module('new')
        self.assertEqual(r.modules(self.root), ['a module', 'new'])
        self.assertEqual(len(r.coverage(r.modules(self.root))), 2)
        self.module('tools/desk')
        self.assertEqual(len(r.coverage(r.modules(self.root))), 6)
        (self.root / 'new/go.mod').unlink()
        with self.assertRaisesRegex(ValueError, 'missing'):
            r.modules(self.root)
        with tempfile.TemporaryDirectory() as outside:
            (Path(outside) / 'go.mod').write_text('module outside')
            (self.root / 'escape').symlink_to(outside, target_is_directory=True)
            with mock.patch.object(r, 'git', return_value=b'escape/go.mod\0'):
                with self.assertRaisesRegex(ValueError, 'escapes'):
                    r.modules(self.root)

    # regression: F-go-analysis-pins
    def test_bad_checksum(self):
        target = self.root / 'download'
        target.write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, 'checksum'):
            r.download('https://example.invalid/never-contact', target, '0' * 64)
        with self.assertRaises(FileNotFoundError):
            r.verify_tool(self.root, OWNER)
        (self.root / 'receipt.json').write_text(json.dumps({'pins_sha256': 'bad'}))
        with self.assertRaisesRegex(ValueError, 'pin changed'):
            r.verify_tool(self.root, OWNER)
        (self.root / 'receipt.json').write_text(json.dumps({'pins_sha256': r.digest((OWNER / r.POLICY[1]).read_bytes()), 'binary_sha256': 'bad'}))
        with self.assertRaises(FileNotFoundError):
            r.verify_tool(self.root, OWNER)

    # regression: F-go-analysis-pins
    def test_offline(self):
        env = r.environment(self.root, self.root)
        self.assertEqual(Path(env['TMPDIR']), self.root / 'tmp')
        self.assertTrue(Path(env['TMPDIR']).is_dir())
        self.assertEqual(env['GOPROXY'], 'off')
        self.assertEqual(env['GOTOOLCHAIN'], 'local')
        self.assertEqual(env['GOWORK'], 'off')
        self.assertEqual(env['GOVCS'], '*:off')
        self.assertEqual(env['GOSUMDB'], 'sum.golang.org ' + (self.root / 'offline-sumdb').as_uri())
        self.assertEqual(r.environment(self.root, self.root, network=True)['GOSUMDB'], 'sum.golang.org')

    # regression: F-go-analysis-offline-sumdb
    def test_sumdb_cache_miss(self):
        prepared = Path(os.environ.get('GO_ANALYSIS_TEST_CACHE', str(OWNER / '.go-analysis')))
        available = prepared / 'modules/cache/download/golang.org/x/mod/@v'
        archive = sorted(available.glob('*.zip'))[0]
        version = archive.name[:-4]
        cache = self.root / 'isolated-cache'
        target = cache / 'modules/cache/download/golang.org/x/mod/@v'
        target.mkdir(parents=True)
        for suffix in ['.zip', '.mod', '.info']:
            shutil.copyfile(available / (version + suffix), target / (version + suffix))
        fixture = self.module('cache-miss')
        attempts = []

        class Trap(http.server.BaseHTTPRequestHandler):
            def do_CONNECT(self):
                attempts.append(self.path)
                self.send_error(502, 'offline control intercepted transport')

            def log_message(self, *_args):
                pass

        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Trap)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            env = r.environment(cache, prepared / 'analysis_go/go')
            proxy = 'http://127.0.0.1:' + str(server.server_port)
            env.update(HTTPS_PROXY=proxy, HTTP_PROXY=proxy, ALL_PROXY=proxy,
                       https_proxy=proxy, http_proxy=proxy, all_proxy=proxy, NO_PROXY='', no_proxy='')
            result = subprocess.run([str(prepared / 'analysis_go/go/bin/go'), 'mod', 'download',
                                     '-json', 'golang.org/x/mod@' + version], cwd=fixture, env=env,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=25)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(attempts, [], 'offline checksum cache miss attempted transport')
            self.assertTrue(json.loads(result.stdout)['Error'])
            self.assertFalse((target / (version + '.ziphash')).exists())
            print('REAL checksum cache miss: local failure, zero transport attempts', flush=True)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    # regression: F-go-analysis-propagation
    def test_scan_failures(self):
        for name in ['first', 'middle', 'last']:
            self.module(name)
        for filename in r.POLICY:
            path = self.root / filename
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(OWNER / filename, path)
        subprocess.run(['git', '-C', str(self.root), 'add', '.'], check=True)
        subprocess.run(['git', '-C', str(self.root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                        'commit', '-qm', 'fixture'], check=True)
        head = r.git(self.root, 'rev-parse', 'HEAD').decode().strip()
        cache = self.root / '.go-analysis'
        real_run = subprocess.run
        for broken in ['first', 'middle', 'last', 'malformed', 'missing']:
            def scan(cmd, **kwargs):
                if cmd[0] == 'git':
                    return real_run(cmd, **kwargs)
                output = Path(cmd[cmd.index('--output.json.path') + 1])
                name = kwargs['cwd'].name
                if broken != 'missing':
                    output.write_text('invalid' if broken == 'malformed' else json.dumps({'Issues': [], 'Report': {}}))
                return subprocess.CompletedProcess(cmd, 2 if name == broken else 0)
            with mock.patch.object(r, 'verify_tool', return_value=(Path('/fixture/analyzer'), {}, {})), mock.patch.object(r.subprocess, 'run', side_effect=scan):
                code = r.lint(self.root, cache, OWNER, head)
            report = json.loads((cache / 'report.json').read_text())
            self.assertNotEqual(code, 0, broken)
            self.assertEqual(report['state'], 'could-not-check', broken)
            self.assertEqual(len(report['coverage']), 3, broken)
            self.assertTrue(any(x['state'] == 'could-not-check' for x in report['coverage']), broken)

    # regression: F-go-analysis-related-notes
    def test_related_notes(self):
        target = self.module('mod')
        (target / 'a.go').write_text('package a\n\nfunc F() {\n\tc, e := g()\n\t_ = e != nil\n\t_ = e == nil\n\t_ = c\n\t_ = c\n}\n')
        for filename in r.POLICY:
            path = self.root / filename
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(OWNER / filename, path)
        subprocess.run(['git', '-C', str(self.root), 'add', '.'], check=True)
        subprocess.run(['git', '-C', str(self.root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                        'commit', '-qm', 'fixture'], check=True)
        head = r.git(self.root, 'rev-parse', 'HEAD').decode().strip()
        note = dict(FromLinter='staticcheck', Text='SA4023(related information): the lhs of the comparison',
                    Pos=dict(Filename='a.go', Line=4, Column=2))
        cases = {'paired': ([dict(note), dict(note),
                             dict(note, Text='SA4023: this comparison is always true', Pos=dict(Filename='a.go', Line=5, Column=6)),
                             dict(note, Text='SA4023: this comparison is never true', Pos=dict(Filename='a.go', Line=6, Column=6)),
                             dict(note, FromLinter='govet', Text='SA4023(related information): other linter', Pos=dict(Filename='a.go', Line=7, Column=2)),
                             dict(note, Text='SA4006: see SA4023(related information): mid-text', Pos=dict(Filename='a.go', Line=8, Column=2))], 0),
                 'orphan': ([dict(note)], 2),
                 'foreign': ([dict(note), dict(note, FromLinter='govet', Text='SA4023: other linter primary', Pos=dict(Filename='a.go', Line=5, Column=6))], 2)}
        real_run = subprocess.run
        for name, (issues, expected) in cases.items():
            def scan(cmd, **kwargs):
                if cmd[0] == 'git':
                    return real_run(cmd, **kwargs)
                output = Path(cmd[cmd.index('--output.json.path') + 1])
                output.write_text(json.dumps({'Issues': issues, 'Report': {}}))
                return subprocess.CompletedProcess(cmd, 1)
            cache = self.root / ('.cache-' + name)
            with mock.patch.object(r, 'verify_tool', return_value=(Path('/fixture/analyzer'), {}, {})), mock.patch.object(r.subprocess, 'run', side_effect=scan):
                code = r.lint(self.root, cache, OWNER, head)
            report = json.loads((cache / 'report.json').read_text())
            self.assertEqual(code, expected, name)
            if name == 'paired':
                self.assertEqual(report['state'], 'complete')
                self.assertEqual(sorted(f['line'] for f in report['findings']), [5, 6, 7, 8])
                self.assertEqual(report['coverage'][0]['related'], 2)
                # The head is its own merge base (a push to main): debt is existing, never new.
                self.assertEqual(report['totals']['existing'], 4)
            else:
                self.assertEqual(report['state'], 'could-not-check')

    # regression: F-go-analysis-controls
    def test_real_analyzers(self):
        cache = Path(os.getenv('GO_ANALYSIS_TEST_CACHE', str(OWNER / '.go-analysis'))).resolve()
        binary, env, _ = r.verify_tool(cache, OWNER)
        target = self.module('real')
        fixture = OWNER / 'scripts/tests/testdata/go-analysis'
        for variant in ['bad', 'good']:
            shutil.copyfile(fixture / (variant + '.go.txt'), target / 'code.go')
            output = self.root / (variant + '.json')
            proc = subprocess.run([str(binary), 'run', '--config', str(OWNER / '.golangci.yml'),
                                   '--output.json.path', str(output), './...'], cwd=target, env=env,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180)
            payload = json.loads(output.read_bytes())
            issues = payload.get('Issues') or []
            print('REAL', variant, 'exit', proc.returncode, 'analyzers', sorted({i['FromLinter'] for i in issues}), flush=True)
            self.assertEqual(proc.returncode, 1 if variant == 'bad' else 0, proc.stderr.decode())
            if variant == 'bad':
                self.assertTrue({'errcheck', 'staticcheck', 'unused', 'nolintlint'} <= {i['FromLinter'] for i in issues})
            else:
                self.assertEqual(issues, [])


if __name__ == '__main__':
    unittest.main(verbosity=2)
