---
brief: "assay:assay:desk-improvement:10"
title: "Qualify native evidence across execution modes"
why: "Passing schemas and isolated calculations cannot prove that actual desk/workflow modes preserve complete evidence through diagnosis, decision and later evaluation."
wave: 5
depends: ["desk-improvement/04", "desk-improvement/07", "desk-improvement/08", "desk-improvement/09"]
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 2
id: "b5e6bc3f-3702-4755-a023-0bd4eb16160e"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["qualgen/deskquality/integration_test.go: follow-up desk-improvement/10", "qualgen/deskquality/testdata/native-evidence/: follow-up desk-improvement/10", "loopadmin/runner/conformance/evidence_test.go: follow-up desk-improvement/10", "docs/streams/desk-improvement/qualification.md: follow-up desk-improvement/10", "docs/desk-improvement.md: follow-up desk-improvement/10"]
---

# Brief 10 — Qualify native evidence across execution modes

## Context

files:
- `qualgen/deskquality/integration_test.go (planned)`
- `qualgen/deskquality/testdata/native-evidence/ (planned synthetic native fixtures)`
- `loopadmin/runner/conformance/evidence_test.go (planned emitter conformance)`
- `docs/streams/desk-improvement/qualification.md (planned mode/profile matrix)`
- `docs/desk-improvement.md (03 usage; add supported-mode limits)`
- `changelog/desk-improvement-10.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: existing producer conformance and continuing-operations assessment integration
  contract: none — desk-improvement/01 adds the assessment row to the existing semantic-owner index
  retires: []
  weight: "CLI modes +0; policy refusals +0; rule-text +0 shipped desk method"
  why-add: "Existing source/assessment gaps need one owned extension; a duplicate collector, rule set or execution ledger was rejected."

Single-point-of-failure note: validation cannot establish truth or authority; independent native-source/label assessment backs structural checks. Neither layer may grant acceptance or policy adoption.

## Read first

[Canonical specification](spec.md), [plan and prerequisites](README.md), and the exact upstream owner contracts named in gates. Their required boundaries are restated in Task. Planned paths/tests below are deliverables, not claims that implementation exists.

## Ground rules

- Isolated branch and draft PR; no main push, merge, workflow dispatch, production endpoint or cluster contact.
- Offline synthetic/disposable fixtures; no model spend, real-data export or enrollment is granted here.
- Preserve completed Evidence and current authority; stop at implemented for independent verification.

## Task

1. Assemble actual 06–08 producer outputs under qualified disposable/fake external boundaries, with original work identity and exact source/build/profile versions. Replay source→binding→02 cards→03 assessment→09 read-only proposal. Exercise configured standing roles and workflow stages without requiring a graph instance in standing mode.
2. Independently reconcile native counts, gaps, no-PR/failed/abandoned work, multiple-item usage, unknown cost, explicit human effort sources and privacy projections. A real emitter removed or replaced by hand-written normalized JSON must make its flow test fail.
3. Fault/replay duplicate, late correction, source permission withdrawal, stale acceptance, unknown stop, partial restart and reader rollback. Preserve native execution authority and avoid duplicate effects; measurement recovery never issues a launch. Report incompatible profiles explicitly rather than silently dropping them.
4. Publish a versioned supported/unsupported mode and measurement matrix with evidence and remaining gaps. Qualify the library/CLI/client boundary; no service/cockpit, real adopter corpus, provider spend or production rollout is required for offline conformance. Actual operational benefit remains an independently assessed adopter trial, not this synthetic result.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd qualgen && go test -v ./deskquality/... -run '^TestNativeCardFlow$' > "${TMPDIR:-/tmp}/di-10-TestNativeCardFlow.out" && grep -F -- '--- PASS: TestNativeCardFlow' "${TMPDIR:-/tmp}/di-10-TestNativeCardFlow.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestModeEvidenceReplay$' > "${TMPDIR:-/tmp}/di-10-TestModeEvidenceReplay.out" && grep -F -- '--- PASS: TestModeEvidenceReplay' "${TMPDIR:-/tmp}/di-10-TestModeEvidenceReplay.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestModeEvidenceLimits$' > "${TMPDIR:-/tmp}/di-10-TestModeEvidenceLimits.out" && grep -F -- '--- PASS: TestModeEvidenceLimits' "${TMPDIR:-/tmp}/di-10-TestModeEvidenceLimits.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | check +mutation | `cd loopadmin && go test -v ./runner/conformance/... -run '^TestNativeEvidenceContract$' > "${TMPDIR:-/tmp}/di-native.out" && grep -F -- '--- PASS: TestNativeEvidenceContract' "${TMPDIR:-/tmp}/di-native.out"` | Native producer conformance rejects a missing emitter and preserves native unknown/stop semantics |
| 5 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/10` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
