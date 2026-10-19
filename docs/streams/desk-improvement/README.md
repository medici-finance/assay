---
stream: desk-improvement
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/desk-improvement/spec.md
---

# Desk improvement — reusable assessment and learning

[Draft canonical specification](spec.md). Extend continuing operations/quality with configurable
scorecards, diagnosis, bounded experiments, independent assessment and follow-up. Public source
and fixtures are neutral; adopter data and permission decisions remain with the adopter.
This plan implements no live rollout or policy adoption merely by being authored.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Define the reusable scorecard and experiment contracts](brief-01-define-the-reusable-scorecard-and-experiment-contracts.md) | 0 | M | todo | — | — |
| 02 | [Implement configurable scorecards and pure reduction](brief-02-implement-configurable-scorecards-and-pure-reduction.md) | 1 | M | todo | — | — |
| 03 | [Validate analysis experiments and follow-up offline](brief-03-validate-analysis-experiments-and-follow-up-offline.md) | 2 | M | todo | — | — |
| 04 | [Add evidence-led improvement procedures to desk roles](brief-04-add-evidence-led-improvement-procedures-to-desk-roles.md) | 3 | M | todo | — | — |
| 05 | [Bind desk assessments to canonical evidence records](brief-05-bind-desk-assessments-to-canonical-evidence-records.md) | 1 | M | todo | — | — |
| 06 | [Capture native loop-admin lifecycle and usage evidence](brief-06-capture-native-loop-admin-lifecycle-and-usage-evidence.md) | 2 | M | todo | — | — |
| 07 | [Capture workflow attempt and decision evidence](brief-07-capture-workflow-attempt-and-decision-evidence.md) | 3 | M | todo | — | — |
| 08 | [Capture standing desk subjects and semantic outcomes](brief-08-capture-standing-desk-subjects-and-semantic-outcomes.md) | 4 | M | todo | — | — |
| 09 | [Integrate qualified improvement evidence into Improve](brief-09-integrate-qualified-improvement-evidence-into-improve.md) | 3 | M | todo | — | — |
| 10 | [Qualify native evidence across execution modes](brief-10-qualify-native-evidence-across-execution-modes.md) | 5 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Two delivery paths

**Offline path:** 01 contracts → 02 cards and CLI → 03 analysis/experiment validation →
04 role procedures and 09 Improve projection. An adopter may begin with permitted retained
inputs and explicit gaps. No native-runtime dependency or private trial blocks public design.

**Native evidence path:** 01 → {02 reducer, 05 source bindings} → 06 loop-admin capture →
07 workflow capture; 08 standing desk outcomes joins 06 capture with 04's shared role
reference. Both native paths feed 10 cross-mode qualification.
05 needs existing graph-execution/06 and brief-quality/05 contracts; 06 needs the existing
supervisor and qualified adapter (graph-execution/26 and /22) plus the /02 reducer;
07 needs controller /21; 08 needs /04's shared reference, standing client /27 and
brief-quality/07 adapters. These prerequisites are real
implementation dependencies, not assumed shipped capabilities.

The offline semantic head is statusgen/22 → continuing-operations/01 → 01; /03 also waits on continuing-operations/02. These source contracts remain todo at the inspected revision, so they are actual build prerequisites, not a deployed capability. Local design work can proceed from their proposed intent; implementation cannot silently skip them. The runtime head also includes unfinished upstream record, recovery,
supervisor and controller work; a schema or usage struct alone is not native emission.
A current source audit at public main 2f56cf289 (2026-10-17) found the runner protocol and
unknown-preserving Usage type, while most native consumers are still planned. Record exact
build/release/activation scope at pickup; do not infer deployed support from board labels.

## Dependency waves

Local waves count local depends edges. Same-repo upstream owners appear as typed gates to
keep their independent stream waves intact. Eligibility still holds on every required gate.

| Wave | Briefs | External owner prerequisites |
|---|---|---|
| 0 | 01 | continuing-operations/01, which depends on statusgen/22 |
| 1 | 02, 05 | 05: graph-execution/06, brief-quality/05 |
| 2 | 03, 06 | 03: continuing-operations/02; 06: graph-execution/26 and /22 |
| 3 | 04, 07, 09 | 07: graph-execution/21 |
| 4 | 08 | graph-execution/27 and brief-quality/07 |
| 5 | 10 | Native producer closure and existing role/mode contracts |

Local critical path: `01 → 02 → 03 → 04 → 08 → 10`. The smallest implementation
unblock remains the semantic source contracts (`statusgen/22 → continuing-operations/01`)
before /01. Local wave numbers do not satisfy unfinished external gates. /03 consumes
/02's CLI, /06 consumes its reducer, and /08 extends /04's shared reference; completing
other prerequisites cannot make those producers optional.

## Ownership and future state

This stream owns assessment meaning and the missing evidence adapters, not execution state.
Runtime owners keep their PostgreSQL coordination, authority, claims and recovery; work and
acceptance remain in their canonical domains. See the [future-state amendment](../../proposals/non-ci-architecture/22-desk-improvement.md)
and the [amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md).

Implementation uses existing public contracts and examples only. Adopter qualification is an
outward consumer, never an inbound dependency or private context requirement. Source, release,
transition, deployed support and observed benefit remain distinct. All new briefs are parked
with this draft; future tests/paths in them are deliverables, not execution evidence.
