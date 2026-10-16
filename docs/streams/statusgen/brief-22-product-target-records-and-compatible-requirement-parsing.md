---
brief: assay:assay:statusgen:22
title: Product target records and compatible requirement parsing
why: "A product target currently disappears into prose when a brief closes. Preserve direction, target revisions and observed evidence without changing existing requirement meaning."
wave: 1
depends: []
unblocks: ["statusgen/23", "statusgen/24", "continuing-operations/01"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: 2951f3ce-01cc-46a6-a456-9e6c32e1fa56
consumers:
  - "statusgen/requirements.go, spec/registers-v1.md, schemas/product-intent-v1.json: follow-up statusgen/22"
  - "statusgen/contexthandoff: follow-up statusgen/23"
  - "statusgen/portfolio: follow-up statusgen/24"
  - "statusgen/operations: follow-up continuing-operations/01"
---

# Brief 22 — Product target records and compatible requirement parsing

## Context

files: `spec/registers-v1.md`, `schemas/product-intent-v1.json` (planned), `statusgen/productintent/` (planned), `statusgen/requirements.go`, `statusgen/requirements_test.go`, `docs/contracts.md`; `changelog/statusgen-22-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- The existing requirementEntry stores acceptance/status/satisfied-by, not revisioned product targets.
- REQ IDs and the outcome grammar already exist; satisfied is a claim, not an observed benefit.
- The context handoff implementation is separately owned by statusgen/19; this brief does not edit it.

layering: pure typed records/evaluators in `statusgen/productintent`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/productintent
  contract: product-intent-v1 (planned extension; register its semantic owner in docs/contracts.md)
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

1. Implement the optional product-intent-v1 extension in the REQUIREMENTS owner with direction, target and observed references as specified in the amendment. Retain unique revision history, tombstones and original acceptance/satisfaction claims. Resolve approval from supplied decision evidence, never a record boolean.
2. Provide pure decode/validate/resolve APIs in statusgen/productintent; schema and parser agree on supported versions, required fields and legacy absence. Record unsupported extension versions explicitly. Add typed qualified operational references without implementing operational definitions here.
3. Connect the existing requirement reader without silently accepting unsupported fields as working behavior. Publish the exact supported syntax, compatibility matrix and semantic owner; old records remain valid and unbound.

## Interface contract

Input: base REQ plus optional versioned product-intent extension and explicit decision/assessment references. Output: immutable qualified revisions and known/unknown/unbound states. No observation or approval is manufactured by parsing.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentLegacyAndRevisionFlow$" . > "$check_output" && grep -F -- "--- PASS: TestProductIntentLegacyAndRevisionFlow " "$check_output")` | exit 0; named PASS; legacy REQ parsing and new extension resolve through the actual reader; two target revisions keep their original direction and evidence; mutation: overwrite prior target history |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentApprovalAndUnknown$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentApprovalAndUnknown " "$check_output")` | exit 0; named PASS; a self-declared approval, unavailable decision or unsupported mandatory version cannot become an approved target; mutation: trust the supplied approved flag |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestProductIntentSchemaParity$" ./productintent/ > "$check_output" && grep -F -- "--- PASS: TestProductIntentSchemaParity " "$check_output")` | exit 0; named PASS; same valid/invalid corpus through JSON schema and parser; include wrong-kind operational reference and withdrawn identity; mutation: permit missing target revision |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
