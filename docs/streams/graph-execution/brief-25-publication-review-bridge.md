---
brief: assay:assay:graph-execution:25
title: Exact-candidate publication and independent review evidence bridge
why: Internal review is useful only if the published commit is the one reviewed and its evidence can cross the existing
  forge gate without granting the worker approval authority.
wave: 8
depends:
- graph-execution/24
- graph-execution/04
- graph-execution/14
- graph-execution/16
unblocks:
- graph-execution/18
effort: M
gate: model
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'no'
  sensitive-data: 'no'
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'adopting reviewer executor: out-of-scope (owner-approved activation consumes this report-only contract)'
- 'statusgen/assuranceexperiment.go: follow-up graph-execution/18'
version: 1
id: 57b837cb-e6dd-495f-9349-8addeee220ad
---

# Brief 25 — Exact-candidate publication and independent review evidence bridge

## Context

files: `workflow/publication/` (planned), `workflow/testdata/publication/` (planned), `spec/workflow-publication-v1.md` (planned), `workflow/README.md` (planned), `docs/enforcement-model.md`, `changelog/graph-execution-25-publication-review-bridge.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the reviewer-executor subject check (independent actor, exact current head, current policy, complete evidence) — behind it, the publishing profile ships disabled and only a recorded human decision activates it, and the forge's own review and human merge gates are unchanged.

risk-answers: all `no` because the bridge ships report-only, the publishing profile ships disabled, and no live review policy, ready flip or merge is in scope. The publishing profile can request a reviewer effect, so the implementing change needs an independent security review before it is marked ready.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Task

1. Publish the exact frozen candidate through an injected narrow worker effect port. Record durable intent and authoritative receipt, reconcile unknown and partial outcomes, and revalidate head/base/acceptance/policy at consequential boundaries. Never publish unrelated dirty state.
2. Default the bridge to report-only: attach inspectable provenance/finding summary and keep the normal independent forge review. A separately activated profile may ask the reviewer executor to publish a qualified independent verdict at the exact current head. That profile ships disabled. Activating it is a recorded human decision held in operator configuration outside the controller and worker domains; no controller, worker or task input can switch it. Worker/controller cannot choose reviewer identity, grant itself permission or mark ready.
3. Validate independent actor/capability, exact subject, complete evidence, current policy and required checks before requesting the reviewer effect. A moved target, stale or forged verdict, missing evidence or unsupported atomic binding requires fresh review/hold. Post-merge verification remains separate.
4. Ship the publishing profile disabled and test that controller, worker or task input cannot enable it. Use separately enforcing fake worker/reviewer providers; bypass the controller in negative tests. Exercise compound verdict partial success, lost receipt and resumed publication. Publish profile activation criteria and explicit provider limitations. No live review-policy activation, ready flip or merge here.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPublicationExactCandidateAndReceipt$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPublicationExactCandidateAndReceipt " "$routing_out")` | exit 0; named PASS; export preserves exact reviewed commit; lost acknowledgment reconciles without a second write; mutation: publish the workspace head instead of the reviewed commit — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPublicationForgedOrStaleReviewDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPublicationForgedOrStaleReviewDenied " "$routing_out")` | exit 0; named PASS; implementer impersonation, changed policy or moved head cannot approve; mutation: skip the head comparison on the review subject — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPublicationPartialAndReportOnly$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPublicationPartialAndReportOnly " "$routing_out")` | exit 0; named PASS; partial outcome holds; default report-only never emits a reviewer/ready effect; mutation: default the bridge to the publishing profile — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPublicationActivationDisabledByDefault$" ./... > "$routing_out" && grep -q -- "--- PASS: TestPublicationActivationDisabledByDefault " "$routing_out")` | exit 0; named PASS; the publishing profile is disabled in the shipped default, and controller, worker or task input that tries to enable it is refused with no reviewer effect emitted; mutation: read the activation flag from the controller's run input — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
