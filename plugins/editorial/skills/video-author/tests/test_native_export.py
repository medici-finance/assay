"""Offline native-export contract tests; no TTS, browser, GPU or uploads."""
import json
import io
from contextlib import contextmanager, redirect_stderr
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'tools'))
import native_export as native


class NativeInputs(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.manifest_path = self.root / 'manifest.json'
        (self.root / 'SCRIPT.md').write_text('# Film\n\n| id | narration |\n|---|---|\n| a | First. |\n| b | Second. |\n')
        (self.root / 'preview.html').write_text('window.FALLBACK=[2.0,3.0];')
        (self.root / 'native').write_text('#!/bin/sh\nexit 0\n')
        (self.root / 'native').chmod(0o755)
        (self.root / 'reference.mp4').write_bytes(b'not invoked by input validation')
        self.timing = {'runtime': 5., 'scenes': [
            {'id': 'a', 'narration': 'First.', 'start': 0., 'duration': 2.},
            {'id': 'b', 'narration': 'Second.', 'start': 2., 'duration': 3.}]}
        self.manifest = {'version': 1, 'format': {'width': 1920, 'height': 1080, 'fps': 30, 'tail_seconds': .4},
                         'authored_estimates': [2., 3.], 'inputs': {}}
        self.save()

    def save(self):
        (self.root / 'timeline.json').write_text(json.dumps(self.timing))
        for role, name in [('script', 'SCRIPT.md'), ('preview', 'preview.html'),
                           ('native', 'native'), ('reference', 'reference.mp4'), ('timeline', 'timeline.json')]:
            self.manifest['inputs'][role] = {'path': name, 'sha256': native.sha256(self.root / name)}
        self.manifest_path.write_text(json.dumps(self.manifest))

    def test_valid_input_frame_rounding(self):
        self.timing['runtime'] = 5.01
        self.timing['scenes'][1]['duration'] = 3.01
        self.save()
        self.assertEqual(native.load_manifest(self.manifest_path)[2], 163)

    def test_stale_build_and_audio_rejected_before_execution(self):
        for role in ('native', 'reference.mp4', 'preview.html', 'timeline.json', 'SCRIPT.md'):
            with self.subTest(role=role):
                self.save()
                path = self.root / role
                original = path.read_bytes()
                path.write_bytes(original + b'changed')
                with patch.object(native, 'command') as command:
                    with self.assertRaisesRegex(ValueError, 'stale input'):
                        native.export(self.manifest_path, self.root / 'out.mp4')
                    command.assert_not_called()
                path.write_bytes(original)

    def test_wrong_scene_identity_narration_and_gap(self):
        for key, value in [('id', 'a'), ('narration', 'Different.'), ('start', 2.5)]:
            with self.subTest(key=key):
                original = self.timing['scenes'][1][key]
                self.timing['scenes'][1][key] = value
                self.save()
                with self.assertRaises(ValueError): native.load_manifest(self.manifest_path)
                self.timing['scenes'][1][key] = original

    def test_duration_must_be_finite_positive_and_not_boolean(self):
        for duration in (float('nan'), float('inf'), -1, 0, True):
            with self.subTest(duration=duration):
                self.timing['scenes'][1]['duration'] = duration
                self.timing['runtime'] = 2 + duration
                self.save()
                with self.assertRaisesRegex(ValueError, 'duration must be finite'):
                    native.load_manifest(self.manifest_path)

    def test_per_scene_ceiling_independent_of_total_ceiling(self):
        self.manifest['authored_estimates'] = [2., 30.]
        (self.root / 'preview.html').write_text('window.FALLBACK=[2.0,30.0];')
        self.timing['scenes'][0]['duration'] = 12.
        self.timing['scenes'][1].update(start=12., duration=25.)
        self.timing['runtime'] = 37.  # below total ceiling 32 * 1.3
        self.save()
        with self.assertRaisesRegex(ValueError, 'take-sanity failed at a'):
            native.load_manifest(self.manifest_path)

    def test_total_ceiling_independent_of_scene_ceiling(self):
        self.timing['scenes'][0]['duration'] = 3.
        self.timing['scenes'][1].update(start=3., duration=4.)
        self.timing['runtime'] = 7.
        self.save()
        with self.assertRaisesRegex(ValueError, 'take-sanity failed for total duration'):
            native.load_manifest(self.manifest_path)

    @contextmanager
    def export_backend(self, mode='reference', failure=None):
        """Exercise orchestration with file-producing commands and measured probes.

        These stand-ins test wiring/rollback, not media codecs or GPU correctness.
        """
        if mode == 'mix':
            self.manifest['inputs'].pop('reference')
            for role, content in [('mix', b'prepared audio'),
                                  ('srt', b'1\n00:00:00,250 --> 00:00:01,750\nFirst.\n\n'),
                                  ('vtt', b'WEBVTT\n\n00:00:00.250 --> 00:00:01.750\nFirst.\n\n')]:
                path = self.root / role
                path.write_bytes(content)
                self.manifest['inputs'][role] = {'path': role, 'sha256': native.sha256(path)}
            self.manifest_path.write_text(json.dumps(self.manifest))
        def run(args, **kwargs):
            if args[0] == str(self.root / 'native'):
                self.assertEqual(kwargs['env']['FFRAMES_TIMELINE'], str(self.root / 'timeline.json'))
                Path(args[args.index('-o') + 1]).write_bytes(b'native picture')
            elif args[-1].endswith('final.mp4'):
                Path(args[-1]).write_bytes(b'assembled film')
                if failure == 'stale':
                    (self.root / 'preview.html').write_text('changed during render')
                elif failure == 'manifest':
                    self.manifest_path.write_text(self.manifest_path.read_text() + '\n')
            elif args[-1].endswith('captions.vtt'):
                Path(args[-1]).write_text('WEBVTT\n\n00:00:00.250 --> 00:00:01.750\nFirst.\n\n')
            elif '-xerror' in args:
                if failure == 'decode':
                    raise native.subprocess.CalledProcessError(1, args)
            else:
                raise AssertionError(f'unexpected command: {args}')
            return SimpleNamespace(stdout='', stderr='')
        def probe(path):
            if Path(path).name in ('reference.mp4', 'mix'):
                return {'streams': [{'codec_type': 'audio', 'duration': '5.4', 'codec_name': 'aac'},
                                    {'codec_type': 'subtitle'}]}
            return {'streams': [{'codec_type': 'video', 'width': 1920, 'height': 1080,
                                 'r_frame_rate': '30/1', 'nb_frames': '162', 'duration': '5.4',
                                 'codec_name': 'h264', 'pix_fmt': 'yuv420p'}]}
        def packets(path, selector):
            payload = 'altered' if failure == selector and Path(path).name == 'final.mp4' else 'same'
            return [{'pts_time': '0', 'dts_time': '0', 'duration_time': '0.02', 'data_hash': payload}]
        with patch.object(native, 'command', side_effect=run) as commands, \
             patch.object(native, 'probe', side_effect=probe), \
             patch.object(native, 'packets', side_effect=packets) as packet_probe, \
             patch.object(native, 'probe_loudness', return_value=-30. if failure == 'loudness' else -16.):
            yield commands, packet_probe

    def test_export_reference_and_prepared_mix(self):
        for mode in ('reference', 'mix'):
            with self.subTest(mode=mode), self.export_backend(mode) as (commands, packet_probe):
                out = self.root / f'exported-{mode}.mp4'
                report = native.export(self.manifest_path, out)
                self.assertEqual(out.read_bytes(), b'assembled film')
                self.assertEqual(report['output_sha256'], native.sha256(out))
                self.assertEqual(report, json.loads(out.with_suffix('.render.json').read_text()))
                self.assertTrue(out.with_suffix('.vtt').read_text().startswith('WEBVTT'))
                mux = next(c.args[0] for c in commands.call_args_list if c.args[0][-1].endswith('final.mp4'))
                self.assertTrue(any('-xerror' in c.args[0] for c in commands.call_args_list))
                if mode == 'reference':
                    self.assertEqual(mux[mux.index('-c') + 1], 'copy')
                    self.assertEqual([c.args[1] for c in packet_probe.call_args_list], ['a:0', 'a:0', 's:0', 's:0'])
                else:
                    self.assertEqual(mux[mux.index('-c:a') + 1], 'aac')
                    packet_probe.assert_not_called()
                    self.assertEqual(out.with_suffix('.vtt').read_bytes(), (self.root / 'vtt').read_bytes())
                self.assertEqual(list(self.root.glob('.native-export-*')), [])

    def test_export_failures_leave_no_deliverables(self):
        for failure, message in [('a:0', 'payload/timestamp mismatch'), ('s:0', 'payload/timestamp mismatch'),
                                 ('loudness', 'loudness'), ('stale', 'stale input'),
                                 ('manifest', 'manifest changed'), ('decode', None)]:
            with self.subTest(failure=failure):
                self.save()
                with self.export_backend(failure=failure):
                    out = self.root / 'failed.mp4'
                    if message:
                        with self.assertRaisesRegex(ValueError, message): native.export(self.manifest_path, out)
                    else:
                        with self.assertRaises(native.subprocess.CalledProcessError): native.export(self.manifest_path, out)
                for path in (out, out.with_suffix('.vtt'), out.with_suffix('.render.json')):
                    self.assertFalse(path.exists())
                self.assertEqual(list(self.root.glob('.native-export-*')), [])
                (self.root / 'preview.html').write_text('window.FALLBACK=[2.0,3.0];')

    def test_cli_reports_loudness_and_invalid_script_errors(self):
        for failure in ('loudness', 'script', 'inputs'):
            with self.subTest(failure=failure):
                self.save()
                if failure == 'script':
                    (self.root / 'SCRIPT.md').write_text('# Missing narration table\n')
                    self.save()
                if failure == 'inputs':
                    bad = dict(self.manifest, inputs=[])
                    self.manifest_path.write_text(json.dumps(bad))
                args = ['render-final', '--manifest', str(self.manifest_path), '--out', str(self.root / 'bad.mp4')]
                error = io.StringIO()
                with self.export_backend(), patch.object(sys, 'argv', args), redirect_stderr(error):
                    with patch.object(native, 'probe_loudness', side_effect=RuntimeError('loudness probe failed')):
                        with self.assertRaises(SystemExit) as result: native.main()
                self.assertEqual(result.exception.code, 1)
                self.assertIn('native export failed:', error.getvalue())
                self.assertIn({'loudness': 'loudness probe failed', 'script': 'no scenes table',
                               'inputs': 'inputs must be'}[failure], error.getvalue())

    def test_estimate_cannot_be_relaxed_without_preview_change(self):
        self.manifest['authored_estimates'] = [100., 100.]
        self.save()
        with self.assertRaisesRegex(ValueError, 'FALLBACK'): native.load_manifest(self.manifest_path)

    def test_both_sound_modes_rejected(self):
        self.manifest['inputs']['mix'] = self.manifest['inputs']['reference']
        self.save()
        with self.assertRaisesRegex(ValueError, 'exactly one'): native.load_manifest(self.manifest_path)

    def test_existing_output_preserved(self):
        output = self.root / 'out.mp4'
        output.write_bytes(b'previous good cut')
        with self.assertRaisesRegex(ValueError, 'output exists'): native.export(self.manifest_path, output)
        self.assertEqual(output.read_bytes(), b'previous good cut')

    def test_missing_frame_rejected(self):
        stream = {'codec_type': 'video', 'width': 1920, 'height': 1080,
                  'r_frame_rate': '30/1', 'nb_frames': '161', 'duration': '5.366667',
                  'codec_name': 'h264', 'pix_fmt': 'yuv420p'}
        with patch.object(native, 'probe', return_value={'streams': [stream]}):
            with self.assertRaisesRegex(ValueError, 'frame mismatch'):
                native.validate_picture('unused', self.manifest['format'], 162)

    def test_paired_captions_and_negative_controls(self):
        srt, vtt = self.root / 'a.srt', self.root / 'a.vtt'
        srt.write_text('1\n00:00:00,250 --> 00:00:01,750\nFirst sentence.\n\n')
        vtt.write_text('WEBVTT\n\n00:00:00.250 --> 00:00:01.750\nFirst sentence.\n\n')
        native.validate_captions(srt, vtt, 5.)
        for text in ['Different sentence.', 'First sentence.']:
            end = '01.750' if text.startswith('Different') else '08.750'
            vtt.write_text(f'WEBVTT\n\n00:00:00.250 --> 00:00:{end}\n{text}\n\n')
            with self.assertRaises(ValueError): native.validate_captions(srt, vtt, 5.)

    def test_malformed_caption_cannot_be_silently_dropped(self):
        cue = '00:00:00.250 --> 00:00:01.750\nFirst sentence.\n\n'
        malformed = '00:00:02 --> 00:00:03\nLost sentence.\n\n'
        srt, vtt = self.root / 'a.srt', self.root / 'a.vtt'
        srt.write_text(cue + malformed)
        vtt.write_text('WEBVTT\n\n' + cue + malformed)
        with self.assertRaisesRegex(ValueError, 'unreadable caption'):
            native.validate_captions(srt, vtt, 5.)


if __name__ == '__main__': unittest.main()
