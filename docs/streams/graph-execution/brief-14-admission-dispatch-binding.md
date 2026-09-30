---
brief: assay:assay:graph-execution:14
title: Bind admission and graph eligibility at the dispatch boundary
why: A correct policy is ineffective if one dispatcher can bypass it or use a stale assessment. Every admitted attempt needs the same readiness decision and an enforceable runtime binding.
wave: 3
depends:
- graph-execution/01
- graph-execution/09
- graph-execution/13
- graph-execution/16
unblocks:
- graph-execution/18
effort: M
gate: human
risk:
  regulatory: no
  customer: no
  irreversible: yes
  sensitive-data: no
gate-why: Dispatch affects external writes; the owner confirms that the selected cohort cannot bypass admission or widen existing authority.
decision-trigger: spec
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/work-input-amendment.md
- freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3 (pending scope, not implementation)
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'tools/desk/cmd/deskdispatch: fixed-here'
- 'tools/desk/internal/loopengine: fixed-here'
- 'operator clients: out-of-scope (consume records through their own authorized APIs)'
version: 2
id: 00bb0ac0-ea54-437e-9274-629839c91ada
---

# Brief 14 — Bind admission and graph eligibility at the dispatch boundary

## Context

files: `tools/desk/internal/deskkit/graphadmission.go` (planned), `tools/desk/internal/deskkit/graphadmission_test.go` (planned), `tools/desk/internal/loopengine/`, `tools/desk/cmd/deskdispatch/`, `docs/enforcement-model.md`, `changelog/graph-execution-14-admission-dispatch-binding.md` (planned)

facts: Use the existing executor and graph evaluator; do not create a second scheduler. 16 supplies enforceable ownership/budget reservations; 09 supplies subject-bound instances.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. At implementation pickup, prepare concrete policy choices and negative-path evidence, then file a self-contained decision issue. No response permits no activation; schema/test work may proceed within the declared scope.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Work-input amendment — 2026-09-30

Implement WI-1/WI-2/WI-3 in the existing opt-in cohort only. Resolve known facts once, project
09's contract into the role packet, and record a dispatch reason and input fingerprint.
Coalesce duplicate events/ticks under the existing claim authority; no actionable change
means no model call. A failed launch can retry as a new attempt without losing the work.
The deterministic parent owns waiting; productive task-agent continuity is allowed.

Recheck instance/acceptance/head before dispatch and before accepting a result. The latter
must be atomic or fenced at the authoritative transition, not a read followed by an
unconditional write. If it moved, preserve artifacts and queue revalidation; never carry a
stale PASS or replay an unknown effect. Preserve independent review/verification and the
per-role capability boundary. Prompt/context/model profiles are explicit inputs, not a
second scheduler or an automatic provider switch. Extend the declared graphadmission tests;
retain this brief's human activation gate.

## Task

1. Add an opt-in cohort binding that consumes the existing eligibility verdict plus 13 admission and 16 ownership/budget reservation before dispatch. CLI and loop callers use the same implementation; defaults remain current behavior outside the cohort.
2. Recheck stale subject, policy, stop/lease and revoked capability at consequential boundaries. Persist both prediction and policy references, not just the selected lane. A unavailable mandatory check holds and reports its source age.
3. Use a fake provider/effect boundary to prove direct invocation without admission is denied independently of the UI. Keep cross-Cell receive authorization independent. No new permission, production adapter or real environment operation is introduced.

## Interface contract

Instance → eligibility/admission → reservation → dispatch receipt uses one policy path and denies stale or bypassed calls.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestGraphAdmission" ./...` | exit 0; output includes PASS for TestGraphAdmission, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionDirectBypassDenied" ./...` | exit 0; output includes PASS for TestGraphAdmissionDirectBypassDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionDispatchReceipt" ./...` | exit 0; output includes PASS for TestGraphAdmissionDispatchReceipt, with no [no tests to run] for its owning package |
| 4 | check:ci +flow | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionDuplicateAndRetry$" ./... > /tmp/TestGraphAdmissionDuplicateAndRetry.out && grep -q -- "--- PASS: TestGraphAdmissionDuplicateAndRetry " /tmp/TestGraphAdmissionDuplicateAndRetry.out` | exit 0; named PASS; duplicate same-state events launch once; a failed launch retries under a new attempt |
| 5 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionMovedHeadAtAcceptance$" ./... > /tmp/TestGraphAdmissionMovedHeadAtAcceptance.out && grep -q -- "--- PASS: TestGraphAdmissionMovedHeadAtAcceptance " /tmp/TestGraphAdmissionMovedHeadAtAcceptance.out` | exit 0; named PASS; head movement after launch prevents stale result acceptance at the lower boundary; mutation: bypass the acceptance-boundary head comparison — the named test must fail |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
