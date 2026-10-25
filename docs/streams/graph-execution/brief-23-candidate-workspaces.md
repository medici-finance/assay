---
brief: assay:assay:graph-execution:23
title: Immutable candidate workspaces and independent check inputs
why: Review cannot establish a result if the implementer can alter the files while the reviewer reads them.
wave: 7
depends:
- graph-execution/19
- graph-execution/20
- graph-execution/28
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
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — non-CI architecture reconciliation 2026-10-13 (R1 owner row)
- freshness-checked 2026-10-13 @ a435badbd (origin/main)
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'workflow/internalreview: follow-up graph-execution/24'
- 'workflow/publication: follow-up graph-execution/25'
version: 2
id: f1533c82-d868-4189-a19a-9fdd53369cac
---

# Brief 23 — Immutable candidate workspaces and independent check inputs

## Context

files: `workflow/workspace/` (planned), `workflow/testdata/workspace/` (planned), `spec/workflow-candidate-v1.md` (planned), `workflow/README.md` (planned), `changelog/graph-execution-23-candidate-workspaces.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the candidate freeze (exact commit/tree plus manifest hash) — behind it, the reviewer's own snapshot verification, which rejects a verdict whose subject hash differs.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Non-CI architecture reconciliation — 2026-10-13

The maintainer ratified the non-CI verifier target (medici-finance/assay#2500, revision
23f655f5d); approval activates nothing. The candidate reference and its acceptance binding are
now defined once, by graph-execution/28 (`verification-domain-v1`). This brief consumes those
types: Task 1's candidate reference imports them, and `spec/workflow-candidate-v1.md` (planned), if still
needed for workspace-specific fields, references `verification-domain-v1` instead of defining a
second candidate record. Hence the added `depends: graph-execution/28`. The workspace, freeze,
repair and restore obligations, and the Task and Verify sections below, are unchanged by this
amendment. See `docs/proposals/non-ci-architecture/20-amendment-manifest.md`.

Review follow-up, 2026-10-13: Task 1 now carries a one-clause pointer to graph-execution/28's
candidate and acceptance-binding types, so a literal reader of Task 1 does not mint a second
candidate record (review advisory A3 on medici-finance/assay#2739). No other Task or Verify text
changed.

## Consumer integration amendment — 2026-10-23

The 2026-10-13 unchanged-section statements above describe that earlier amendment only. Version 2 now strengthens Tasks 1 and 3 and the Expect cells of Verify rows 1–3 with actual shared candidate and acceptance-binding flows, exact frozen subjects, restored generation checks, positive baselines and independent mutation controls. Dependencies and historical Evidence remain unchanged. See [the acceptance and consumer integration manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md#contract-acceptance-and-consumer-integration--23-october-2026). These are prospective obligations, not passing results.

## Task

1. Define candidate references binding repository, base/candidate commit and tree, acceptance version and artifact manifest by importing graph-execution/28's candidate and acceptance-binding types (`verification-domain-v1`), never a second candidate record. Construct and validate those actual shared types at freeze, reviewer-input and restore boundaries; a serialized lookalike or owning-brief corroboration is not contract use. Consume an injected workspace/effect port; the profile supplies the existing git library/role executor implementation. Do not vendor a second git/claim implementation.
2. Allow one implementation writer per mutable attempt. Freeze a candidate before checks/review; provide independent immutable/read-only snapshots. Dirty or incomplete snapshots cannot be candidates. Parallel checks may read the same frozen input; concurrent code writers are excluded from this profile.
3. Repairs produce a new candidate identity and leave prior evidence immutable. Dependency/acceptance changes invalidate affected claims; file non-overlap does not establish applicability. Preserve evidence artifacts until retention policy permits deletion. Restored references retain the exact shared acceptance version/digest and canonical generation; repairs bind their new candidate explicitly and never rebind old evidence to it or renew stale observations.
4. Test reviewer input while implementation changes, foreign-repository candidate, artifact tampering and restored reference integrity through real workspace port fixtures. Update lifecycle docs and module/path CI.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCandidateImmutableDuringRepair$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCandidateImmutableDuringRepair " "$routing_out")` | exit 0; named PASS; the real workspace port constructs /28 shared candidate/acceptance types; independent reviewer input keeps exact frozen A while repair creates B, including on the same branch; A's binding and verdict cannot attach to B, while valid A remains readable; mutation controls, each planted independently: let repair write into frozen A; compare branch names instead of the shared exact subject — each must make the named test fail, while the healthy positive baseline passes |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCandidateDirtyOrForeignDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCandidateDirtyOrForeignDenied " "$routing_out")` | exit 0; named PASS; a valid clean snapshot retains its shared repository/base commit/base tree/commit/tree/manifest binding; a dirty tree, substituted repository or mismatching observed tree is rejected through the real workspace port; mutation controls, each planted independently: skip the clean-tree check; omit repository comparison; omit observed-tree comparison when freezing a candidate — each must make the named test fail, while the healthy positive baseline passes |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCandidateRestoreManifest$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCandidateRestoreManifest " "$routing_out")` | exit 0; named PASS; valid restore retains the exact shared candidate/acceptance version and digest with its canonical generation; a missing/mutated artifact, changed acceptance binding or stale generation cannot restore as valid or attach old evidence; mutation controls, each planted independently: skip artifact hash verification; skip shared acceptance-binding validation; skip generation validation on restore — each must make the named test fail, while the healthy positive baseline passes |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
