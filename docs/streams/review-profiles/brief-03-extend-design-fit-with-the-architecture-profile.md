---
brief: "assay:assay:review-profiles:03"
title: "Extend design-fit with the architecture profile"
why: "A locally correct change can create another owner or temporary mechanism that moves the system away from its intended architecture. Make that direction explicit without demanding unrelated migration work."
wave: 1
depends: ["review-profiles/01"]
unblocks: ["review-profiles/04"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "d81545d8-7331-4e38-b65f-a2b687bce156"
authored: "2026-10-15 by review profile authoring"
sources: ["docs/streams/review-profiles/spec.md", "freshness-checked 2026-10-15 @ 7d52be61b66cb81f5ad2ab7e1d250747a6f125de"]
exec-tier: "strong"
exec-tier-why: "Cross-profile evidence, algorithm and authority judgments require the applicable contracts and negative controls."
domain: "complicated"
outcome: "none"
consumers: ["tools/desk/internal/reviewprofile/architecture.go: follow-up review-profiles/03", "tools/desk/internal/reviewprofile/architecture_test.go: follow-up review-profiles/03", "tools/desk/cmd/deskdispatch/references/architecture-review.md: follow-up review-profiles/03", "tools/desk/internal/arch/arch_test.go: follow-up review-profiles/03", "docs/contracts.md: follow-up review-profiles/03"]
---

# Brief 03 — Extend design-fit with the architecture profile

## Context

files: `tools/desk/internal/reviewprofile/architecture.go` (planned), `tools/desk/internal/reviewprofile/architecture_test.go` (planned), `tools/desk/cmd/deskdispatch/references/architecture-review.md` (planned), `tools/desk/internal/arch/arch_test.go`, `docs/contracts.md`. New profile, calibration and operations files are planned deliverables; existing desk owners remain controlling.

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

1. Extend existing design-fit semantics with the lead question: does this PR move us toward our future state? Explain advances/preserves/moves-away/cannot-assess against an exact architecture revision, approval status and applicable migration stage. A preserving change or accepted temporary seam can be valid.

2. Assess semantic ownership, dependency direction, pure evaluation/effects, credential custody, single coordination/lease/scheduler ownership, consumer migration, restart/recovery and retirement. Separate a new obligation from an existing one and preserve independent enforcement at distinct trust boundaries.

3. Bind concerns to changed mechanisms, cited target/decision, consequence and feasible alternative. Existing adopted obligations may be findings under existing scope; proposed destination alone cannot block. Missing inputs require an explicit cannot-assess reason rather than invented current architecture.

4. Reuse existing architectural fitness functions and owner index. Add deterministic cases only for demonstrated rules; include planted rival owners and wrong dependency direction plus valid adapters, approved transitional paths and unrelated legacy debt. Preserve historical build-less-brittle Evidence.

## Interface contract

Versioned, scoped inputs and exact source revisions → explained assessment with evidence, alternatives, coverage and resource provenance. Existing runtime, finding and acceptance owners retain authority. See Task for the exact producer/consumer seam this brief delivers.

## Verify

The named tests below are implementation deliverables, not claims that they exist today. Each must contain positive, negative and flow assertions described in Task; an empty test selection is not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v -run '^TestArchitectureProfile$' ./internal/reviewprofile > "$o" && grep -F -- '--- PASS: TestArchitectureProfile ' "$o")` | Explains movement, preserves valid transitional boundaries, detects rival authority, and refuses to promote proposed target prose into a blocking rule. |
| 2 | check:ci | `cd tools/desk && go test -count=1 ./internal/arch ./internal/deskkit` | Existing ownership, dependency, scope and finding tests remain green; do not weaken controls to pass. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief review-profiles/03` | exit 0; the brief's routing claims are corroborated against the implementation diff, including each changed consumer. |

## Definition of done

All Task cases are demonstrated through the Verify tests and a reproducible local record with source versions, actual cost/coverage and unresolved limits. Runtime claims require observed evidence. No profile promotion or cadence activation is inferred.

## Evidence

Independent verifier appends evidence after implementation; none is claimed by authoring.

## Review

Apply the frontmatter gate and existing review scope. Acceptance remains independent of implementation.
