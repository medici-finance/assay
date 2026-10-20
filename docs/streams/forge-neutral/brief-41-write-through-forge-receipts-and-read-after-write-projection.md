---
brief: assay:assay:forge-neutral:41
title: "Write-through forge receipts and read-after-write projection"
why: "Waiting for a webhook after our own forge write makes follow-on reads stale; retrying after a lost reply can duplicate issues or comments. Projecting confirmed write receipts into the same mirror gives immediate visibility while preserving unknown outcomes for reconciliation."
wave: 8
depends: ["forge-neutral/37", "forge-neutral/40", "graph-execution/04", "graph-execution/19"]
unblocks: ["forge-neutral/45"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: yes}
gate-why: "Confirm the two initial write operations and their ambiguity/receipt behavior from the demonstrated implementation. Choose read and write activation separately; a successful provider write cannot be retried merely because its cached projection is unavailable."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 4
id: 1fc36b0b-e3d0-4497-8863-0a039180a0f3
authored: 2026-10-14 by forge-service authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/forge-neutral/README.md
  - freshness-checked 2026-10-14 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
  - R6 proposal inspected at c5e8a3f8da3c715b72efa8d8d6b5e74e659ebd52; main merged into this authoring branch without editing historical Evidence
  - "docs/desk-workflow-efficiency-spec.md"
  - "freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d; approved spec and consolidation amendment"
exec-tier: strong
exec-tier-why: Cross-component identity, privacy, concurrency and failure semantics require design judgment and negative controls.
domain: complicated
consumers:
  - "workflow/store/forgeeffects.go: follow-up forge-neutral/41"
  - "workflow/store/forgeeffects_test.go: follow-up forge-neutral/41"
  - "workflow/store/forgemirror.go: follow-up forge-neutral/41"
  - "workflow/store/forgemirror_test.go: follow-up forge-neutral/41"
  - "tools/desk/internal/forgeservice/write/: follow-up forge-neutral/41"
  - "tools/desk/internal/forgeexecutor/: follow-up forge-neutral/41"
  - "tools/desk/cmd/deskfile/: follow-up forge-neutral/41"
  - "tools/desk/cmd/deskreply/: follow-up forge-neutral/41"
  - "docs/forge-service.md: follow-up forge-neutral/41"
  - "docs/enforcement-model.md: follow-up forge-neutral/41"
  - "changelog/forge-write-through.md: follow-up forge-neutral/41"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
  - "tools/desk/internal/forgeservice/write/contextreceipt_test.go: follow-up forge-neutral/41"
---

# Brief 41 — Write-through forge receipts and read-after-write projection

## Context

files: `workflow/store/forgeeffects.go` (planned), `workflow/store/forgeeffects_test.go` (planned), `workflow/store/forgemirror.go` (planned), `workflow/store/forgemirror_test.go` (planned), `tools/desk/internal/forgeservice/write/` (planned), `tools/desk/internal/forgeexecutor/` (planned), `tools/desk/cmd/deskfile/`, `tools/desk/cmd/deskreply/`, `docs/forge-service.md` (planned), `docs/enforcement-model.md`, `changelog/forge-write-through.md` (planned), `tools/desk/internal/forgeservice/write/contextreceipt_test.go` (planned). Paths not present at the source pin are planned deliverables; existing
files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- 37 owns the admission-bound mirror; 19 owns PostgreSQL and 04 owns intent/receipt/reconciliation. Their qualified implementations are prerequisites, not features this brief reimplements.
- The existing deskfile issue-create and deskreply change-comment operations supply concrete consumers. Review approval, ready flip, merge, release and evidence publication are outside this initial write surface.
- A local SQL transaction cannot atomically commit a GitHub or GitLab effect. HTTP timeout does not establish failure.

layering: R2 effect ledger and mirror transactions; tools/desk role-bound service composition. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: recognizing that an unknown submission may have succeeded; durable intent precedes contact, executor authority constrains repetition, and provider-specific reconciliation checks the external object. No universal provider idempotency is assumed.

design-fit:
  owner: R2 effect ledger and mirror transactions; tools/desk role-bound service composition
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs 0; flags +1 explicit service profile for selected existing clients; refusals +3 unknown/conflicting/pending classes; rule-text +0
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Confirm the two initial write operations and their ambiguity/receipt behavior from the demonstrated implementation. Choose read and write activation separately; a successful provider write cannot be retried merely because its cached projection is unavailable.

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

1. Compose synchronous create-issue and create-change-comment operations through 40 into the existing effect protocol. Record a stable logical operation identity and payload digest before contact in R2; use that identity across attempts. Different payload under one key conflicts. Apply existing relevant trust, content, budget, stop and candidate controls before provider contact; issue creation binds its approved input digest where no candidate commit exists.

2. Add adapters to the existing workflow/store effect and mirror transactions; do not introduce a second journal, inbox/outbox or database. On confirmed success, atomically persist the receipt and confirmed mirror fields where possible. Preserve exact source admission and per-object refresh generation so an earlier in-flight read cannot overwrite the receipt. Confirm only returned fields; do not fabricate missing labels/reviews/checks.

3. Return separate provider and projection outcomes plus the scoped opaque observation token; never export global database progress or another admission's activity. If the provider succeeded but SQL/mirror update failed, report projection pending/unavailable and preserve an unknown ledger interval where necessary. Minimum-token queries must return effect-containing/newer authoritative data or pending/unavailable, never silently older rows.

4. Implement operation-specific reconciliation for lost replies/crashes and retain held unknown when no trustworthy proof exists. Use provider idempotency only when the endpoint documents it. A marker aids lookup but is not an atomic uniqueness guarantee; partial enumeration cannot prove absence. Replaying a valid authorization may retrieve an authorized receipt, not execute a second request.

5. Integrate the two legacy clients behind an explicit service profile without weakening their guards or allowing automatic raw/direct fallback. Default remains the supported current route until 45 cutover. Inventory which process holds each token; the service profile refuses if a legacy independent publisher remains in the managed caller environment.

6. Exercise external edits and event/poll repair through the same mirror; newer authoritative state can supersede our write. Retention, access loss, complete database reconstruction and old-incarnation behavior use R2 and mirror contracts. Raw mirrored table UPDATE remains unable to authorize any effect.


### Approved workflow-efficiency amendment — 17 October 2026

An observation/ReviewContext is an optimization input, never a replacement for current effect
admission or provider preconditions. Preserve exact-head claim/ready/verdict applicability, payload
identity and unknown operation reconciliation; stale cached context cannot repeat an uncertain
write. Receipt projection and minimum-token reads remain distinct from provider success. Exercise
reuse followed by head/permission change and a lost receipt; no new effect journal, authority
boolean or automatic direct fallback.

This is an amendment to unfinished implementation acceptance. Preserve every prior Task, Verify
obligation, risk decision and Evidence entry. All tests named here are planned deliverables; this
authoring change claims no implementation or operational adoption.

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
| 1 | check:ci +flow +mutation | `(cd workflow && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestWriteThroughPostgresFlow$' ./store/... > "$r" && grep -F -- '--- PASS: TestWriteThroughPostgresFlow ' "$r")` | exit 0; named PASS. Real disposable PostgreSQL plus fake provider: intent -> executor receipt -> transaction -> mirror query at returned token. Both initial operations are visible, with provider actor/provenance and no second effect on duplicate delivery. |
| 2 | check:ci +flow +mutation | `(cd workflow && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestWriteThroughUnknownRecovery$' ./store/... > "$r" && grep -F -- '--- PASS: TestWriteThroughUnknownRecovery ' "$r")` | exit 0; named PASS. Inject death after send, lost ACK, provider-success/SQL-failure, restore and ambiguous lookup. Submitted unknown is held/reconciled and never blindly re-applied; persisted receipts/projection recover within declared loss policy. Partial absence and a marker alone cannot authorize retry. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestMinimumObservationFlow$' ./internal/forgeservice/... > "$r" && grep -F -- '--- PASS: TestMinimumObservationFlow ' "$r")` | exit 0; named PASS. Actual HTTP operation -> caller loses reply -> authorized lookup -> minimum-token query. Delayed pre-write refresh cannot roll back data; newer external edit is shown. Permission revoked between write and lookup hides the receipt/body. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestLegacyIssueCommentServiceParity$' ./internal/forgeservice/... > "$r" && grep -F -- '--- PASS: TestLegacyIssueCommentServiceParity ' "$r")` | exit 0; named PASS. Real selected desk clients -> served service -> separate executor -> fake GitHub/GitLab. Existing guard refusal remains before contact; service outage cannot trigger direct/raw publication; execution identity and unknown semantics survive the client output. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/41` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |
| 6 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestContextReceiptBoundary$' ./internal/forgeservice/... > "$o" && grep -F -- '--- PASS: TestContextReceiptBoundary ' "$o")` | exit 0; named PASS. Stale context/changed head refuses effect admission; a lost reply with unknown outcome is held/reconciled without another provider effect, even when observation bytes are reusable. |

## Pre-mortem and detection

For each critical Verify case, retain the valid control and a deliberately wrong subject
variant: removed permission/generation check, unknown converted to success, wrong source key,
omitted invalidation or unsafe replay, as applicable. The independent expected result must
reject that variant; record the failing control with the restored production pass. Do not
satisfy a flow row with a route registry, a prose legend or zero selected tests. Test fixtures
qualify their named local boundary; provider permission inventories, deployment workload and
operator cutover remain explicit evidence/acceptance obligations.

## Perf-v3 delivery amendment — 19 October 2026

For this brief's existing performance observations, reuse the [perf-v3 contract](../../performance-attribution-spec.md) and /63–65's
shared typed context and request-cost seam when those adapters are available. Each process
records its own attempts, timing and observed GraphQL points with units/basis/coverage;
client-to-service and service-to-provider work are distinct. Carry admitted parent invocation
links when known, preserve missing context and never turn it into operational authority.
No extra provider request or expanded business query for monitoring; qualified bounded cost
metadata may join the same request. No copied descendant call total,
and no v2-only replacement instrument. Keep this brief's scope/Verify/Evidence and independent
service delivery; record unsupported boundaries until producer integration. /45's existing
matched service qualification must include own-request and parent/child no-double-count cases.

## Query/cache measurement amendment — 19 October 2026

Apply docs/performance-attribution-spec.md, including its query/cache boundary contract.

Passthrough/write-through records actual executor provider attempts, retries and point cost at the provider leaf. Service client/handler timing is a separate view. A replayed receipt may be a service hit with zero observed provider sends, while unknown publication remains unknown. Diagnostics never change effect identity, replay policy, receipt authority or permit another write.

Extend this brief's existing production-flow Verify cases with these boundary/false-count
negative fixtures; all existing Tasks, exact selectors and Evidence remain obligations.
No service activation or extra dependency on a live collector is introduced.

## Evidence

<!-- Independent verifier records source/subject revision, environment, literal command,
exit, named result, positive and failing control, and date. No result is asserted at authoring. -->

## Review

Gate: human. Inspect actual dependency/import and credential reachability, private data and
count isolation, current authority and unknown outcomes. Tests cannot authorize provisioning
credentials, switching a managed writer, answering a human decision or changing a merge policy.
