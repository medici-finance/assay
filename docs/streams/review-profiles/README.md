---
stream: review-profiles
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: [3080]
board: generated
spec: docs/streams/review-profiles/spec.md
---

# Performance and architecture review profiles

[Draft specification](spec.md). Extend existing review and assessment owners with two
selectively applied profiles. This plan does not install two unconditional approval lanes,
start a pilot, enable a cadence or promote advisory findings to blockers.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Freeze assessment contracts and the offline replay kit](brief-01-freeze-assessment-contracts-and-the-offline-replay-kit.md) | 0 | M | todo | — | — |
| 02 | [Implement the evidence-led performance profile](brief-02-implement-the-evidence-led-performance-profile.md) | 1 | L | todo | — | — |
| 03 | [Extend design-fit with the architecture profile](brief-03-extend-design-fit-with-the-architecture-profile.md) | 1 | M | todo | — | — |
| 04 | [Calibrate incremental findings and full review cost](brief-04-calibrate-incremental-findings-and-full-review-cost.md) | 2 | L | todo | — | — |
| 05 | [Integrate approved advisory routing and finding continuity](brief-05-integrate-approved-advisory-routing-and-finding-continuity.md) | 3 | L | todo | — | — |
| 06 | [Qualify operational assessment and the bounded repair handoff](brief-06-qualify-operational-assessment-and-the-bounded-repair-handoff.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path and dependency waves

01 frozen assessment/context contract → 02 performance and 03 architecture profiles →
04 offline paired calibration → 05 opt-in review integration. The parallel 04 → 06 path
qualifies manual operational assessment and the bounded issue/repair handoff.
This is a dependency chain; measured effort is needed to identify elapsed-time criticality.

- Wave 0: 01 — local fixtures, contracts and replay kit.
- Wave 1: 02 and 03 — profile implementations sharing the same record and evidence basis.
- Wave 2: 04 — paired, held-out calibration and cost report.
- Wave 3: 05 — explicitly approved advisory routing; 06 — manual operational qualification.

The real head was checked at public main `7d52be61b66cb81f5ad2ab7e1d250747a6f125de`
on 2026-10-15: four current lanes exist in `reviewlanes.go`; design-fit scope and
architectural fitness tests already exist. Neither proposed profile has a calibrated
routing policy or assessment contract. Local contract/fixture work can begin without a
cell, database, telemetry deployment or new service. Actual model evaluation needs a
frozen numeric budget and permitted provider/data scope; live routing needs the later
explicit decision in 05. Missing those holds the affected evaluation, not offline authoring.

## Ownership and consumer routing

Current finding identity, review scope, round counting and ready aggregation remain owned
by desk review. Architecture extends design-fit; it does not fork that meaning. Operational
assessment belongs to continuing operations/quality. Optional performance collection
carries observations, while required CI evidence and runtime budget accounting remain
independent. See the spec for exact consumers and the future-architecture mapping.

All briefs are proposed and unstarted. Preserve historical implementation and verification
records in build-less-brittle and other streams. Deployment-specific pilot corpora and data
grants stay with each adopter; public fixtures are synthetic or explicitly public.
