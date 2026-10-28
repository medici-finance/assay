#!/usr/bin/env python3
"""Offline mutation controls for graph-execution/28. Run from any directory.

Copies only declared source and fixture inputs into an owned temporary directory.
No Git, providers, repository edits, dependency downloads or environment discovery.
Each mutation must compile and fail its named test; a build failure is not a kill.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
RECORDS = "statusgen/domain/records.go"
ASSESS = "statusgen/domain/assess.go"
CONTEXT = "statusgen/domain/context.go"
MUTATIONS = [
    ("context-currency-key", "TestContextExpiry", [(CONTEXT, "return contextHash(map[string]any{", 'return contextHash(map[string]any{ "observed-cutoff":c.ObservedCutoff,')]),
    ("context-currency-check", "TestContextInputs", [(CONTEXT, "if cached.AccessEpoch != current.AccessEpoch || cached.Generation != current.Generation {", "if false {")]),
    ("context-requires-set", "TestContextInputs", [(CONTEXT, "if len(c.Requires) != 1 {", "if c.Requires == nil {")]),
    ("context-evaluator", "TestContextInputs", [(CONTEXT, "return contextHash(map[string]any{", 'c.Evaluator.Version = "omitted"; return contextHash(map[string]any{')]),
    ("context-expiry", "TestContextExpiry", [(CONTEXT, "if !now.Before(expiry) {", "if false && !now.Before(expiry) {")]),
    ("context-authority", "TestContextAdmissionFlow", [("statusgen/domain/context_test.go", "func currentAuthority(hit, authorized bool, grantExpiry, now string) bool {", "func currentAuthority(hit, authorized bool, grantExpiry, now string) bool {\n if hit { return true }")]),
    ("context-membership", "TestContextMembership", [(CONTEXT, "old, err := cached.Fingerprint()", """// Planted defect: compare only dependencies surviving in both inventories.
 left, right := []Dependency{}, []Dependency{}
 for _,a := range cached.Dependencies { for _,b := range current.Dependencies { if a.Work==b.Work {left=append(left,a);right=append(right,b)} } }
 cached.Dependencies=left; current.Dependencies=right
 cached.DependencyDigest,_=DependencySetDigest(left); current.DependencyDigest,_=DependencySetDigest(right)
 old, err := cached.Fingerprint()""") ]),
    ("context-schema", "TestContextSchema", [("schemas/evaluation-context-v1.json", '    "evaluator",', '')]),
    ("path-identity", "TestDomainPathIsNotIdentity", [(RECORDS, "key := q.String()", "key := item.Path + q.String()")]),
    ("unknown-fact", "TestDomainMissingFactHolds", [(ASSESS, "if state != Present {", "if state != Present && state != CouldNotCheck {")]),
    ("candidate-binding", "TestDomainAcceptanceBindsCandidate", [(RECORDS, "if b.Candidate != c {", "if b.Candidate.Repository != c.Repository {")]),
    ("success-acceptance", "TestDomainExecutionIsNotAcceptance", [(ASSESS, 'out.Holds = append(out.Holds, FactHold{"acceptance", Missing})', 'out.State = "supported"')]),
    ("generation", "TestDomainReevaluationGeneration", [
        (RECORDS, "if g.Recorded != g.Requested {", "if false {"),
        (ASSESS, "if o.Generation != r.Generation.Requested {", "if false {"),
        (ASSESS, "if a.Generation != r.Generation.Requested {", "if false {")]),
    ("commit-for-tree", "TestDomainEvaluationSeam", [(ASSESS, "in.Revision = r.Candidate.Tree", "in.Revision = r.Candidate.Commit")]),
    ("stage-conflation", "TestDomainStageSeparation", [(RECORDS, "func (r Record) Canonical() ([]byte, error) {", 'func (r Record) Canonical() ([]byte, error) {\n if r.Progress.Execution == "succeeded" { r.Progress.Acceptance = "accepted" }')]),
    ("effect-import", "TestDomainPureImports", []),
    ("schema-open", "TestDomainSchemaFixtures", [("schemas/verification-domain-v1.json", '"additionalProperties": false', '"additionalProperties": true')]),
    ("schema-drift", "TestDomainSchemaSurface", [("schemas/verification-domain-v1.json", '"unaccepted",', '"unchecked",')]),
    ("canonical-order", "TestDomainCanonical", [(RECORDS, "return json.Marshal(tree)", "return json.Marshal(r)")]),
]

def snapshot(dest):
    for package in ("domain", "evaluation", "workref", "internal/testschema"):
        source = ROOT / "statusgen" / package
        for f in source.glob("*.go"):
            if package in ("evaluation", "workref") and f.name.endswith("_test.go"):
                continue
            out = dest / f.relative_to(ROOT)
            out.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(f, out)
    # Domain consumes only the qualified-identity API. Copy its real production
    # owner, not unrelated resolver code (and that code's dependency closure).
    identity = Path("statusgen/contexthandoff/ids.go")
    (dest / identity).parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(ROOT / identity, dest / identity)
    for name in ("verification-domain-v1.json", "workflow-instance-v1.json", "evaluation-context-v1.json"):
        out = dest / "schemas" / name
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / "schemas" / name, out)
    out = dest / "spec/verification-domain-v1.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(ROOT / "spec/verification-domain-v1.md", out)
    extra = dest / "spec/evaluation-context-v1.md"
    shutil.copyfile(ROOT / "spec/evaluation-context-v1.md", extra)
    for f in (ROOT / "statusgen/testdata/domain").glob("*/*.json"):
        out = dest / f.relative_to(ROOT)
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(f, out)
    shutil.copyfile(ROOT / "statusgen/testdata/domain/context/fingerprint.txt", dest / "statusgen/testdata/domain/context/fingerprint.txt")
    (dest / "statusgen/go.mod").write_text("module github.com/medici-finance/assay/statusgen\n\ngo 1.27.0\n")


def offline_env():
    return dict(os.environ, GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local", KUBECONFIG="/dev/null")


def is_kill(run, test):
    return (run.returncode != 0 and "--- FAIL: " + test in run.stdout
            and "[build failed]" not in run.stdout and "[setup failed]" not in run.stdout)


def main():
    env = offline_env()
    # A broken snapshot must fail before any mutation can claim a behavior kill.
    with tempfile.TemporaryDirectory(prefix="domain-baseline-") as scratch:
        dest = Path(scratch)
        snapshot(dest)
        baseline = subprocess.run(["go", "test", "-count=1", "-timeout=30s", "./domain"],
                                  cwd=dest / "statusgen", env=env, text=True,
                                  stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60)
        if baseline.returncode != 0:
            print(baseline.stdout)
            raise SystemExit("unmutated snapshot did not pass")
        print("PASS unmutated snapshot")
    for label, test, replacements in MUTATIONS:
        with tempfile.TemporaryDirectory(prefix="domain-mutation-") as scratch:
            dest = Path(scratch)
            snapshot(dest)
            for file, before, after in replacements:
                path = dest / file
                source = path.read_text()
                if before not in source:
                    raise SystemExit("mutation anchor missing: " + label)
                path.write_text(source.replace(before, after))
            if label == "effect-import":
                (dest / "statusgen/domain/planted.go").write_text('package domain\nimport _ "os"\n')
            run = subprocess.run(["go", "test", "-count=1", "-timeout=30s", "-run=^" + test + "$", "./domain"], cwd=dest / "statusgen", env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=60)
            if not is_kill(run, test):
                print(run.stdout)
                raise SystemExit("mutation did not fail its behavior test: " + label)
            print("KILLED " + label + ": --- FAIL: " + test)

if __name__ == "__main__":
    main()
