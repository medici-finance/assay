#!/usr/bin/env python3
"""Offline mutation controls for graph-execution/28 and /31. Run from any directory.

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
PROVENANCE = "statusgen/domain/provenance.go"
LINEAGE = "\tset := map[string]bool{p.Responsible: true}\n\tfor _, principal := range p.Delegation {\n\t\tset[principal] = true\n\t}\n\treturn set"
SELECTED = '\tif !reflect.DeepEqual(trusted, claim) {\n\t\treturn Provenance{}, refuse("provenance-not-selected")\n\t}\n\treturn trusted, nil'
MUTATIONS = [
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
    ("leaf-keys", "TestDutyPrincipals", [(PROVENANCE, LINEAGE, "\treturn map[string]bool{p.Key: true}")]),
    ("ignore-delegation", "TestDutySponsor", [(PROVENANCE, LINEAGE, "\treturn map[string]bool{p.Responsible: true}")]),
    ("trust-declared-proof", "TestProofClass", [(PROVENANCE, "func (p Provenance) EffectiveProof() string {\n", "func (p Provenance) EffectiveProof() string {\n\treturn p.DeclaredProof\n")]),
    ("model-text-identity", "TestDutyEvidenceFlow", [(PROVENANCE, SELECTED, SELECTED.replace("(trusted, claim)", "(trusted.Work, claim.Work)").replace("return trusted, nil", "return claim, nil"))]),
    ("drop-required-field", "TestProvenanceSchema", [("schemas/evidence-provenance-v1.json", '    "environment",\n    "declared-proof",', '    "declared-proof",')]),
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
    for name in ("verification-domain-v1.json", "workflow-instance-v1.json", "evidence-provenance-v1.json"):
        out = dest / "schemas" / name
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / "schemas" / name, out)
    for name in ("verification-domain-v1.md", "evidence-provenance-v1.md"):
        out = dest / "spec" / name
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / "spec" / name, out)
    fixtures = ROOT / "statusgen/testdata/domain"
    for f in [*fixtures.glob("*/*.json"), *fixtures.glob("provenance/*/*.json")]:
        out = dest / f.relative_to(ROOT)
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(f, out)
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
