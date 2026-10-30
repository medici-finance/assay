"""Offline mutation controls for the checked release handoff. No network calls."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
FILES = ('tools/release/transport.py', 'tools/release/transport_test.py',
         'tools/release/transport_workflow_test.py', 'tools/release/images_test.py',
         'tools/release/images_contract.py', 'tools/release/bundles.py',
         'tools/release/install-object-client.sh', '.github/workflows/release.yml',
         'ci/staged-workflows/release.yml')
RUNTIME = 'tools/release/transport.py'
WORKFLOW = '.github/workflows/release.yml'
CASES = (
 ('bundle-digest', RUNTIME, 'bundle.stat().st_size != expected_size or sha256_file(bundle) != expected_digest', 'bundle.stat().st_size != expected_size'),
 ('manifest-digest', RUNTIME, "if hashlib.sha256(manifest).hexdigest() != expected_manifest:", 'if False:'),
 ('run-source-version', RUNTIME, 'or any(decoded.get(k) != v for k, v in meta.items())', 'or False'),
 ('archive-member-name', RUNTIME, "or entry.name != record['name']", 'or False'),
 ('member-digest', RUNTIME, "if h.hexdigest() != record['sha256']:", 'if False:'),
 ('explicit-credentials', RUNTIME, 'if not key.strip() or not secret.strip():', 'if False:'),
 ('https-only', RUNTIME, "parsed.scheme != 'https' or ", ''),
 ('absolute-ca-file', RUNTIME, 'ca.is_absolute() and not ca.is_symlink() and ca.is_file()', 'not ca.is_symlink() and ca.is_file()'),
 ('metadata-fallback-off', RUNTIME, "AWS_EC2_METADATA_DISABLED='true'", "AWS_EC2_METADATA_DISABLED='false'"),
 ('shared-credentials-off', RUNTIME, "AWS_SHARED_CREDENTIALS_FILE='/dev/null'", "AWS_SHARED_CREDENTIALS_FILE='/ambient'"),
 ('workflow-expected-digest', WORKFLOW, '--expected-sha256 "$BUNDLE_SHA256"', '--unused-expected-digest "$BUNDLE_SHA256"'),
 ('workflow-trusted-output', WORKFLOW, 'BUNDLE_SHA256: ${{ needs.artifacts.outputs.bundle_sha256 }}', 'BUNDLE_SHA256: ${{ secrets.UNTRUSTED_DIGEST }}'),
 ('installer-checksum', 'tools/release/install-object-client.sh', ' | sha256sum -c -', ' | cat'),
)
CONTROL = ('refuse-healthy-input', RUNTIME, 'def _regular(path):', "def _regular(path):\n    raise ValueError('distinct broken-reader control')")


def run_case(out, name, mutation=None):
    with tempfile.TemporaryDirectory(prefix='release-handoff-mutation-') as directory:
        root = Path(directory)
        for rel in FILES:
            target = root / rel
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / rel, target)
        if mutation:
            _, rel, old, new = mutation
            target = root / rel
            text = target.read_text()
            if text.count(old) != 1:
                return dict(name=name, verdict='could-not-mutate', matches=text.count(old))
            target.write_text(text.replace(old, new, 1))
        env = {k: os.environ[k] for k in ('PATH', 'LANG', 'LC_ALL', 'TMPDIR') if k in os.environ}
        env.update(KUBECONFIG='/dev/null', PYTHONDONTWRITEBYTECODE='1')
        args = [sys.executable, '-B', '-m', 'unittest', 'tools.release.transport_test',
                'tools.release.transport_workflow_test', '-v']
        try:
            result = subprocess.run(args, cwd=root, env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=90)
        except subprocess.TimeoutExpired as exc:
            (out / (name + '.log')).write_text(str(exc))
            return dict(name=name, verdict='could-not-check', reason='test deadline')
        (out / (name + '.log')).write_text(result.stdout)
        return dict(name=name, exit=result.returncode,
                    verdict=('healthy' if result.returncode == 0 else 'caught') if name == 'baseline'
                    else ('caught' if result.returncode else 'survived'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=False)
    baseline = run_case(out, 'baseline')
    control = run_case(out, CONTROL[0], CONTROL)
    results = []
    if baseline['verdict'] == 'healthy' and control['verdict'] == 'caught':
        for case in CASES:
            row = run_case(out, case[0], case)
            results.append(row)
            print(row['name'] + ': ' + row['verdict'], flush=True)
    report = dict(baseline=baseline, distinctControl=control, results=results,
                  caught=sum(r['verdict'] == 'caught' for r in results),
                  survivors=sum(r['verdict'] == 'survived' for r in results),
                  couldNotMutate=sum(r['verdict'] == 'could-not-mutate' for r in results),
                  couldNotCheck=sum(r['verdict'] == 'could-not-check' for r in results))
    (out / 'receipt.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report), flush=True)
    if baseline['verdict'] != 'healthy' or control['verdict'] != 'caught' or len(results) != len(CASES) or any(r['verdict'] != 'caught' for r in results):
        raise SystemExit(1)


if __name__ == '__main__':
    main()
