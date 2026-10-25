#!/usr/bin/env python3
"""Offline production mutation replay. No source edits, providers or credentials.
Run from any directory: python3 tools/desk/cmd/deskusage/testdata/mutations.py
Each overlay must compile and produce the named test failure; a compile error is not a kill.
"""
import json
from pathlib import Path
import subprocess
import tempfile

MODULE = Path(__file__).resolve().parents[3]
LIB = "internal/deskkit/usage.go"
CMD = "cmd/deskusage/"
MUTATIONS = [
    ("transcript-as-count", LIB, [( 'if s == "could-not-check" {', 'if strings.HasPrefix(s, "{") { return MeasuredInt(0), nil }; if s == "could-not-check" {')], CMD, "TestUsageRecord_RefusesTranscriptShapedInput"),
    ("polluted-store", LIB, [('if len(fields) != 11 {', 'if false {'), ('if len(fields) != 0 {', 'if false {')], CMD, "TestUsageSummary_RefusesPollutedStore"),
    ("git-worktree-store", LIB, [('return "", Refused("usage store must be outside every git working tree")', 'break')], "internal/deskkit/", "TestUsageStore_RefusesGitWorkTree"),
    ("unknown-becomes-zero", LIB, [('r.Schema = UsageSchema', 'r.InputTokens = MeasuredInt(0)\n\tr.Schema = UsageSchema')], "internal/deskkit/", "TestUsageRecord_ThreeState"),
    ("first-dispatch-wins", "internal/deskkit/dispatchrecord.go", [('latest = r\n\t\t\tfound = true', 'if !found { latest = r; found = true }')], CMD, "TestUsage_RecordToSummaryFlow"),
    ("torn-store-wedge", "internal/deskkit/dispatchrecord.go", [('if json.Unmarshal(sc.Bytes(), &r) != nil {', 'if json.Unmarshal(sc.Bytes(), &r) != nil { return latest, Refused("invalid dispatch record")')], CMD, "TestUsageClaimKeyDispatchRecovery"),
    ("ambiguous-tail-accepted", "internal/deskkit/dispatchrecord.go", [('if ambiguous {', 'if false { _ = ambiguous;')], CMD, "TestUsageClaimKeyDispatchRecovery"),
    ("repo-mismatch", CMD + "record.go", [('if d.Repo != r.Repo {', 'if false {')], CMD, "TestUsage_RecordToSummaryFlow"),
    ("last-count-as-sum", CMD + "summary.go", [('s.Sum += n', 's.Sum = n')], CMD, "TestUsage_RecordToSummaryFlow"),
    ("missing-audit", CMD + "main.go", [('deskkit.Log(deskkit.Entry{Tool: \"deskusage\", Verb: verb, Result: result, Detail: \"local usage counts\"})', 'func() error { _ = result; return nil }()')], CMD, "TestUsageAuditAndStop"),
    ("stop-ignored", CMD + "main.go", [('err := deskkit.Guard()', 'var err error')], CMD, "TestUsageAuditAndStop"),
    ("unlocked-append", LIB, [('if e = TryLockExclusive(f); e != nil {\n\t\treturn fmt.Errorf("usage store lock: %w", e)', 'if false {\n\t\treturn fmt.Errorf("usage store lock: %w", e)')], "internal/deskkit/", "TestUsageStore_RefusesGitWorkTree"),
    ("file-symlink-store", LIB, [('err == nil && !st.Mode().IsRegular()', 'err == nil && !st.Mode().IsRegular() && false'), ('return openRosterBeaconFile(path, create)', 'return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)')], "internal/deskkit/", "TestUsageStore_RefusesGitWorkTree"),
    ("public-counts", LIB, [('f.Chmod(0600)', 'f.Chmod(0644)')], "internal/deskkit/", "TestUsageRecord_ThreeState"),
]

def run():
    for name, file, replacements, package, test in MUTATIONS:
        source = MODULE / file
        changed = source.read_text()
        for old, new in replacements:
            if changed.count(old) != 1:
                raise SystemExit(f"{name}: expected one production anchor {old!r}")
            changed = changed.replace(old, new, 1)
        # A bounded overlay avoids copying tracked sources or changing a working file.
        with tempfile.TemporaryDirectory(prefix=".usage-mutation-", dir=MODULE) as tmp:
            tmp = Path(tmp)
            alternate = tmp / source.name
            alternate.write_text(changed)
            overlay = tmp / "overlay.json"
            overlay.write_text(json.dumps({"Replace": {str(source): str(alternate)}}))
            result = subprocess.run(["go", "test", f"-overlay={overlay}", "-timeout=45s", "-count=1", "-v", "-run", f"^{test}$", f"./{package}"], cwd=MODULE, capture_output=True, text=True, timeout=60)
            output = result.stdout + result.stderr
            if result.returncode == 0 or f"--- FAIL: {test}" not in output or "[build failed]" in output:
                raise SystemExit(f"{name}: not killed by behavioral assertion\n{output}")
            print(f"KILLED {name}: {test}", flush=True)
    print(f"{len(MUTATIONS)}/{len(MUTATIONS)} production mutations killed", flush=True)

if __name__ == "__main__":
    run()
