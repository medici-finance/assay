---
brief: continuing-operations/01
title: Operational subjects linked to product intent
why: "Product requirements can describe the same journey. Stable operational identities keep their definitions and assessments from being copied into competing records."
wave: 2
depends: ["statusgen/22"]
unblocks: ["continuing-operations/02"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: 6e926827-9af4-4d89-a62d-84c808e364c1
consumers:
  - "spec/operational-subject-v1.md, schemas/operational-subject-v1.json, statusgen/operations: follow-up continuing-operations/01"
  - "statusgen/operations observation obligations: follow-up continuing-operations/02"
---

# Brief 01 — Operational subjects linked to product intent

## Context

files: `spec/operational-subject-v1.md` (planned), `schemas/operational-subject-v1.json` (planned), `statusgen/operations/` (planned), `statusgen/testdata/operations/` (planned), `docs/operations/README.md` (planned), `docs/contracts.md`; `changelog/continuing-operations-01-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- Continuing-operations is parked; outline O01 proposed operational subjects but has no implemented contract.
- Product intent is owned by REQUIREMENTS; operational workflows are not delivery workflow instances.
- This is the bounded O01 contract slice, not activation of an outcomes desk.

layering: pure typed records/evaluators in `statusgen/operations`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/operations
  contract: operational-subject-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Define operational journey/workflow identities, accountable owner, versioned definitions and typed REQ relations. Preserve many-to-many links, rename/split/merge lineage and retired tombstones; no copied target or assessment authority.
2. Implement pure subject/reference validation and explicit snapshot loading conventions. Resolve the /22 extension through typed references; unresolved/denied references remain unknown with no hidden names or counts. A product owner link never grants operational procedure-change authority.
3. Publish synthetic examples and migration rules; map outline O01 to this brief while leaving cadence and procedure activation deferred.

## Interface contract

Input: /22 intent references and explicit operational definition snapshots. Output: qualified operational identities, lineage and resolution/coverage results. No control-plane, operational procedure or observation schedule is activated.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestOperationalSubjectRequirementFlow$" ./operations/ > "$check_output" && grep -F -- "--- PASS: TestOperationalSubjectRequirementFlow " "$check_output")` | exit 0; named PASS; two requirements reference one journey via /22 records with one operational definition; mutation: duplicate the journey per REQ |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestOperationalSubjectLineage$" ./operations/ > "$check_output" && grep -F -- "--- PASS: TestOperationalSubjectLineage " "$check_output")` | exit 0; named PASS; rename/retirement preserves references and rejects wrong-kind or duplicate identities; mutation: reuse a retired identity |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestOperationalSubjectVisibility$" ./operations/ > "$check_output" && grep -F -- "--- PASS: TestOperationalSubjectVisibility " "$check_output")` | exit 0; named PASS; denied source yields constant unavailable data without hidden counts; mutation: aggregate before source filtering |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
