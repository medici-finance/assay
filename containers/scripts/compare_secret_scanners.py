"""Reproduce the bounded OSS comparison; output counts, never matched content.

Inputs are locally retained Docker-save fixtures and the exact failed release's
Linux suite archive. No provider verification or cluster access is performed.
Install the versions named in the comparison report before running this tool.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import time

from image_secret_scan import surfaces


def compare(items, directory):
    inputs = directory / 'inputs'; inputs.mkdir()
    for n, (_, data) in enumerate(items):
        (inputs / f'{n:08d}.txt').write_bytes(data)
    config = directory / 'gitleaks.toml'
    config.write_text('[extend]\nuseDefault = true\n')
    commands = {
        'gitleaks': ['gitleaks', 'dir', str(inputs), '--config', str(config),
                     '--gitleaks-ignore-path', str(directory / 'absent'),
                     '--ignore-gitleaks-allow', '--redact', '--no-banner',
                     '--exit-code', '9', '--report-format', 'json', '--report-path', '-'],
        'trufflehog': ['trufflehog', 'filesystem', str(inputs), '--json',
                       '--no-verification', '--no-update', '--no-ignore-tag', '--fail-on-scan-errors'],
        'trivy': ['trivy', 'filesystem', '--scanners', 'secret', '--config', '',
                  '--secret-config', '', '--ignorefile', '', '--skip-version-check',
                  '--skip-db-update', '--skip-java-db-update', '--skip-check-update',
                  '--offline-scan', '--format', 'json', str(inputs)],
    }
    rows = {}
    for name, args in commands.items():
        start = time.monotonic()
        p = subprocess.run(args, capture_output=True, timeout=600)
        if p.returncode not in ({0, 9} if name == 'gitleaks' else {0}):
            raise RuntimeError(name + ' did not complete')
        if name == 'gitleaks':
            rules = [r['RuleID'] for r in json.loads(p.stdout)]
        elif name == 'trufflehog':
            rules = [json.loads(line)['DetectorName'] for line in p.stdout.splitlines()]
        else:
            rules = [s['RuleID'] for r in json.loads(p.stdout).get('Results', []) for s in r.get('Secrets', [])]
        rows[name] = {'findings': len(rules), 'rules': {rule: rules.count(rule) for rule in sorted(set(rules))},
                      'seconds': round(time.monotonic() - start, 2)}
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--legacy-images', type=Path, required=True)
    parser.add_argument('--release-tar', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    output = {'versions': {}, 'corpora': {}}
    for name, flag in [('gitleaks', 'version'), ('trufflehog', '--version'), ('trivy', '--version')]:
        p = subprocess.run([name, flag], capture_output=True, check=True)
        output['versions'][name] = (p.stdout + p.stderr).decode().strip()
    with args.release_tar.open('rb') as f:
        output['release_tar_sha256'] = hashlib.file_digest(f, 'sha256').hexdigest()
    fixtures = {p.name: list(surfaces(p)) for p in sorted(args.legacy_images.glob('*.tar'))}
    with tarfile.open(args.release_tar, 'r:gz') as archive:
        fixtures['failed-release-linux-suite'] = [(m.name, archive.extractfile(m).read()) for m in archive if m.isfile()]
    for name, items in fixtures.items():
        with tempfile.TemporaryDirectory(prefix='compare-secret-scanners-') as d:
            output['corpora'][name] = {'files': len(items), 'scanners': compare(items, Path(d))}
        # Safe summaries remain useful if interrupted; no raw findings persist.
        args.output.write_text(json.dumps(output, indent=2) + '\n')
        print(name, json.dumps(output['corpora'][name]), flush=True)


if __name__ == '__main__':
    main()
