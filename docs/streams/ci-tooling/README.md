---
stream: ci-tooling
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: [2751]
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
| 01 | [Layout inventory and workflow staging coexistence](brief-01-layout-inventory-and-canonical-workflow-staging.md) | 1 | M | todo | — | — |
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

On 2026-10-14, two public staging areas exist: `ci/staged-workflows/` (with its drift
declaration) and `tools/ci-load/activation/`; three workflows are staged in both.
`core/` and the proposed CI module/actions do not exist. The real first implementation
step is the layout/consumer inventory and staging contract, not relocation of
commands that still import `tools/desk/internal`. The runtime service is not a prerequisite.

External prerequisites, each with its stop rule:

- **Workflow landing path (owner: desk-supervision/10–12).** Every brief delivers workflow
  changes through the path in force at pickup: a staged candidate under
  `ci/staged-workflows/` until desk-supervision/12 is `done`, a workflow-only PR through
  the workflow App path after. This stream never writes into `tools/ci-load/activation/`
  and never adds a staging root. Stop rule: a candidate that would duplicate a workflow
  staged elsewhere without a recorded reconciliation is refused, not landed.
- **Publication of adopter-held pilot tools (owner: each tool's source owner).** Brief 03
  is clean-room and needs none. Brief 05 imports an adopter-held tool only with that
  owner's recorded per-tool human publication ruling, checked at its human gate. Stop
  rule: an absent or refused ruling drops that tool from the pilot; if none is cleared,
  brief 05 runs the fallback pilot (public `tools/claimguard`), so the path does not stall.
- **`tools/harnesslint` CLI migration (desktools-v2/49).** Brief 06 relocates
  `tools/harnesslint` only after desktools-v2/49 lands, and relocates the migrated tree.
- **Submodule tags (owner: the release process).** External consumer cutovers in briefs
  04–07 stop until a published `core/` or `ci/` module version resolves.
