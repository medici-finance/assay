#!/usr/bin/env python3
"""Offline production-owner mutants. Run from tools/desk; always restore source."""
import os
import re
from pathlib import Path
import subprocess
import sys

cases = [
    ("validation error ignored", "internal/deskkit/activationindex.go",
     'if err == nil && source.valid', 'if (err == nil || err != nil) && source.valid',
     'TestSnapshotValidationError'),
    ("lossy payload admission", "internal/deskkit/activationindex.go",
     'reflect.DeepEqual(data, decoded)', 'reflect.DeepEqual(decoded, decoded)',
     'TestSnapshotByteParity'),
    ("content bytes not hashed", "internal/deskkit/activationindex.go",
     '_, _ = s.hash.Write(data)', '_ = data',
     'TestActivationSnapshotMatchesFreshDiscovery'),
    ("root spelling not bound", "internal/deskkit/activationindex.go",
     'cached.root == root && ', '', 'TestSnapshotSourceBinding'),
    ("root identity not bound", "internal/deskkit/activationindex.go",
     ' && os.SameFile(info, cached.identity)', '', 'TestSnapshotSourceBinding'),
    ("candidate validation ignored", "internal/deskkit/activationindex.go",
     'source.digest() == cached.source', 'true /* planted unvalidated candidate */',
     'TestSnapshotAdmissionRecheck'),
    ("partial read candidate", "internal/deskkit/activation.go",
     'source.valid = false', 'source.valid = true', 'TestSnapshotUnreadableCapture'),
    ("cold duplicate traversal", "internal/deskkit/activationindex.go",
     'activationIndex.slot = &manifestSnapshot{',
     '_, _ = validateManifests(root)\n        activationIndex.slot = &manifestSnapshot{',
     'TestSnapshotColdReference'),
    ("membership shortcut", "internal/deskkit/activationindex.go",
     'source.digest() == cached.source', 'true /* planted HEAD-only equivalent */',
     'TestActivationSnapshotMatchesFreshDiscovery'),
    ("cached acceptance", "internal/deskkit/activation.go",
     'results := ComputeActivation(manifests, cfg.Ext, cfg.Configured())',
     'results := ComputeActivation(manifests, nil, true); _ = cfg',
     'TestActivationSnapshotIsNotAuthority'),
    ("corrupt index accepted", "internal/deskkit/activationindex.go",
     'sha256.Sum256(cached.payload) == cached.checksum', 'true /* planted corrupt acceptance */',
     'TestActivationSnapshotCorruptionAndBounds'),
    ("pre-parser startup scan", "cmd/deskboard/main.go", 'args := os.Args[1:]',
     'args := os.Args[1:]; _ = deskkit.VerbActivationRefusal(".")',
     'TestCompiledStartupDiscoveryWorkload'),
]
selected = sys.argv[1:]
env = dict(os.environ, GOWORK='off', GOPROXY='off', KUBECONFIG='/dev/null')
def run(test):
    return subprocess.run(['go', 'test', './internal/deskkit', '-run', '^'+test+'$',
                           '-count=1', '-timeout=180s', '-v'], env=env,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          text=True, errors="backslashreplace")

def nondiscriminating(output):
    return (re.search(r'build \S+:', output) or '[build failed]' in output
            or 'module lookup disabled' in output or 'panic:' in output
            or 'cannot run' in output or 'exec format error' in output
            or 'fork/exec' in output or 'executable file not found' in output
            or 'context deadline exceeded' in output or 'signal: killed' in output)

# Qualify every discriminator on unmodified source before planting anything.
for test in dict.fromkeys(case[4] for case in cases if not selected or case[0] in selected):
    baseline = run(test)
    if baseline.returncode or f"--- SKIP: {test} (" in baseline.stdout or nondiscriminating(baseline.stdout):
        print(baseline.stdout)
        print('COULD-NOT-CHECK: unmutated baseline did not qualify: '+test)
        raise SystemExit(6)
    print('BASELINE PASS:', test, flush=True)

for name, target, old, new, test in cases:
    if selected and name not in selected:
        continue
    path = Path(target)
    original = path.read_text()
    if original.count(old) != 1:
        print(f'COULD-NOT-CHECK: {name}: expected one production site')
        raise SystemExit(6)
    try:
        path.write_text(original.replace(old, new, 1))
        result = run(test)
        if nondiscriminating(result.stdout):
            print(result.stdout)
            print('COULD-NOT-CHECK: build/tool failure, not a behavioral kill: '+name)
            raise SystemExit(6)
        if f"--- SKIP: {test} (" in result.stdout:
            print(result.stdout)
            print(f'COULD-NOT-CHECK: {name}: required fixture did not execute')
            raise SystemExit(6)
        # A build failure is not a killed behavior mutant.
        if result.returncode == 0 or '--- FAIL: '+test not in result.stdout:
            print(result.stdout)
            raise SystemExit(f'FAILED TO KILL: {name}')
        failures = [line.strip() for line in result.stdout.splitlines()
                    if '--- FAIL:' in line or '.go:' in line]
        print('KILLED:', name, '\n'+'\n'.join(failures[:6]), flush=True)
    finally:
        path.write_text(original)
