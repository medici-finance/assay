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


class Ratchet(unittest.TestCase):
    # regression: F-go-analysis-ratchet
    def test_legacy(self):
        old = record()
        out = r.compare([copy.deepcopy(old)], baseline(old), baseline(old), True, 'policy')
        self.assertEqual(out[0]['disposition'], 'existing')

    # regression: F-go-analysis-ratchet
    def test_new_and_worse(self):
        old = record()
        new = record('two')
        out = r.compare([new], baseline(), baseline(old), True, 'policy')
        self.assertEqual([f['disposition'] for f in out], ['new', 'resolved'])
        worse = dict(old, diagnostic=r.digest(b'worse'))
        self.assertEqual(r.compare([worse], baseline(old), baseline(old), True, 'policy')[0]['disposition'], 'worsened')
        self.assertEqual(r.compare([old], baseline(), baseline(), True, 'policy')[0]['disposition'], 'new')

    # regression: F-go-analysis-ratchet
    def test_baseline_growth(self):
        old = record()
        with self.assertRaisesRegex(ValueError, 'expansion'):
            r.compare([old], baseline(old), baseline(), True, 'policy')
        changed = dict(old, diagnostic='new')
        with self.assertRaisesRegex(ValueError, 'rewrite'):
            r.compare([changed], baseline(changed), baseline(old), True, 'policy')
        with self.assertRaisesRegex(ValueError, 'migration'):
            r.compare([old], baseline(old), baseline(old), True, 'changed')

    # regression: F-go-analysis-ratchet
    def test_resolved_allowance(self):
        old = record()
        with self.assertRaisesRegex(ValueError, 'retain'):
            r.compare([], baseline(old), baseline(old), True, 'policy')
        self.assertEqual(r.compare([], baseline(), baseline(old), False, 'policy'), [])
        self.assertEqual(r.compare([], baseline(), baseline(old), True, 'policy')[0]['disposition'], 'resolved')

    # regression: F-go-analysis-ratchet
    def test_ambiguous(self):
        old = record()
        with self.assertRaisesRegex(ValueError, 'ambiguous'):
            r.compare([dict(old), dict(old)], baseline(old), baseline(old), True, 'policy')
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
        debt = dict(schema='go-analysis-baseline/v1', contract=r.contract(OWNER), findings=[])
        (self.root / r.BASELINE).write_text(json.dumps(debt))
        subprocess.run(['git', '-C', str(self.root), 'add', '.'], check=True)
        subprocess.run(['git', '-C', str(self.root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                        'commit', '-qm', 'fixture'], check=True)
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
                code = r.lint(self.root, cache, OWNER, '', self.root)
            report = json.loads((cache / 'report.json').read_text())
            self.assertNotEqual(code, 0, broken)
            self.assertEqual(report['state'], 'could-not-check', broken)
            self.assertEqual(len(report['coverage']), 3, broken)
            self.assertTrue(any(x['state'] == 'could-not-check' for x in report['coverage']), broken)

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
