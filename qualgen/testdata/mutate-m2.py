#!/usr/bin/env python3
"""Bounded M2 failure demonstrations; run only in an owned disposable checkout."""
import json
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
mutations = json.loads(Path(__file__).with_name('m2-mutations.json').read_text())
for mutation in mutations:
    path = root / mutation['path']
    original = path.read_bytes()
    before = mutation['before'].encode()
    if original.count(before) != 1:
        raise SystemExit(f"{mutation['name']}: expected exactly one patch site")
    try:
        path.write_bytes(original.replace(before, mutation['after'].encode(), 1))
        result = subprocess.run(
            ['go', 'test', '-count=1', '-timeout=45s', '-v',
             '-run', '^' + mutation['test'] + '$', '.'],
            cwd=root, capture_output=True, text=True, timeout=90)
        output = result.stdout + result.stderr
        if result.returncode == 0 or '--- FAIL: ' + mutation['test'] not in output:
            print(output)
            raise SystemExit(f"{mutation['name']}: no named behavioral failure")
        print(f"CAUGHT {mutation['name']}: {mutation['test']} (exit {result.returncode})")
    finally:
        path.write_bytes(original)
