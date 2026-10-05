#!/usr/bin/env python3
"""Run cache mutation evidence in an isolated checkout, restoring each file.

Run from tools/desk: python3 internal/cellcache/mutate.py
Only behavior-test failures count; compilation failures do not.
"""
import json
import os
from pathlib import Path
import shlex
import subprocess

module = Path(__file__).resolve().parents[2]
spec = json.loads(Path(__file__).with_name("mutations.json").read_text())
command = shlex.split(spec["test"])
env = dict(os.environ, GOPROXY="off", GOTOOLCHAIN="local", KUBECONFIG="/dev/null")
env.pop("BASH_ENV", None)
env.pop("ENV", None)

def check():
    return subprocess.run(command, cwd=module, env=env, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          timeout=90)

base = check()
if base.returncode:
    print(base.stdout)
    raise SystemExit("baseline failed")
failed = []
for mutation in spec["mutations"]:
    path = module / mutation["file"]
    original = path.read_text()
    if mutation["old"] not in original:
        raise SystemExit("mutation anchor missing: " + mutation["name"])
    try:
        path.write_text(original.replace(mutation["old"], mutation["new"], 1))
        result = check()
        caught = result.returncode != 0 and "--- FAIL:" in result.stdout
        print(("CAUGHT " if caught else "MISSED ") + mutation["name"], flush=True)
        for line in result.stdout.splitlines():
            if "--- FAIL:" in line or "uncustodied raw launch" in line:
                print(line, flush=True)
        if not caught:
            failed.append(mutation["name"])
            print(result.stdout, flush=True)
    finally:
        path.write_text(original)
if failed:
    raise SystemExit("mutations not behavior-caught: " + ", ".join(failed))
print("All cache mutants caught; original files restored.")
