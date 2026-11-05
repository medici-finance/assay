#!/usr/bin/env python3
"""Committed falsification map: each removed guard must redden its focused test."""
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
source = (ROOT / 'tools/go-analysis/runner.py').read_text()
cases = [
    ('lifted-platforms', "for module in ['tools/desk', 'gitcore']:", "for module in ['tools/desk']:", 'Coverage.test_lifted_owner_tuples'),
    ('growth', "if any(key not in prior or record != prior[key] for key, record in current.items()):", 'if False:', 'Ratchet.test_baseline_growth'),
    ('policy', "if candidate.get('contract') != policy or trusted.get('contract') != policy:", 'if False:', 'Ratchet.test_baseline_growth'),
    ('stale', 'if stale:', 'if False:', 'Ratchet.test_resolved_allowance'),
    ('duplicates', "if item['id'] in live:", 'if False:', 'Ratchet.test_ambiguous'),
    ('legacy', "'existing' if old['diagnostic'] == item['diagnostic']", "'new' if old['diagnostic'] == item['diagnostic']", 'Ratchet.test_legacy'),
    ('regression', "'new' if old is None", "'existing' if old is None", 'Ratchet.test_new_and_worse'),
    ('line', "'anchor': digest(encoded([scopes[line - 1][1], window, prefix]))", "'anchor': digest(encoded([scopes[line - 1][1], window, prefix, line]))", 'Ratchet.test_shifts'),
    ('scope', "scopes = declaration_scopes(lines)", "scopes = [(-1, 'package')] * len(lines)", 'Ratchet.test_sibling_independence'),
    ('preamble', "return scopes[start][1] if start >= 0 else 'package'", 'return scopes[start][1]', 'Ratchet.test_preamble_finding'),
    ('checksum', 'if digest(target.read_bytes()) != expected:', 'if False:', 'Harness.test_bad_checksum'),
    ('coverage', "found = sorted({str(Path(x).parent) for x in found if x})", "found = sorted({str(Path(x).parent) for x in found if x})[:1]", 'Harness.test_discovery'),
    ('network', "GOPROXY='https://proxy.golang.org' if network else 'off'", "GOPROXY='https://proxy.golang.org'", 'Harness.test_offline'),
    ('sumdb', "GOSUMDB=sumdb", "GOSUMDB='sum.golang.org'", 'Harness.test_offline'),
    ('exit', "if proc.returncode not in [0, 1] or (proc.returncode == 0 and issues) or (proc.returncode == 1 and not issues):", 'if False:', 'Harness.test_scan_failures'),
]
with tempfile.TemporaryDirectory() as tmp:
    mutant = Path(tmp) / 'runner.py'
    for name, before, after, test in cases:
        if source.count(before) != 1:
            raise SystemExit('mutation anchor missing/ambiguous: ' + name)
        mutant.write_text(source.replace(before, after, 1))
        env = dict(os.environ, GO_ANALYSIS_RUNNER=str(mutant), PYTHONDONTWRITEBYTECODE='1')
        result = subprocess.run(['python3', str(ROOT / 'scripts/tests/go-analysis.py'), test], env=env,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=40)
        if result.returncode == 0 or b'FAILED (' not in result.stdout:
            raise SystemExit('mutation did not trigger its assertion: ' + name + '\n' + result.stdout.decode())
        print('MUTATION', name, 'RED assertion observed', flush=True)
print(str(len(cases)) + ' guard mutations discriminated')
