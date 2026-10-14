---
brief: assay:assay:forge-neutral:44
title: "Shared Git object and fetch reuse qualification trial"
why: "Shared Git objects and fetches could reduce repeated work across agents, but a new daemon would also add cache, permission and maintenance duties. A bounded comparison of our existing common-store/worktree paths can establish whether another mechanism earns its cost."
wave: 8
depends: ["forge-neutral/42"]
unblocks: ["forge-neutral/45"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Choose whether the measured common-store and fetch-reuse trial justifies a separately scoped production change. The valid outcomes are reuse the current path, author a bounded improvement, or retain uncertainty. No daemon or live fetch is activated by this experiment."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 1
id: 1cb50a0e-129b-4009-a1e1-bf9fe498f765
authored: 2026-10-14 by forge-service authoring
sources:
  - https://github.com/medici-finance/assay/blob/11495ed016574a24ea89481d0bb8ba2dab3d2aac/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/forge-neutral/README.md
  - freshness-checked 2026-10-14 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
  - R6 proposal inspected at c5e8a3f8da3c715b72efa8d8d6b5e74e659ebd52; main merged into this authoring branch without editing historical Evidence
exec-tier: strong
exec-tier-why: Cross-component identity, privacy, concurrency and failure semantics require design judgment and negative controls.
domain: complicated
consumers:
  - "tools/desk/internal/gitcore/sharedreuse_test.go: follow-up forge-neutral/44"
  - "tools/desk/internal/gitcore/testdata/sharedreuse/: follow-up forge-neutral/44"
  - "docs/research/2026-10-14-scion-git-reuse/README.md: follow-up forge-neutral/44"
  - "docs/cellctl-maintenance.md: follow-up forge-neutral/44"
  - "changelog/scion-git-reuse-trial.md: follow-up forge-neutral/44"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
---

# Brief 44 — Shared Git object and fetch reuse qualification trial

## Context

files: `tools/desk/internal/gitcore/sharedreuse_test.go` (planned), `tools/desk/internal/gitcore/testdata/sharedreuse/` (planned), `docs/research/2026-10-14-scion-git-reuse/README.md` (planned), `docs/cellctl-maintenance.md`, `changelog/scion-git-reuse-trial.md` (planned). Paths not present at the source pin are planned deliverables; existing
files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- gitcore already has alternate readers and object-store helpers; worktree allocation/retirement and maintenance have existing owners.
- 42 supplies the source-query correctness and measurement boundary. This trial compares implementations at pinned revisions rather than inventing a version-control backend.
- Scion scale is not our measured workload. CPU/latency and object reuse for this fleet have not been established.

layering: existing gitcore and common Git-store maintenance/retirement owners. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: performance evidence that silently skips retention or access behavior; independent exact-source comparison and denied-source/owner-loss cases constrain the trial.

design-fit:
  owner: existing gitcore and common Git-store maintenance/retirement owners
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs 0; flags 0; refusals 0; rule-text +0; no production daemon
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/11495ed016574a24ea89481d0bb8ba2dab3d2aac/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Choose whether the measured common-store and fetch-reuse trial justifies a separately scoped production change. The valid outcomes are reuse the current path, author a bounded improvement, or retain uncertainty. No daemon or live fetch is activated by this experiment.

Options:
1. **Accept the demonstrated bounded design** — accept only the proven scope and failure behavior; keep live credential provisioning and activation as separate operator acts.
2. **Hold for correction** — identify the unproven boundary or budget and revise before acceptance.

Default if no answer: none — acceptance remains pending. This implementation-evidence decision
does not prevent authoring or independent offline fixture work. Existing mirror retention and
webhook payload decisions are not answered by this section.

## Ground rules

- Isolate changes; never push main, merge, ready-flip, release, dispatch workflows or contact live infrastructure.
- Use synthetic data, disposable processes/repositories and explicit local database fixtures; no production tokens or data.
- Preserve role, candidate, unknown-outcome, required audit/budget and canonical acceptance meanings.
- Stop at implemented; independent verification and acceptance remain separate.
- Do not widen an unsupported provider capability or duplicate another store/evaluator to make a test pass.

## Task

1. Build a reproducible experiment using disposable local bare remotes and worktrees, frozen repository seeds and named baseline/subject revisions. Compare current per-operation reads, the existing common-store/isolated-worktree path and coalesced/serialized local-remote fetch. Use existing Git tools/readers; do not introduce a shipped daemon, new global configuration or live network fetch.

2. Vary cold/warm cache, repo size/history, ref/pack count, concurrent clients, sparse work and retained evidence. Exercise native Git and go-git alternate readers. Keep exact FileAt/Diff/MergeBase answers and permitted access identical; source permission and object membership precede reuse.

3. Test object-store owner exit, interrupted fetch, missing alternate, concurrent maintenance, disk pressure and required evidence retention. A disposable workspace retiring cannot invalidate another active reader or delete retained objects. Reuse the existing one-maintenance-owner contract; no age/prefix-based cleanup shortcuts.

4. Declare the experimental workload and decision threshold before sampling. Record p50/p95/p99 latency, fetched bytes, provider-equivalent request count, CPU, object opens, cache reuse, temporary/retained disk, failure recovery and maintenance work. Keep raw samples and platform/tool versions; do not infer a universal performance win from a warm trivial fixture.

5. Write the public report with correctness results and cost/operability tradeoffs. A no-go/current-path result completes the trial. If an improvement is warranted, propose its owner, migration, custody and acceptance budget for a separate brief; do not ship it under this investigation. Detached review heads remain under existing candidate/workspace authority.

## Interface contract

Use forge-service-v1 from 39, mirror admission/freshness from 37, event hints/demand from 38,
R2/R4 logical effect identity and current R2/R3 authority. Cache/observation tokens never grant
permission. Preserve requester, represented actor, provider executor and exact source inputs.
Provider outcome and local projection availability are separate. Unsupported, stale, incomplete
and permission-denied are explicit; loss does not become success, an empty queue or fresh authority.

## Verify

Named tests below are implementation deliverables, not tests already run. Each command requires
the named PASS line, so a missing test or skipped database case fails. Database tests must use
the inherited graph-execution/19 explicitly disposable PostgreSQL harness and fail when it is
unavailable; no live context or silent skip. Faulty controls exercise the actual production
boundary and independent expectation, not another implementation of the same checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedGitReuseExperiment$' ./internal/gitcore/... > "$r" && grep -F -- '--- PASS: TestSharedGitReuseExperiment ' "$r")` | exit 0; named PASS. Run the reproducible local-only experiment; exact-source answers match independent literals, raw metrics identify baseline/subject/seeds and no forge network requests occur. A deliberately wrong cached result fails correctness regardless of speed. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedGitReuseLifecycle$' ./internal/gitcore/... > "$r" && grep -F -- '--- PASS: TestSharedGitReuseLifecycle ' "$r")` | exit 0; named PASS. Concurrent disposable clients retain access after another workspace retires; interrupted fetch, removed alternate, maintenance overlap and disk pressure expose safe held/unavailable states. Required evidence survives. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedGitReuseAuthorization$' ./internal/gitcore/... > "$r" && grep -F -- '--- PASS: TestSharedGitReuseAuthorization ' "$r")` | exit 0; named PASS. Shared bytes cannot disclose a private canary through another repo/admission. Query fixture bypasses the route and tests source membership, not only an upper UI filter. |
| 4 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/44` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |

## Pre-mortem and detection

For each critical Verify case, retain the valid control and a deliberately wrong subject
variant: removed permission/generation check, unknown converted to success, wrong source key,
omitted invalidation or unsafe replay, as applicable. The independent expected result must
reject that variant; record the failing control with the restored production pass. Do not
satisfy a flow row with a route registry, a prose legend or zero selected tests. Test fixtures
qualify their named local boundary; provider permission inventories, deployment workload and
operator cutover remain explicit evidence/acceptance obligations.

## Evidence

<!-- Independent verifier records source/subject revision, environment, literal command,
exit, named result, positive and failing control, and date. No result is asserted at authoring. -->

## Review

Gate: human. Inspect actual dependency/import and credential reachability, private data and
count isolation, current authority and unknown outcomes. Tests cannot authorize provisioning
credentials, switching a managed writer, answering a human decision or changing a merge policy.
