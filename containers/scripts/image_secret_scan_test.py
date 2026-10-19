"""Security contracts independent of shell/grep implementation details.

Run unit tests without Docker; integration tests use the pinned scanner image,
no network and runtime-generated synthetic credentials. No real credential is
committed or verified against a provider.
"""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import random
import string
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('scan', os.environ.get('IMAGE_SCAN_IMPL', HERE / 'image_secret_scan.py'))
scan = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scan)


def tar_bytes(files):
    out = io.BytesIO()
    with tarfile.open(fileobj=out, mode='w') as t:
        for name, data in files:
            m = tarfile.TarInfo(name)
            if isinstance(data, tarfile.TarInfo):
                t.addfile(data)
            else:
                m.size = len(data)
                m.mode = 0  # tar reads must not depend on image permissions
                t.addfile(m, io.BytesIO(data))
    return out.getvalue()


def image_bytes(layers, config=None, extra=()):
    files = []
    names = []
    for data in layers:
        name = 'blobs/sha256/' + hashlib.sha256(data).hexdigest()
        files.append((name, data))
        names.append(name)
    files += [('config.json', json.dumps(config or {'config': {}, 'history': []}).encode()),
              ('manifest.json', json.dumps([{'Config': 'config.json', 'Layers': names}]).encode())]
    return tar_bytes(files + list(extra))


def token(prefix='ghp', n=36):
    rng = random.Random(7123)
    return (prefix + '_' + ''.join(rng.choices(string.ascii_letters + string.digits, k=n))).encode()


def canary_report(name='canary.txt'):
    return [{'File': name, 'RuleID': 'github-pat', 'StartLine': 1}]


class Traversal(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def surfaces(self, data):
        p = self.root / 'image.tar'
        p.write_bytes(data)
        return list(scan.surfaces(p))

    def test_every_layer_survives_overwrite_and_whiteout(self):
        key = token()
        rows = self.surfaces(image_bytes([
            tar_bytes([('app/key', key), ('app/gone', key)]),
            tar_bytes([('app/key', b'clean'), ('app/.wh.gone', b'')]),
        ]))
        self.assertEqual(sum(data == key for _, data in rows), 2)

    def test_symlink_and_directory_replacement_do_not_hide_content(self):
        link = tarfile.TarInfo('app'); link.type = tarfile.SYMTYPE; link.linkname = '/outside'
        directory = tarfile.TarInfo('app/key'); directory.type = tarfile.DIRTYPE
        rows = self.surfaces(image_bytes([
            tar_bytes([('app', link), ('app/key', token())]),
            tar_bytes([('app/key', directory)])]))
        self.assertTrue(any(data == token() for _, data in rows))
        self.assertFalse((self.root / 'outside').exists())

    def test_paths_are_not_extracted_or_executed(self):
        rows = self.surfaces(image_bytes([tar_bytes([
            ('../../outside', token()), ('line\nname', token()),
            ('app/.gitleaksignore', b'*'), ('app/.gitleaks.toml', b'disabled = true')])]))
        self.assertEqual(sum(data == token() for _, data in rows), 2)
        self.assertFalse((self.root.parent / 'outside').exists())

    def test_config_and_history_are_scanned(self):
        config = {'config': {'Env': ['TOKEN=' + token().decode()]},
                  'history': [{'created_by': 'RUN TOKEN=' + token().decode()}]}
        rows = self.surfaces(image_bytes([tar_bytes([('clean', b'clean')])], config))
        self.assertIn(token(), rows[0][1])

    def test_bad_archives_never_clean(self):
        good_layer = tar_bytes([('app/key', token())])
        samples = [b'not a tar', tar_bytes([]), image_bytes([]),
                   tar_bytes([('manifest.json', b'not json')]),
                   tar_bytes([('manifest.json', b'[]')]),
                   image_bytes([b'not a layer']),
                   image_bytes([good_layer[:530]])]
        for data in samples:
            with self.subTest(size=len(data)), self.assertRaises((scan.Incomplete, tarfile.TarError, ValueError, KeyError)):
                self.surfaces(data)

    def test_missing_layer_and_config(self):
        for key in ['Config', 'Layers']:
            entry = {'Config': 'config.json', 'Layers': ['layer.tar']}
            data = tar_bytes([('manifest.json', json.dumps([entry]).encode()),
                              ('layer.tar', tar_bytes([('x', b'clean')]))] if key == 'Config' else
                             [('manifest.json', json.dumps([entry]).encode()), ('config.json', b'{}')])
            with self.subTest(key=key), self.assertRaises(scan.Incomplete):
                self.surfaces(data)

    def test_corrupt_digest(self):
        layer = tar_bytes([('key', token())])
        name = 'blobs/sha256/' + '0' * 64
        data = tar_bytes([(name, layer), ('config.json', b'{}'),
                          ('manifest.json', json.dumps([{'Config': 'config.json', 'Layers': [name]}]).encode())])
        with self.assertRaisesRegex(scan.Incomplete, 'digest'):
            self.surfaces(data)

    def test_duplicate_save_members_refused(self):
        data = image_bytes([tar_bytes([('x', b'clean')])], extra=[('config.json', b'{}')])
        with self.assertRaisesRegex(scan.Incomplete, 'duplicate'):
            self.surfaces(data)

    def test_raw_bytes_preserved_including_binary_magic(self):
        src = b'\x7fELF\0' + token() + b'\0'
        index = scan.project([('binary', src)], self.root)
        self.assertEqual((self.root / next(iter(index))).read_bytes(), src)

    def test_input_identity_changes_with_any_content_or_path_change(self):
        keys = []
        for label, data in [('file', token()), ('file', token() + b'changed'), ('other', token())]:
            keys.append(next(iter(scan.project([(label, data)], self.root))))
        self.assertEqual(len(set(keys)), 3)

    def test_identical_file_in_another_layer_keeps_content_identity(self):
        a = scan.project([('layer first: /file', token())], self.root)
        b = scan.project([('layer second: /file', token())], self.root)
        self.assertEqual(set(a), set(b))


class Refusals(unittest.TestCase):
    def test_missing_canary_is_incomplete(self):
        with self.assertRaises(scan.Incomplete):
            scan.verdict([], {'00000000.txt': 'source'})

    def test_real_findings_are_not_swallowed(self):
        findings = canary_report() + [{'File': '00000000.txt', 'RuleID': 'private-key'}]
        self.assertEqual(scan.verdict(findings, {'00000000.txt': 'config'}), [('config', 'private-key')])

    def test_unknown_input_is_incomplete(self):
        with self.assertRaises(scan.Incomplete):
            scan.verdict(canary_report() + [{'File': 'unknown', 'RuleID': 'private-key'}], {})

    def check_report_refused(self, code, body, exception):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / 'inputs'; p.mkdir()
            result = subprocess.CompletedProcess([], code, body, b'')
            with patch.object(scan.subprocess, 'run', return_value=result):
                with self.assertRaises(exception):
                    scan.gitleaks(p)

    def test_scanner_exit_error_is_incomplete(self):
        self.check_report_refused(2, b'[]', scan.Incomplete)

    def test_malformed_report_is_incomplete(self):
        self.check_report_refused(0, b'not json', ValueError)

    def test_non_list_report_is_incomplete(self):
        self.check_report_refused(0, b'{}', scan.Incomplete)

    def test_report_exit_disagreement_is_incomplete(self):
        self.check_report_refused(0, json.dumps(canary_report()).encode(), scan.Incomplete)
        self.check_report_refused(9, b'[]', scan.Incomplete)

    def test_scan_never_pulls_and_has_no_host_config_or_network(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / 'inputs'; p.mkdir()
            result = subprocess.CompletedProcess([], 9, json.dumps(canary_report()).encode(), b'')
            with patch.object(scan.subprocess, 'run', return_value=result) as run:
                scan.gitleaks(p)
            args = run.call_args.args[0]
            self.assertIn('never', args)
            self.assertIn('none', args)
            self.assertIn('--ignore-gitleaks-allow', args)
            self.assertIn('--redact', args)
            self.assertIn('@sha256:', scan.SCANNER_IMAGE)
            self.assertNotIn('-e', args)
            self.assertNotIn('pull', args)

    def test_tool_error_never_echoes_secret_output(self):
        with patch.object(scan.subprocess, 'run', return_value=subprocess.CompletedProcess([], 2, token(), token())):
            with self.assertRaises(scan.Incomplete) as exc:
                scan.command(['docker', 'save', 'fixture'])
            self.assertNotIn(token().decode(), str(exc.exception))

    def test_scanner_error_with_findings_is_still_incomplete(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / 'inputs'; p.mkdir()
            result = subprocess.CompletedProcess([], 9, json.dumps(canary_report()).encode(), b'read error')
            with patch.object(scan.subprocess, 'run', return_value=result), self.assertRaises(scan.Incomplete):
                scan.gitleaks(p)

    def test_main_returns_two_and_redacts_parser_errors(self):
        out = io.StringIO()
        with patch.object(scan.sys, 'argv', ['scan', 'fixture']), patch.object(scan, 'image_archive', side_effect=ValueError(token().decode())), contextlib.redirect_stderr(out):
            self.assertEqual(scan.main(), 2)
        self.assertNotIn(token().decode(), out.getvalue())


@unittest.skipUnless(os.environ.get('IMAGE_SCAN_INTEGRATION') == '1', 'set IMAGE_SCAN_INTEGRATION=1 for the pinned OSS engine')
class Engine(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.pem = subprocess.run(['openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048'], check=True, capture_output=True).stdout

    def check(self, items):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d); inputs = root / 'inputs'; inputs.mkdir()
            index = scan.project(items, inputs)
            return scan.verdict(scan.gitleaks(inputs), index)

    def test_provider_formats_in_source_and_packaging_text(self):
        rng = random.Random(7)
        alphabet = string.ascii_letters + string.digits + '_-'
        anthro = ('sk-' + 'ant-api03-' + ''.join(rng.choices(alphabet, k=93)) + 'AA').encode()
        openai = ('sk-' + 'proj-' + ''.join(rng.choices(alphabet, k=74)) + 'T3BlbkFJ' + ''.join(rng.choices(alphabet, k=74))).encode()
        fine_alphabet = string.ascii_letters + string.digits
        fine = ('github_' + 'pat_' + ''.join(rng.choices(fine_alphabet, k=22)) + '_' + ''.join(rng.choices(fine_alphabet, k=59))).encode()
        items = [('source.go', b'package main\nvar credential = "' + token() + b'"\n'),
                 ('file', self.pem), ('ANTHROPIC_API_KEY', anthro), ('OPENAI_API_KEY', openai),
                 ('app-token', token('ghs')), ('fine-token', fine)]
        findings = self.check(items)
        self.assertEqual({label for label, _ in findings}, {label for label, _ in items})

    def test_config_and_history_and_earlier_layers(self):
        config = {'config': {'Env': ['TOKEN=' + token().decode()]},
                  'history': [{'created_by': 'RUN KEY=' + token('ghs').decode()}]}
        with tempfile.TemporaryDirectory() as d:
            root = Path(d); archive = root / 'image.tar'
            archive.write_bytes(image_bytes([
                tar_bytes([('app/key', self.pem), ('app/gone', token())]),
                tar_bytes([('app/key', b'clean'), ('app/.wh.gone', b'')])], config))
            findings = scan.scan_archive(archive, root)
        self.assertTrue(any(label == 'image config/history' for label, _ in findings))
        self.assertTrue(any('/app/key' in label for label, _ in findings))
        self.assertTrue(any('/app/gone' in label for label, _ in findings))

    def test_same_file_multiple_keys_and_inline_ignore(self):
        content = token() + b' # gitleaks:allow\n' + self.pem
        findings = self.check([('file', content), ('.gitleaksignore', b'*')])
        self.assertEqual({rule for label, rule in findings if label == 'file'}, {'github-pat', 'private-key'})

    def test_image_control_files_cannot_disable_detection(self):
        items = [('.gitleaks.toml', b'[extend]\nuseDefault = false\n# ' + token()),
                 ('.gitleaksignore', b'*\n' + token())]
        self.assertEqual({label for label, _ in self.check(items)}, {label for label, _ in items})

    def test_clean_label_strings_need_no_exceptions(self):
        # Include both historical and changed compiler neighbors, in ordinary
        # text, without baking a credential-shaped constant into the source.
        labels = [b'desk-' + b'decidedunverifiablecannot', b'desk-' + b'decidedunverifiablecheck',
                  b'desk-' + b'decidedhasPreviousPageGITLAB', b'desk-' + b'decidedunclassifiedunverifiable']
        self.assertEqual(self.check([(str(i), text) for i, text in enumerate(labels)]), [])

    def test_normal_binary_handling_is_explicit(self):
        # Compiled-code inspection is intentionally outside this gate. Source
        # and packaged text are scanned; never disguise binaries as text.
        self.assertEqual(self.check([('binary', b'\x7fELF' + b'\0' * 100 + token())]), [])

    def test_baseline_cannot_clear_a_changed_file_or_another_path(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d); inputs = root / 'inputs'; inputs.mkdir()
            index = scan.project([('file', token())], inputs)
            report = scan.gitleaks(inputs)
            baseline = root / 'baseline.json'
            baseline.write_text(json.dumps([r for r in report if Path(r['File']).name != 'canary.txt']))
            with patch.object(scan, 'BASELINE', baseline):
                self.assertEqual(self.check([('file', token())]), [])
                # Same length, line and character counts (identical entropy).
                # The redacted baseline does not compare secret values;
                # content binding must prevent it from hiding this change.
                changed = bytearray(token())
                other = next(i for i in range(5, len(changed)) if changed[i] != changed[4])
                changed[4], changed[other] = changed[other], changed[4]
                self.assertTrue(self.check([('file', bytes(changed))]))
                self.assertTrue(self.check([('another-path', token())]))


if __name__ == '__main__':
    unittest.main()
