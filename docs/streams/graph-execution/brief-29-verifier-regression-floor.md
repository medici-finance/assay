---
brief: assay:assay:graph-execution:29
title: Old and new regression floor for the verifier domain contracts
why: "Replacing the verifier must not quietly lose behaviour that already protects us: unavailable evidence is not a pass, advice cannot stand in for a witness, offline reads never answer empty. A pinned inventory run against both the old and new code makes any lost case or unreviewed difference fail before the old path is retired."
wave: 3
depends: ["graph-execution/28"]
unblocks: ["graph-execution/04", "graph-execution/16", "graph-execution/19", "graph-execution/26", "forge-neutral/37", "forge-neutral/38"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
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

## Consumer integration clarification — 2026-10-23

Version 2 clarifies Task 3's existing production-adapter use of shared domain records and assessment. Existing Verify row 2 already requires that old/new flow; the entire delivered Verify section, historical Evidence and dependencies remain unchanged. See [the acceptance and consumer integration manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md#contract-acceptance-and-consumer-integration--23-october-2026). This clarification adds no acceptance result or store/workspace integration claim.

## Task

1. Write `inventory.yaml`: for each selected baseline test record its file, test name, owning
   contract (from `docs/contracts.md`), pinned source revision and disposition. Include the runner
   conformance kit and record the claim-test intent as transferred to the R2 follow-up.
2. Extract each reusable case's frozen facts into `cases/` (planned) with independent expected results
   written by hand from the test's assertion, not produced by either implementation.
3. Implement old and new adapters inside `statusgen/regressionfloor_test.go` (planned) and compare results
   per case. Actual consumption means the existing package-main observation adapter and
   the new adapter's shared domain.Record and domain.Assess execute on the same frozen
   facts; fixture serialization or owning-brief corroboration alone is insufficient.
   This is the production adapter flow already required by Verify 2, not a new acceptance
   row or a claim that /19 or /23 integration ran. A difference not listed in `differences.yaml` fails; a listed difference that no
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
### Independent post-merge verification — 2026-10-27

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 0c292571a3e0794598cb11733d0b6c966c89aae0, detached home, unchanged except this Evidence section. Admission: `deskdispatch --check-verifier` exited 0 before any row ran (attestation #4601), and again at the bracket's open and close steps. Host: Darwin arm64, go1.27.2, git 2.56.0, installed statusgen v1.0.63. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8 (statusgen v1.0.63, go1.27.2), linux/amd64, network none (only loopback and a down tunl0), non-root user, home mounted read-only. KUBECONFIG=/dev/null; scratch output stayed outside the home.

Delivery provenance, resolved from the forge this run: PR #4133 merged=true at 2026-10-22T23:14:58Z with merge commit df7f301fd8d1179495df17960026617c9b1914cb; its first parent is 866a169aecab730e1acdaad7cbe57f9c41af8553 and the delivery is an ancestor of the observed source. `deskpathguard rederive` exited 0 with 6 rows and 0 author-added; every row's Class, Command and Expect equal the delivered table.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorInventoryResolves$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorInventoryResolves " "$out")` | pass exit=0 sandbox=container-netns | sha256:3d6a5e6870b1 | 2026-10-27 | assay-verifier-app[bot] @ 0c292571a3e0 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorOldNewAgree$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorOldNewAgree " "$out")` | pass exit=0 sandbox=container-netns | sha256:dcd543fefed0 | 2026-10-27 | assay-verifier-app[bot] @ 0c292571a3e0 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorUnlistedDifferenceFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorUnlistedDifferenceFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:46f680c1a1af | 2026-10-27 | assay-verifier-app[bot] @ 0c292571a3e0 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorZeroSelectedFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorZeroSelectedFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:90b30f91b30f | 2026-10-27 | assay-verifier-app[bot] @ 0c292571a3e0 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && for t in TestCoverageCouldNotCheckIsNotPass TestCoverageAdviceCannotSupplyWitness TestCoverageAcceptanceDigestChanged TestEligibilityCouldNotCheckHolds TestOfflineReaderNeverAnswersEmpty TestGraphJSONLDeterministic; do GOWORK=off go test -count=1 -v -run "^${t}\$" . > "$out" && grep -qF -- "--- PASS: $t " "$out" && echo "PASS $t" && continue; exit 1; done)` | pass exit=0 sandbox=container-netns | sha256:55abc7ddfad9 | 2026-10-27 | assay-verifier-app[bot] @ 0c292571a3e0 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/` | pass exit=0 sandbox=container-netns | sha256:25c6fbbcb4b6 | 2026-10-27 | assay-verifier-app[bot] @ 0c292571a3e0 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:3ee0bd8b42b6d5b4e4253fa850046c0d784a26019702912aac2b6e719e99f084
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4601 run=290a7ac52098e7a8fcc60d8f2a29d4812508ea5850b76252fe1278e000df1a72 source=0c292571a3e0794598cb11733d0b6c966c89aae0 brief=docs/streams/graph-execution/brief-29-verifier-regression-floor.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=290a7ac52098e7a8fcc60d8f2a29d4812508ea5850b76252fe1278e000df1a72 source=0c292571a3e0794598cb11733d0b6c966c89aae0 tree=sha256:3ee0bd8b42b6d5b4e4253fa850046c0d784a26019702912aac2b6e719e99f084 witness=sha256:ef7b8923304c1336a7490645160ea909b2b2d94a5f4262912e900841ac05ae7e opened=2026-10-27T12:54:32Z closed=2026-10-27T12:57:00Z before=receipt after=check-verifier

Run accounting: 6 original obligations, 6 pass, 0 fail, 0 could-not-run. All six rows ran inside the closed admission bracket above; `statusgen verifyrun --bracket-digest` on the witness file printed verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. Uncredited attempts, disclosed: three earlier container invocations in this same bracket mounted the home at its host path and were refused before any row ran (the container computed the empty-tree digest, not the receipt's tree); no row executed and no witness was written by them. The credited run mounted the same home read-only at a neutral container path; the host close step re-admitted it and the tree digest matched the receipt.

Supplemental observation, no acceptance credit (same pinned image, network none, on a git-archive copy of the observed source outside the home): `go test -v -run '^(TestRegressionFloor|TestFloorFreezeAuthoring)' .` in statusgen printed `--- PASS` for all five tests and `ok`; TestRegressionFloorOldNewAgree ran six cases (advice, changed, feather, gate, passing, unavailable), each logging `implementation=package-main-observation-adapter` and `implementation=domain.Assess` with evaluator statusgen-evaluation/3 and baseline 51bf925475920eee9db259e9c00c4a518d88b61f. Row 6 printed `ok  github.com/medici-finance/assay/loopadmin/runner/conformance`.

Supplemental failure controls, no acceptance credit: `python3 statusgen/testdata/regressionfloor/mutate.py` exited 0 and printed KILLED for all 17 delivered mutations, each by its named failing test (inventory-name, uncertainty-pass, stale-difference, unlisted-difference, empty-selection, missing-selected, unselected-file, duplicate-allowance, unknown-allowance, empty-reason, shared-expectation, allowed-expectation, allowance-old-outcome, allowance-new-outcome, case-identity, freeze-source, freeze-method). Each Expect cell's named mutation, applied to the scratch copy, made that row's exact command exit 1: row 1 (one inventory test name misspelled), row 2 (new adapter rounds could-not-check to pass), row 3 (staleness check disabled), row 4 (zero selection returns success). Every mutated file was restored and compared equal to the observed source afterwards.

Input accounting: between the delivery and the observed source, the only changes among the floor's inputs are additions to loopadmin/runner/conformance (conformance_test.go, hostile.go, kit.go, profiles.go). Row 1 re-hashes every inventoried test body against its pinned AST digest at the observed source and passes, so none of the 31 retained bodies changed; row 6 runs the whole kit, added tests included.

Risk-value inventory: the brief is all-risk-no and irreversible-no, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no row needs a fail-first base witness (the four named Regression Floor tests do not exist at the delivery base). Enumerated over the delivered diff 866a169aecab730e1acdaad7cbe57f9c41af8553..df7f301fd8d1179495df17960026617c9b1914cb (16 files) and re-read at the observed source: the inventory source pin and digest grammars, the disposition set, the hand-written case expectations, the allowance list, the fixture directory and the authoring-helper timeout. The fixture directory (`floorDir = "testdata/regressionfloor"` @ statusgen/regressionfloor_test.go:26) and the `-timeout 45s` @ statusgen/regressionfloor_test.go:512 are reversible test knobs and rank last.

RISK-VALUE: DERIVED — `"source": "51bf925475920eee9db259e9c00c4a518d88b61f"` (31 of 31 entries) @ statusgen/testdata/regressionfloor/inventory.yaml:15 — it is the first parent of the first delivery commit ff4fe11e8cd2, an ancestor of the delivery base, so every retained body is pinned at the last revision before this brief's change; row 1 confirms each pinned body digest still matches at the observed source.
RISK-VALUE: DERIVED — `^[0-9a-f]{40}$` @ statusgen/regressionfloor_test.go:132 — a full SHA-1 commit id (20 bytes, 40 hex); abbreviations or ref names could move, so only an immutable full id qualifies as a pinned source.
RISK-VALUE: DERIVED — `^[0-9a-f]{64}$` @ statusgen/regressionfloor_test.go:178 — SHA-256 is 32 bytes, 64 hex digits; row 1's own controls reject an empty, malformed or all-zero digest.
RISK-VALUE: DERIVED — `case "adapt harness", "reuse unchanged", "replace", "add"` @ statusgen/regressionfloor_test.go:139 — exactly the four dispositions Task 1's facts list; any other value refuses.
RISK-VALUE: DERIVED — `"released": false, "claim": "could-not-check"` @ statusgen/testdata/regressionfloor/cases/unavailable.json:7-8 — TestCoverageCouldNotCheckIsNotPass asserts `c.Released` false and the single claim equals covCouldNotCheck; likewise advice.json (`missing`), changed.json (`error`), gate.json (`held` with one could-not-check hold) and feather.json (`eligible-with-notice`, no hold, one could-not-check notice) match their original assertions in coverage_test.go and eligibility_test.go line for line.
RISK-VALUE: DERIVED — `[]` @ statusgen/testdata/regressionfloor/differences.yaml:2 — the brief authorizes no semantic change, so the allowance list must be empty; any old/new divergence fails as unlisted, and row 3's mutation shows a stale entry would fail.

**VERIFY: PASS** — 6/6 original rows pass at 0c292571a3e0794598cb11733d0b6c966c89aae0 inside a closed container-netns admission bracket (verdict=match); 17/17 delivered mutation controls and 4/4 Expect-cell mutations killed (supplemental, no acceptance credit).
### Independent post-merge re-verification — 2026-10-28

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 9363e5df55fa6f6a9b39cf87d83e4f5648fa6065, detached home, unchanged except this Evidence section. Admission: `deskdispatch --check-verifier` exited 0 before any row ran (attestation #4752), and again at the bracket's open and close steps. Host: Darwin arm64, go1.27.2, git 2.56.0, installed statusgen v1.0.64. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner:v1.0.64@sha256:a00e65af51fd8d6d7ddcfd71627486f08d70c7a9bd6bd136f944d1ff526957a0 (the digest named by the v1.0.64 release asset verify-runner-image.txt, whose sha256 matches that release's checksums.txt; statusgen v1.0.64, go1.27.3, git 2.56.0), linux/amd64, network none (only loopback and a down tunl0), non-root user, home mounted read-only at a neutral container path, GOPROXY=off over a read-only module cache. KUBECONFIG=/dev/null; scratch output stayed outside the home.

Why this entry exists: the 2026-10-27 entry above passed at 0c292571a3e0794598cb11733d0b6c966c89aae0, but its board flip did not land; see #4536. A later re-verify proposed on #4719 was admitted under the prior tool release and its follow-up could not be admitted, so it never landed. This entry is a fresh, complete run of every row at a newer source under the current tool release.

Delivery provenance, resolved from the forge this run: PR #4133 merged=true at 2026-10-22T23:14:58Z with merge commit df7f301fd8d1179495df17960026617c9b1914cb; its first parent is 866a169aecab730e1acdaad7cbe57f9c41af8553 and the delivery is an ancestor of the observed source. `deskpathguard rederive` exited 0 with 6 rows and 0 author-added; the Verify section at the observed source is byte-identical to the delivered one at df7f301fd8d1.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorInventoryResolves$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorInventoryResolves " "$out")` | pass exit=0 sandbox=container-netns | sha256:c97df4fa4272 | 2026-10-28 | assay-verifier-app[bot] @ 9363e5df55fa (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorOldNewAgree$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorOldNewAgree " "$out")` | pass exit=0 sandbox=container-netns | sha256:dcd543fefed0 | 2026-10-28 | assay-verifier-app[bot] @ 9363e5df55fa (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorUnlistedDifferenceFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorUnlistedDifferenceFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:46f680c1a1af | 2026-10-28 | assay-verifier-app[bot] @ 9363e5df55fa (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorZeroSelectedFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorZeroSelectedFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:90b30f91b30f | 2026-10-28 | assay-verifier-app[bot] @ 9363e5df55fa (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && for t in TestCoverageCouldNotCheckIsNotPass TestCoverageAdviceCannotSupplyWitness TestCoverageAcceptanceDigestChanged TestEligibilityCouldNotCheckHolds TestOfflineReaderNeverAnswersEmpty TestGraphJSONLDeterministic; do GOWORK=off go test -count=1 -v -run "^${t}\$" . > "$out" && grep -qF -- "--- PASS: $t " "$out" && echo "PASS $t" && continue; exit 1; done)` | pass exit=0 sandbox=container-netns | sha256:55abc7ddfad9 | 2026-10-28 | assay-verifier-app[bot] @ 9363e5df55fa (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/` | pass exit=0 sandbox=container-netns | sha256:e1584635d1aa | 2026-10-28 | assay-verifier-app[bot] @ 9363e5df55fa (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8b30031d1d490423aa97a887a460fd5d1ed2f777cc502d0b467548ce39497595
Container-Image: sha256:a00e65af51fd8d6d7ddcfd71627486f08d70c7a9bd6bd136f944d1ff526957a0
Verification-Attestation: medici-finance/assay#4752 run=092b24422f92ea4fc06c912b9e298d9cccbfaf7d47d0c56e40d4c6e22c5c3265 source=9363e5df55fa6f6a9b39cf87d83e4f5648fa6065 brief=docs/streams/graph-execution/brief-29-verifier-regression-floor.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=092b24422f92ea4fc06c912b9e298d9cccbfaf7d47d0c56e40d4c6e22c5c3265 source=9363e5df55fa6f6a9b39cf87d83e4f5648fa6065 tree=sha256:8b30031d1d490423aa97a887a460fd5d1ed2f777cc502d0b467548ce39497595 witness=sha256:61a765d8351a82198003a17bfdf473a29c56cd930b7a42693f8fbf7ef9104ffd opened=2026-10-28T13:48:36Z closed=2026-10-28T13:49:52Z before=receipt after=check-verifier

Run accounting: 6 original obligations, 6 pass, 0 fail, 0 could-not-run. All six rows ran in one container invocation inside the closed admission bracket above; `statusgen verifyrun --bracket-digest` on the witness file printed verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. No uncredited row attempt occurred in this bracket. No Verify row carries a risk-bearing, live, mutating or end-to-end tag and the brief's risk block is all no, so no row needs a fail-first base witness.

Supplemental observation, no acceptance credit (same pinned image, network none, on a git-archive copy of the observed source outside the home): `go test -v -run '^(TestRegressionFloor|TestFloorFreezeAuthoring)' .` in statusgen printed `--- PASS` for all five tests and `ok`; TestRegressionFloorOldNewAgree ran six cases (advice, changed, feather, gate, passing, unavailable), each logging `implementation=package-main-observation-adapter` and `implementation=domain.Assess` with evaluator statusgen-evaluation/3 and baseline 51bf925475920eee9db259e9c00c4a518d88b61f, and the two implementations printed identical results per case. Row 6 covered the whole conformance kit, including the files added to it after the delivery.

Supplemental failure controls, no acceptance credit: `python3 statusgen/testdata/regressionfloor/mutate.py` exited 0 and printed KILLED for all 17 delivered mutations, each by its named failing test (inventory-name, uncertainty-pass, stale-difference, unlisted-difference, empty-selection, missing-selected, unselected-file, duplicate-allowance, unknown-allowance, empty-reason, shared-expectation, allowed-expectation, allowance-old-outcome, allowance-new-outcome, case-identity, freeze-source, freeze-method). Four of them are the Expect cells' named mutations: row 1 (inventory-name, one inventory test name misspelled), row 2 (uncertainty-pass, the new adapter rounds the unavailable claim to pass), row 3 (stale-difference, staleness check disabled) and row 4 (empty-selection, zero selection returns success); each made its row's named test print a FAIL line, so that row's command exits nonzero. The script restored every mutated file, and a recursive compare of the scratch statusgen tree against the home afterwards found no difference. Disclosed uncredited attempts: two earlier supplemental invocations used an archive holding only statusgen (and then only statusgen plus its replaced sibling modules); they stopped on files outside that archive (docs/contracts.md), which is a scratch-setup error, and are superseded by the full-tree run above. Neither touched the home or the witness.

Input accounting: between the delivery and the observed source, the regression floor's own files (statusgen/regressionfloor_test.go and statusgen/testdata/regressionfloor) and the six baseline test files are unchanged; the only changed inputs are additions to loopadmin/runner/conformance (conformance_test.go, hostile.go, kit.go, profiles.go), which row 6 runs, and statusgen/domain/mutate.py, a mutation helper no row invokes. Row 1 re-hashes every inventoried test body against its pinned digest at the observed source and passes, so none of the 31 retained bodies changed.

Risk-value inventory: the brief is all-risk-no and irreversible-no. Enumerated over the delivered diff 866a169aecab730e1acdaad7cbe57f9c41af8553..df7f301fd8d1179495df17960026617c9b1914cb (16 files) and re-read at the observed source: the inventory source pin, the commit-id and digest grammars, the disposition set, the hand-written case expectations, the allowance list, the fixture directory and the authoring-helper timeout. The fixture directory (`floorDir = "testdata/regressionfloor"` @ statusgen/regressionfloor_test.go:26) and the `-timeout 45s` @ statusgen/regressionfloor_test.go:512 are reversible test knobs and rank last.

RISK-VALUE: DERIVED — `"source": "51bf925475920eee9db259e9c00c4a518d88b61f"` (31 of 31 entries) @ statusgen/testdata/regressionfloor/inventory.yaml:15 — it is the first parent of the first delivery commit ff4fe11e8cd2 and an ancestor of the delivery base 866a169aecab, so every retained body is pinned at the last revision before this brief's change; row 1 confirms each pinned body digest still matches at the observed source.
RISK-VALUE: DERIVED — `^[0-9a-f]{40}$` @ statusgen/regressionfloor_test.go:132 — a full SHA-1 commit id is 20 bytes, 40 hex digits; an abbreviation or ref name could move, so only an immutable full id qualifies as a pinned source (the freeze helper applies the same grammar at statusgen/testdata/regressionfloor/freeze.go:49).
RISK-VALUE: DERIVED — `^[0-9a-f]{64}$` @ statusgen/regressionfloor_test.go:178 — SHA-256 is 32 bytes, 64 hex digits, and the same line recomputes the body digest and compares it.
RISK-VALUE: DERIVED — `case "adapt harness", "reuse unchanged", "replace", "add"` @ statusgen/regressionfloor_test.go:139 — exactly the four dispositions the brief's Context facts list; any other value refuses.
RISK-VALUE: DERIVED — `"released": false, "claim": "could-not-check"` @ statusgen/testdata/regressionfloor/cases/unavailable.json:7-8 — the claim literal for the unavailable case; TestCoverageCouldNotCheckIsNotPass (statusgen/coverage_test.go:223-227) asserts the brief is not released and its single claim equals covCouldNotCheck, which is evaluation.ResultCouldNotCheck. Likewise advice.json (claim missing), changed.json (claim error), gate.json (one hold state, matching TestEligibilityCouldNotCheckHolds at statusgen/eligibility_test.go:135-138) and feather.json (eligible-with-notice, one notice, no hold state) match their original assertions.
RISK-VALUE: DERIVED — `[]` @ statusgen/testdata/regressionfloor/differences.yaml:2 — the brief authorizes no semantic change, so the allowance list must be empty; any old/new divergence then fails as unlisted, and the stale-difference and unlisted-difference controls above show both directions are enforced.

**VERIFY: PASS** — 6/6 original rows pass at 9363e5df55fa6f6a9b39cf87d83e4f5648fa6065 inside a closed container-netns admission bracket (verdict=match); 17/17 delivered mutation controls killed, 4 of them the Expect cells' named mutations (supplemental, no acceptance credit).

### Independent batch re-witness at the pinned main commit — 2026-11-02

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main 2c6c21336a606e64df057294f21177f16db7b9f8 (the batch re-witness pin), detached home, unchanged except this Evidence section. Admission: `deskdispatch --check-verifier` exited 0 before any row ran (attestation #5268), and again at the bracket's open and close steps. Host: Darwin arm64, git 2.56.0, installed statusgen v1.0.69. Container lane: ghcr.io/medici-finance/verify-runner:v1.0.66@sha256:097db7d3e20415bc7a2c524250e2ad89158ed93a31a7d8cd861e6cd38fc3b3b4 (native linux/arm64; statusgen v1.0.66, go1.27.3, git 2.56.0), network none, non-root user 501:20, home mounted read-only at a neutral container path, GOPROXY=off over the image's module cache. KUBECONFIG=/dev/null; scratch output stayed outside the home.

Why this entry exists: the entries above witnessed this table at earlier main commits, so the CI flip to done reads their coverage as wrong-revision (medici-finance/assay#2026). This entry is a fresh, complete run of every row at the single batch pin. Earlier blocks are left as they were. `deskpathguard rederive` against the pin exited 0 with 6 rows, 0 author-added, every row from base.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorInventoryResolves$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorInventoryResolves " "$out")` | pass exit=0 sandbox=container-netns | sha256:a928e58f5c9f | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorOldNewAgree$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorOldNewAgree " "$out")` | pass exit=0 sandbox=container-netns | sha256:cc55e463811c | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorUnlistedDifferenceFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorUnlistedDifferenceFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:35449219ec02 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorZeroSelectedFails$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorZeroSelectedFails " "$out")` | pass exit=0 sandbox=container-netns | sha256:6fc8894f45f4 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && for t in TestCoverageCouldNotCheckIsNotPass TestCoverageAdviceCannotSupplyWitness TestCoverageAcceptanceDigestChanged TestEligibilityCouldNotCheckHolds TestOfflineReaderNeverAnswersEmpty TestGraphJSONLDeterministic; do GOWORK=off go test -count=1 -v -run "^${t}\$" . > "$out" && grep -qF -- "--- PASS: $t " "$out" && echo "PASS $t" && continue; exit 1; done)` | pass exit=0 sandbox=container-netns | sha256:55abc7ddfad9 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/` | pass exit=0 sandbox=container-netns | sha256:09be8a6e53f9 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:ddf14768e4b78f02e94f6fae4c310c08dc06e0a8db50a2fb68244111759cd68c
Container-Image: sha256:097db7d3e20415bc7a2c524250e2ad89158ed93a31a7d8cd861e6cd38fc3b3b4
Verification-Attestation: medici-finance/assay#5268 run=0b8ae45eb571607050d76305739fdb7c40765050c9f06310619e7eba58463ae0 source=2c6c21336a606e64df057294f21177f16db7b9f8 brief=docs/streams/graph-execution/brief-29-verifier-regression-floor.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=0b8ae45eb571607050d76305739fdb7c40765050c9f06310619e7eba58463ae0 source=2c6c21336a606e64df057294f21177f16db7b9f8 tree=sha256:ddf14768e4b78f02e94f6fae4c310c08dc06e0a8db50a2fb68244111759cd68c witness=sha256:d122828f28346fe1f450441d2da9c4be763ce0ac5c0026b82d3727abe0be5931 opened=2026-11-02T01:44:14Z closed=2026-11-02T01:44:56Z before=receipt after=check-verifier

Run accounting: 6 original obligations, 6 pass, 0 fail, 0 could-not-run. All six rows ran in one container invocation inside the closed admission bracket above; `statusgen verifyrun --bracket-digest` on the witness file printed verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. No uncredited row attempt occurred in this bracket. No Verify row carries a risk-bearing, live, mutating or end-to-end tag and the brief's risk block is all no, so no row needs a fail-first base witness.

Supplemental failure controls, no acceptance credit (same pinned image, network none, on a git-archive copy of the pinned source outside the home): `python3 statusgen/testdata/regressionfloor/mutate.py` exited 0 and printed KILLED for all 17 delivered mutations, four of them the Expect cells' named mutations: inventory-name (row 1), uncertainty-pass (row 2), stale-difference (row 3) and empty-selection (row 4). The script restored every mutated file, and a recursive compare of the scratch statusgen tree against the home afterwards found no difference.

Input accounting: between the 2026-10-28 entry's source 9363e5df55fa and this pin, the regression floor's own files, the six baseline test files and loopadmin/runner/conformance are unchanged (empty git diff over those paths).

Risk-value inventory: the brief is all-risk-no and irreversible-no. Re-read at the pin, the literals the earlier entries derived are unchanged and sit at the same lines: the inventory source pin (31 of 31 entries) @ statusgen/testdata/regressionfloor/inventory.yaml:15, the commit-id grammar @ statusgen/regressionfloor_test.go:132, the disposition set @ statusgen/regressionfloor_test.go:139, the body-digest grammar @ statusgen/regressionfloor_test.go:178, the unavailable case's claim @ statusgen/testdata/regressionfloor/cases/unavailable.json:7-8 and the empty allowance list @ statusgen/testdata/regressionfloor/differences.yaml:2. The fixture directory @ statusgen/regressionfloor_test.go:26 and the 45s timeout @ statusgen/regressionfloor_test.go:512 are reversible test knobs and rank last.

RISK-VALUE: DERIVED — `"source": "51bf925475920eee9db259e9c00c4a518d88b61f"` @ statusgen/testdata/regressionfloor/inventory.yaml:15 — the first parent of the first delivery commit, so every retained body is pinned at the last revision before this brief's change; row 1 re-hashes each pinned body at this source and passes.
RISK-VALUE: DERIVED — `^[0-9a-f]{40}$` @ statusgen/regressionfloor_test.go:132 — a full SHA-1 commit id is 40 hex digits; only an immutable full id qualifies as a pinned source.
RISK-VALUE: DERIVED — `^[0-9a-f]{64}$` @ statusgen/regressionfloor_test.go:178 — SHA-256 is 64 hex digits, and the same line recomputes and compares the body digest.
RISK-VALUE: DERIVED — `case "adapt harness", "reuse unchanged", "replace", "add"` @ statusgen/regressionfloor_test.go:139 — exactly the four dispositions the brief's Context facts list.
RISK-VALUE: DERIVED — `"released": false, "claim": "could-not-check"` @ statusgen/testdata/regressionfloor/cases/unavailable.json:7-8 — matches TestCoverageCouldNotCheckIsNotPass's assertion that unavailable evidence is not released and its claim is could-not-check.
RISK-VALUE: DERIVED — `[]` @ statusgen/testdata/regressionfloor/differences.yaml:2 — the brief authorizes no semantic change, so the allowance list must be empty; the stale-difference and unlisted-difference controls show both directions are enforced.

**VERIFY: PASS** — 6/6 original rows pass at 2c6c21336a606e64df057294f21177f16db7b9f8 inside a closed container-netns admission bracket (verdict=match); 17/17 delivered mutation controls killed, 4 of them the Expect cells' named mutations (supplemental, no acceptance credit).

## Review

Gate: model (all four risk answers no). The reviewer reads `differences.yaml` entry by entry; an
intended difference needs an approved semantic change behind it, not convenience.
