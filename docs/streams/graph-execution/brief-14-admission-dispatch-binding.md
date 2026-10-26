---
brief: assay:assay:graph-execution:14
title: Bind admission and graph eligibility at the dispatch boundary
why: A correct policy is ineffective if one dispatcher can bypass it or use a stale assessment. Every admitted attempt needs the same readiness decision and an enforceable runtime binding.
wave: 6
depends:
- graph-execution/01
- graph-execution/09
- graph-execution/13
- graph-execution/16
- graph-execution/30
- graph-execution/31
- graph-execution/33
unblocks:
- graph-execution/18
- graph-execution/21
- graph-execution/25
- performance-collection/09
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
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md — 2026-10-14 consumer amendment
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'tools/desk/cmd/deskdispatch: fixed-here'
- 'tools/desk/internal/loopengine: fixed-here'
- 'operator clients: out-of-scope (consume records through their own authorized APIs)'
- 'workflow/controller: follow-up graph-execution/21'
version: 5
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

## Task workflow amendment — 2026-10-02

Expose one callable admission/dispatch seam to existing desk callers and the new host. Assignment names a backend, work/node, fingerprint, lease and generation at the existing authority. Only the assigned backend launches; queue exclusion is not the security boundary. Recheck current assignment and capabilities at effect/result admission; no all-role broker.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Public runtime context/provenance amendment — 2026-10-14

Use /30's complete evaluation context and /31's corroborated accountable-principal/proof
records before dispatch and at acceptance/effect admission. Keep the existing evaluator,
lease/incarnation and role controls. Obtain current time/access/ownership from trusted
composition; a cached eligible result or model-supplied actor cannot authorize work.
Expired bounds, changed membership/policy, incomplete context and missing independence
corroboration hold the affected action. Offline results are snapshot-bound advice.

Add `TestAdmissionContext` and `TestAdmissionDuty` to the planned graphadmission test
surface (the file is itself new in this brief): the former changes time/access without changing source hashes; the latter supplies
different keys for one responsible principal and bypasses the model-facing check to exercise
trusted provenance selection. Both must refuse the affected admission. Keep a permitted
positive case. No stronger custody claim or one-human waiver is added. Existing human
activation gate and historical Evidence remain unchanged.

## Human-floor amendment — 2026-10-26

graph-execution/33 makes the implementation pattern's `merge` node a `human` decision and leaves the
merge act itself with no verified control until this brief binds admission at dispatch. That layer is
owned here. Admission refuses by operation name, and an assessment's operations are supplied by the
caller (`AgenticAssessment.Operations`, `tools/desk/internal/deskkit/admission.go`). Today a node's
operation is its effect kind, so an agent node shaped `{kind: dispatch, target: merge-commit}` would
reach admission as `dispatch` and the human-floor refusal would never engage. This brief therefore
derives the admission operations of a node from its floor artifact, not from its effect kind or label:
any node output or effect target in graph-execution/33's closed floor-artifact list adds the matching
floor operation (`merge-commit` -> `merge`, `release` -> `release`, `deployment` -> `deploy`, the same
one-to-one list that brief fixes against `humanFloorOperations`), whatever the effect is declared as.
Add `TestGraphAdmissionHumanFloorMerge` (planned) to
`tools/desk/internal/deskkit/graphadmission_test.go` (planned). It feeds admission an agent node shaped
`{kind: dispatch, target: merge-commit}` in a category whose permitted operations include `dispatch`,
with the pattern lint and instance validator bypassed. It asserts the node is not admitted to the agent at any
disposition (reason `human-floor:merge`), and that a well-formed `decisions[]` record naming that merge
does not authorize the agent. Because the category permits `dispatch`, removing the derivation lets
the node through as a permitted `dispatch`; the unknown-operation path cannot mask that. Verify
row 9 holds it.
Scope: admission sees only the operations requested through dispatch or routed through this brief's
effect-admission recheck. A merge an agent performs out of band with a forge credential it holds is
bounded only by that credential's scope, which graph-execution/33 records as unverified.

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
| 4 | check:ci +flow | `(cd tools/desk && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestGraphAdmissionDuplicateAndRetry.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionDuplicateAndRetry$" ./... > "$wi_out" && grep -q -- "--- PASS: TestGraphAdmissionDuplicateAndRetry " "$wi_out")` | exit 0; named PASS; duplicate same-state events launch once; a failed launch retries under a new attempt |
| 5 | check:ci +mutation | `(cd tools/desk && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestGraphAdmissionMovedHeadAtAcceptance.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionMovedHeadAtAcceptance$" ./... > "$wi_out" && grep -q -- "--- PASS: TestGraphAdmissionMovedHeadAtAcceptance " "$wi_out")` | exit 0; named PASS; head movement after launch prevents stale result acceptance at the lower boundary; mutation: bypass the acceptance-boundary head comparison — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionDeskProgramRace$" ./... > "$routing_out" && grep -q -- "--- PASS: TestGraphAdmissionDeskProgramRace " "$routing_out")` | exit 0; named PASS; concurrent desk/program claims permit one owner and one launch; direct stale invocation is denied; mutation: skip the claim-generation compare on the program path — the named test must fail |
| 7 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAdmissionContext$" ./internal/deskkit/ > "$out" && grep -F -- "--- PASS: TestAdmissionContext " "$out")` | exit 0; named PASS; expired or incomplete evaluation context holds, while a current authorized context permits the existing admission path; mutation: accept a cached eligibility flag |
| 8 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAdmissionDuty$" ./internal/deskkit/ > "$out" && grep -F -- "--- PASS: TestAdmissionDuty " "$out")` | exit 0; named PASS; same principal under different keys and uncorroborated caller provenance cannot satisfy independent evidence; mutation: compare only role labels |
| 9 | check:ci +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestGraphAdmissionHumanFloorMerge$" ./internal/deskkit/ > "$out" && grep -F -- "--- PASS: TestGraphAdmissionHumanFloorMerge " "$out")` | exit 0; named PASS; an agent node `{kind: dispatch, target: merge-commit}` in a category that permits `dispatch`, fed to admission with the pattern lint and instance validator bypassed, is not admitted to the agent at any disposition (reason `human-floor:merge`), and a recorded decision does not authorize it; mutation: remove the floor-artifact derivation so the node's operation is its effect kind `dispatch` — the named test must fail |


The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Review the assembled-packet fixture at the dispatch boundary: credential material must
remain excluded, and copied reporter text must not override the source trust-gate result.
Exercise missing and spoofed dispositions; neither may grant authority.

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
