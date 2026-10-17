---
brief: "assay:assay:desk-improvement:04"
title: "Add evidence-led improvement procedures to desk roles"
why: "Roles need a repeatable way to explain evidence, test a change and revisit its outcome; collecting metrics alone does not establish improvement."
wave: 2
depends: ["desk-improvement/03"]
unblocks: ["desk-improvement/10"]
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "yes", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "7825545d-ccfe-4444-8a88-5dca6b4a7a17"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["plugins/assay/references/desk-improvement.md: follow-up desk-improvement/04", "plugins/assay/skills/the-desk/SKILL.md: follow-up desk-improvement/04", "plugins/assay/skills/intake-desk/SKILL.md: follow-up desk-improvement/04", "plugins/assay/skills/worker-desk/SKILL.md: follow-up desk-improvement/04", "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-improvement/04", "plugins/assay/skills/verify-desk/SKILL.md: follow-up desk-improvement/04", "tools/skillslint/desk_improvement_test.go: follow-up desk-improvement/04"]
gate-why: "Changes shipped role procedure references; maintainer confirms the concrete opt-in profile, independence and adoption obligations before activation."
decision-trigger: "spec"
---

# Brief 04 — Add evidence-led improvement procedures to desk roles

## Context

files:
- `plugins/assay/references/desk-improvement.md (planned shared reference)`
- `plugins/assay/skills/the-desk/SKILL.md (existing; reference integration)`
- `plugins/assay/skills/intake-desk/SKILL.md (existing; reference integration)`
- `plugins/assay/skills/worker-desk/SKILL.md (existing; reference integration)`
- `plugins/assay/skills/pr-review-desk/SKILL.md (existing; reference integration)`
- `plugins/assay/skills/verify-desk/SKILL.md (existing; reference integration)`
- `tools/skillslint/desk_improvement_test.go (planned routing/control tests)`
- `changelog/desk-improvement-04.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: shared role procedures and continuing-operations profile reference
  contract: none — desk-improvement/01 adds the assessment row to the existing semantic-owner index
  retires: []
  weight: "CLI modes +0; policy refusals +0; rule-text one shared opt-in reference, five short links"
  why-add: "Existing source/assessment gaps need one owned extension; a duplicate collector, rule set or execution ledger was rejected."

Single-point-of-failure note: validation cannot establish truth or authority; independent native-source/label assessment backs structural checks. Neither layer may grant acceptance or policy adoption.

## Read first

[Canonical specification](spec.md), [plan and prerequisites](README.md), and the exact upstream owner contracts named in gates. Their required boundaries are restated in Task. Planned paths/tests below are deliverables, not claims that implementation exists.

## Human decision

Prepare the exact shared procedure/profile changes and supported capture/validation versions. Decide whether to ship the opt-in procedure, preserving current role boundaries and requiring separate deployment cadence approval.

Options:
1. **Ship the reviewed opt-in procedure** — roles can use the documented measurement/diagnosis/experiment/follow-up path; existing acceptance and human adoption remain binding.
2. **Keep templates only** — defer shipped role wiring until missing evidence or procedure questions are resolved.

Default if no answer: none — affected activation remains held; neutral offline preparation can continue.

## Ground rules

- Isolated branch and draft PR; no main push, merge, workflow dispatch, production endpoint or cluster contact.
- Offline synthetic/disposable fixtures; no model spend, real-data export or enrollment is granted here.
- Preserve completed Evidence and current authority; stop at implemented for independent verification.

## Task

1. Add one shared role-profile/procedure reference rather than copying the full method into each skill. The five supplied profiles remain configurable bindings. Preserve generated guardrail blocks, existing claim/independence/stop/human gates and role capability limits.
2. Each role supplies source receipts and explains its process; the coordinator profile completes the recurring diagnosis/experiment/assessment/decision/follow-up cycle. Independent review challenges coordinator changes too. Specify competing explanations, removal/no-change alternatives, materiality, uncertainty and the next discriminating observation; an insufficient-evidence result is valid.
3. Prepare the exact opt-in profile/cadence and affected skill changes for maintainer review before activation. Reuse existing reporting/adoption owners. References must name only shipped or explicitly planned commands; manual templates remain usable before a qualified tool release. The skill does not invent missing telemetry or schedule experiments by default.
4. Exercise a synthetic role handoff with missing evidence, an unwarranted root-cause claim and a favorable but unauthorized adoption. Prove the documented route holds/asks for the applicable independent evidence rather than granting policy. Reference checks establish wiring; independent review owns procedure adequacy.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd tools/skillslint && go test -v ./... -run '^TestDeskProfileRoutes$' > "${TMPDIR:-/tmp}/di-04-TestDeskProfileRoutes.out" && grep -F -- '--- PASS: TestDeskProfileRoutes' "${TMPDIR:-/tmp}/di-04-TestDeskProfileRoutes.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd tools/skillslint && go test -v ./... -run '^TestDeskCycleNoAdopt$' > "${TMPDIR:-/tmp}/di-04-TestDeskCycleNoAdopt.out" && grep -F -- '--- PASS: TestDeskCycleNoAdopt' "${TMPDIR:-/tmp}/di-04-TestDeskCycleNoAdopt.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd tools/skillslint && go test -v ./... -run '^TestDeskEvidenceGap$' > "${TMPDIR:-/tmp}/di-04-TestDeskEvidenceGap.out" && grep -F -- '--- PASS: TestDeskEvidenceGap' "${TMPDIR:-/tmp}/di-04-TestDeskEvidenceGap.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | gate:human +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/04` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: human for the concrete shipped procedure scope; no verified/done assertion is made by this plan.
