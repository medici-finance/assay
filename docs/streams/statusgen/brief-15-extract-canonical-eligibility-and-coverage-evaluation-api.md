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

## Review

Gate: model. This is behavior-preserving offline extraction, not an authority change. Any
proposal to change evidence acceptance, identity or admission is separate work with its own gate.
