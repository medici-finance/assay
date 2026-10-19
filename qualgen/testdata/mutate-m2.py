#!/usr/bin/env python3
"""Bounded M2 failure demonstrations; run only in an owned disposable checkout."""
import argparse
import json
import re
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
mutations = json.loads(Path(__file__).with_name('m2-mutations.json').read_text())
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--ci', action='store_true',
                    help='plant skip-trace-pipeline against the exact PR-CI command')
args = parser.parse_args()
ci_command = None
if args.ci:
    workflow = (root.parent / '.github/workflows/ci.yml').read_text()
    commands = re.findall(r'^\s*\*/qualgen\|qualgen\) extra="(.+)" ;;$',
                          workflow, re.MULTILINE)
    if len(commands) != 1:
        raise SystemExit('CI control: expected one qualgen test command in ci.yml')
    ci_command = ['bash', '-c', commands[0]]
    baseline = subprocess.run(ci_command, cwd=root, capture_output=True,
                              text=True, timeout=90)
    if baseline.returncode != 0:
        print(baseline.stdout + baseline.stderr)
        raise SystemExit('CI control: unmodified selected command must pass')
    print('CI command: ' + commands[0], flush=True)
    print('CI baseline: PASS', flush=True)
    mutations = [m for m in mutations if m['name'] == 'skip-trace-pipeline']
    if len(mutations) != 1:
        raise SystemExit('CI control: expected one committed pipeline-skip plant')
for mutation in mutations:
    path = root / mutation['path']
    original = path.read_bytes()
    before = mutation['before'].encode()
    if original.count(before) != 1:
        raise SystemExit(f"{mutation['name']}: expected exactly one patch site")
    try:
        path.write_bytes(original.replace(before, mutation['after'].encode(), 1))
        result = subprocess.run(
            ci_command or ['go', 'test', '-count=1', '-timeout=45s', '-v',
             '-run', '^' + mutation['test'] + '$', '.'],
            cwd=root, capture_output=True, text=True, timeout=90)
        output = result.stdout + result.stderr
        if result.returncode == 0 or '--- FAIL: ' + mutation['test'] not in output:
            print(output)
            raise SystemExit(f"{mutation['name']}: no named behavioral failure")
        if ci_command:
            print(next(line for line in output.splitlines()
                       if line.startswith('--- FAIL: ' + mutation['test'])))
        print(f"CAUGHT {mutation['name']}: {mutation['test']} (exit {result.returncode})")
    finally:
        path.write_bytes(original)
