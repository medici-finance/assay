---
brief: assay:assay:graph-execution:09
title: Versioned workflow instances and shared identity
why: An approved pattern is not yet an executable contract for a particular change. Binding its subject, Cell and acceptance prevents a retry from silently using new inputs or a different permission set.
wave: 1
depends:
- graph-execution/02
unblocks:
- graph-execution/14
- graph-execution/16
- graph-execution/19
- graph-execution/20
effort: M
gate: model
risk:
  regulatory: no
  customer: no
  irreversible: no
  sensitive-data: no
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/work-input-amendment.md
- freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3 (pending scope, not implementation)
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ a944ad1103aadaba919c11fe425089057f5c2f4e
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'statusgen/experiment.go: follow-up graph-execution/05'
- 'drainloop: follow-up graph-execution/16'
- 'tools/desk: follow-up graph-execution/14'
- 'workflow/store and bindings: follow-up graph-execution/19'
- 'workflow/runner: follow-up graph-execution/20'
version: 3
id: eb0a9f7c-669d-42f6-9686-e30c661a2450
---

# Brief 09 — Versioned workflow instances and shared identity

## Context

files: `spec/workflow-instance-v1.md` (planned), `schemas/workflow-instance-v1.json` (planned), `statusgen/instance.go` (planned), `statusgen/instance_test.go` (planned), `statusgen/main.go`, `statusgen/testdata/instances/` (planned), `docs/lifecycle.md`, `changelog/graph-execution-09-instance-contract.md` (planned)

facts: Pattern v1 explicitly excludes instantiation; keep its implemented schema intact until a compatible extension is reviewed. Canonical work IDs already exist; instance/node/attempt IDs are additional identities.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Work-input amendment — 2026-09-30

Implement WI-1/WI-2 in the instance schema and validator already owned here. The packet is
an exported role view of the canonical instance, never a new run record. Bind intent,
acceptance scope, source/dependency fingerprints, coverage/omissions, unresolved inputs,
artifact references, delta and next safe action. Keep runner/control/reservation references
without inventing authority. A complete diff may be large; reject silent truncation.

Add positive and negative fixtures in the declared instance testdata: same-input round trip,
changed policy outside the touched source file, omitted mandatory evidence and unknown
coverage. Raw artifacts remain accessible. Serialize result/handoff provenance using the
same IDs. Actual prompt assembly/dispatch belongs to 14; usage settlement belongs to 16.
No semantic dependency inference or provider-specific model client is in scope.

## Task workflow amendment — 2026-10-02

Extend the existing instance contract with backend-assignment/generation and runner-profile references, plus a versioned execution-state projection. IDs and acceptance remain canonical. 19 owns persisted instances and the real coverage-binding adapter; a declared instance with a missing binding cannot degrade to no-pattern behavior. Keep packet schemas provider-neutral.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Task

1. Define and validate the GEA-01 instance fields, pinned input/acceptance/policy references and supersession. Include export/import and old-reader capability refusal; no in-place rewrite of prior decisions.
2. Add statusgen instance validate --file PATH --json. Reuse pattern lint and eligibility. Instance effects must be a subset of both pattern declarations and resolved actor capabilities; an instance never mints credentials.
3. Publish fixtures for immutable identity, stale acceptance, unknown mandatory field and cross-Cell substitution. Document that human decisions are external authenticated acts, not a new human token-bearing model role.

## Interface contract

Instantiator → validator → serialized instance preserves exact references; tampering is rejected.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstance" ./...` | exit 0; output includes PASS for TestInstance, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceCrossCellDenied" ./...` | exit 0; output includes PASS for TestInstanceCrossCellDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestInstanceRoundTripIdentity" ./...` | exit 0; output includes PASS for TestInstanceRoundTripIdentity, with no [no tests to run] for its owning package |
| 4 | check:ci +flow | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketViewProvenance.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketViewProvenance$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketViewProvenance " "$wi_out")` | exit 0; named PASS; canonical instance → role view → handoff preserves revision, scope and omissions |
| 5 | check:ci +mutation | `(cd statusgen && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestInstancePacketChangedDependency.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstancePacketChangedDependency$" ./... > "$wi_out" && grep -q -- "--- PASS: TestInstancePacketChangedDependency " "$wi_out")` | exit 0; named PASS; changed dependency or omitted required evidence cannot masquerade as the previous current packet; mutation: accept a stale dependency fingerprint — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd statusgen && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInstanceBackendRoundTrip$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInstanceBackendRoundTrip " "$routing_out")` | exit 0; named PASS; export/import preserves identity, assignment and acceptance; substitution is rejected |


The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Review the packet fixtures for credential exclusion and trust provenance: a synthetic secret
must be rejected or replaced with an authorized reference, and a reporter-supplied trust
label must not become an authoritative disposition through export/import.

Gate: model. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
