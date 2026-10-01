---
brief: assay:assay:graph-execution:19
title: Durable instance store and production coverage binding
why: A task must survive its parent process without losing intent, artifacts or the evidence obligations that make
  it acceptable.
wave: 2
depends:
- graph-execution/09
- graph-execution/03
unblocks:
- graph-execution/23
- graph-execution/20
- graph-execution/21
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
- freshness-checked 2026-10-02 @ a944ad1103aadaba919c11fe425089057f5c2f4e
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'workflow/controller: follow-up graph-execution/21'
- 'statusgen/coverage.go: fixed-here'
- 'workflow/workspace: follow-up graph-execution/23'
version: 1
id: ead4e747-4d61-4ec6-abaf-332873c79b21
---

# Brief 19 — Durable instance store and production coverage binding

## Context

files: `workflow/go.mod` (planned), `workflow/README.md` (planned), `docs/lifecycle.md` (planned), `workflow/store/` (planned), `workflow/bindings/` (planned), `workflow/testdata/store/` (planned), `statusgen/graphcontract/` (planned), `statusgen/coverage.go` (planned), `statusgen/instance.go` (planned), `changelog/graph-execution-19-durable-instance-store.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: controller correctness alone is insufficient — independently enforcing effect/role boundaries and fixture oracles must still reject a bypass.

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

1. Create the workflow Go module and a single-controller SQLite store with transactions, versioned migrations and content-addressed artifact references. Select and pin a maintained driver; document supported filesystem/durability assumptions. Persist canonical instance/node/attempt IDs, revisions, wait/stop state and artifact manifests. No second work identity or claim authority.
2. Implement expected-version/generation transitions, complete export/import and restore validation. Reject missing artifacts, unsupported mandatory schema and stale ownership; never acknowledge a transition before durability. Restore is passive until the existing ownership authority fences the previous owner.
3. Connect real instance-to-pattern-node bindings to coverage. Extract/export the existing pure validator/evaluator contract only where needed under statusgen/graphcontract; update existing callers without changing their semantics. Do not copy coverage logic into workflow. No binding found for a declared instance means held, not an unpatterned success.
4. Add restart, version-conflict, missing-artifact and stale-acceptance flow fixtures. Wire cross-module CI paths for statusgen and workflow so consumer tests run on changes in either. Update lifecycle/storage documentation and the changelog.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestWorkflowStoreRestartBinding$" ./... > "$routing_out" && grep -q -- "--- PASS: TestWorkflowStoreRestartBinding " "$routing_out")` | exit 0; named PASS; restart preserves work and all pattern obligations; missing binding holds |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestWorkflowStoreIncompleteRestoreDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestWorkflowStoreIncompleteRestoreDenied " "$routing_out")` | exit 0; named PASS; missing artifact or tampered acceptance refuses restore; mutation: skip manifest validation makes test fail |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestWorkflowStoreStaleTransitionDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestWorkflowStoreStaleTransitionDenied " "$routing_out")` | exit 0; named PASS; two expected-version transitions cannot both advance |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Fail a deliberate mutation of the named control. Facts/source revisions, declared files, risks, consumers and sizing were checked at authoring; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
