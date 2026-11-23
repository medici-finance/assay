"""Prove restart assertions detect altered, inert fixture observations."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


HERE = Path(__file__).resolve().parents[2]
DESK = HERE.parents[2] / "tools/desk"
GO_TEST = DESK / "cmd/cellctl/kube_compat_qualification_test.go"


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise ValueError(f"expected one fixture control point: {old}")
    return source.replace(old, new)


def require_red(command, env, assertions):
    run = subprocess.run(command, cwd=DESK, env=env, capture_output=True, text=True, timeout=180)
    output = run.stdout + run.stderr
    missing = [message for message in assertions if message not in output]
    if run.returncode == 0 or missing:
        sys.stderr.write(output)
        raise ValueError(f"expected a failing run and these assertions: {assertions}")
    for message in assertions:
        print("detected:", message)


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--offline", required=True, action="store_true")
    parser.parse_args()
    env = dict(os.environ, KUBECONFIG=os.devnull, GOWORK="off", GOTOOLCHAIN="local",
               GOPROXY="off", GOSUMDB="off", GOFLAGS="")
    env.pop("ASSAY_COMPAT_TEST_MUTATION", None)
    with tempfile.TemporaryDirectory(prefix="controlled-restart-controls-") as scratch:
        root = Path(scratch)
        # Keep a valid receipt and a matching observed image. Both streams now
        # succeed, so each test that expects its precise refusal must go red.
        modified = root / "records.go"
        source = replace_once(GO_TEST.read_text(), "os.Remove(path)", "os.Chmod(path, 0600)")
        source = replace_once(source, 's.Profile.Roles["intake-desk"].Image', "s.Profile.Roles[role].Image")
        modified.write_text(source)
        overlay = root / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {str(GO_TEST): str(modified)}}))
        require_red(["go", "test", "-count=1", "-overlay", str(overlay), "-run",
                     "^TestKubeCompatRestartHolds$/(missing-receipt|wrong-candidate-image)$", "./cmd/cellctl"],
                    env, ["expected attach refusal no measured current execution",
                          "expected attach refusal execution could not be measured"])

        for name in ("desk-session.py", "desk-session.test.py", "fake-agent.fixture.py"):
            shutil.copyfile(HERE / name, root / name)
        command = [sys.executable, "-I", str(root / "desk-session.test.py")]
        agent = root / "fake-agent.fixture.py"
        original = agent.read_text()
        agent.write_text(replace_once(original, "hex=retained.read_bytes().hex()", "hex='00'"))
        require_red(command, env, ["AssertionError: ('new execution did not read the retained terminal byte'"])
        agent.write_text(original)
        # The actual launcher has already refused. Introduce only an inert
        # journal entry to demonstrate that an extra launch invalidates a hold.
        driver = root / "desk-session.test.py"
        assertion = " assert rc==78 and len(events(w))==before_unclean_retry"
        added = """ with (w/'events.jsonl').open('a') as log:
  print(json.dumps({'kind':'agent','fixture':'synthetic observation'}),file=log)
"""
        driver.write_text(replace_once(driver.read_text(), assertion, added + assertion))
        require_red(command, env, ["AssertionError: ('unclean predecessor must hold with zero additional launches'"])


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        sys.exit(f"restart assertion control failed: {error}")
