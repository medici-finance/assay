"""Run the offline Kubernetes compatibility qualification suites."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
GO_TESTS = {
    "lifecycle": "TestKubeCompatCellctlLifecycle",
    "interaction": "TestKubeCompatCellctlInteraction",
}
COMPANIONS = {
    "controlled-restart": HERE / "qualify_controlled_restart.py",
    "custody-target": HERE / "qualify_custody_target.py",
}


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--offline", action="store_true", help="use a synthetic local API (default)")
    installed = {name: path for name, path in COMPANIONS.items() if path.is_file()}
    parser.add_argument("--suite", choices=[*GO_TESTS, *installed], default="lifecycle")
    parser.add_argument("--self-test", action="store_true", help="run the selected companion suite's controls")
    args = parser.parse_args()
    env = dict(os.environ, KUBECONFIG=os.devnull, GOPROXY="off", GOSUMDB="off",
               GOTOOLCHAIN="local", GOWORK="off", GOFLAGS="")
    if args.suite in installed:
        command = [sys.executable, "-I", str(installed[args.suite]), "--offline"]
        if args.self_test:
            command.append("--self-test")
        return subprocess.run(command, env=env).returncode
    if args.self_test:
        parser.error("--self-test requires a suite with self-test controls")

    test = GO_TESTS[args.suite]
    if args.suite == "interaction":
        env["ASSAY_QUALIFY_INTERACTION"] = "1"

    candidate = HERE.parent / "components" / "compatibility-cellctl" / "candidate.json"
    roles = sorted(json.loads(candidate.read_text(encoding="utf-8"))["images"])
    if len(roles) != 5:
        raise SystemExit("qualification requires the five-role candidate inventory")
    expected = {test, *(f"{test}/{role}" for role in roles)}
    try:
        run = subprocess.run(
            ["go", "test", "-json", "-count=1", "-timeout=5m", "-run", f"^{test}$", "./cmd/cellctl"],
            cwd=ROOT / "tools" / "desk", env=env, capture_output=True, text=True,
        )
    except FileNotFoundError:
        raise SystemExit("could-not-check: the Go toolchain and tools/desk checkout are required")
    passed = set()
    records = {}
    partial = {}
    role_tests = {f"{test}/{role}": role for role in roles}
    marker = "INTERACTION_RECORD "
    for line in run.stdout.splitlines():
        event = json.loads(line)
        if event["Action"] == "pass":
            passed.add(event.get("Test"))
        name = event.get("Test")
        if args.suite == "interaction" and name in role_tests and event["Action"] == "output":
            output = event.get("Output", "")
            # test2json can split one long t.Log record across multiple output events.
            if marker in output:
                partial[name] = output.split(marker, 1)[1]
            elif name in partial:
                partial[name] += output
            if name in partial and partial[name].endswith("\n"):
                record = json.loads(partial.pop(name))
                role = role_tests[name]
                if record.get("role") != role or role in records:
                    raise SystemExit("duplicate or inconsistent interaction record: " + role)
                records[role] = record
        if run.returncode and event.get("Output"):
            sys.stderr.write(event["Output"])
    if run.stderr:
        sys.stderr.write(run.stderr)
    if run.returncode:
        return run.returncode
    missing = expected - passed
    if missing:
        raise SystemExit("missing named test passes: " + ", ".join(sorted(missing)))
    if args.suite == "interaction":
        if partial or set(records) != set(roles):
            raise SystemExit("missing or incomplete five-role interaction records")
        for role in roles:
            print(json.dumps(records[role], sort_keys=True))
    print(json.dumps({"suite": args.suite, "environment": "offline-fixture",
                      "result": "pass", "roles": roles}, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
