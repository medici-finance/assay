#!/usr/bin/env python3
"""Run in an owned checkout; each guard removal must cause an assertion failure."""
from pathlib import Path
import subprocess

pkg = Path(__file__).resolve().parents[1]
module = pkg.parents[1]
path = pkg / "verifierattestation.go"
mutations = [
    ("typed-issue-author", 'strings.HasPrefix(title, VerifierAttestationTitle) && verifierAuthority(author)', 'strings.HasPrefix(title, VerifierAttestationTitle)', "TestAttestationExcludedDuringOpenWindowBothForges"),
    ("qualified-source", '"rev-parse", "refs/remotes/origin/main"', '"rev-parse", "origin/main"', "TestAttestRemoteRef"),
    ("actor-separation", "!SameActor(desk, verifier)", 'verifier != ""', "TestAttestActorSeparation"),
    ("detached-head", 'head != b.Source || branch != "HEAD"', 'head != b.Source || (false && branch != \"HEAD\")', "TestAttestSourceClosure/branch"),
    ("source-commit", 'head != b.Source || branch != "HEAD"', '(false && head != b.Source) || branch != "HEAD"', "TestAttestSourceClosure/commit"),
    ("tracked-source", 'name != "" && name != b.Brief && name != index', 'false && name != "" && name != b.Brief && name != index', "TestAttestSourceClosure/(tracked|staged)"),
    ("all-additional-files", 'if name != "" {', 'if false && name != "" {', "TestAttestSourceClosure/(untracked|ignored|second-site)"),
    ("ignored-files", '"ls-files", "--others", "-z", "--"', '"ls-files", "--others", "--exclude-standard", "-z", "--"', "TestAttestSourceClosure/ignored"),
]
for name, before, after, tests in mutations:
    original = path.read_text()
    if original.count(before) != 1:
        raise SystemExit(f"{name}: mutation anchor absent/ambiguous")
    try:
        path.write_text(original.replace(before, after))
        run = subprocess.run(["go", "test", "./internal/deskkit", "-run", tests, "-count=1", "-timeout", "60s"], cwd=module, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if run.returncode == 0 or "--- FAIL:" not in run.stdout or "build failed" in run.stdout:
            raise SystemExit(f"{name}: no assertion failure\n{run.stdout}")
        print(f"{name}: killed")
        for line in run.stdout.splitlines():
            if "--- FAIL:" in line or "want refusal" in line or "actor admitted" in line:
                print(line)
    finally:
        path.write_text(original)
