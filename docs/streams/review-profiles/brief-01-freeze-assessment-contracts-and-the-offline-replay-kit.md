---
brief: "assay:assay:review-profiles:01"
title: "Freeze assessment contracts and the offline replay kit"
why: "A specialist review cannot be calibrated or replayed if its evidence, assumptions and cost are lost. Freeze one local contract before extending prompts or dispatch."
wave: 0
depends: []
unblocks: ["review-profiles/02", "review-profiles/03"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "2f4a3639-7582-4c7d-8c2e-a4a0f5946ded"
authored: "2026-10-15 by review profile authoring"
sources: ["docs/streams/review-profiles/spec.md", "freshness-checked 2026-10-15 @ 7d52be61b66cb81f5ad2ab7e1d250747a6f125de"]
exec-tier: "strong"
exec-tier-why: "Cross-profile evidence, algorithm and authority judgments require the applicable contracts and negative controls."
domain: "complicated"
outcome: "none"
consumers: ["spec/review-assessment-v1.md: follow-up review-profiles/01", "schemas/review-assessment-v1.json: follow-up review-profiles/01", "tools/desk/internal/reviewprofile/contract.go: follow-up review-profiles/01", "tools/desk/internal/reviewprofile/contract_test.go: follow-up review-profiles/01", "tools/desk/internal/reviewprofile/testdata/: follow-up review-profiles/01"]
---

# Brief 01 — Freeze assessment contracts and the offline replay kit

## Context

files: `spec/review-assessment-v1.md` (planned), `schemas/review-assessment-v1.json` (planned), `tools/desk/internal/reviewprofile/contract.go` (planned), `tools/desk/internal/reviewprofile/contract_test.go` (planned), `tools/desk/internal/reviewprofile/testdata/` (planned). New profile, calibration and operations files are planned deliverables; existing desk owners remain controlling.

facts:
- At the pinned main there are four review lanes, existing advisory design-fit and architectural fitness tests; no calibrated performance/architecture routing policy exists.
- Review finding identity, scope and rounds retain their existing owner; required acceptance evidence and budget accounting do not depend on optional telemetry.
- This brief implements only its Task; the profile spec remains a draft and actual model/data use requires a permitted scoped budget.

layering: pure assessment contracts/evaluation in tools/desk/internal/reviewprofile; deskkit and existing commands adapt inputs/effects. No controller, credential store, general service or parallel finding ledger. Task and Verify exercise that boundary.

design-fit:
  owner: existing review/policy and continuing-operations meanings; reviewprofile owns only assessment validation/evaluation
  contract: none — register the bounded assessment extension in review-profiles/01 without cloning existing owners
  retires: []
  weight: verbs +0; flags +0 until concrete adapter design; refusals +0 authoritative policy; rule-text increases by the bounded profile definition
  why-add: Two requested assessment perspectives need explicit evidence and calibration; extend existing owners instead of adding standing services or unconditional lanes. Measure actual adapter weight at implementation.

## Read first

Read [the controlling spec](spec.md), [the plan](README.md), `tools/desk/internal/deskkit/reviewlanes.go`, `docs/streams/desk-supervision/review-finding-v1.md`, `docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md`, and the exact consumer paths above where already present. Refresh source/claims at pickup; planned paths are not existing capabilities.

## Ground rules

- Own isolated branch and draft PR; no main push, merge, workflow dispatch or live infrastructure contact.
- Use synthetic/public fixtures unless an explicit source/provider grant permits more. No live activation follows from this brief alone.
- Preserve correctness/security obligations and independent verification; stop at implemented.
- Missing information is explicit; do not convert unknown into zero or approval.

## Task

1. Define the versioned assessment envelope and context manifest described in the controlling spec. Keep exact base/head, contract and approved-architecture revisions, migration stage, source scope/freshness, applicability, profile/model/prompt/router/workload versions and finding references. Do not clone review-finding-v1 or run-record authority.

2. Implement pure validation and a local, offline fixture packet/replay format. Keep findings/evidence separate from optional resource observations. Design-opportunity, measured result, explicit-budget violation and unknown must be distinguishable; architecture direction is advances/preserves/moves-away/cannot-assess with reasons.

3. Freeze synthetic positive, negative and legitimate-transition fixtures, trusted benchmark definitions and independent expected results. Withhold labels and later repairs from reviewer packets. Add mutation cases for missing/wrong candidate, contract, scope, profile version and unsupported input.

4. Register the assessment meaning and adapter consumers in docs/contracts.md. Define explicit compatibility/refusal for unsupported required fields. No database, cell controller, model request or telemetry source is needed for local validation.

## Interface contract

Versioned, scoped inputs and exact source revisions → explained assessment with evidence, alternatives, coverage and resource provenance. Existing runtime, finding and acceptance owners retain authority. See Task for the exact producer/consumer seam this brief delivers.

## Verify

The named tests below are implementation deliverables, not claims that they exist today. Each must contain positive, negative and flow assertions described in Task; an empty test selection is not a pass.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test -count=1 -v -run '^TestAssessmentContract$' ./internal/reviewprofile > /tmp/review-profile-01.log && grep -F -- '--- PASS: TestAssessmentContract ' /tmp/review-profile-01.log` | Wrong subject/contract and absent required evidence are rejected; unknown optional usage stays unknown; clean fixture round-trips offline. |
| 2 | `cd tools/desk && go test -count=1 ./internal/arch ./internal/deskkit` | Existing ownership, dependency, scope and finding tests remain green; do not weaken controls to pass. |

## Definition of done

All Task cases are demonstrated through the Verify tests and a reproducible local record with source versions, actual cost/coverage and unresolved limits. Runtime claims require observed evidence. No profile promotion or cadence activation is inferred.

## Evidence

Independent verifier appends evidence after implementation; none is claimed by authoring.

## Review

Apply the frontmatter gate and existing review scope. Acceptance remains independent of implementation.
