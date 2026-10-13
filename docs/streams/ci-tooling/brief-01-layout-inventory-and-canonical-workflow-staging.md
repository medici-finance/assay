---
brief: assay:assay:ci-tooling:01
title: "Layout inventory and canonical workflow staging"
why: "Mixed command and service locations conceal owners and strand path-dependent checks during moves. One classified inventory and staging contract make each later cutover reviewable."
wave: 1
depends: []
unblocks: ["ci-tooling/02"]
effort: "M"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "d153d6f8-ef23-4b4b-bb49-d6018609475f"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/20-ci-tooling.md"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["ci/README.md: follow-up ci-tooling/01", "desk/README.md: follow-up ci-tooling/01", "operator/README.md: follow-up ci-tooling/01", "services/README.md: follow-up ci-tooling/01", "ci/migration/inventory.json: follow-up ci-tooling/01", "ci/tests/layout_contract.py: follow-up ci-tooling/01", "ci/staged-workflows/README.md: follow-up ci-tooling/01", "ci/staged-workflows/declared-changes.json: follow-up ci-tooling/01", "docs/contracts.md: follow-up ci-tooling/01", "docs/library-first.md: follow-up ci-tooling/01"]
gate-why: "This changes publication, workflow execution or private-data handling boundaries; review the concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 01 — Layout inventory and canonical workflow staging

## Context

files: `ci/README.md` (planned), `desk/README.md` (planned), `operator/README.md` (planned), `services/README.md` (planned), `ci/migration/inventory.json` (planned), `ci/tests/layout_contract.py` (planned), `ci/staged-workflows/README.md`, `ci/staged-workflows/declared-changes.json`, `docs/contracts.md`, `docs/library-first.md`, `changelog/ci-tooling-01.md` (planned)

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

1. Create family ownership documents and a versioned inventory enumerating every tracked executable/module and workflow helper, including non-Go programs. Each row records current/target path, semantic owner, callers, module/release identity, private/public disposition, staged/live state and retirement condition. Unclassified entries fail qualification.
2. Retain ci/staged-workflows as the canonical path. Inventory promoter/guard/skill/rule/test readers; document the adopter transition from the legacy path, conflict refusal and alias expiry. Preserve active workflow permissions and required-check names; author any workflow changes only as staged candidates.
3. Route every desk/operator/service move to a bounded existing runtime brief or explicitly authored follow-on before calling the overall directory split complete. Initial CI delivery does not move comms or expose deskkit internals. Reconcile unfinished plans without editing dated Evidence.
4. Implement layout_contract.py with current-tree coverage and synthetic negative cases: unclassified tool, duplicate conflicting staged file, retired-path writer and missing trigger reader. Update contracts and library-first only with proposed/implemented state accurately labeled.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `python3 ci/tests/layout_contract.py` | exit 0; LAYOUT PASS; all four negative cases rejected | check +flow +dereference |
| 2 | `statusgen --root . --lint` | exit 0; no new stream or contract problems | check |
| 3 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Verify row 1 exercises behavior and negative controls; the remaining row covers the
module/contract integration. Publication adequacy and retained authority also require review.
