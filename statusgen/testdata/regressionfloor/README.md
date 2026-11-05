# Verifier regression floor

`inventory.yaml` pins the selected test sources at an immutable baseline. The
inventory check resolves each test and compares its formatted function body with
an AST-body SHA256 frozen from that baseline. `freeze.go` reads the pinned Git
objects with replacement objects disabled to reproduce those provenance hashes
at authoring time; it does not generate semantic expectations. Routine tests
need no history, credentials or network. Missing/invalid/mismatched body hashes
fail the check. The optional checkout-head log is metadata, not a historical
binary execution or authenticity claim.

`cases/` contains hand-written raw Verify/Evidence facts and expected semantic
results extracted from the original assertions, plus an explicit passing control.
No witness renderer or evaluator generated the expected results. Fixture object
IDs and dates are synthetic. `record.json` freezes the candidate/acceptance
binding from the domain's candidate-a example, independently of future changes
to that example. The old and new adapters execute the current checked-out source:
old uses the existing package-main observation and eligibility adapters, new uses
`domain.Assess`; both reuse the single `evaluation` owner. The log names each
adapter, evaluator version/policy digest and source baseline. These are adapter
comparisons, not independent implementations of the coverage policy.

The graph and offline-reader tests retain their existing frozen inputs and are
run unchanged through their original test functions. The domain does not ship
replacement reader/serializer APIs. Runner conformance stays in its own module;
all selected kit tests are inventoried and run unchanged. Claim-holder intent is
explicitly transferred to graph-execution/19; this floor does not qualify its
PostgreSQL replacement. Contract links on reuse rows identify the relevant
cross-cutting semantic boundary, not a claim those domain APIs replace the kit.

`differences.yaml` is empty because no semantic change is authorized here. A
future reviewed allowance must name its case, reason and exact old/new results;
stale, unknown, duplicate and mismatched allowances fail. Expectations continue
to pin the old result. Mutation controls use synthetic allowances only.

Run from the repository root:

```
(cd statusgen && GOWORK=off go test -timeout 90s -count=1 -v -run '^(TestRegressionFloor|TestFloorFreezeAuthoring)' .)
python3 statusgen/testdata/regressionfloor/mutate.py
(cd loopadmin && GOWORK=off go test -timeout 90s -count=1 ./runner/conformance/)
```

The mutation script restores each modified owned file in `finally`, requires a
named behavioral failure, and never treats a compile error as a mutation kill.
Do not run another test or source consumer concurrently with these mutations.
No baseline test is removed or weakened. The existing testledger remains the
only departure report. Local test results confer no independent verification,
publication, isolation or recovery qualification.

## Repository scope and updating provenance

Run this repository-level floor from a complete Assay source checkout. It reads
`docs/contracts.md`, the selected sibling-module test files and local fixtures;
a statusgen-only export is not its supported test envelope. The cross-module
CI registry lists these readers. The unconditional `ci.yml` statusgen shard runs
on documentation-only and sibling-module changes as well as statusgen edits.
No workflow trigger or permission change is needed.

An intentional strengthening or approved replacement of a retained test needs
review of its preservation obligations. Update only the affected inventory
entry's `source` to the approved immutable commit containing that test, then run
`cd statusgen && GOWORK=off go run testdata/regressionfloor/freeze.go` and review
the source/digest diff. Keep unrelated pins and hand-written expected outcomes
unchanged. Never regenerate provenance to conceal a weakened or retired test;
use the existing testledger departure procedure when a test leaves.

The named empty-selection check also covers a nonempty selection with a missing
file and an unselected file. The named difference check covers known allowances,
duplicate allowances, unknown case references and empty reasons in addition to
staleness and exact old/new outcomes. Both equal-adapter and allowed-difference
paths independently enforce the hand-written old expectation. These comparison
and case-selection refusals each have a behavioral mutation control. The
inventory resolver has provenance checks; this statement does not claim mutation
coverage for every inventory validation or file-decoding error.
The authoring helper separately checks pinned revision/path syntax and ignores
same-named methods when resolving a retained top-level test.

After an interrupted mutation run, inspect `git status` and the owned-file diff
before using or committing the tree. `finally` cannot restore files after
SIGKILL or unhandled termination; an interrupted tree is not evidence of a
successful restored run.
