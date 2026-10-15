---
brief: assay:assay:forge-neutral:42
title: "SHA-bound source queries and immutable result caching"
why: "Reviewers and inspection jobs currently need local repository setup or repeated provider reads to answer exact-source questions. A bounded source-query API can reuse immutable answers by SHA while keeping changing PR heads and private access distinct."
wave: 7
depends: ["forge-neutral/39", "forge-neutral/40"]
unblocks: ["forge-neutral/43", "forge-neutral/44"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the query bounds and source-access profile from exact-SHA and denied-cache evidence. A warm object cache is never evidence that a caller may read a private repository; a missing historical object remains unavailable."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 1
id: 702efe9f-7a6c-4079-95cd-0ef3c82f9e69
authored: 2026-10-14 by forge-service authoring
sources:
  - https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/forge-neutral/README.md
  - freshness-checked 2026-10-14 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
  - R6 proposal inspected at c5e8a3f8da3c715b72efa8d8d6b5e74e659ebd52; main merged into this authoring branch without editing historical Evidence
exec-tier: strong
exec-tier-why: Cross-component identity, privacy, concurrency and failure semantics require design judgment and negative controls.
domain: complicated
consumers:
  - "forgeread/source/: follow-up forge-neutral/42"
  - "forgeread/testdata/source/: follow-up forge-neutral/42"
  - "tools/desk/internal/forgesource/: follow-up forge-neutral/42"
  - "tools/desk/internal/forgeservice/server/: follow-up forge-neutral/42"
  - "tools/desk/internal/gitcore/: follow-up forge-neutral/42"
  - "tools/desk/cmd/deskread/: follow-up forge-neutral/42"
  - "docs/forge-service.md: follow-up forge-neutral/42"
  - "docs/library-first.md: follow-up forge-neutral/42"
  - "changelog/forge-source-query.md: follow-up forge-neutral/42"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
---

# Brief 42 — SHA-bound source queries and immutable result caching

## Context

files: `forgeread/source/` (planned), `forgeread/testdata/source/` (planned), `tools/desk/internal/forgesource/` (planned), `tools/desk/internal/forgeservice/server/` (planned), `tools/desk/internal/gitcore/`, `tools/desk/cmd/deskread/`, `docs/forge-service.md` (planned), `docs/library-first.md`, `changelog/forge-source-query.md` (planned). Paths not present at the source pin are planned deliverables; existing
files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- gitcore is internal to tools/desk and includes native/go-git and alternate-store readers. Trusted composition can reuse it; forgeread cannot import it.
- Scion separates immutable content-addressed data from mutable references. This application retains Git and existing provider facts rather than replacing either authority.
- Current candidate/workspace ownership remains graph-execution/23–25. This brief introduces no new branch, cleanup or merge policy.

layering: forgeread source-query contract and trusted tools/desk Git composition. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: treating immutable bytes as a permission grant; served authorization and admitted repository/object membership precede cache delivery, independently of digest/integrity validation.

design-fit:
  owner: forgeread source-query contract and trusted tools/desk Git composition
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs 0; flags +1 bounded source query kind in deskread; refusals +3 unavailable/version/access classes; rule-text +0
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Confirm the query bounds and source-access profile from exact-SHA and denied-cache evidence. A warm object cache is never evidence that a caller may read a private repository; a missing historical object remains unavailable.

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

1. Deliver typed FileAt(repository, commit, path), Diff(repository, base commit, head commit, options) and MergeBase(repository, commits) contracts in a pure forgeread/source package. Bound paths, output bytes, walk/query cost and options; define multiple merge bases explicitly. Preserve missing shallow history, unsupported representation and permission as unavailable, not empty diff/equal ancestry.

2. Compose actual Git readers inside tools/desk over an operator-configured, admitted repository/common store. Reuse gitcore and its serialized maintenance conventions; do not pass arbitrary filesystem paths or remotes from callers. Exact commit/object resolution happens within the authorized repository. First implementation needs no new forge operation; any later provider adapter requires its own concrete consuming read and frozen-surface review.

3. Cache immutable results on repository/instance identity, commit/object IDs, path and options, source representation and derivation version. Separate reusable physical bytes from access/membership metadata. Validate integrity and current caller admission before hits, including cross-repository/fork aliases. A SHA alone grants neither repository membership nor access.

4. Expose the three queries through 40 and a thin deskread consumer/client; one complete fixture client gets changed paths and file contents without cloning a workspace. Branch-name/PR-head resolution is a separately fresh mutable observation; never cache that mapping indefinitely by name. Return exact source inputs and coverage.

5. Keep fact ownership explicit: Git for file content/ancestry; forge mirror for merge/review/check observations; existing evidence owner for acceptance. No inferred merge time from a linear log, no acceptance from a cached diff, no cached approval on another candidate. Keep necessary PR refs and merge-never-rebase behavior.

6. Benchmark cold/warm queries and bytes/object opens against current native/go-git reads on frozen disposable repos. Large outputs and expensive ancestry are bounded/refused visibly. Missing cache can use one admitted bounded source read, not an unbounded checkout or caller-specific poll storm.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestExactSourceQueryFlow$' ./internal/forgesource/... > "$r" && grep -F -- '--- PASS: TestExactSourceQueryFlow ' "$r")` | exit 0; named PASS. Real disposable bare Git -> served authenticated FileAt/Diff/MergeBase -> fixture client. Literal independently selected commits/content/merge bases match; no client checkout is created and unsupported shallow history is explicit. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestImmutableCacheAndMutableHeads$' ./internal/forgesource/... > "$r" && grep -F -- '--- PASS: TestImmutableCacheAndMutableHeads ' "$r")` | exit 0; named PASS. Repeat exact query warms cache; changing branch tip leaves old-SHA bytes stable, while mutable-head resolution refreshes. Query options and derivation versions change keys; corrupt cached bytes fail integrity and do not leak another repository. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestPrivateSourceCacheRefusal$' ./internal/forgesource/... > "$r" && grep -F -- '--- PASS: TestPrivateSourceCacheRefusal ' "$r")` | exit 0; named PASS. Warm private canary under reader A; denied/revoked reader B and another repository/host using the same SHA get no content/existence/counts. Bypassing the upper route still fails source admission/membership. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestSourceQueryBoundsAndParity$' ./internal/forgesource/... > "$r" && grep -F -- '--- PASS: TestSourceQueryBoundsAndParity ' "$r")` | exit 0; named PASS. Native Git and go-git answer the same declared complete fixtures, multiple merge bases and alternate-store cases; missing objects, excessive bytes/cost and unsupported history fail visibly. Record cold/warm measurements without claiming a fleet latency target. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/42` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |

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
