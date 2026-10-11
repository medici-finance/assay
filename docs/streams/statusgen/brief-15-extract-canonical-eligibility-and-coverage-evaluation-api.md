---
brief: assay:assay:statusgen:15
title: Extract canonical eligibility and coverage evaluation API
why: "Workflow and projection consumers need the same eligibility and coverage answers without launching statusgen or copying its evaluator."
wave: 1
depends: []
unblocks: ["graph-execution/19"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-08 by statusgen newbrief
sources:
  - "#2395 — library-first API planning"
  - docs/library-first.md
  - "freshness-checked 2026-10-08 @ 403b8ec8c26b21b85d88b5faea4cff619550e602"
exec-tier: strong
exec-tier-why: Extraction must preserve cross-component evidence and eligibility semantics while separating existing filesystem effects.
domain: complicated
consumers:
  - "statusgen/eligibility.go, statusgen/coverage.go, statusgen/nextup.go, statusgen/drivefrontier.go: follow-up statusgen/15"
  - "statusgen/evaluation, statusgen/go.mod, docs/contracts.md, tools/desk/internal/arch: follow-up statusgen/15"
  - "workflow/bindings: follow-up graph-execution/19"
  - "projection consumers outside this repository: out-of-scope (adopter pins and consumer rollout remain adopter-owned)"
id: a82664a9-c0a8-456f-b152-66214afff2e1
---

# Brief 15 — Extract canonical eligibility and coverage evaluation API

## Context

files: `statusgen/evaluation/` (new), `statusgen/eligibility.go`, `statusgen/coverage.go`,
`statusgen/nextup.go`, `statusgen/drivefrontier.go`, their tests and model-loading helpers,
`statusgen/testdata/evaluation/` (new), `statusgen/evaluation_contract_test.go` (new),
`docs/contracts.md`, `tools/desk/internal/arch/` (existing owner/marker checks),
`.github/workflows/assay-statusgen.yml`, `docs/library-first.md`,
`changelog/statusgen-evaluation-library.md` (new).

facts (403b8ec8c26b21b85d88b5faea4cff619550e602, 2026-10-08): eligibility and coverage
are implemented in package main; eligibility imports os/filepath and coverage includes git
observation. Offline does not mean pure. `statusgen/streamview` already contains importable
wire types. `statusgen/graphcontract` does not exist; graph-execution/19 formerly owned a
second extraction. It now consumes this brief. The existing coverage binding seam accepts
explicit bindings; this brief adds no production workflow-store binding.

layering: statusgen owns the meaning; a pure evaluation package receives normalized models,
observations and an explicit evaluation context. Existing command adapters load files/git
observations and map results. Task 1–3 and Verify 1–4 enforce the boundary.
design-fit:
  owner: statusgen/evaluation (extracted from statusgen, same semantic owner)
  contract: S-eligibility
  retires: [package-main-only evaluator implementation, graph-execution/19 independent extraction]
  weight: "verbs 0, flags 0, refusals 0; one importable API, no duplicate evaluator"
  why-add: "A supported package replaces process-only access; exporting the whole CLI would retain effects and prevent reuse."

Read first: `docs/library-first.md`, `docs/contracts.md` (S-eligibility and S-delivery),
`docs/streams/graph-execution/brief-03-evidence-coverage-rule.md`,
`docs/streams/graph-execution/brief-19-durable-instance-store.md`.

## Ground rules

Work in an isolated branch with offline fixtures. No live providers, release, merge or rollout.
Keep existing command/wire behavior. Stop at implemented; verification is independent.
Effort L is deliberate: eligibility and coverage share models and caller mappings; one atomic
extraction avoids two temporary model owners. Re-cut before implementation if the dependency
inventory requires unrelated parser, policy or full-board migration.

## Task

1. Capture pre-extraction golden verdicts/reasons from existing fixtures, including unresolved
   cross-repo gates, stale acceptance, wrong revision, missing evidence and absent versus declared
   but unresolved pattern bindings. Inventory the transitive inputs; move filesystem/git loading
   into adapters. Do not make formerly unavailable forge refs resolvable as part of extraction.
2. Move the canonical evaluation functions and minimal immutable input/result types into
   `statusgen/evaluation`. Public entrypoints are typed EvaluateEligibility and EvaluateCoverage
   operations over supplied observations; no global environment, implicit clock, network, process,
   filesystem discovery, token or write operation. Preserve distinct revision kinds and reasons.
   Update existing statusgen callers in the same change. Remove the old implementations; thin
   mapping wrappers are allowed, independent recalculation is not.
3. Add a direct-package consumer fixture and compare it with the real statusgen CLI on identical
   inputs. Eligibility still does not grant admission; coverage still does not grant publication.
   Update semantic-owner/marker checks to the actual moved owner without weakening ceilings.
   Add transitive capability checks with planted violations and an effect-trapping offline flow.
4. Qualify external-module consumption through a temporary local module proxy with `GOWORK=off`,
   no relative replace and networking disabled. Pin/release documentation must cover the statusgen
   module's public API and evaluator identity. Extend existing CI path coverage for every file the
   cross-module checks read. Document the API, migration and provenance; add the changelog fragment.

## Verify

The named tests below are deliverables, not existing evidence. Each must run production code,
assert independent expected results and retain a fail-first mutation; no skipped/missing test passes.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | exit 0; named PASS; Same frozen inputs produce equal CLI/library verdicts and reasons; changing a mapping makes the test fail. |
| 2 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | exit 0; named PASS; Missing/wrong-revision/stale/unresolved evidence holds; known no-binding remains distinct; treating unknown as satisfied fails. |
| 3 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | exit 0; named PASS; Transitive process/network/filesystem/credential dependencies refused; planted indirect import fails, pure control passes. |
| 4 | check:ci +flow +mutation | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | exit 0; named PASS; External module builds and exercises both APIs with GOWORK=off, no relative replace and local proxy only; unsupported API expectation fails. |

## Pre-mortem

A moved function can change missing-data meaning (row 2), leave a second implementation or
bad mapping (row 1 and owner checks), hide effectful helpers (row 3), or build only in the
workspace (row 4). API ergonomics and L scope remain review judgments.

## Evidence

<!-- Non-implementer records merged revision, commands, results and retained red controls. -->

### Non-implementer verifier run — 2026-10-11 assay-verifier-app[bot] @ 40b45182b3c0 (claude-opus-5-5) (on-behalf-of human:ian)

Run on merged main at 40b45182b3c011cef823309c4e1c8d1aecbf6967 (medici-finance/assay). The implementation is commit 1235e830437a, the squash of #2552; the runner took no part in it, and no later commit touches the evaluation package, its contract test or its mutation spec. All 4 rows pass by hand. Go 1.27.1, git 2.56.0, darwin/arm64.

- **Execution witness is could-not-run on all 4 rows.** Every row is check:ci, and the witness re-executes check:ci rows inside `unshare --net`, which darwin does not have. The witness table below records that refusal as itself. The rows were therefore run by hand, unchanged, from the repository root with bash. The hand runs were not network-isolated at the OS level; row 4's test itself points GOPROXY at a local file proxy with GOSUMDB off.
- **Each row ran twice.** First as written. Then as a diagnostic rerun with GIT_CONFIG_GLOBAL=/dev/null and GIT_CONFIG_NOSYSTEM=1, so that the verifier's own git configuration could not influence the result. Both runs exited 0 with the named PASS line on every row. A third, ungrepped capture of each test confirmed one RUN and one PASS line for the named test, no SKIP line, and the two other packages reporting no tests to run.
- **Mutation controls.** The retained spec (statusgen/evaluation-mutations.json) ran under the repository's own harness (tools/desk/cmd/muhar, built from this commit, sequential mode), against a tracked-file snapshot of this commit outside the home. Baseline was green and the positive control (the eligibility adapter swapping an edge's type and reason) was caught. Of 13 mutants, 13 were caught and none was not caught or could not be applied. The failing test matched each mutant's group: parity mutants failed TestEvaluationLibraryCLIParity, uncertain-evidence mutants failed TestEvaluationUncertainEvidenceHolds, boundary mutants failed TestEvaluationPureDependencyBoundary, and external-consumer mutants failed TestEvaluationExternalConsumer. A parallel run (-j 4) reported the harness as broken. In that mode each worker's copy rewrote the module's relative forgeread replace to an absolute path, and row 4's own guard refuses an absolute replace. That is a harness limit, not a defect in the code under verification.
- **Risk-bearing value: the trigger did not fire.** The risk field is present and all four flags read no, irreversible included. The change is a behaviour-preserving offline extraction with gate: model.

| # | Command | Expect | Result (exit + real output line) | Date | Runner |
|---|---------|--------|----------------------------------|------|--------|
| 1 | as written (TestEvaluationLibraryCLIParity, GOWORK=off, all packages under statusgen) | exit 0; named PASS; mapping change fails | exit 0. --- PASS: TestEvaluationLibraryCLIParity (13.26s). Diagnostic rerun with global and system git config off: exit 0, --- PASS: TestEvaluationLibraryCLIParity (9.91s). Mutants: 3 parity mutants plus the positive control, all caught by this test | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | as written (TestEvaluationUncertainEvidenceHolds, GOWORK=off, all packages under statusgen) | exit 0; named PASS; unknown-as-satisfied fails | exit 0. --- PASS: TestEvaluationUncertainEvidenceHolds (2.70s). Diagnostic rerun with global and system git config off: exit 0, --- PASS: TestEvaluationUncertainEvidenceHolds (2.22s). Mutants: 5 uncertain-evidence mutants (missing brief, forge-backed gate, missing witness, wrong revision, unreadable row), all caught | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | as written (TestEvaluationPureDependencyBoundary, GOWORK=off, all packages under statusgen) | exit 0; named PASS; planted indirect import fails | exit 0. --- PASS: TestEvaluationPureDependencyBoundary (1.06s). Diagnostic rerun with global and system git config off: exit 0, --- PASS: TestEvaluationPureDependencyBoundary (0.80s). Mutants: 3 boundary mutants (os/exec import, os import, a checker that stops at the first package), all caught | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | as written (TestEvaluationExternalConsumer, GOWORK=off, all packages under statusgen) | exit 0; named PASS; unsupported API fails | exit 0. --- PASS: TestEvaluationExternalConsumer (3.30s). Diagnostic rerun with global and system git config off: exit 0, --- PASS: TestEvaluationExternalConsumer (2.87s). Mutants: 2 external-consumer mutants (an exported admission verdict, a manifest naming the internal tool module), both caught | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

**VERIFY: PASS** — 4 of 4 rows pass by hand at 40b45182b3c0; execution witness could-not-run on all 4 (check:ci needs `unshare --net`, absent on darwin), so the hand table is the evidence for this run.

Execution witness for this run (statusgen verifyrun, exit 2):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationLibraryCLIParity$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationLibraryCLIParity " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationUncertainEvidenceHolds$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationUncertainEvidenceHolds " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationPureDependencyBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationPureDependencyBoundary " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestEvaluationExternalConsumer$" ./... > "$result" && grep -F -- "--- PASS: TestEvaluationExternalConsumer " "$result")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2623 run=379e48e7046603fd0c743b90e990be43f7c016a121a97bc27ccb8c322ee3a6e1 source=40b45182b3c011cef823309c4e1c8d1aecbf6967 brief=docs/streams/statusgen/brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md model=claude-opus-5-5 tier=any


## Review

Gate: model. This is behavior-preserving offline extraction, not an authority change. Any
proposal to change evidence acceptance, identity or admission is separate work with its own gate.
