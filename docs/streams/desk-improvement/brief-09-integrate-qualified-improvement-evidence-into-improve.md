---
brief: "assay:assay:desk-improvement:09"
title: "Integrate qualified improvement evidence into Improve"
why: "Qualified evidence must be visible at the existing process-decision surface while preserving human adoption and the cadence limit."
wave: 3
depends: ["desk-improvement/02", "desk-improvement/03"]
unblocks: ["desk-improvement/10"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 3
id: "7b83ddb1-68c8-4931-a732-04321c16efe2"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["tools/desk/internal/improve/deskquality.go: follow-up desk-improvement/09", "tools/desk/internal/improve/deskquality_test.go: follow-up desk-improvement/09", "tools/desk/internal/improve/rows.go: follow-up desk-improvement/09", "tools/desk/internal/improve/gate_test.go: follow-up desk-improvement/09", "tools/desk/internal/improve/readonly_test.go: follow-up desk-improvement/09", "tools/desk/internal/improve/testdata/deskquality/: follow-up desk-improvement/09", "tools/desk/go.mod: follow-up desk-improvement/09", "tools/desk/go.sum: follow-up desk-improvement/09"]
---

# Brief 09 — Integrate qualified improvement evidence into Improve

## Context

files:
- `tools/desk/internal/improve/deskquality.go (planned typed artifact adapter)`
- `tools/desk/internal/improve/deskquality_test.go (planned)`
- `tools/desk/internal/improve/rows.go (existing)`
- `tools/desk/internal/improve/gate_test.go (existing)`
- `tools/desk/internal/improve/readonly_test.go (existing)`
- `tools/desk/internal/improve/testdata/deskquality/ (planned synthetic fixtures)`
- `tools/desk/go.mod (only if qualified public module dependency is selected)`
- `tools/desk/go.sum (same condition)`
- `changelog/desk-improvement-09.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: existing tools/desk/internal/improve read-only view
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

1. Bind /02 scorecards and /03 diagnosis/protocol/assessment/follow-up records into existing proposal rows. Consume a compatible exported package or validated typed artifacts. Internal Improve is not exported to other modules and no private UI dependency enters public tools.
2. Render partial/stale/disputed/unsupported evidence explicitly and preserve all role profiles, source freshness and permitted drill-down. The view does not run an experiment, edit a skill, issue a ruling, deploy a policy or establish acceptance.
3. Preserve RouteHumanCommit, AdoptionsPerCadence = 1, glyph/label accessibility and independent side-effect refusals. A favorable card cannot restore an exhausted cadence or make an agent decision human. Keep existing tests and add negative source/protocol/version cases.
4. Prove the tools/desk consumer builds with GOWORK=off using the published dependency or qualified artifact route, without a sibling replace; old and new inputs remain supported or explicitly unknown. Document the same API/artifact interface for authorized clients rather than creating a new decision service.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd tools/desk && go test -v ./internal/improve/... -run '^TestDeskEvidence$' > "${TMPDIR:-/tmp}/di-09-TestDeskEvidence.out" && grep -F -- '--- PASS: TestDeskEvidence' "${TMPDIR:-/tmp}/di-09-TestDeskEvidence.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd tools/desk && go test -v ./internal/improve/... -run '^TestDeskCadence$' > "${TMPDIR:-/tmp}/di-09-TestDeskCadence.out" && grep -F -- '--- PASS: TestDeskCadence' "${TMPDIR:-/tmp}/di-09-TestDeskCadence.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd tools/desk && go test -v ./internal/improve/... -run '^TestDeskReadOnly$' > "${TMPDIR:-/tmp}/di-09-TestDeskReadOnly.out" && grep -F -- '--- PASS: TestDeskReadOnly' "${TMPDIR:-/tmp}/di-09-TestDeskReadOnly.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | check +mutation | `cd tools/desk && GOWORK=off go test ./internal/improve/...` | Consumer builds with declared published dependencies or the pure artifact adapter; no private/internal external import |
| 5 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/09` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
