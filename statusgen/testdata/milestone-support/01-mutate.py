#!/usr/bin/env python3
"""Bounded offline plants in the owned worktree; restore every source in finally.

Run after committing. A build error or missing named failure never counts as a kill.
Use --name NAME to reproduce one manifest entry. No network/provider/cluster access.
"""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess

ROOT = Path(__file__).resolve().parents[3]
MANIFEST = Path(__file__).with_name("01-mutations.json")

def interrupted(signum, _frame):
    raise SystemExit(f"interrupted by signal {signum}; plant restored")

for sig in (signal.SIGTERM, signal.SIGHUP):
    signal.signal(sig, interrupted)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--name")
    args = parser.parse_args()
    manifest = json.loads(MANIFEST.read_text())
    if manifest.get("schema") != "workref-mutations-v1":
        raise SystemExit("unsupported mutation manifest schema")
    mutations = manifest["cases"]
    if not mutations:
        raise SystemExit("empty mutation manifest")
    if args.name:
        mutations = [m for m in mutations if m["name"] == args.name]
        if not mutations:
            raise SystemExit("unknown mutation")
    env = dict(os.environ, KUBECONFIG="/dev/null", GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local")
    baselines = set()
    for mutation in mutations:
        command = ["go", "test", "-count=1", "-timeout=150s", "-v", "-run", "^" + mutation["test"] + "$", mutation["package"]]
        baseline = (mutation["module"], mutation["package"], mutation["test"])
        if baseline not in baselines:
            result = subprocess.run(command, cwd=ROOT / mutation["module"], env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=170)
            if result.returncode != 0 or "--- PASS: " + mutation["test"] + " " not in result.stdout:
                print(result.stdout)
                raise SystemExit("missing named baseline pass: " + mutation["name"])
            baselines.add(baseline)
            print("BASELINE PASS " + mutation["test"], flush=True)
        path = ROOT / mutation["target"]
        original = path.read_bytes() if path.exists() else None
        if "plant" in mutation:
            if original is not None:
                raise SystemExit("plant target exists")
            changed = mutation["plant"].encode()
        else:
            check = subprocess.run(["git", "diff", "--quiet", "HEAD", "--", mutation["target"]], cwd=ROOT)
            if check.returncode != 0:
                raise SystemExit("source differs from committed baseline: " + mutation["target"])
            before = mutation["old"].encode()
            if original.count(before) != 1:
                raise SystemExit("expected exactly one anchor: " + mutation["name"])
            changed = original.replace(before, mutation["new"].encode(), 1)
        try:
            path.write_bytes(changed)
            result = subprocess.run(command, cwd=ROOT / mutation["module"], env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=170)
            if result.returncode == 0 or "--- FAIL: " + mutation["test"] not in result.stdout or "[build failed]" in result.stdout or "[setup failed]" in result.stdout:
                print(result.stdout)
                raise SystemExit("missing named behavioral failure: " + mutation["name"])
            print("CAUGHT " + mutation["name"] + ": " + mutation["test"], flush=True)
        finally:
            if original is None:
                path.unlink()
            else:
                path.write_bytes(original)

if __name__ == "__main__":
    main()
