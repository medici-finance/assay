"""Exercise explicit stop/start through compiled cellctl and the Linux PID-1 fixture."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
CMD = "./cmd/cellctl"
NATIVE = "./internal/kubecompat/native"
CORE = "./internal/kubecompat"


def go_checks(package, groups, env):
    names = set(groups)
    names.update(f"{name}/{case}" for name, cases in groups.items() for case in cases)
    expression = "^(" + "|".join(groups) + ")$"
    run = subprocess.run(["go", "test", "-json", "-count=1", "-timeout=5m", "-run", expression, package],
                         cwd=ROOT / "tools/desk", env=env, capture_output=True, text=True)
    events = [json.loads(line) for line in run.stdout.splitlines()]
    if run.returncode:
        sys.stderr.write("".join(e.get("Output", "") for e in events) + run.stderr)
        raise SystemExit(run.returncode)
    missing = names - {e.get("Test") for e in events if e["Action"] == "pass"}
    if missing:
        raise SystemExit("could-not-check: missing named Go passes: " + ", ".join(sorted(missing)))
    for event in events:
        if event.get("Test") == "TestKubeCompatControlledRestart" and "cellctl-sha256=" in event.get("Output", ""):
            print(event["Output"].strip())


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, allow_abbrev=False,
        epilog="Set ASSAY_TEST_DOCKER_HOST to an explicit local Unix socket. "
               "The PID-1 fixture uses cached python:3.13-slim or a local sha256 image ID "
               "in ASSAY_TEST_FIXTURE_IMAGE. Every test container has networking disabled.",
    )
    parser.add_argument("--offline", action="store_true", required=True)
    parser.add_argument("--self-test", action="store_true", help="also exercise refusal fixtures")
    args = parser.parse_args()
    # Reuse the packaging check for the five published role pins and their
    # release source. The native API and image runtime below are synthetic.
    checker = runpy.run_path(str(HERE.parent / "check.py"))
    try:
        pins = checker["candidate_images"]()
    except (checker["Invalid"], checker["Unavailable"]) as error:
        raise SystemExit(f"could-not-check: {error}") from error
    env = dict(os.environ, KUBECONFIG=os.devnull, GOWORK="off", GOTOOLCHAIN="local",
               GOPROXY="off", GOSUMDB="off", GOFLAGS="")
    env.pop("ASSAY_COMPAT_TEST_MUTATION", None)
    groups = {"TestKubeCompatControlledRestart": []}
    if args.self_test:
        groups["TestKubeCompatRestartHolds"] = ["missing-receipt", "stale-receipt", "changed-pod",
                                                "same-pod-new-container", "unknown-stop", "wrong-candidate-image"]
        # Interaction owns the instrumented model adapter. A missing neighbour
        # must surface as a missing named control, even though Go exits zero.
        groups["TestKubeCompatInteractionCountControls"] = ["duplicate-start", "model-on-attach"]
        env["ASSAY_QUALIFY_INTERACTION"] = "1"
        groups["TestKubeCompatStopUncertainty"] = ["busy", "unknown-work", "absent", "changed",
                                                   "no-ack", "survivor", "partition"]
    go_checks(CMD, groups, env)
    if args.self_test:
        go_checks(NATIVE, {"TestKubeCompatUpgradeRace": []}, env)
        go_checks(CORE, {"TestKubeCompatStopHolds": ["partition", "absent", "changed", "new-container",
                                                     "no-ack", "survivor"]}, env)
    run = subprocess.run([sys.executable, "-I", str(HERE / "desk-session.test.py")],
                         env=env, capture_output=True, text=True)
    if run.returncode:
        sys.stderr.write(run.stdout + run.stderr)
        return run.returncode
    results = [json.loads(line) for line in run.stdout.splitlines() if line.startswith("{")]
    if len(results) != 1 or results[0].get("suite") != "pid1-controlled-restart":
        raise SystemExit("could-not-check: missing PID-1 controlled restart result")
    if args.self_test:
        controls = HERE / "testdata/qualification/restart_controls.py"
        subprocess.run([sys.executable, "-I", str(controls), "--offline"], env=env, check=True)
    source = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, env=env, text=True).strip()
    go_version = subprocess.check_output(["go", "version"], env=env, text=True).strip()
    print(json.dumps({"source": source, "go": go_version, "python": sys.version.split()[0],
                      "harness_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                      "self_test": args.self_test, "suite": "controlled-restart", "environment": "offline-fixture", "result": "pass",
                      "retained_file": results[0], "same_process_after_restart": False,
                      "conversation_resume": "unsupported", "work_effect_acceptance": "synthetic file only",
                      "candidate_image_reference": pins["worker-desk"]}, sort_keys=True))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        sys.exit(f"could-not-check: {error}")
