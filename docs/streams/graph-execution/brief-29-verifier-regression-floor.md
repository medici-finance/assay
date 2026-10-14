---
brief: assay:assay:graph-execution:29
title: Old and new regression floor for the verifier domain contracts
why: "Replacing the verifier must not quietly lose behaviour that already protects us: unavailable evidence is not a pass, advice cannot stand in for a witness, offline reads never answer empty. A pinned inventory run against both the old and new code makes any lost case or unreviewed difference fail before the old path is retired."
wave: 3
depends: ["graph-execution/28"]
unblocks: ["graph-execution/04", "graph-execution/16", "graph-execution/19", "graph-execution/26"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by worker-desk (author-brief), non-CI architecture reconciliation
sources:
  - docs/proposals/non-ci-architecture/15-regression-from-day-one.md — "Establishing the implementation baseline" and "How to establish the suite without preserving accidents"
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md — package R1 proof column
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — R1 owner row
  - "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
  - freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Choosing which old behaviours are preserved, adapted or intentionally changed is a semantic judgment across the evaluator, the reader and the runner kit.
domain: complicated
outcome: none
consumers:
  - "statusgen/regressionfloor_test.go, statusgen/testdata/regressionfloor: follow-up graph-execution/29"
  - "loopadmin/runner/conformance: out-of-scope (selected and run unchanged; a real-adapter subject arrives with the non-CI R3 follow-up)"
  - "tools/desk/cmd/deskclaim-ref claim tests: out-of-scope (their intent transfers to graph-execution/19 row 8, TestPGClaimHolderSemantics, per the non-CI R2 amendment of 2026-10-13; recorded here as an inventory disposition only)"
id: 233d898c-9498-4a14-84af-ae489d113c3c
---

# Brief 29 — Old and new regression floor for the verifier domain contracts

## Context

files: `statusgen/regressionfloor_test.go` (planned),
`statusgen/testdata/regressionfloor/inventory.yaml` (planned),
`statusgen/testdata/regressionfloor/cases/` (planned),
`statusgen/testdata/regressionfloor/differences.yaml` (planned),
`changelog/graph-execution-29-verifier-regression-floor.md` (planned).

facts (a435badbd, 2026-10-13):
- The selected baseline tests exist and pass on main: `TestCoverageCouldNotCheckIsNotPass`,
  `TestCoverageAdviceCannotSupplyWitness`, `TestCoverageAcceptanceDigestChanged`
  (`statusgen/coverage_test.go`), `TestEligibilityCouldNotCheckHolds`
  (`statusgen/eligibility_test.go`), `TestOfflineReaderNeverAnswersEmpty`
  (`statusgen/forgeread_test.go`) and `TestGraphJSONLDeterministic` (`statusgen/graph_test.go`).
- The runner conformance kit passes on main: `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/`.
- Many baseline tests call package-main helpers, so not every case can be imported; the inventory
  records a disposition per case: reuse unchanged, adapt harness, replace (approved semantics
  changed) or add (missing).
- The existing testledger contract (`// regression:` tags and `Retires-test:` explanations,
  build-less-brittle/11) is the departure report; this brief adds no second ledger.
- A fake runner passing conformance proves protocol shape only, not isolation, database recovery
  or publication fencing; those proofs belong to the non-CI R2–R5 follow-ups.

layering: the inventory and frozen case facts are data; the "old" adapter calls the existing
package-main code paths, the "new" adapter calls `statusgen/domain` (planned) (graph-execution/28). Both run
on identical frozen facts and a test compares them. Rows 2 and 3 enforce the comparison.

single-point-of-failure: the old/new comparison test — behind it, the six baseline tests keep
running unchanged in their own files (row 5), so deleting or weakening the comparison does not
remove the original assertions.

## Read first

- `docs/proposals/non-ci-architecture/15-regression-from-day-one.md` (whole document).
- `docs/streams/graph-execution/brief-28-verifier-domain-contracts.md`.
- `tools/desk/internal/testledger/` (existing departure report).

## Ground rules

- Isolated branch; draft PR only. No merge, release or live infrastructure.
- Do not regenerate golden files to make the new path pass; an intended difference is recorded in
  `differences.yaml` with its reason and reviewed.
- Do not delete or weaken any baseline test. Stop at implemented.

## Task

1. Write `inventory.yaml`: for each selected baseline test record its file, test name, owning
   contract (from `docs/contracts.md`), pinned source revision and disposition. Include the runner
   conformance kit and record the claim-test intent as transferred to the R2 follow-up.
2. Extract each reusable case's frozen facts into `cases/` (planned) with independent expected results
   written by hand from the test's assertion, not produced by either implementation.
3. Implement old and new adapters inside `statusgen/regressionfloor_test.go` (planned) and compare results
   per case. A difference not listed in `differences.yaml` fails; a listed difference that no
   longer occurs also fails (stale entries cannot accumulate).
4. Fail when zero cases are selected or an inventory entry names a test or file that does not
   exist. Each result line names which implementation and evaluator identity ran.
5. Add the changelog fragment and a `// regression:` tag per adapted case.

## Verify

The named `TestRegressionFloor*` tests are deliverables of this brief and do not exist at
a435badbd. Rows 5 and 6 run tests that already exist and must keep passing.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +dereference | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorInventoryResolves$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorInventoryResolves " "$out")` | exit 0; named PASS; every inventory entry's file and test name resolve in the tree and its contract row exists in `docs/contracts.md`; mutation: misspell one inventory test name — the test must fail |
| 2 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorOldNewAgree$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorOldNewAgree " "$out")` | exit 0; named PASS; old and new adapters agree with the hand-written expected results on every case; mutation: make the new adapter round could-not-check to pass — the test must fail |
| 3 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorUnlistedDifferenceFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorUnlistedDifferenceFails " "$out")` | exit 0; named PASS; an unlisted difference fails and a listed difference that no longer occurs fails; mutation: ignore `differences.yaml` staleness — the test must fail |
| 4 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorZeroSelectedFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorZeroSelectedFails " "$out")` | exit 0; named PASS; an empty selection or empty case directory fails rather than passing vacuously; mutation: return success when no case is selected — the test must fail |
| 5 | check:ci | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && for t in TestCoverageCouldNotCheckIsNotPass TestCoverageAdviceCannotSupplyWitness TestCoverageAcceptanceDigestChanged TestEligibilityCouldNotCheckHolds TestOfflineReaderNeverAnswersEmpty TestGraphJSONLDeterministic; do GOWORK=off go test -count=1 -v -run "^${t}\$" . > "$out" && grep -qF -- "--- PASS: $t " "$out" && echo "PASS $t" && continue; exit 1; done)` | exit 0; six `PASS <name>` lines: each baseline case still runs and passes unchanged |
| 6 | check:ci | `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/` | exit 0; output contains "ok"; the runner conformance kit still passes |

## Pre-mortem

Likely wrong builds: an inventory naming tests that are not there (row 1), expected results
produced by the code under test (row 2), differences silently allowed or never cleaned (row 3),
a vacuous pass on zero cases (row 4), or a baseline test deleted once the floor exists (row 5).
Whether each disposition is right stays a review judgment.

## Evidence

<!-- Independent verifier records command, exit, named result, subject revision, environment and date. No implementation evidence is asserted by this authoring change. -->

## Review

Gate: model (all four risk answers no). The reviewer reads `differences.yaml` entry by entry; an
intended difference needs an approved semantic change behind it, not convenience.
