---
brief: assay:assay:graph-execution:23
title: Immutable candidate workspaces and independent check inputs
why: Review cannot establish a result if the implementer can alter the files while the reviewer reads them.
wave: 3
depends:
- graph-execution/19
- graph-execution/20
unblocks:
- graph-execution/24
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
- 'workflow/internalreview: follow-up graph-execution/24'
- 'workflow/publication: follow-up graph-execution/25'
version: 1
id: f1533c82-d868-4189-a19a-9fdd53369cac
---

# Brief 23 — Immutable candidate workspaces and independent check inputs

## Context

files: `workflow/workspace/` (planned), `workflow/testdata/workspace/` (planned), `spec/workflow-candidate-v1.md` (planned), `workflow/README.md` (planned), `changelog/graph-execution-23-candidate-workspaces.md` (planned).

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

1. Define candidate references binding repository, base/candidate commit and tree, acceptance version and artifact manifest. Consume an injected workspace/effect port; the profile supplies the existing git library/role executor implementation. Do not vendor a second git/claim implementation.
2. Allow one implementation writer per mutable attempt. Freeze a candidate before checks/review; provide independent immutable/read-only snapshots. Dirty or incomplete snapshots cannot be candidates. Parallel checks may read the same frozen input; concurrent code writers are excluded from this profile.
3. Repairs produce a new candidate identity and leave prior evidence immutable. Dependency/acceptance changes invalidate affected claims; file non-overlap does not establish applicability. Preserve evidence artifacts until retention policy permits deletion.
4. Test reviewer input while implementation changes, foreign-repository candidate, artifact tampering and restored reference integrity through real workspace port fixtures. Update lifecycle docs and module/path CI.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCandidateImmutableDuringRepair$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCandidateImmutableDuringRepair " "$routing_out")` | exit 0; named PASS; reviewer keeps frozen A while repair creates B; verdict on A cannot attach to B |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCandidateDirtyOrForeignDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCandidateDirtyOrForeignDenied " "$routing_out")` | exit 0; named PASS; dirty tree or substituted repository is rejected |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCandidateRestoreManifest$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCandidateRestoreManifest " "$routing_out")` | exit 0; named PASS; missing or mutated candidate artifact cannot be restored as valid |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Fail a deliberate mutation of the named control. Facts/source revisions, declared files, risks, consumers and sizing were checked at authoring; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
