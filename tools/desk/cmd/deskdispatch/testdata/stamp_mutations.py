#!/usr/bin/env python3
"""Run only in an owned checkout: transient mutations, restored even on failure.
Usage from repo root: python3 tools/desk/cmd/deskdispatch/testdata/stamp_mutations.py
Each planted defect must produce an assertion failure, not a build failure.
"""
from pathlib import Path
import subprocess

pkg = Path(__file__).resolve().parents[1]
module = pkg.parents[1]
mutations = [
    ("verifier-admission", "../../internal/deskkit/verifierattestation.go", 'if state != ModelStamped || stamp.Model != r.Binding.Model || stamp.Tier != r.Binding.Tier {', 'if false && (state != ModelStamped || stamp.Model != r.Binding.Model || stamp.Tier != r.Binding.Tier) {', "TestVerifierDispatchAdmissionEndToEnd"),
    ("verifier-issuer", "verifierattestation.go", 'receipt, err := deskkit.RecoverVerifierAttestation(home)', 'receipt, err := deskkit.VerifierReceipt{}, error(nil)', "TestVerifierDispatchAdmissionEndToEnd"),
    ("worker-handoff", "prompt.go", 'to the coordinator desk (the-desk), which runs:', 'to the worker itself, which runs:', "TestWorkerDeskPostOpenCoordinatorHandoff"),
    ("caller", "stamponly.go", 'if role != stampRoleForKit(o.kit) {', 'if false {', "TestStampOnlyCaller"),
    ("preview", "dispatch.go", 'if o.dryRun {\n\t\treturn fmt.Sprintf("PLAN:', 'if false {\n\t\treturn fmt.Sprintf("PLAN:', "TestStampOnlyDryRun"),
    ("readback", "../../internal/deskkit/stampapply.go", 'if state != ModelStamped || got != expected {', 'if false {', "TestStampReadbackBothPaths|TestStampOnlyFailure|TestStampOnlyAudit"),
    # A second false-success path, planted in the new entry independently of stepStamp.
    ("second-success", "stamponly.go", 'line, e := stepStamp(o, o.repo)', 'line, e := "OK: skipped", error(nil)', "TestStampOnlyRoundTrip|TestStampOnlyRepair|TestStampOnlyGitLab"),
    ("args", "stamponly.go", 'fs.StringVar(&o.tier, "tier", "",', 'fs.StringVar(&o.tier, "tier", "any",', "TestStampOnlyRejectsArgs"),
    ("pending", "dispatch.go", 'coordinator runs deskdispatch --stamp-only', 'coordinator runs obsolete-command', "TestPendingStampCommand"),
    # Plant an otherwise-successful child process in the bounded path. The recorder
    # must reject it even though the resulting stamp is completely correct.
    ("extra-child", "stamponly.go", 'line, e := stepStamp(o, o.repo)', 'runCmd(".", "deskwt", "add", "unwanted")\n\tline, e := stepStamp(o, o.repo)', "TestStampOnlyRoundTrip|TestStampOnlyReview"),
]
for name, filename, before, after, tests in mutations:
    path = pkg / filename
    original = path.read_text()
    if original.count(before) != 1:
        raise SystemExit(f"{name}: mutation anchor absent/ambiguous")
    try:
        path.write_text(original.replace(before, after))
        p = subprocess.run(["go", "test", "./cmd/deskdispatch", "-run", f"^({tests})$", "-count=1", "-timeout", "60s"], cwd=module, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if p.returncode == 0 or "--- FAIL:" not in p.stdout or "build failed" in p.stdout:
            raise SystemExit(f"{name}: did not observe an assertion failure\n{p.stdout}")
        failures = [line.strip() for line in p.stdout.splitlines() if "--- FAIL:" in line]
        print(f"{name}: killed: " + "; ".join(failures))
    finally:
        path.write_text(original)
