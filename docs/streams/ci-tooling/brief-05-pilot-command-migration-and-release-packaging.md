---
brief: assay:assay:ci-tooling:05
title: "Pilot command migration and release packaging"
why: "A library that no shipped binary uses leaves both implementations alive. The pilot proves commands, release composition and external pins work before wider relocation."
wave: 5
depends: ["ci-tooling/03", "ci-tooling/04"]
unblocks: ["ci-tooling/06", "ci-tooling/07"]
effort: "L"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "8e16d89e-fe8c-4176-9c9d-f79e139dcc0f"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/20-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["ci/cmd/claimsguard/: follow-up ci-tooling/05", "ci/cmd/closecheck/: follow-up ci-tooling/05", "ci/cmd/releaseguard/: follow-up ci-tooling/05", "ci/tests/pilot_parity.py: follow-up ci-tooling/05", "ci/migration/inventory.json: follow-up ci-tooling/05", "Makefile: follow-up ci-tooling/05", "scripts/build-windows.ps1: follow-up ci-tooling/05", "ci/staged-workflows/release.yml: follow-up ci-tooling/05", "ci/staged-workflows/ci.yml: follow-up ci-tooling/05"]
gate-why: "This changes publication, workflow execution or private-data handling boundaries; review the concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 05 — Pilot command migration and release packaging

## Context

files: `ci/cmd/claimsguard/` (planned), `ci/cmd/closecheck/` (planned), `ci/cmd/releaseguard/` (planned), `ci/tests/pilot_parity.py` (planned), `ci/migration/inventory.json` (planned), `Makefile`, `scripts/build-windows.ps1`, `ci/staged-workflows/release.yml`, `ci/staged-workflows/ci.yml` (planned), `changelog/ci-tooling-05.md` (planned)

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

1. Require publication-cleared pilot exports and a recorded existing-path library adoption run from the source owner. If not supplied, report the prerequisite explicitly; public development uses synthetic consumers and must not depend on private access.
2. Move thin pilot entry points to ci/cmd and remove duplicated check logic. Preserve claimsguard versus the separate singular claimguard tool. Releaseguard calls the typed claims API when no isolation boundary requires a subprocess.
3. Update Unix/Windows builds, assets/checksums, module dependencies, examples, cross-module readers and staged CI path filters. Ensure both the old path and newly shared core/ci paths trigger relevant checks throughout transition. Keep required-check identities unchanged.
4. Build actual binaries and compare library, binary and action results over the same corpus. Record reviewed deltas for incomplete advisory output. Qualify a cold external consumer using released artifacts before marking released/adopted; retain source-owner copies until their independent consumer proof permits deletion.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `(cd ci && GOWORK=off go test ./...)` | exit 0; complete module suite passes | check |
| 2 | `python3 ci/tests/pilot_parity.py` | exit 0; PILOT PARITY PASS; built binaries and actions cover clean/fail/incomplete states | check +flow +dereference |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 exercises behavior and negative controls; the remaining row covers the
module/contract integration. Publication adequacy and retained authority also require review.
