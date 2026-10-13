---
stream: ci-tooling
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/ci-tooling/spec.md
---

# CI tooling and shared mechanisms

Public library, action and command migration; private adoption stays in its owning repository.

The [spec](spec.md) is draft. This stream is parked so its reviewable briefs do not
enter dispatch before revision-bound approval. No runtime or workflow is activated here.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Layout inventory and canonical workflow staging](brief-01-layout-inventory-and-canonical-workflow-staging.md) | 1 | M | todo | — | — |
| 02 | [Assay-wide result and repository input libraries](brief-02-assay-wide-result-and-repository-input-libraries.md) | 2 | M | todo | — | — |
| 03 | [CI-wide check and reporting libraries](brief-03-ci-wide-check-and-reporting-libraries.md) | 3 | L | todo | — | — |
| 04 | [Pinned installation and check actions](brief-04-pinned-installation-and-check-actions.md) | 4 | M | todo | — | — |
| 05 | [Pilot command migration and release packaging](brief-05-pilot-command-migration-and-release-packaging.md) | 5 | L | todo | — | — |
| 06 | [Relocate an existing public CI cohort](brief-06-relocate-an-existing-public-ci-cohort.md) | 6 | L | todo | — | — |
| 07 | [Workflow adoption and source retirement qualification](brief-07-workflow-adoption-and-source-retirement-qualification.md) | 7 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

Approval and a complete path/consumer inventory precede implementation. The smallest
unblocking move is brief 01; creating empty directories alone does not satisfy it.
See the dependency waves below and each brief's external prerequisites.

## Dependency waves

- Wave 1: `ci-tooling/01`.
- Wave 2: `ci-tooling/02`.
- Wave 3: `ci-tooling/03`.
- Wave 4: `ci-tooling/04`.
- Wave 5: `ci-tooling/05`.
- Wave 6: `ci-tooling/06`.
- Wave 7: `ci-tooling/07`.

The critical path is `01 → 02 → 03 → 04 → 05 → 06 → 07`.

## Entry conditions

On 2026-10-13, public `ci/staged-workflows/` and its drift declaration exist;
`core/` and the proposed CI module/actions do not. The real first implementation
step is the layout/consumer inventory and staging contract, not relocation of
commands that still import `tools/desk/internal`. The runtime service is not a prerequisite.
