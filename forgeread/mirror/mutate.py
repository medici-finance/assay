#!/usr/bin/env python3
"""Run discriminating controls against disposable copies of production sources.

The original module is never edited. A mutation counts only when its named Go
behavior test actually fails; compilation errors and missing tests do not count.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

module = Path(__file__).resolve().parents[1]
manifest = json.loads(Path(__file__).with_name('mutations.json').read_text())
env = dict(os.environ, GOWORK='off', GOPROXY='off', GOTOOLCHAIN='local')
for mutation in manifest:
    with tempfile.TemporaryDirectory(prefix='mirror-mutation-') as scratch:
        target = Path(scratch) / 'forgeread'
        shutil.copytree(module, target)
        for edit in mutation['edits']:
            source = target / edit['file']
            text = source.read_text()
            if text.count(edit['find']) != 1:
                raise SystemExit(f"{mutation['id']}: mutation target is missing or ambiguous")
            source.write_text(text.replace(edit['find'], edit['replace'], 1))
        command = ['go', 'test', '-count=1', '-timeout=30s', '-v',
                   '-run', '^' + mutation['test'] + '$', './mirror/...']
        result = subprocess.run(command, cwd=target, env=env, text=True,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                timeout=45)
        marker = '--- FAIL: ' + mutation['test'] + ' '
        if result.returncode == 0 or marker not in result.stdout:
            print(result.stdout)
            raise SystemExit(f"{mutation['id']}: mutant survived or test did not execute")
        print(f"CAUGHT {mutation['id']}: {mutation['test']}")
print(f"CAUGHT {len(manifest)}/{len(manifest)} production-source mutations")
