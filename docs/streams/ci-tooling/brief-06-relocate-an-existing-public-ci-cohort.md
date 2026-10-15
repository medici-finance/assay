---
brief: assay:assay:ci-tooling:06
title: "Relocate an existing public CI cohort"
why: "The public tree already contains reusable checkers. Migrating a bounded cohort prevents a second copy being imported from an adopter and validates the pattern beyond the pilot."
wave: 6
depends: ["ci-tooling/05"]
unblocks: ["ci-tooling/07"]
effort: "L"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "0e25d867-24b0-44e2-a8a4-395b17fcd501"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/24-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["tools/claimguard/: follow-up ci-tooling/06", "tools/harnesslint/: follow-up ci-tooling/06", "tools/plugindrift/: follow-up ci-tooling/06", "ci/cmd/: follow-up ci-tooling/06", "ci/checks/: follow-up ci-tooling/06", "ci/tests/cohort_parity.py: follow-up ci-tooling/06", "ci/migration/inventory.json: follow-up ci-tooling/06", "Makefile: follow-up ci-tooling/06", "scripts/build-windows.ps1: follow-up ci-tooling/06", "ci/staged-workflows/ci.yml: follow-up ci-tooling/06", "ci/staged-workflows/release.yml: follow-up ci-tooling/06"]
gate-why: "This changes publication, workflow execution or private-data handling boundaries; review the concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 06 — Relocate an existing public CI cohort

## Context

files: `tools/claimguard/`, `tools/harnesslint/`, `tools/plugindrift/`, `ci/cmd/` (planned), `ci/checks/` (planned), `ci/tests/cohort_parity.py` (planned), `ci/migration/inventory.json` (planned), `Makefile`, `scripts/build-windows.ps1`, `ci/staged-workflows/ci.yml` (planned), `ci/staged-workflows/release.yml`, `changelog/ci-tooling-06.md` (planned)

facts:
- Source inspection: 2026-10-13; public Assay base 4696d598e. Refresh inventory and consumer pins at pickup.
- The stream spec defines the two library layers, private namespace and staged/released/adopted distinction.
- This is an authored migration obligation; the named new tests and paths are deliverables, not existing capabilities.

layering: Keep domain checks explicit-input and shared mechanisms narrow; process/provider and CI presentation adapters stay outside pure evaluators. Task and Verify below exercise the complete consumer flow.

design-fit:
  owner: CI tool family and the existing domain owners named in the spec
  contract: none — this draft proposes the packaging/check-result seam; existing S-eligibility and S-evidence-coverage remain unchanged
  retires: [duplicated adapters or obsolete paths named in this brief after consumer proof]
  weight: verbs +0, flags +0, refusals +0 intended externally; rule-text lines added for the bounded migration contract
  why-add: a source move alone cannot preserve pin, publication and workflow authority; this contract assigns the retirement proof to its owner

## Read first

- [Stream spec](spec.md) and [dependency map](README.md).
- Public Assay `docs/contracts.md` and `docs/library-first.md`; do not introduce a rival semantic owner.

## Human decision

The proposed migration changes a boundary used to run checks or keep private material local.
Approve the reviewed scope only after the implementation identifies actual versions,
consumer coverage and negative-control results. Approval does not expand credentials.

Options:
1. **Approve the qualified cutover** — retain current authority and use the tested rollback combination.
2. **Hold the cutover** — keep the supported implementation until the missing proof is supplied.

Default if no answer: none — activation blocks until answered.

## Ground rules

- Author staged workflows only; no workflow activation, credential expansion or live infrastructure probes.
- Preserve historical Evidence and independent controls. Stop at implementation; do not assert verified/done.
- Missing publication, release or consumer evidence is could-not-check, never permission to delete source.

## Task

1. Move only the existing public claimguard, harnesslint and plugindrift cohort in this brief. Map each old module/import and command to its new owner; retain a supported compatibility module only with a named external consumer and expiry.
2. Adopt shared mechanisms only where behavior matches. Preserve each tool's domain-specific semantics, side-effect boundaries and declared exclusions. Keep other inventory rows explicitly routed; this cohort is not an all-tools rewrite.
3. Update build/release roots, staged workflow triggers, scripts, tests and documentation together. Cross-module readers must trigger on both their code and every external fixture/policy path they read.
4. Compare old/new builds with recorded compatible toolchains and synthetic inputs, including unavailable upstream observations and malformed metadata. Retire old paths only after declared callers are updated and the replacement release is qualified.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd ci && GOWORK=off go test ./...)` | exit 0; retained domain and shared suites pass | check |
| 2 | `python3 ci/tests/cohort_parity.py` | exit 0; COHORT PARITY PASS; missing fixture and missing trigger controls fail | check +flow +dereference |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 exercises behavior and negative controls; the remaining row covers the
module/contract integration. Publication adequacy and retained authority also require review.
