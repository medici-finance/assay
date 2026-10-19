---
brief: "assay:assay:desk-improvement:02"
title: "Implement configurable scorecards and pure reduction"
why: "A common deterministic reducer makes scorecards comparable without creating another collector, lifecycle or source of execution authority."
wave: 1
depends: ["desk-improvement/01"]
unblocks: ["desk-improvement/03", "desk-improvement/06", "desk-improvement/09"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 3
id: "fa4383a3-e8e0-46be-8527-e39fed79c964"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["qualgen/deskquality/reduce.go: follow-up desk-improvement/02", "qualgen/deskquality/profiles.go: follow-up desk-improvement/02", "qualgen/deskquality/render.go: follow-up desk-improvement/02", "qualgen/deskquality/reduce_test.go: follow-up desk-improvement/02", "qualgen/deskquality/benchmark_test.go: follow-up desk-improvement/02", "qualgen/deskquality/testdata/replay/: follow-up desk-improvement/02", "qualgen/desks.go: follow-up desk-improvement/02", "qualgen/desks_test.go: follow-up desk-improvement/02", "qualgen/main.go: follow-up desk-improvement/02", "qualgen/README.md: follow-up desk-improvement/02"]
---

# Brief 02 — Implement configurable scorecards and pure reduction

## Context

files:
- `qualgen/deskquality/reduce.go (planned)`
- `qualgen/deskquality/profiles.go (planned)`
- `qualgen/deskquality/render.go (planned)`
- `qualgen/deskquality/reduce_test.go (planned)`
- `qualgen/deskquality/benchmark_test.go (planned)`
- `qualgen/deskquality/testdata/replay/ (planned synthetic vectors)`
- `qualgen/desks.go (planned CLI adapter)`
- `qualgen/desks_test.go (planned)`
- `qualgen/main.go (existing dispatch)`
- `qualgen/README.md (existing)`
- `changelog/desk-improvement-02.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: qualgen/deskquality pure reducer
  contract: none — desk-improvement/01 adds the assessment row to the existing semantic-owner index
  retires: []
  weight: "CLI modes +1 in existing qualgen; three explicit input/output/as-of parameters; policy refusals +0; rule-text +0 shipped desk method"
  why-add: "Existing source/assessment gaps need one owned extension; a duplicate collector, rule set or execution ledger was rejected."

Single-point-of-failure note: validation cannot establish truth or authority; independent native-source/label assessment backs structural checks. Neither layer may grant acceptance or policy adoption.

## Read first

[Canonical specification](spec.md), [plan and prerequisites](README.md), and the exact upstream owner contracts named in gates. Their required boundaries are restated in Task. Planned paths/tests below are deliverables, not claims that implementation exists.

## Ground rules

- Isolated branch and draft PR; no main push, merge, workflow dispatch, production endpoint or cluster contact.
- Offline synthetic/disposable fixtures; no model spend, real-data export or enrollment is granted here.
- Preserve completed Evidence and current authority; stop at implemented for independent verification.

## Task

1. Implement 01's pure configurable profiles/system reduction. Use stable indexed joins and deterministic sorting; retain failed/no-PR/abandoned/censored work, source coverage and explicit shared/unallocated cost. A profile need not correspond to one of exactly five desks.
2. Implement duplicate-idempotent/conflict/late-supersession/deletion handling and compatible versions. Hand-calculated fixtures must demonstrate that dropping failed attempts creates an apparent benefit that disappears under correct fixed-cohort accounting. No active human time inferred from waits and no model agreement promoted to truth.
3. Produce JSON/Markdown from one result with independent restricted/publication profiles. Add the existing qualgen command's desks adapter with explicit input/output/as-of parameters. Keep current CLI exit conventions: 0 valid report (possibly incomplete), 1 runtime/IO, 2 invalid input. Invalid input never replaces a prior complete artifact. No network, scheduler or effect imports.
4. Demonstrate 10k/100k synthetic-record runtime/allocation scaling and record build/hardware/results. Do not invent an activation SLO; deployment limits are separately qualified. Document how independent consumers build or use the qualified artifact interface.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd qualgen && go test -v ./deskquality/... -run '^TestCardCoverage$' > "${TMPDIR:-/tmp}/di-02-TestCardCoverage.out" && grep -F -- '--- PASS: TestCardCoverage' "${TMPDIR:-/tmp}/di-02-TestCardCoverage.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestCohortReplay$' > "${TMPDIR:-/tmp}/di-02-TestCohortReplay.out" && grep -F -- '--- PASS: TestCohortReplay' "${TMPDIR:-/tmp}/di-02-TestCohortReplay.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestNoAuthority$' > "${TMPDIR:-/tmp}/di-02-TestNoAuthority.out" && grep -F -- '--- PASS: TestNoAuthority' "${TMPDIR:-/tmp}/di-02-TestNoAuthority.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | check +mutation | `cd qualgen && go test -run '^$' -bench 'BenchmarkDeskCards' -benchmem ./deskquality` | Both 10k and 100k benchmarks execute; record hardware/build and investigate worse-than-indexed-plus-sort scaling |
| 5 | check +mutation | `cd qualgen && go test -v . -run '^TestDesksCLI$' > "${TMPDIR:-/tmp}/di-cli.out" && grep -F -- '--- PASS: TestDesksCLI' "${TMPDIR:-/tmp}/di-cli.out"` | CLI actually exercises duplicate/censored source→both renderers and incomplete-report exit behavior |
| 6 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/02` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
