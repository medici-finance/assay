#!/usr/bin/env python3
"""Run bounded offline controls from the statusgen module; restore every mutation."""
import os
from pathlib import Path
import subprocess

module = Path(__file__).resolve().parents[2]
source = module / 'regressionfloor_test.go'
inventory = module / 'testdata/regressionfloor/inventory.yaml'
env = dict(os.environ, KUBECONFIG='/dev/null', GOWORK='off', GOPROXY='off', GOTOOLCHAIN='local')
controls = [
 ('inventory-name', inventory, 'TestCoverageCouldNotCheckIsNotPass', 'TestCoverageMissing', 'TestRegressionFloorInventoryResolves', 'inventory test does not resolve'),
 ('uncertainty-pass', source, 'return old, newer', 'if newer.Claim == "could-not-check" { newer.Claim = "pass"; newer.Released = true }; return old, newer', 'TestRegressionFloorOldNewAgree', 'unlisted difference unavailable'),
 ('stale-difference', source, 'if equal && match != nil {', 'if false && equal && match != nil {', 'TestRegressionFloorUnlistedDifferenceFails', 'stale difference accepted'),
 ('unlisted-difference', source, 'if match == nil {\n\t\t\treturn fmt.Errorf("unlisted difference %s", id)\n\t\t}', 'if match == nil { return nil }', 'TestRegressionFloorUnlistedDifferenceFails', 'unlisted difference accepted'),
 ('empty-selection', source, 'return inv, nil, nil, fmt.Errorf("zero selected cases")', 'return inv, nil, nil, nil', 'TestRegressionFloorZeroSelectedFails', 'zero selected cases accepted'),
 ('missing-selected', source, 'if len(selected) > 0 {', 'if false && len(selected) > 0 {', 'TestRegressionFloorZeroSelectedFails', 'missing-selected guard did not refuse'),
 ('unselected-file', source, 'if !selected[c.ID] {', 'if false && !selected[c.ID] {', 'TestRegressionFloorZeroSelectedFails', 'unselected-file guard did not refuse'),
 ('duplicate-allowance', source, 'if match != nil {', 'if false && match != nil {', 'TestRegressionFloorUnlistedDifferenceFails', 'duplicate allowance guard did not refuse'),
 ('unknown-allowance', source, 'if !known[d.Case] {', 'if false && !known[d.Case] {', 'TestRegressionFloorUnlistedDifferenceFails', 'unknown allowance guard did not refuse'),
 ('empty-reason', source, 'strings.TrimSpace(match.Reason) == "" || ', '', 'TestRegressionFloorUnlistedDifferenceFails', 'empty reason guard did not refuse'),
 ('shared-expectation', source, 'if !reflect.DeepEqual(old, expected) {\n\t\treturn fmt.Errorf("expectation mismatch', 'if false && !reflect.DeepEqual(old, expected) {\n\t\treturn fmt.Errorf("expectation mismatch', 'TestRegressionFloorUnlistedDifferenceFails', 'shared expected outcome guard did not refuse'),
 ('allowed-expectation', source, 'if !reflect.DeepEqual(old, expected) {\n\t\t\treturn fmt.Errorf("old expectation mismatch', 'if false && !reflect.DeepEqual(old, expected) {\n\t\t\treturn fmt.Errorf("old expectation mismatch', 'TestRegressionFloorUnlistedDifferenceFails', 'allowed old expectation guard did not refuse'),
 ('allowance-old-outcome', source, '!reflect.DeepEqual(old, match.Old) || ', '', 'TestRegressionFloorUnlistedDifferenceFails', 'different old outcome accepted'),
 ('allowance-new-outcome', source, ' || !reflect.DeepEqual(newer, match.New)', '', 'TestRegressionFloorUnlistedDifferenceFails', 'different new outcome accepted'),
 ('case-identity', source, 'if c.ID == "" || filepath.Base(p) != c.ID+".json" {', 'if false && (c.ID == "" || filepath.Base(p) != c.ID+".json") {', 'TestRegressionFloorZeroSelectedFails', 'case identity guard did not refuse'),
 ('freeze-source', module / 'testdata/regressionfloor/freeze.go', 'if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(source) {', 'if false && !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(source) {', 'TestFloorFreezeAuthoring', 'invalid pinned source accepted'),
 ('freeze-method', module / 'testdata/regressionfloor/freeze.go', 'fn.Recv == nil && ', '', 'TestFloorFreezeAuthoring', 'method selected instead of retained test function'),
]
for name, path, old, new, test, witness in controls:
    original = path.read_text()
    if original.count(old) != 1:
        raise SystemExit(f'{name}: mutation anchor not unique')
    try:
        path.write_text(original.replace(old, new, 1))
        run = subprocess.run(['go', 'test', '-timeout', '45s', '-count=1', '-v', '-run', f'^{test}$', '.'], cwd=module, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=75)
        if run.returncode == 0 or witness not in run.stdout or f'--- FAIL: {test}' not in run.stdout:
            print(run.stdout)
            raise SystemExit(f'{name}: no named behavioral kill')
        print(f'KILLED {name}: {witness}', flush=True)
    finally:
        path.write_text(original)
