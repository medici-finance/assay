"""Local final export from a hash-bound fframes build and a prepared soundtrack.

No TTS, dependency installation, browser fallback or upload. A manifest binds a
particular native build to the reviewed script, preview, measured timeline and
sound inputs. It is a stale-input guard, not evidence of visual/editorial approval.
"""
from __future__ import annotations

import argparse
from fractions import Fraction
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

from script_scenes import parse_script
from deliver_gate import check_loudness, probe_loudness


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def command(args, **kwargs):
    kwargs.setdefault("timeout", 600)
    return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs)


def probe(path):
    return json.loads(command(['ffprobe', '-v', 'error', '-show_streams',
                               '-show_format', '-of', 'json', str(path)]).stdout)


def packets(path, selector):
    return json.loads(command([
        'ffprobe', '-v', 'error', '-select_streams', selector, '-show_packets',
        '-show_data_hash', 'sha256', '-show_entries',
        'packet=pts_time,dts_time,duration_time,data_hash', '-of', 'json', str(path)
    ]).stdout)['packets']


def number(value, name, minimum=0):
    if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value < minimum:
        raise ValueError(f'{name} must be finite and >= {minimum}')
    return value


def load_manifest(path):
    path = Path(path).resolve()
    manifest = json.loads(path.read_text())
    if manifest.get('version') != 1:
        raise ValueError('unsupported native export manifest version')
    files = {}
    if not isinstance(manifest['inputs'], dict):
        raise ValueError('inputs must be a role-to-file object')
    for role, entry in manifest['inputs'].items():
        file = (path.parent / entry['path']).resolve()
        if sha256(file) != entry['sha256']:
            raise ValueError(f'stale input: {role}; rebuild/review the affected inputs')
        files[role] = file
    for role in ('script', 'preview', 'timeline', 'native'):
        if role not in files:
            raise ValueError(f'missing bound input: {role}')
    reference = 'reference' in files
    if reference == ('mix' in files):
        raise ValueError('bind exactly one of reference MP4 or prepared mix WAV')
    if not reference and not {'srt', 'vtt'} <= files.keys():
        raise ValueError('prepared mix requires bound SRT and VTT captions')
    if not os.access(files['native'], os.X_OK):
        raise ValueError('native input must be an executable fframes build')
    timing = json.loads(files['timeline'].read_text())
    scenes = timing['scenes']
    try:
        script = parse_script(files['script'])
    except SystemExit as exc:
        # The shared parser is also a CLI helper; normalize its input failures
        # locally without catching argparse's help/usage exits in main().
        raise ValueError(str(exc)) from exc
    if len(scenes) != len(script) or not scenes:
        raise ValueError('script/timeline scene count mismatch')
    ids = [s['id'] for s in scenes]
    if len(set(ids)) != len(ids):
        raise ValueError('duplicate scene IDs')
    estimates = manifest['authored_estimates']
    fallback = re.search(r'(?:window\.)?FALLBACK\s*=\s*(\[[^\]]*\])', files['preview'].read_text())
    if not fallback or json.loads(fallback[1]) != estimates:
        raise ValueError('authored estimates must match the bound HTML FALLBACK')
    if len(estimates) != len(scenes):
        raise ValueError('estimate/scene count mismatch')
    cursor = 0.0
    for index, (scene, row, estimate) in enumerate(zip(scenes, script, estimates)):
        if (scene['id'], scene['narration']) != (row.id, row.narration):
            raise ValueError(f'script/timeline identity or narration mismatch at scene {index + 1}')
        duration = number(scene['duration'], 'duration', 0.001)
        start = number(scene['start'], 'start')
        estimate = number(estimate, 'estimate', 0.001)
        if abs(start - cursor) > 0.0011:
            raise ValueError(f'noncontiguous timeline at {scene["id"]}')
        if duration > max(estimate * 1.8, estimate + 8):
            raise ValueError(f'take-sanity failed at {scene["id"]}')
        cursor += duration
    if cursor > sum(estimates) * 1.3:
        raise ValueError('take-sanity failed for total duration')
    if abs(number(timing['runtime'], 'runtime') - cursor) > 0.0011:
        raise ValueError('timeline runtime does not equal scene durations')
    fmt = manifest['format']
    for key in ('width', 'height', 'fps'):
        if type(fmt[key]) is not int or fmt[key] <= 0:
            raise ValueError(f'{key} must be a positive integer')
    tail = number(fmt['tail_seconds'], 'tail_seconds')
    expected = math.ceil(round((cursor + tail) * fmt['fps'], 6))
    return manifest, files, expected


def validate_picture(path, fmt, frames):
    streams = [s for s in probe(path)['streams'] if s['codec_type'] == 'video']
    if len(streams) != 1:
        raise ValueError('expected one video stream')
    stream = streams[0]
    expected = (fmt['width'], fmt['height'], Fraction(fmt['fps']), frames)
    observed = (stream['width'], stream['height'], Fraction(stream['r_frame_rate']),
                int(stream.get('nb_frames', -1)))
    if observed != expected:
        raise ValueError(f'picture format/frame mismatch: {observed} != {expected}')
    if stream['codec_name'] != 'h264' or stream.get('pix_fmt') != 'yuv420p':
        raise ValueError('native export must produce H.264 yuv420p')
    if abs(float(stream['duration']) - frames / fmt['fps']) > 0.001:
        raise ValueError('picture duration does not match frame count')


def caption_cues(path):
    text = Path(path).read_text(encoding='utf-8-sig').replace('\r\n', '\n')
    pattern = r'(\d{2}:\d{2}:\d{2}[.,]\d{3}) --> (\d{2}:\d{2}:\d{2}[.,]\d{3})[^\n]*\n([^\n]+(?:\n(?!\n)[^\n]+)*)'
    def seconds(value):
        h, m, sec = value.replace(',', '.').split(':')
        return round(int(h) * 3600 + int(m) * 60 + float(sec), 3)
    matches = re.findall(pattern, text)
    if len(matches) != text.count('-->'):
        raise ValueError('unreadable caption cue')
    return [(seconds(a), seconds(b), ' '.join(body.split()))
            for a, b, body in matches]


def validate_captions(srt, vtt, span):
    cues = caption_cues(srt)
    if not cues or cues != caption_cues(vtt):
        raise ValueError('SRT/VTT captions differ or cannot be read')
    previous_end = 0
    for start, end, text in cues:
        if start < previous_end or end <= start or end > span or not text:
            raise ValueError('captions overlap or exceed the measured film')
        previous_end = end


def export(manifest_path, output):
    manifest_digest = sha256(manifest_path)
    manifest, files, frames = load_manifest(manifest_path)
    output = Path(output).resolve()
    if output.suffix.lower() != '.mp4':
        raise ValueError('output must end in .mp4')
    report_path = output.with_suffix('.render.json')
    vtt_path = output.with_suffix('.vtt')
    if any(p in files.values() or p == Path(manifest_path).resolve()
           for p in (output, report_path, vtt_path)):
        raise ValueError('output must not overwrite an input')
    if any(p.exists() for p in (output, report_path, vtt_path)):
        raise ValueError('output exists; select a new output name for this revision')
    output.parent.mkdir(parents=True, exist_ok=True)
    fmt = manifest['format']
    span = frames / fmt['fps']
    reference = files.get('reference')
    audio = reference or files['mix']
    sound_info = probe(audio)
    audio_streams = [s for s in sound_info['streams'] if s['codec_type'] == 'audio']
    if len(audio_streams) != 1:
        raise ValueError('sound input must have exactly one audio stream')
    audio_span = float(audio_streams[0]['duration'])
    runtime = json.loads(files['timeline'].read_text())['runtime']
    if audio_span < runtime - 1 / fmt['fps'] or audio_span > span + 1 / fmt['fps']:
        raise ValueError('sound duration is inconsistent with the bound timeline')
    if reference and audio_streams[0]['codec_name'] != 'aac':
        raise ValueError('reference copy mode requires an AAC soundtrack')
    if reference and len([s for s in sound_info['streams'] if s['codec_type'] == 'subtitle']) != 1:
        raise ValueError('reference requires exactly one subtitle stream')
    if not reference:
        validate_captions(files['srt'], files['vtt'], span)
    before = time.perf_counter()
    with tempfile.TemporaryDirectory(prefix='.native-export-', dir=output.parent) as tmp:
        tmp = Path(tmp)
        picture, assembled = tmp / 'picture.mp4', tmp / 'final.mp4'
        env = dict(os.environ, FFRAMES_TIMELINE=str(files['timeline']))
        command([str(files['native']), 'render', '-o', str(picture), '--json'], env=env, timeout=max(120, span * 4))
        render_seconds = time.perf_counter() - before
        validate_picture(picture, fmt, frames)
        mux_start = time.perf_counter()
        args = ['ffmpeg', '-v', 'error', '-y', '-i', str(picture), '-i', str(audio)]
        if reference:
            args += ['-map', '0:v:0', '-map', '1:a:0', '-map', '1:s:0', '-c', 'copy']
        else:
            args += ['-i', str(files['srt']), '-map', '0:v:0', '-map', '1:a:0',
                     '-map', '2:s:0', '-c:v', 'copy', '-c:a', 'aac', '-b:a', '192k',
                     '-ar', '48000', '-ac', '2', '-af', 'apad', '-t', str(span),
                     '-c:s', 'mov_text', '-metadata:s:s:0', 'language=eng']
        args += ['-movflags', '+faststart', str(assembled)]
        command(args)
        mux_seconds = time.perf_counter() - mux_start
        validate_picture(assembled, fmt, frames)
        command(['ffmpeg', '-v', 'error', '-xerror', '-i', str(assembled),
                 '-map', '0:v', '-map', '0:a', '-f', 'null', '-'])
        loudness = check_loudness(probe_loudness(assembled))
        if not loudness.ok:
            raise ValueError(loudness.detail)
        if reference:
            for selector in ('a:0', 's:0'):
                original = packets(reference, selector)
                if not original or original != packets(assembled, selector):
                    raise ValueError(f'{selector} payload/timestamp mismatch against reference')
            command(['ffmpeg', '-v', 'error', '-i', str(assembled), '-map', '0:s:0', str(tmp / 'captions.vtt')])
        else:
            (tmp / 'captions.vtt').write_bytes(files['vtt'].read_bytes())
        # A renderer can read inputs: refuse if any changed during the export.
        if sha256(manifest_path) != manifest_digest:
            raise ValueError('manifest changed during export')
        load_manifest(manifest_path)
        report = {'renderer': 'fframes', 'frames': frames, 'format': fmt,
                  'render_seconds': render_seconds, 'mux_seconds': mux_seconds,
                  'export_seconds': render_seconds + mux_seconds,
                  'including_validation_seconds': time.perf_counter() - before,
                  'manifest_sha256': manifest_digest, 'output_sha256': sha256(assembled),
                  'audio_mode': 'reference packet copy' if reference else 'prepared mix; AAC encode',
                  'loudness': loudness.detail,
                  'editorial_and_visual_review': 'not established by this mechanical export gate'}
        assembled.replace(output)
        (tmp / 'captions.vtt').replace(vtt_path)
        report_path.write_text(json.dumps(report, indent=2) + '\n')
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--check', action='store_true', help='validate bound inputs; do not render')
    args = parser.parse_args()
    try:
        if args.check:
            _, _, frames = load_manifest(args.manifest)
            print(f'Bound inputs valid; expected {frames} frames. Output not checked.')
        else:
            print(json.dumps(export(args.manifest, args.out), indent=2))
    except (ValueError, KeyError, TypeError, OSError, RuntimeError, subprocess.SubprocessError) as exc:
        parser.exit(1, f'native export failed: {exc}\n')


if __name__ == "__main__":
    main()
