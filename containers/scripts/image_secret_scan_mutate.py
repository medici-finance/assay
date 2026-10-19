"""Prove traversal, refusal and detector-verdict assertions can fail."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
source = (HERE / 'image_secret_scan.py').read_text()
mutations = {
    'ignore-scanner-exit': ('if p.returncode not in (0, 9):', 'if False:'),
    'ignore-scanner-stderr': ('if p.stderr.strip():', 'if False:'),
    'ignore-malformed-json': ('report = json.loads(p.stdout)', "report = json.loads(p.stdout) if p.stdout != b'not json' else []"),
    'ignore-report-type': ('not isinstance(report, list) or bool(report)', 'bool(report)'),
    'ignore-report-exit-agreement': (' or bool(report) != (p.returncode == 9)', ''),
    'last-layer-only': ('for layer in layers:', 'for layer in layers[-1:]:'),
    'ignore-digest': ("if hashlib.sha256(data).hexdigest() != name.removeprefix('blobs/sha256/'):", 'if False:'),
    'missing-canary-clean': ('if not canary:', 'if False:'),
    'drop-findings': ("findings.add((index[name], finding['RuleID']))", 'pass'),
    'baseline-without-content-binding': ('identity.update(data)', 'pass'),
}
with tempfile.TemporaryDirectory() as d:
    shutil.copyfile(HERE / 'image-secret-baseline.json', Path(d) / 'image-secret-baseline.json')
    for name, (old, new) in [('baseline', ('', '')), *mutations.items()]:
        if old and source.count(old) != 1:
            sys.exit('cannot construct mutation: ' + name)
        path = Path(d) / (name + '.py')
        path.write_text(source.replace(old, new, 1) if old else source)
        # The scanner policy is embedded-default-only and created in the temp
        # workspace, so implementation copies need no untrusted config files.
        env = dict(os.environ, IMAGE_SCAN_IMPL=str(path), IMAGE_SCAN_INTEGRATION='1')
        p = subprocess.run([sys.executable, str(HERE / 'image_secret_scan_test.py')],
                           env=env, capture_output=True)
        expected_success = name == 'baseline'
        if (p.returncode == 0) != expected_success:
            sys.stderr.buffer.write(p.stdout + p.stderr)
            sys.exit('mutation expectation failed: ' + name)
        print(('PASS baseline: ' if expected_success else 'KILLED mutation: ') + name)
