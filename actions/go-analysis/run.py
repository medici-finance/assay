#!/usr/bin/env python3
"""GitHub Action plumbing; analyzer and ratchet semantics stay in the shared runner."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


def output(key, value):
    value = str(value)
    if '\n' in value or '\r' in value:
        raise ValueError('multiline action output refused')
    with open(os.environ['GITHUB_OUTPUT'], 'a') as stream:
        stream.write(f'{key}={value}\n')


def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()


def prepare():
    if sys.version_info < (3, 12):
        raise ValueError('Go analysis requires Python 3.12+')
    workspace = Path(os.environ['GITHUB_WORKSPACE']).resolve()
    root = (workspace / os.environ['INPUT_ROOT']).resolve()
    root = Path(git(root, 'rev-parse', '--show-toplevel')).resolve()
    rev = os.environ.get('BOOTSTRAP_REV', '')
    if rev and not re.fullmatch('[0-9a-f]{40}', rev):
        raise ValueError('bootstrap revision must be an immutable full commit SHA')
    cache = Path(tempfile.mkdtemp(prefix='go-analysis-', dir=os.environ['RUNNER_TEMP'])).resolve()
    if cache.is_relative_to(root):
        raise ValueError('runner temporary directory must be outside analyzed checkout')
    base_dir = Path(tempfile.mkdtemp(prefix='.go-analysis-base-', dir=workspace))
    bootstrap_dir = Path(tempfile.mkdtemp(prefix='.go-analysis-bootstrap-', dir=workspace))
    output('root', root)
    output('cache', cache)
    output('base-directory', base_dir.name)
    output('bootstrap-directory', bootstrap_dir.name)
    output('code-root', Path(__file__).resolve().parents[2])
    output('source-sha', git(root, 'rev-parse', 'HEAD'))


def trust():
    root = Path(os.environ['ANALYZED'])
    base_dir = Path(os.environ['BASE_DIR'])
    base = git(base_dir, 'rev-parse', 'HEAD')
    paths = ['tools/go-analysis/baseline.json',
             'tools/go-analysis/pins.env', '.golangci.yml']
    present = [subprocess.run(['git', '-C', str(base_dir), 'cat-file', '-e', f'HEAD:{p}'],
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
               for p in paths]
    if not all(present):
        if any(present):
            raise ValueError('partial base installation; bootstrap fallback refused')
        if not os.environ.get('BOOTSTRAP_REV'):
            raise ValueError('no reviewed baseline; explicit bootstrap revision required')
    # Transfer the exact already-authenticated checkout object, without a second network fetch.
    subprocess.run(['git', '-C', str(root), 'fetch', '--no-tags', '--no-write-fetch-head',
                    str(base_dir), base], check=True)
    output('base-sha', base)
    output('bootstrap', 'false' if all(present) else 'true')


def scan():
    code = Path(os.environ['CODE_ROOT'])
    args = ['bash', str(code / 'scripts/go-analysis.sh'), 'lint', '--root', os.environ['ANALYZED'],
            '--base', os.environ['BASE_SHA'], '--cache', os.environ['CACHE']]
    if os.environ.get('BOOTSTRAP') == 'true':
        directory = Path(os.environ['BOOTSTRAP_DIR'])
        if git(directory, 'rev-parse', 'HEAD') != os.environ['BOOTSTRAP_REV']:
            raise ValueError('bootstrap checkout revision mismatch')
        args += ['--trusted-root', str(directory)]
    return subprocess.run(args, check=False).returncode


def summary():
    path = Path(os.environ['CACHE']) / 'report.json'
    output('report-path', path)
    if not path.is_file():
        output('state', 'could-not-check')
        output('gate', 'fail')
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as stream:
            stream.write('## Go analysis\n\nCould not measure: scan report unavailable.\n')
        return 2
    report = json.loads(path.read_text())
    state, gate = report['state'], report['gate']
    if report.get('schema') != 'go-analysis/v1' or state not in ['complete', 'could-not-check'] or gate not in ['pass', 'fail']:
        raise ValueError('unsupported scan report')
    if state == 'could-not-check' and gate != 'fail':
        raise ValueError('incomplete scan cannot pass')
    expected_exit = 2 if state == 'could-not-check' else (0 if gate == 'pass' else 1)
    if type(report.get('exit_code')) is not int or report['exit_code'] != expected_exit:
        raise ValueError('inconsistent scan outcome')
    rows = []
    if state == 'complete':
        for key in ['existing', 'new', 'worsened', 'resolved']:
            count = report['totals'][key]
            if type(count) is not int or count < 0:
                raise ValueError('invalid finding count')
            rows.append(f'| {key} | {count} |')
        if gate == 'pass' and (report['totals']['new'] or report['totals']['worsened']):
            raise ValueError('regressions cannot pass')
        for key in ['existing', 'new', 'worsened', 'resolved']:
            output(key, report['totals'][key])
    output('state', state)
    output('gate', gate)
    with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as stream:
        stream.write(f'## Go analysis\n\nScan: **{state}**. Gate: **{gate}**.\n\n')
        if rows:
            stream.write('| Disposition | Findings |\n|---|---:|\n' + '\n'.join(rows) + '\n')
        else:
            stream.write('Complete totals are unknown; existing debt is not cleared.\n')
    return 0


def main():
    phase = sys.argv[1]
    if phase == 'prepare':
        prepare()
    elif phase == 'trust':
        trust()
    elif phase == 'scan':
        return scan()
    elif phase == 'summary':
        return summary()
    else:
        raise ValueError('unknown action phase')
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, subprocess.SubprocessError):
        print('Go analysis action: setup or report validation failed; no clean result claimed', file=sys.stderr)
        sys.exit(2)
