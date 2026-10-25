#!/usr/bin/env python3
"""Run bounded fail-first plants in an owned checkout, restoring each file.

Every patched file must match HEAD before its plant: a file that already differs
(an uncommitted edit, or a plant left by a run killed before its restore) stops
the run instead of being treated as the baseline. SIGTERM/SIGHUP restore through
the same `finally` as a normal exit; SIGKILL cannot, and the next run's HEAD check
then refuses until `git checkout -- <file>` restores it.

An optional argument names another package's mutations file, relative to the
statusgen module root (portfolio/testdata/mutations.json); paths and packages in
it are relative to that root too. With none, this package's own file runs.
"""
import json
from pathlib import Path
import signal
import subprocess
import sys

root = Path(__file__).resolve().parents[2]
source = root / sys.argv[1] if len(sys.argv) > 1 else Path(__file__).with_name('mutations.json')
mutations = json.loads(source.read_text())


def interrupted(signum, _frame):
    raise SystemExit(f"interrupted by signal {signum}; plant restored")


for sig in (signal.SIGTERM, signal.SIGHUP):
    signal.signal(sig, interrupted)


def matches_head(path):
    return subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--', str(path)],
                          cwd=root).returncode == 0


for mutation in mutations:
    path = root / mutation['path']
    if not matches_head(path):
        raise SystemExit(f"{mutation['name']}: {mutation['path']} differs from HEAD; "
                         "commit or restore it before planting")
    original = path.read_bytes()
    before = mutation['before'].encode()
    if original.count(before) != 1:
        raise SystemExit(f"{mutation['name']}: expected one patch site")
    try:
        path.write_bytes(original.replace(before, mutation['after'].encode(), 1))
        result = subprocess.run(
            ['go', 'test', '-count=1', '-timeout=45s', '-v', '-run',
             '^' + mutation['test'] + '$', mutation['package']],
            cwd=root, capture_output=True, text=True, timeout=90)
        output = result.stdout + result.stderr
        if result.returncode == 0 or '--- FAIL: ' + mutation['test'] not in output:
            print(output)
            raise SystemExit(f"{mutation['name']}: missing named behavioral failure")
        print(f"CAUGHT {mutation['name']}: {mutation['test']} (exit {result.returncode})")
    finally:
        path.write_bytes(original)
    if not matches_head(path):
        raise SystemExit(f"{mutation['name']}: restore left {mutation['path']} differing from HEAD")
