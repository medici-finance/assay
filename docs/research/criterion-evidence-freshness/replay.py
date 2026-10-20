#!/usr/bin/env python3
"""Offline research replay. No production acceptance or lifecycle authority."""

import argparse
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import tempfile


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
BRIEF = "docs/streams/study/brief-01-numeric.md"
REQ = "docs/streams/requirements/numeric-condition.md"
LIMIT = "docs/streams/study/inherited-limit.txt"
REQ_ID = "REQ-numeric-condition"
BASE_CRITERIA = ["Value is at least one.",
                 "Value is at least one and the inherited limit is positive."]
CHECK = """from pathlib import Path
from value import value

def test_value():
    assert value() >= 1
    assert int(Path('docs/streams/study/inherited-limit.txt').read_text()) > 0

test_value()
print('numeric check passed')
"""


def digest(data):
    return hashlib.sha256(data).hexdigest()


def cli_json_bytes(value, layout):
    """Reconstruct this pinned CLI's declared JSON codec for validation only."""
    if layout not in ("compact", "indent-2"):
        raise ValueError("unsupported CLI JSON layout")
    opts = {"ensure_ascii": False, "allow_nan": False}
    if layout == "compact":
        opts["separators"] = (",", ":")
    else:
        opts["indent"] = 2
    text = json.dumps(value, **opts)
    # encoding/json's documented HTML escaping, including JavaScript line separators.
    for char, escaped in (("&", "\\u0026"), ("<", "\\u003c"), (">", "\\u003e"),
                          ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        text = text.replace(char, escaped)
    return (text + "\n").encode("utf-8")


def check_json_record(record):
    if record["format"] != "decoded-cli-json/v1":
        raise ValueError("unknown structured stdout format")
    meta = record["provenance"]
    if meta["encoding"] != "UTF-8" or meta["string_contents_changed"] is not False:
        raise ValueError("JSON string content or encoding changed")
    if not re.fullmatch(r"raw-json/[0-9]{4}\.stdout", meta["artifact"]):
        raise ValueError("raw artifact reference is not a portable capture identifier")
    reconstructed = cli_json_bytes(record["value"], meta["layout"])
    if len(reconstructed) != meta["bytes"] or digest(reconstructed) != meta["sha256"]:
        raise ValueError("decoded JSON does not reproduce its original byte commitment")


def json_outputs(data):
    return [command["stdout"] for case in data["cases"] for command in case["commands"]
            if isinstance(command["stdout"], dict)]


def put(root, name, text):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)


def clean_env(scratch):
    # Keep only execution/build inputs, never provider credentials or desk custody.
    env = {k: os.environ[k] for k in ("PATH", "GOCACHE", "GOMODCACHE") if k in os.environ}
    env.update({
        "HOME": str(scratch / "home"), "TMPDIR": str(scratch / "tmp"),
        "KUBECONFIG": "/dev/null", "GOWORK": "off", "GOENV": "off",
        "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off",
        "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
        "GIT_CONFIG_COUNT": "3", "GIT_CONFIG_KEY_0": "user.name",
        "GIT_CONFIG_VALUE_0": "synthetic-fixture",
        "GIT_CONFIG_KEY_1": "user.email",
        "GIT_CONFIG_VALUE_1": "fixture@example.invalid",
        "GIT_CONFIG_KEY_2": "commit.gpgsign", "GIT_CONFIG_VALUE_2": "false",
        "GIT_AUTHOR_DATE": "2026-10-16T00:00:00Z",
        "GIT_COMMITTER_DATE": "2026-10-16T00:00:00Z", "LC_ALL": "C",
        "PYTHONDONTWRITEBYTECODE": "1",
    })
    # Existing read-only build caches are permitted; use the caller's normal caches
    # when those variables were not explicitly set. Downloads remain disabled.
    for key, suffix in (("GOCACHE", "Library/Caches/go-build"),
                        ("GOMODCACHE", "go/pkg/mod")):
        env.setdefault(key, str(Path.home() / suffix))
    for path in (scratch / "home", scratch / "tmp"):
        path.mkdir()
    return env


class Runner:
    def __init__(self, env, scratch, raw_dir):
        self.env, self.scratch, self.log = env, scratch, []
        self.raw_dir = raw_dir
        self.raw_dir.mkdir(parents=True, exist_ok=False)
        (self.raw_dir / "raw-json").mkdir()
        self.json_count = 0

    def display(self, text):
        # HOME is a separate fixture capability, not a machine locator. Normalize
        # it first, consistently in argv/cwd/stdout/stderr, preserving its suffix.
        return (text.replace(str(self.scratch / "home"), "<fixture-home>")
                .replace(str(self.scratch), "<scratch>")
                .replace(str(ROOT), "<checkout>"))

    def run(self, args, cwd, allow=(0,), timeout=60):
        proc = subprocess.run([str(a) for a in args], cwd=cwd, env=self.env,
                              capture_output=True, timeout=timeout)
        # Capture actual bytes before decoding. Never synthesize a raw artifact
        # from the structured observation or from a predicted result.
        stdout = proc.stdout.decode("utf-8")
        stderr = proc.stderr.decode("utf-8")
        captured = self.display(stdout)
        if "--json" in args and proc.returncode == 0:
            self.json_count += 1
            artifact = f"raw-json/{self.json_count:04d}.stdout"
            (self.raw_dir / artifact).write_bytes(proc.stdout)
            value = json.loads(stdout)
            # These finite cases have no machine paths in JSON stdout. Other
            # text channels retain their existing explicit path aliases. Refuse
            # an unmodelled JSON-path redaction rather than claiming byte identity.
            if self.display(stdout) != stdout:
                raise ValueError("CLI JSON needs path redaction; byte equivalence is unestablished")
            layout = "compact" if "--coverage" in args else "indent-2"
            captured = {"format": "decoded-cli-json/v1", "value": value,
                        "provenance": {"encoding": "UTF-8", "layout": layout,
                            "artifact": artifact, "sha256": digest(proc.stdout),
                            "bytes": len(proc.stdout), "string_contents_changed": False,
                            "capture": "original process stdout bytes; private runtime artifact",
                            "codec": "Go encoding/json HTML escaping; one final LF"}}
            check_json_record(captured)
        rec = {"argv": [self.display(str(a)) for a in args],
               "cwd": self.display(str(cwd)), "exit": proc.returncode,
               "stdout": captured, "stderr": self.display(stderr),
               "executed": True}
        self.log.append(rec)
        if proc.returncode not in allow:
            raise ValueError(f"command failed: {rec}")
        return stdout


def requirement(criteria):
    acceptance = "\n".join("  - " + json.dumps(c) for c in criteria)
    return f"""---
id: {REQ_ID}
date: 2026-10-16
title: Synthetic numeric condition
impact: major
asked-by: synthetic fixture
acceptance:
{acceptance}
status: satisfied
satisfied-by: [study/01]
---
Synthetic authored satisfaction claim, not an observation of current acceptance.
"""


def fixture_brief(declare):
    files = "`value.py`, `check_value.py`"
    if declare:
        files += f", `{REQ}`"
    return f"""---
brief: study/01
title: Synthetic numeric condition
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {{regulatory: no, customer: no, irreversible: no, sensitive-data: no}}
issues: []
schema: brief-v1
authored: 2026-10-16 by synthetic fixture
sources: ["synthetic requirement"]
satisfies: [{REQ_ID}]
---
# Synthetic brief

## Context
files: {files}

## Verify
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `python3 check_value.py` | exit 0 |

## Evidence

## Review
Synthetic fixture only; no independent review is claimed.
"""


def commit(run, root, message):
    names = ["docs", "value.py", "check_value.py", "notes.txt"]
    run.run(["git", "add", "--", *names], root)
    run.run(["git", "commit", "-q", "--allow-empty", "-m", message], root)
    return run.run(["git", "rev-parse", "HEAD"], root).strip()


def change_fixture(root, case):
    change = case["change"]
    criteria = list(BASE_CRITERIA)
    if change == "tighten":
        criteria[0] = "Value is at least two."
    elif change == "add":
        criteria.append("Value is at most zero.")
    elif change == "wording":
        criteria[0] = "Value is >= 1."
    elif change == "reorder":
        criteria.reverse()
    elif change == "split":
        criteria = ["Value is at least one.", "Value is at least one.",
                    "The inherited limit is positive."]
    elif change == "retire":
        criteria = [criteria[0]]
    elif change == "inherited":
        put(root, LIMIT, "0\n")
    elif change in ("command", "expect"):
        brief = (root / BRIEF).read_text()
        if change == "command":
            brief = brief.replace("`python3 check_value.py`", "`python3 check_value.py && test -f value.py`", 1)
        else:
            brief = brief.replace("| exit 0 |", "| exit 0; output contains EXTRA |")
        put(root, BRIEF, brief)
    elif change == "weaken":
        put(root, "check_value.py", "def test_value():\n    pass\ntest_value()\nprint('numeric check passed')\n")
    elif change == "unrelated":
        put(root, "notes.txt", "unrelated note, revised\n")
    elif change == "unavailable":
        (root / REQ).unlink()  # Only this run's synthetic fixture.
        return
    elif change != "none":
        raise ValueError("unknown fixture change: " + change)
    put(root, REQ, requirement(criteria))


def current_probe(change):
    # Independent, explicitly limited numeric oracle; never a production coverage rule.
    expression = {"tighten": "v >= 2 and limit > 0",
                  "add": "v >= 1 and limit > 0 and v <= 0",
                  "retire": "v >= 1"}.get(change, "v >= 1 and limit > 0")
    return ("from pathlib import Path; from value import value; import sys; "
            f"v=value(); limit=int(Path('{LIMIT}').read_text()); "
            f"ok=({expression}); print('numeric current condition:', ok); sys.exit(0 if ok else 1)")


def observe_case(run, binary, scratch, case):
    root = scratch / "fixtures" / case["id"]
    root.mkdir(parents=True)
    start = len(run.log)
    put(root, "docs/streams/study/README.md", """---
stream: study
repo: example-org/study
status: active
priority: P1
---
# Synthetic study
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|---|---|---|---|---|---|
| 01 | [Numeric](brief-01-numeric.md) | 0 | S | done | 2026-10-16 synthetic-fixture | 2026-10-16 synthetic-fixture |
""")
    put(root, BRIEF, fixture_brief(case.get("declare_requirement", False)))
    put(root, REQ, requirement(BASE_CRITERIA))
    put(root, LIMIT, "1\n")
    put(root, "value.py", "def value():\n    return 1\n")
    put(root, "check_value.py", CHECK)
    put(root, "notes.txt", "unrelated note\n")
    run.run(["git", "init", "-q", "--initial-branch=main"], root)
    witness_rev = commit(run, root, "synthetic original question")
    before_req = digest((root / REQ).read_bytes())
    before_limit = digest((root / LIMIT).read_bytes())
    # The real runner executes this check and writes its actual witness. No
    # authored table is substituted for execution, including in the saved results.
    run.run([binary, "verifyrun", "--root", root, "--brief", root / BRIEF,
             "--timeout", "10s"], root)
    witness = (root / BRIEF).read_text().split("## Evidence\n", 1)[1].split("## Review", 1)[0]
    baseline = json.loads(run.run([binary, "--root", root, "--coverage", "--json"], root))
    if len(baseline) != 1 or not baseline[0]["released"]:
        raise ValueError("original execution did not establish baseline coverage: " + str(baseline))
    change_fixture(root, case)
    subject_rev = commit(run, root, "synthetic " + case["id"])
    coverage = json.loads(run.run([binary, "--root", root, "--coverage", "--json"], root))
    rollup = json.loads(run.run([binary, "--root", root, "--requirements-rollup", "--json"], root))
    if case["change"] == "unavailable":
        probe = {"state": "could-not-check", "reason": "governing requirement unavailable", "executed": False}
        prediction = "unknown"
    else:
        run.run(["python3", "-c", current_probe(case["change"])], root, allow=(0, 1))
        probe = {"state": "pass" if run.log[-1]["exit"] == 0 else "fail", "executed": True,
                 "scope": "numeric current condition only; not a Verify execution or an assertion of adequate test coverage"}
        changed = (before_req != digest((root / REQ).read_bytes())
                   or before_limit != digest((root / LIMIT).read_bytes()))
        prediction = "review-binding" if changed else "defer-to-baseline"
    return {"id": case["id"], "witness_revision": witness_rev, "subject_revision": subject_rev,
            "historical_execution": {"executed": True, "witness": witness},
            "baseline_coverage": baseline, "current_coverage": coverage, "rollup": rollup,
            "current_numeric_probe": probe,
            "experimental_prediction": {"kind": "report-only", "result": prediction,
                "rule": "byte revision of the existing REQ plus inherited source; changed means review binding, never semantic equivalence or automatic fresh execution"},
            "commands": run.log[start:]}


def replay(cases, raw_dir=None):
    # Retained under the runtime's temp root; the runtime owns reclamation.
    scratch = Path(tempfile.mkdtemp(prefix="criterion-study-"))
    raw_dir = Path(raw_dir).resolve() if raw_dir else scratch / "raw-artifacts"
    run = Runner(clean_env(scratch), scratch, raw_dir)
    source = cases["source_commit"]
    archive = subprocess.run(["git", "archive", source, "statusgen", "forgeread", "perfrec"],
                             cwd=ROOT, env=run.env, capture_output=True, check=True, timeout=30)
    snapshot = scratch / "source"
    snapshot.mkdir()
    with tarfile.open(fileobj=io.BytesIO(archive.stdout)) as tar:
        tar.extractall(snapshot, filter="data")
    binary = scratch / "statusgen"
    run.run(["go", "build", "-trimpath", "-o", binary, "."], snapshot / "statusgen", timeout=120)
    versions = {"python": platform.python_version(), "os": platform.system(), "machine": platform.machine(),
                "go": run.run(["go", "version"], snapshot).strip(),
                "git": run.run(["git", "--version"], snapshot).strip(),
                "statusgen": run.run([binary, "--version"], snapshot).strip()}
    setup = list(run.log)
    return {"study": cases["study"], "source_commit": source,
            "cases_sha256": digest((HERE / "cases.json").read_bytes()),
            "driver_sha256": digest(Path(__file__).read_bytes()),
            "versions": versions, "archive": {"argv": ["git", "archive", source, "statusgen", "forgeread", "perfrec"],
                "sha256": digest(archive.stdout), "exit": archive.returncode},
            "setup_commands": setup, "cases": [observe_case(run, binary, scratch, c) for c in cases["cases"]]}


def validate(data, cases):
    ids = [c["id"] for c in cases["cases"]]
    got = [c["id"] for c in data["cases"]]
    if len(set(got)) != len(got) or sorted(got) != sorted(ids):
        raise ValueError("skipped, duplicate or undeclared case")
    if data["cases_sha256"] != digest((HERE / "cases.json").read_bytes()):
        raise ValueError("frozen cases changed")
    if data["source_commit"] != cases["source_commit"]:
        raise ValueError("source revision changed")
    outputs = json_outputs(data)
    artifacts = [record["provenance"]["artifact"] for record in outputs]
    if len(outputs) != 3 * len(ids) or len(set(artifacts)) != len(artifacts):
        raise ValueError("missing or duplicate CLI JSON capture")
    for record in outputs:
        check_json_record(record)
    for case in data["cases"]:
        if case["experimental_prediction"]["kind"] != "report-only":
            raise ValueError("experimental prediction presented as observation")
        commands = case["commands"]
        executions = [c for c in commands if "verifyrun" in c["argv"] and c["exit"] == 0 and c["executed"]]
        if len(executions) != 1 or case["historical_execution"]["executed"] is not True:
            raise ValueError("fabricated or missing historical execution")
        if case["id"] == "unavailable" and case["current_numeric_probe"]["state"] != "could-not-check":
            raise ValueError("unavailable governing source is not current acceptance")


def comparable(data):
    data = copy.deepcopy(data)
    # Versions are retained for independent assessment, not required identical on
    # another supported host. Dates and generated fixture commit ids also vary.
    data.pop("versions")
    for command in data["setup_commands"]:
        if command["argv"][1:] in (["version"], ["--version"]):
            command["stdout"] = "<tool-version>"
    # Each byte commitment was independently reconstructed/checked above. Its
    # digest varies with the known run-date/fixture-revision fields normalized
    # below. Keep format, layout, artifact id and capture provenance compared.
    for record in json_outputs(data):
        record["provenance"].pop("sha256")
        record["provenance"].pop("bytes")
    replacements = {}
    for case in data["cases"]:
        for field in ("witness_revision", "subject_revision"):
            sha = case[field]
            replacements[sha] = f"<{case['id']}:{field}>"
            replacements[sha[:12]] = f"<{case['id']}:{field}:short>"
    raw = json.dumps(data, sort_keys=True)
    for old in sorted(replacements, key=len, reverse=True):
        raw = raw.replace(old, replacements[old])
    return re.sub(r"\b\d{4}-\d{2}-\d{2}(?= \| human:syntheticfixture @)", "<run-date>", raw)


def compare(saved, fresh, cases):
    validate(saved, cases)
    validate(fresh, cases)
    if comparable(saved) != comparable(fresh):
        raise ValueError("fresh actual replay differs from saved observations")
    pairs = list(zip(json_outputs(saved), json_outputs(fresh), strict=True))
    same = sum(a["provenance"]["sha256"] == b["provenance"]["sha256"]
               and a["provenance"]["bytes"] == b["provenance"]["bytes"] for a, b in pairs)
    print(f"CLI JSON byte commitments: {same}/{len(pairs)} identical; semantic replay matches")


# regression: graph-execution/32
def self_test(fresh, cases):
    plants = {}
    tamper = copy.deepcopy(fresh)
    tamper["cases"][0]["current_coverage"][0]["released"] = False
    plants["tampered-observation"] = tamper
    skip = copy.deepcopy(fresh)
    skip["cases"].pop()
    plants["skipped-case"] = skip
    duplicate = copy.deepcopy(fresh)
    duplicate["cases"].append(copy.deepcopy(duplicate["cases"][0]))
    plants["duplicate-case"] = duplicate
    fabricated = copy.deepcopy(fresh)
    fabricated["cases"][0]["historical_execution"]["executed"] = False
    plants["fabricated-execution"] = fabricated
    unknown = copy.deepcopy(fresh)
    next(c for c in unknown["cases"] if c["id"] == "unavailable")["current_numeric_probe"]["state"] = "pass"
    plants["unknown-as-pass"] = unknown
    prediction = copy.deepcopy(fresh)
    prediction["cases"][0]["experimental_prediction"]["kind"] = "measured"
    plants["prediction-as-measured"] = prediction
    provenance = copy.deepcopy(fresh)
    json_outputs(provenance)[0]["provenance"]["sha256"] = "0" * 64
    plants["wrong-raw-commitment"] = provenance
    decoded = copy.deepcopy(fresh)
    json_outputs(decoded)[0]["value"][0]["released"] = False
    plants["changed-decoded-value"] = decoded
    # Actual child command exit codes, not just a claim that comparison rejects.
    work = Path(tempfile.mkdtemp(prefix="criterion-controls-"))
    fresh_path = work / "observed.json"
    fresh_path.write_text(json.dumps(fresh))
    positive = subprocess.run([sys.executable, __file__, "--compare", str(fresh_path), str(fresh_path)],
                              capture_output=True, text=True, timeout=10)
    if positive.returncode != 0:
        raise ValueError("positive control: identical genuine observations must be accepted")
    print("identical-observations: accepted exit=0")
    for name, planted in plants.items():
        path = work / (name + ".json")
        path.write_text(json.dumps(planted))
        proc = subprocess.run([sys.executable, __file__, "--compare", str(path), str(fresh_path)],
                              capture_output=True, text=True, timeout=10)
        if proc.returncode != 1:
            raise ValueError(f"control {name}: wanted rejection exit 1, got {proc.returncode}")
        print(f"{name}: rejected exit=1: {proc.stderr.strip()}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument("--record", action="store_true", help="save fresh actual observations")
    modes.add_argument("--check", action="store_true", help="replay and compare saved observations")
    modes.add_argument("--self-test", action="store_true", help="replay and plant rejected observations")
    modes.add_argument("--compare", nargs=2, metavar=("SAVED", "OBSERVED"), help=argparse.SUPPRESS)
    parser.add_argument("--raw-artifacts", metavar="DIR",
                        help="retain original CLI JSON stdout bytes in this new private directory")
    args = parser.parse_args()
    cases = json.loads((HERE / "cases.json").read_text())
    ids = [c["id"] for c in cases["cases"]]
    if len(ids) < 12 or len(set(ids)) != len(ids):
        raise ValueError("need at least twelve distinct predeclared cases")
    if args.compare:
        compare(*(json.loads(Path(p).read_text()) for p in args.compare), cases)
        return
    fresh = replay(cases, args.raw_artifacts)
    validate(fresh, cases)
    if args.record:
        # Captured object member order is semantically irrelevant but belongs to
        # the original byte codec; never sort it away before provenance validation.
        (HERE / "results.json").write_text(json.dumps(fresh, indent=2) + "\n")
    elif args.check:
        compare(json.loads((HERE / "results.json").read_text()), fresh, cases)
    else:
        self_test(fresh, cases)
    for case in fresh["cases"]:
        cov = case["current_coverage"][0]
        print(case["id"], "released=" + str(cov["released"]).lower(),
              "result=" + cov["claims"][0]["result"],
              "numeric=" + case["current_numeric_probe"]["state"],
              "prediction=" + case["experimental_prediction"]["result"])
    print(f"{len(fresh['cases'])} declared cases executed; original witnesses retain their subjects")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
        print("replay rejected: " + str(exc), file=sys.stderr)
        sys.exit(1)
