"""Bounded fail-first receipts. Run in a disposable owned checkout, from any cwd.

The omission briefly edits only the source declaration and restores exact bytes in
finally. Go overlays mutate the test join / existing owners without editing Go
sources. Temporary logs are private; only aggregate results go to stdout.
"""
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import tempfile

MODULE = Path(__file__).resolve().parents[2]
ROOT = MODULE.parent
BRIEF = next((ROOT / "docs/streams/statusgen").glob("brief-26-*.md"))
ENTRY = ('  - {in: "assay#3759", covers: partial, note: '
         '"sub-split (a); snapshot reuse, workloads and Verify 2 remain owed"}\n')
# Child tests need local build/cache locations, not ambient forge credentials,
# credential helpers or caller-controlled Go flags. This is test-launcher hygiene;
# selected Go tests still use loopback fake servers, not a no-sockets guarantee.
BUILD_ENV = ("PATH", "HOME", "TMPDIR", "TMP", "TEMP", "GOCACHE", "GOPATH",
             "GOMODCACHE", "GOROOT", "SYSTEMROOT", "WINDIR", "USERPROFILE",
             "LOCALAPPDATA", "APPDATA", "LANG", "LC_ALL", "TZ")
ENV = {key: os.environ[key] for key in BUILD_ENV if key in os.environ}
ENV.update(KUBECONFIG="/dev/null", GOPROXY="off", GOSUMDB="off",
           GOENV="off", GOTOOLCHAIN="local")
OUT = Path(tempfile.mkdtemp(prefix="board-cohort-mutations-"))

def run(name, selector, failure=None, overlay=None):
    cmd = ["go", "test"]
    if overlay:
        cmd += ["-overlay", str(overlay)]
    cmd += [".", "-run", selector, "-count=1", "-timeout", "60s", "-v"]
    result = subprocess.run(cmd, cwd=MODULE, env=ENV, capture_output=True, text=True)
    output = result.stdout + result.stderr
    (OUT / (name + ".log")).write_text(output)
    if failure:
        assert result.returncode != 0 and failure in output, (name, "mutation not caught")
    else:
        assert result.returncode == 0 and "--- PASS: TestBoardCohort" in output, (name, "baseline failed")
    print(name + (": CAUGHT" if failure else ": PASS"))

def overlay(name, file, signature, inserted, selector, diagnostic):
    source = MODULE / file
    raw = source.read_text()
    assert raw.count(signature) == 1, name
    mutated = OUT / (name + ".go")
    mutated.write_text(raw.replace(signature, signature + "\n" + inserted, 1))
    mapping = OUT / (name + ".json")
    mapping.write_text(json.dumps({"Replace": {str(source): str(mutated)}}))
    run(name, selector, diagnostic, mapping)

def production():
    paths = subprocess.check_output(["git", "ls-files", "*.go"], cwd=ROOT, text=True).splitlines()
    return {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest()
            for p in paths if not p.endswith("_test.go")}

before = production()
original = BRIEF.read_bytes()
run("baseline", "^TestBoardCohort")
try:
    text = original.decode()
    assert text.count(ENTRY) == 1
    # Exercise the SAME live-source path that the whole-file hash froze. Every
    # cohort test must tolerate ordinary later maintenance, not just a helper.
    maintained = re.sub(r"(?m)^version: [0-9]+$", "version: 100", text, count=1)
    maintained = maintained.replace("delivery:\n", 'delivery:\n  - {in: "assay#4001", '
                                    'covers: partial, note: "later synthetic slice"}\n', 1)
    maintained = maintained.replace("## Evidence\n", "## Evidence\n\nLater independent evidence.\n", 1)
    BRIEF.write_text(maintained + "\n<!-- Later maintenance note. -->\n")
    run("later-maintenance", "^TestBoardCohort")
    BRIEF.write_text(re.sub(r"(?m)^delivery:\n([^ \t\n])", r"\1", text.replace(ENTRY, "", 1)))
    run("source-omission", "^TestBoardCohortSource$", "PR #3759 (merged 4482989)")
    run("flow-omission", "^TestBoardCohortFlow$", "got implemented want in-progress")
finally:
    BRIEF.write_bytes(original)

overlay("coverage-disabled", "boardcohort_test.go",
        "func cohortCoverage(f cohortFixture, prs []PRRecord, d declaredPartials) error {",
        "return nil", "^TestBoardCohortPlant$", "coverage accepted older unresolved second site")
overlay("write-refusal-disabled", "reconcile.go",
        "func reconcileWriteRefusal(in LifecycleInput, pullsUnread string, unread []string) string {",
        'return ""', "^TestBoardCohortFloors$", "missing refusal")
overlay("evidence-exclusion-disabled", "briefevidence.go",
        "func (a *authoringClassifier) notImplementation(number int, briefID string) (bool, error) {",
        "return false, nil", "^TestBoardCohortFull$", "PR #2295 (merged 9137784)")
run("restored", "^TestBoardCohort")
assert BRIEF.read_bytes() == original and production() == before
print("source restoration and production Go byte identity: PASS")
print("Private receipts: " + str(OUT))
