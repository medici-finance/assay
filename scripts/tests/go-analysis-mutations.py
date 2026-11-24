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
    ('resolved-complete', "    if complete:\n        resolved = ", "    if True:\n        resolved = ", 'Ratchet.test_resolved_allowance'),
    ('duplicates', "if item['id'] in result:", 'if False:', 'Ratchet.test_ambiguous'),
    ('base-scan', "report['base_coverage'], prior = scan_tree(tree, cache, owner, binary, env, 'merge-base')", "report['base_coverage'], prior = [], []", 'MergeBase.test_untouched_debt_stays_green'),
    ('base-source', "git(root, 'merge-base', base, 'HEAD')", "git(root, 'rev-parse', base)", 'MergeBase.test_base_moved_after_fork'),
    ('base-sha', "if not base or not re.fullmatch('[0-9a-f]{40}', base):", 'if False:', 'MergeBase.test_base_required'),
    ('base-policy', "                require_policy(tree, owner)\n", "                pass\n", 'MergeBase.test_policy_at_merge_base'),
    ('base-complete', "for s in report['coverage'] + report['base_coverage'])", "for s in report['coverage'])", 'MergeBase.test_merge_base_scan_failure'),
    ('base-cleanup', "        self.remove()\n        return False", "        return False", 'MergeBase.test_adds_finding'),
    ('report-never-gates', "exit_code = 0 if complete else 2", "exit_code = (1 if findings else 0) if complete else 2", 'MergeBase.test_debt_report'),
    ('legacy', "'existing' if old['diagnostic'] == item['diagnostic']", "'new' if old['diagnostic'] == item['diagnostic']", 'Ratchet.test_legacy'),
    ('regression', "'new' if old is None", "'existing' if old is None", 'Ratchet.test_new_and_worse'),
    ('line', "'anchor': digest(encoded([scopes[line - 1][1], window, prefix]))", "'anchor': digest(encoded([scopes[line - 1][1], window, prefix, line]))", 'Ratchet.test_shifts'),
    ('scope', "scopes = declaration_scopes(lines)", "scopes = [(-1, 'package')] * len(lines)", 'Ratchet.test_sibling_independence'),
    ('preamble', "return scopes[start][1] if start >= 0 else 'package'", 'return scopes[start][1]', 'Ratchet.test_preamble_finding'),
    ('checksum', 'if digest(target.read_bytes()) != expected:', 'if False:', 'Harness.test_bad_checksum'),
    ('coverage', "found = sorted({str(Path(x).parent) for x in found if x})", "found = sorted({str(Path(x).parent) for x in found if x})[:1]", 'Harness.test_discovery'),
    ('network', "GOPROXY='https://proxy.golang.org' if network else 'off'", "GOPROXY='https://proxy.golang.org'", 'Harness.test_offline'),
    ('sumdb', "GOSUMDB=sumdb", "GOSUMDB='sum.golang.org'", 'Harness.test_offline'),
    ('related-linter', "if i.get('FromLinter') == 'staticcheck' and RELATED.match(str(i.get('Text')))]", "if RELATED.match(str(i.get('Text')))]", 'Harness.test_related_notes'),
    ('related-rules', "for i in kept if i.get('FromLinter') == 'staticcheck'}", 'for i in kept}', 'Harness.test_related_notes'),
    ('related-anchor', "RELATED = re.compile(r'(", "RELATED = re.compile(r'.*?(", 'Harness.test_related_notes'),
    ('related', "if any(RELATED.match(n['Text']).group(1) not in rules for n in notes):", 'if False:', 'Harness.test_related_notes'),
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
