---
brief: assay:assay:ci-tooling:07
title: "Workflow adoption and source retirement qualification"
why: "Moved files are not migrated consumers. Final qualification binds source retirement to working pinned releases and observed workflow adoption, preventing silent fallback and stranded checks."
wave: 7
depends: ["ci-tooling/05", "ci-tooling/06"]
unblocks: []
effort: "M"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "6f494350-ed0e-476d-b2b6-51323861d154"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/24-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["ci/tests/migration_contract.py: follow-up ci-tooling/07", "ci/migration/inventory.json: follow-up ci-tooling/07", "ci/README.md: follow-up ci-tooling/07", "ci/staged-workflows/: follow-up ci-tooling/07", "docs/distribution.md: follow-up ci-tooling/07", "docs/contracts.md: follow-up ci-tooling/07", "docs/library-first.md: follow-up ci-tooling/07"]
gate-why: "This changes publication, workflow execution or private-data handling boundaries; review the concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 07 — Workflow adoption and source retirement qualification

## Context

files: `ci/tests/migration_contract.py` (planned), `ci/migration/inventory.json` (planned), `ci/README.md` (planned), `ci/staged-workflows/`, `docs/distribution.md`, `docs/contracts.md`, `docs/library-first.md`, `changelog/ci-tooling-07.md` (planned)

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

1. Convert the selected Assay staged workflows to the qualified actions and actual immutable pins. Retain provider-required active workflow location, triggers, job permissions, concurrency and required-check names. Reconcile declared staged/live deltas honestly; promotion is a separate authorized act.
2. Publish a neutral adopter cutover recipe: public module and binary release qualification, old-path wrapper adoption, local policy isolation, workflow promotion, observed run and then source deletion. Public records expose no private identifiers, corpus paths or counts.
3. Implement migration_contract.py over the inventory with refusal cases for unreleased versions, missing consumer evidence, conflicting canonical sources, unmerged fixes and stale import/action pins. Test rollback to the previous qualified pin combination without source fallback.
4. Refresh the ownership register and archive only fulfilled transition entries. Every residual tool has an owning bounded brief, including desk/operator/service paths assigned to runtime migration. Report net production/test change separately from relocation and cross-repository copy retirement. Update docs and run statusgen lint; no generated board/status files are branch-committed.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `python3 ci/tests/migration_contract.py` | exit 0; MIGRATION PASS; all premature-retirement controls rejected | check +flow +dereference |
| 2 | `statusgen --root . --lint` | exit 0; no new documentation/consumer problems | check |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 exercises behavior and negative controls; the remaining row covers the
module/contract integration. Publication adequacy and retained authority also require review.
