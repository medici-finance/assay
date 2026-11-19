"""Run the first offline Kubernetes compatibility qualification milestone."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
LIFECYCLE = "TestKubeCompatCellctlLifecycle"


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--offline", action="store_true", help="use a synthetic local API (default)")
    parser.add_argument("--suite", choices=["lifecycle"], default="lifecycle")
    parser.parse_args()

    candidate = HERE.parent / "components" / "compatibility-cellctl" / "candidate.json"
    roles = sorted(json.loads(candidate.read_text(encoding="utf-8"))["images"])
    if len(roles) != 5:
        raise SystemExit("qualification requires the five-role candidate inventory")
    expected = {LIFECYCLE, *(f"{LIFECYCLE}/{role}" for role in roles)}
    env = dict(os.environ, KUBECONFIG=os.devnull, GOPROXY="off", GOSUMDB="off",
               GOTOOLCHAIN="local", GOWORK="off", GOFLAGS="")
    try:
        run = subprocess.run(
            ["go", "test", "-json", "-count=1", "-timeout=5m", "-run", f"^{LIFECYCLE}$", "./cmd/cellctl"],
            cwd=ROOT / "tools" / "desk", env=env, capture_output=True, text=True,
        )
    except FileNotFoundError:
        raise SystemExit("could-not-check: the Go toolchain and tools/desk checkout are required")
    passed = set()
    for line in run.stdout.splitlines():
        event = json.loads(line)
        if event["Action"] == "pass":
            passed.add(event.get("Test"))
        if run.returncode and event.get("Output"):
            sys.stderr.write(event["Output"])
    if run.stderr:
        sys.stderr.write(run.stderr)
    if run.returncode:
        return run.returncode
    missing = expected - passed
    if missing:
        raise SystemExit("missing named test passes: " + ", ".join(sorted(missing)))
    print(json.dumps({"suite": "lifecycle", "environment": "offline-fixture",
                      "result": "pass", "roles": roles}, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
