---
brief: assay:assay:forge-neutral:40
title: "Authenticated forge serving and role executor custody"
why: "A shared cache increases the number of callers that can reach private forge data and write paths. Caller authentication, source admission and a separate role executor must be enforced at actual served boundaries so a cached answer or agent-supplied role cannot become authority."
wave: 6
depends: ["forge-neutral/39", "graph-execution/16", "desktools-v2/58", "desktools-v2/59"]
unblocks: ["forge-neutral/41", "forge-neutral/42"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the scoped caller/executor configuration, maximum entitlement lease and revocation behavior from real boundary evidence before exposure. Keep remote human login disabled in this slice; a browser or agent choosing a role cannot grant it."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 4
id: 2b089f0c-6ce5-4e2f-a5bf-d62842845b29
authored: 2026-10-14 by forge-service authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
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
  - "tools/desk/internal/credential/: follow-up forge-neutral/40 (consume /58\u201359 inside the role executor; no second minter/cache)"
  - "tools/desk/internal/forgeservice/auth/: follow-up forge-neutral/40"
  - "tools/desk/internal/forgeservice/server/: follow-up forge-neutral/40"
  - "tools/desk/internal/forgeexecutor/: follow-up forge-neutral/40"
  - "tools/desk/cmd/forgeserve/: follow-up forge-neutral/40"
  - "tools/desk/cmd/forge-executor/: follow-up forge-neutral/40"
  - "tools/desk/internal/deskkit/forgeresolve.go: follow-up forge-neutral/40"
  - "tools/desk/internal/deskkit/outboundforge.go: follow-up forge-neutral/40"
  - "tools/desk/go.mod: follow-up forge-neutral/40"
  - "tools/desk/go.sum: follow-up forge-neutral/40"
  - "docs/forge-service.md: follow-up forge-neutral/40"
  - "docs/enforcement-model.md: follow-up forge-neutral/40"
  - "changelog/forge-service-auth.md: follow-up forge-neutral/40"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
  - "tools/desk/internal/forgeservice/auth/reviewcustody_test.go: follow-up forge-neutral/40"
---

# Brief 40 — Authenticated forge serving and role executor custody

## Context

files: `tools/desk/internal/forgeservice/auth/` (planned), `tools/desk/internal/forgeservice/server/` (planned), `tools/desk/internal/forgeexecutor/` (planned), `tools/desk/cmd/forgeserve/` (planned), `tools/desk/cmd/forge-executor/` (planned), `tools/desk/internal/deskkit/forgeresolve.go`, `tools/desk/internal/deskkit/outboundforge.go`, `tools/desk/go.mod`, `tools/desk/go.sum`, `docs/forge-service.md` (planned), `docs/enforcement-model.md`, `changelog/forge-service-auth.md` (planned), `tools/desk/internal/forgeservice/auth/reviewcustody_test.go` (planned). Paths not present at the source pin are planned deliverables; existing
files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- ForgeFor resolves provider from operator repository configuration and hands its backend an already-admitted role credential. It refuses ambient fallback.
- graph-execution/16 owns incarnation/ownership restoration. This brief consumes that contract, not a second key/lease authority.
- The initial query process must not hold publisher keys. Per-role executor process isolation is required before a managed agent can receive a write capability.

layering: tools/desk trusted service and custody composition; R2/R3 owns capability authority. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: front-door authorization; independent private-executor capability checks, store admission enforcement and scoped provider credentials constrain a bypass. Same-UID socket permissions alone are not role isolation.

design-fit:
  owner: tools/desk trusted service and custody composition; R2/R3 owns capability authority
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs +2 trusted launch entrypoints; flags limited to operator configuration; refusals +6 boundary classes; rule-text +0
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Confirm the scoped caller/executor configuration, maximum entitlement lease and revocation behavior from real boundary evidence before exposure. Keep remote human login disabled in this slice; a browser or agent choosing a role cannot grant it.

Options:
1. **Accept the demonstrated bounded design** — accept only the proven scope and failure behavior; keep live credential provisioning and activation as separate operator acts.
2. **Hold for correction** — identify the unproven boundary or budget and revise before acceptance.

Default if no answer: none — acceptance remains pending. This implementation-evidence decision
does not prevent authoring or independent offline fixture work. Existing mirror retention and
webhook payload decisions are not answered by this section.

## Ground rules

- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
- Isolate changes; never push main, merge, ready-flip, release, dispatch workflows or contact live infrastructure.
- Use synthetic data, disposable processes/repositories and explicit local database fixtures; no production tokens or data.
- Preserve role, candidate, unknown-outcome, required audit/budget and canonical acceptance meanings.
- Stop at implemented; independent verification and acceptance remain separate.
- Do not widen an unsupported provider capability or duplicate another store/evaluator to make a test pass.

## Task

1. Compose the typed 39 handlers in the desk module behind authenticated local ingress and injected current-authority/policy ports. Expose a thin forgeserve command configured by the operator, initially on a private local channel. Authenticate signed launcher capabilities; the capability-signing key sits behind the same R3 boundary and is never readable or mintable by a caller. Peer credentials/socket ACL constrain access but do not distinguish roles sharing a UID. Bind stable caller, audience, cell, provider/host/repo, admitted source, operation, logical effect identity and payload digest for writes, candidate where required, incarnation/generation, policy/access epoch and expiry.

2. Authorize every query and receipt lookup server-side, including cache hits, joins, counts, errors and pagination. Map principal to the exact composition-derived mirror admission; reject a different admission even if it covers the repo. Never accept a caller-selected role/admission as a grant or infer human visibility from the ingestion App. Keep source-access policy and mirror retention separate.

3. Build a private per-role executor command/socket in the same Go module, reusing existing deskkit custody and guarded effect routines through a narrow callable seam. The executor has no generic URL, token-mint or SQL-update operation. The query process holds only read access. Isolate executor credentials/key mounts from callers using the selected R3 boundary; an unproven shared-UID sandbox holds activation.

4. Independently validate capability, admitted logical effect intent and current authority again at the executor immediately before contact. Use the existing effect-admission/receipt port to refuse already-submitted or unknown operations; replay returns a permitted receipt or held unknown, not another provider call. Write admission stays disabled until 41 supplies the real R2 adapter. Preserve the existing stop and trust checks before provider requests. Scan outgoing content for credentials and withheld data. Keep idempotency, audit records, budgets and rate limits ahead of contact. Preserve represented human, authenticated requester and executing provider actor separately. A worker cannot submit an independent reviewer/human decision. Where provider permissions are coarse, document and test the narrower executor restriction.

5. Preserve repository/permission-restricted GitHub role-App tokens where supported and the configured GitLab role/scope/edition custody. Neither process silently borrows ambient human credentials. Fake providers exercise both mappings; published docs cite actual supported permission names and retain missing capability as unavailable.

6. Use operator-selected entitlement leases/access epochs and injected clocks; test invalidation, expiry, revocation and authority rollback. No instantaneous recall of returned bytes or already-submitted requests is promised. R7 will bind remote OIDC/workload authentication to the same policy ports; do not ship an exposed remote login or new identity provider here.


### Approved workflow-efficiency amendment — 17 October 2026

Serve the shared queue/context/issue contracts under current caller admission including counts,
errors and pagination; one cached result cannot answer a different admitted scope or newer cutoff.
Query results remain advisory. Mutable permission/claim/head and effect preconditions remain
rechecked at their consequential boundaries. Do not transfer publisher credentials or role authority
into query composition, and do not assume a source-qualified read bridge qualifies executor
activation. Reuse existing custody and the selected boundary; no second broker or service.

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
| 1 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestMountedForgeAuthorization$' ./internal/forgeservice/... > "$r" && grep -F -- '--- PASS: TestMountedForgeAuthorization ' "$r")` | exit 0; named PASS. Actual served permitted request succeeds; unauthenticated/wrong audience, forged role, a capability minted by the caller without the signing key, wrong host/cell/admission, expired/revoked grants fail. Cached bodies/counts/errors/pagination cannot reveal the private canary. Bypassing middleware must fail the oracle. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestExecutorIndependentAuthority$' ./internal/forgeexecutor/... > "$r" && grep -F -- '--- PASS: TestExecutorIndependentAuthority ' "$r")` | exit 0; named PASS. Send directly to the executor, bypassing ingress: valid scoped worker comment reaches only its provider account; reviewer escalation, changed payload/candidate, stale incarnation/generation, stop and expiry cause zero provider effects; replay of a valid capability cannot cause a second effect. Provider scope independently refuses an out-of-installation target when the application gate is bypassed. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeCredentialContainment$' ./internal/forgeservice/... > "$r" && grep -F -- '--- PASS: TestForgeCredentialContainment ' "$r")` | exit 0; named PASS. Real disposable process boundary hides executor custody from the client/query process, preserves represented/executing identities, and contains no credentials in results/logs/snapshots. A fake credential inventory alone is not a process-isolation pass. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestAccessEpochAndRevocation$' ./internal/forgeservice/... > "$r" && grep -F -- '--- PASS: TestAccessEpochAndRevocation ' "$r")` | exit 0; named PASS. Warm cache -> revoke/increment access epoch or expire entitlement lease -> next in-bound query/operation refuses within the configured contract; old SQL generation after rollback cannot reactivate a grant. New permitted epoch remains a positive control. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/40` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |
| 6 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewServingCustody$' ./internal/forgeservice/... > "$o" && grep -F -- '--- PASS: TestReviewServingCustody ' "$o")` | exit 0; named PASS. Permitted review-context/queue path succeeds while changed admission/expiry refuses even for reused bytes/counts; saved context cannot authorize a role write or bypass the executor recheck. |

| # | Class | Command | Expect |
|---|---|---|---|
| 7 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestExecutorProviderReuseAndRoleSeparation$' ./internal/forgeservice/... > "$out" && grep -F -- '--- PASS: TestExecutorProviderReuseAndRoleSeparation ' "$out")` | exit 0; named PASS; served repeated requests use the real internal provider with zero minter children, expire correctly and refuse cross-role/executor access; token broker or exec-per-request plants fail |

## Pre-mortem and detection

For each critical Verify case, retain the valid control and a deliberately wrong subject
variant: removed permission/generation check, unknown converted to success, wrong source key,
omitted invalidation or unsafe replay, as applicable. The independent expected result must
reject that variant; record the failing control with the restored production pass. Do not
satisfy a flow row with a route registry, a prose legend or zero selected tests. Test fixtures
qualify their named local boundary; provider permission inventories, deployment workload and
operator cutover remain explicit evidence/acceptance obligations.

## Performance reuse amendment — 2026-10-16

Consume the canonical internal credential provider from desktools-v2/58 and its keyed
in-flight/expiry handling from /59 inside each already-isolated role executor. Those are
typed prerequisites, not a reason to pool role keys in the query process. A long-lived
executor must not exec desktoken per operation or start another credential cache. Carry its
explicit resolved custody scope and refresh/configuration generation; recheck operation
authority independently before each effect. Credential reuse is an efficiency aid, never
proof an executor capability remains authorized. The read service and clients receive no
publisher token and cannot request arbitrary roles or token values.

Retain the exact separate process/revocation boundary of this brief. Extracting code inside
one role environment does not remove that boundary or amend statusgen's online read contract.


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

Instrument per-handler service latency and directly observed phases separately from executor-to-provider requests. Keep per-request immutable context and actual authenticated server role distinct from descriptive caller role. Do not attribute concurrent process-wide CPU to each handler. Denial/expiry/cache-hit paths retain result/coverage and make no extra provider calls for diagnostics.

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
