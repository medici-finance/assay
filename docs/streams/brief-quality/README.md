---
stream: brief-quality
repo: medici-finance/assay
serves: assay
status: parked
spec: docs/streams/brief-quality/spec.md
priority: P1
track: platform
board: generated
issues: []
---

# Brief quality — measure the contract before judging the worker

[Design specification](spec.md). Proposed stream, not an enabled policy. Origin: the 2026-09-26 request to codify six brief dimensions, measure authoring quality and evaluate independent acceptance review. Deliverables live in this public repository; deployment-specific telemetry stays with its operator.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Define the brief assessment and outcome contract](brief-01-design-contract.md) | 0 | M | todo | — | — |
| 02 | [Ratify acceptance-review and pilot policy](brief-02-policy-rulings.md) | 1 | S | todo | — | — |
| 03 | [Extend brief parsing, lint and templates with six dimensions](brief-03-assessment-schema.md) | 2 | M | todo | — | — |
| 04 | [Rewrite authoring and review procedures around assessed contracts](brief-04-desk-procedures.md) | 3 | M | todo | — | — |
| 05 | [Add portable brief-quality events and immutable snapshots](brief-05-event-contract.md) | 3 | M | todo | — | — |
| 06 | [Bind independent acceptance approval to the dispatch contract](brief-06-acceptance-admission.md) | 4 | M | todo | — | — |
| 07 | [Collect execution outcomes and adjudicated authoring discoveries](brief-07-outcome-adapters.md) | 4 | M | todo | — | — |
| 08 | [Report brief-authoring quality and cost with coverage](brief-08-quality-analysis.md) | 5 | M | todo | — | — |
| 09 | [Evaluate authoring quality and decide whether to expand](brief-09-pilot-rollout.md) | 6 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path and the real head

01 → 02 → 03 → 05 → 07 → 08 → 09, with 03 → 04 → 06 and 05 → 06 joining before 08. This is the dependency chain; elapsed-time criticality needs measured durations, which do not exist yet. Brief 01 delivers this design; 02 is the first unresolved authority prerequisite. No author may turn D2–D5 recommendations into approvals to accelerate later work.

The head was checked at 7aa3835d7 on 2026-09-26: profile parsing, approval-bound dispatch and brief-quality event reduction do not exist under the proposed names. Existing `qualgen/telemetry`, `qualgen/attribution`, `qualgen/reflex`, statusgen brief-v2 and witnesses do exist. Open PR titles/branches were checked for an overlapping stream; none named brief-quality. This is an extension to those seams, not a replacement miner.

## Dependency waves

- Wave 0: brief-quality/01.
- Wave 1: brief-quality/02.
- Wave 2: brief-quality/03.
- Wave 3: brief-quality/04, brief-quality/05.
- Wave 4: brief-quality/06, brief-quality/07.
- Wave 5: brief-quality/08.
- Wave 6: brief-quality/09.

## Acceptance and release boundary

All implementation briefs remain unstarted. This change authors the plan and supplies the design deliverable for 01; it does not claim independent verification. The current brief schema remains in force until 03 lands. This stream does not self-certify its own acceptance review or fabricate retrospective assessments.

A companion article and illustrative explainer describing this idea-to-spec-to-brief path may be authored separately, subject to their own publication review; neither is evidence that the proposed monitoring tool or admission gate has shipped, and this stream does not depend on either existing.
