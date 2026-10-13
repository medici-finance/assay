#!/usr/bin/env python3
"""Regression #2686 guard controls; mutate scratch copies, never the checkout."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
# Each guard has a named executable counterexample. A mutation that no longer
# applies is a failure, not a skipped control; the unmutated baseline runs first.
MUTATIONS = [
    ('author', "author['login'] != 'assay-board-writer[bot]'", 'False', 'test_metadata_not_a_branch_or_author_waiver'),
    ('same-repository', "head['repo']['full_name'] != repo", 'False', 'test_metadata_not_a_branch_or_author_waiver'),
    ('checked-head', "head['sha'] != head_sha", 'False', 'test_metadata_not_a_branch_or_author_waiver'),
    ('content', 'normalized(before) != normalized(after)', 'False', 'test_readme_content_boundary'),
    ('planted-second-stream', 'normalized(before) != normalized(after)', 'False', 'test_second_stream_does_not_hide_foreign_prose'),
    ('mode', "meta[1] != b'100644'", 'False', 'test_entire_diff_and_file_modes'),
    ('generated-opt-in', "frontmatter[1].splitlines().count('board: generated') != 1", 'False', 'test_missing_or_non_generated_markers'),
    ('recognized-state', 'cells[5].strip() not in STATES', 'False', 'test_readme_content_boundary'),
    ('nonempty-diff', "changes == [b'']", 'False', 'test_empty_diff_and_absolute_guard'),
    ('merge-base', "ancestor = bases[0].decode('ascii')", 'ancestor = base_sha', 'test_merge_base_diff_and_repeat'),
]


def run(check, test=None):
    command = ['python3', str(HERE / 'reconcile_test.py')]
    if test:
        command.append('ReconcileGate.' + test)
    return subprocess.run(command, env={**os.environ, 'CHECK_IMPL': str(check)},
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)


baseline = run(HERE / 'check.sh')
print('baseline:', 'GREEN' if baseline.returncode == 0 else baseline.stdout, flush=True)
if baseline.returncode:
    raise SystemExit(1)
with tempfile.TemporaryDirectory() as directory:
    dest = Path(directory)
    shutil.copyfile(HERE / 'check.sh', dest / 'check.sh')
    original = (HERE / 'reconcile.py').read_text()
    for name, old, new, test in MUTATIONS:
        if original.count(old) != 1:
            raise SystemExit('could-not-mutate: ' + name)
        (dest / 'reconcile.py').write_text(original.replace(old, new))
        result = run(dest / 'check.sh', test)
        # A failing assertion, not a syntax/crash failure, is the control.
        if result.returncode == 0 or 'AssertionError:' not in result.stdout:
            print(result.stdout)
            raise SystemExit('UNCAUGHT: ' + name)
        print('CAUGHT:', name, flush=True)
print(f'{len(MUTATIONS)} caught, 0 uncaught, 0 could-not-mutate')
