---
brief: "assay:assay:review-profiles:04"
title: "Calibrate incremental findings and full review cost"
why: "Extra review can consume more than the defects it prevents. Compare validated additional findings and total burden before asking adopters to enable either profile."
wave: 2
depends: ["review-profiles/02", "review-profiles/03"]
unblocks: ["review-profiles/05", "review-profiles/06"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "977325ab-07d0-459e-9d2d-68098d214fea"
authored: "2026-10-15 by review profile authoring"
sources: ["docs/streams/review-profiles/spec.md", "freshness-checked 2026-10-15 @ 7d52be61b66cb81f5ad2ab7e1d250747a6f125de"]
exec-tier: "strong"
exec-tier-why: "Cross-profile evidence, algorithm and authority judgments require the applicable contracts and negative controls."
domain: "complicated"
outcome: "none"
consumers: ["tools/desk/cmd/deskcalibrate/profilepilot.go: follow-up review-profiles/04", "tools/desk/cmd/deskcalibrate/profilepilot_test.go: follow-up review-profiles/04", "tools/desk/internal/reviewprofile/calibration.go: follow-up review-profiles/04", "tools/desk/internal/reviewprofile/calibration_test.go: follow-up review-profiles/04", "docs/review-profile-calibration.md: follow-up review-profiles/04"]
---

# Brief 04 — Calibrate incremental findings and full review cost

## Context

files: `tools/desk/cmd/deskcalibrate/profilepilot.go` (planned), `tools/desk/cmd/deskcalibrate/profilepilot_test.go` (planned), `tools/desk/internal/reviewprofile/calibration.go` (planned), `tools/desk/internal/reviewprofile/calibration_test.go` (planned), `docs/review-profile-calibration.md` (planned). New profile, calibration and operations files are planned deliverables; existing desk owners remain controlling.

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

1. Extend existing deskcalibrate with an offline/imported-output path using the assessment contract; do not create a second calibration ledger or model execution service. Before any actual model run require numeric per-case/total effort, tokens/spend/CPU/wall/forge caps, allowed provider/source scope, sample plan, threshold/uncertainty method, stop rules and decision owner. Missing values hold that run; synthetic scoring remains usable.

2. Provide deterministic case selection and defect-family-separated development/held-out partitions. Freeze original base/head packets and exclude later fixes, diagnoses, comments and answer labels. Compare existing baseline with baseline+performance, baseline+architecture and combined profiles on identical inputs and equal caps. Freeze versions; seal outputs before independently adjudicating condition-hidden findings.

3. Report incremental important findings, precision/false positives, known-corpus recall, missed cases, severity, duplicates, disagreements, repair/reversal rates and measured resource coverage. Include failed/retried runs, packet preparation, collection, benchmark and adjudication costs, and human hands-on effort separately from elapsed waits.

4. Produce mean and P50/P95 with raw/mergeable distributions, sample counts and method; no average of daily P95s. Preserve unattributed/shared overhead and unavailable CPU/usage. Calculate cost per validated incremental important finding only with a nonzero denominator; pair delivery cost/lead time per verified brief with correctness/security outcomes. Incomplete holdout/support is inconclusive. No promotion follows automatically.

## Interface contract

Versioned, scoped inputs and exact source revisions → explained assessment with evidence, alternatives, coverage and resource provenance. Existing runtime, finding and acceptance owners retain authority. See Task for the exact producer/consumer seam this brief delivers.

## Verify

The named tests below are implementation deliverables, not claims that they exist today. Each must contain positive, negative and flow assertions described in Task; an empty test selection is not a pass.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test -count=1 -v -run '^TestProfileCalibration$' ./cmd/deskcalibrate > /tmp/review-profile-04.log && grep -F -- '--- PASS: TestProfileCalibration ' /tmp/review-profile-04.log` | Seeded cohorts replay identically; family/label leakage and unequal caps are flagged; missing usage and zero-finding ratios stay unknown/undefined; paired totals include failures. |
| 2 | `cd tools/desk && go test -count=1 ./internal/arch ./internal/deskkit` | Existing ownership, dependency, scope and finding tests remain green; do not weaken controls to pass. |

## Definition of done

All Task cases are demonstrated through the Verify tests and a reproducible local record with source versions, actual cost/coverage and unresolved limits. Runtime claims require observed evidence. No profile promotion or cadence activation is inferred.

## Evidence

Independent verifier appends evidence after implementation; none is claimed by authoring.

## Review

Apply the frontmatter gate and existing review scope. Acceptance remains independent of implementation.
