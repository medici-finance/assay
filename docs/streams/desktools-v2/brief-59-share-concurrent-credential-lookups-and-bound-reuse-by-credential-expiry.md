---
brief: assay:assay:desktools-v2:59
title: "Share concurrent credential lookups and bound reuse by credential expiry"
why: "Concurrent board workers can all miss the current memo before any result is stored. Sharing one in-flight lookup per exact credential identity removes duplicate work while expiry and refresh invalidation prevent a long-lived caller from keeping an obsolete credential."
wave: 2
depends: ["desktools-v2/58"]
unblocks: ["desktools-v2/62", "forge-neutral/40"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
outcome: none
version: 2
id: fbb98894-2b66-401f-863a-9a4857b40edd
authored: 2026-10-16 by performance brief authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - docs/streams/desktools-v2/README.md — Performance delivery priority, 2026-10-16
  - docs/library-first.md
  - docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - freshness-checked 2026-10-16 @ eba941721 (origin/main); installed reference e70f08e4
exec-tier: strong
exec-tier-why: Cross-consumer compatibility, concurrency and unavailable-state behavior require negative-path and flow proofs.
domain: complicated
gate-why: "This is credential reuse under concurrency. Human acceptance confirms no cross-role/scope sharing, no expiry extension and no stale result published after an explicit fresh refresh or custody change."
decision-trigger: spec
consumers:
  - "tools/desk/internal/credential/ and tools/desk/internal/deskkit/roletoken.go: follow-up desktools-v2/59"
  - "tools/desk/cmd/deskboard, deskdispatch, deskread and deskflip: follow-up desktools-v2/59 (shared resolver flow tests)"
  - "tools/desk/internal/forgeexecutor/: follow-up forge-neutral/40"
  - "statusgen/: out-of-scope (no credential memo is linked into this process; deskread inherits the provider)"
  - "tools/desk/README.md: follow-up desktools-v2/59"
---

# Brief 59 — Share concurrent credential lookups and bound reuse by credential expiry

## Context

files:
- `tools/desk/internal/credential/ (from desktools-v2/58)`
- `tools/desk/internal/deskkit/roletoken.go`
- `tools/desk/internal/deskkit/roletoken_memo_test.go`
- `tools/desk/cmd/deskboard/tokenforks_test.go`
- `tools/desk/cmd/deskflip/*token*_test.go` — token-related tests
- `tools/desk/README.md`
- `changelog/credential-concurrency.md` (planned)

facts (source and date in frontmatter):
- The source at eba941721 separately locks memo lookup and store around an unlocked minter call. Existing once-per-owner tests issue sequential requests.
- A 2026-10-16 source-extracted synthetic repro forced six concurrent misses for the same role/account and observed six minter calls. This is an execution-count defect, not a measured production mint-storm rate.
- Current memo age begins when the lookup completes; a credential already near the disk-cache reuse limit must not gain a new full process-memo lifetime. Brief 58 supplies explicit expiry/legacy metadata.
- Different role, host, App, installation, repository grant, custody/config revision and refresh generation cannot share a result. The broader scoping policy remains desktools-v2/06's responsibility.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: internal credential provider
  contract: internal provider/custody parity
  retires: [racy memo lookups]
  weight: verbs +0; flags +0; no new service, credential holder or online endpoint
  why-add: Reuse the existing semantic owner and retire duplicated work; proposed measurements are local, optional and bounded.

## Read first

Read the existing implementations listed in Context, the source-pinned performance priority
section of the desktools-v2 README, and docs/library-first.md. Refresh current claims and the
source revision before implementation; dated measurements do not certify current deployment.

## Human decision

Confirm that concurrent sharing uses the complete credential identity and that expiry, cancellation, failed lookups and explicit refresh cannot reuse a wrong or obsolete credential. Approval preserves existing scope and GitLab rotation authority.

Options:
1. **Accept the demonstrated implementation** — accept the bounded change and its stated
   compatibility; grant no new credential, role, endpoint or deployment authority.
2. **Hold or narrow the change** — identify the unsupported boundary and revise it before acceptance.

Default if no answer: none — blocks until answered.

Acceptance remains pending. The implementer presents the concrete
compatibility and negative-path results before this decision; authoring and offline fixtures
do not activate the change.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure
contact. Use isolated temporary homes, fake providers and synthetic repositories. Preserve
existing custody, stop, trust, audit, budget and unavailable-state semantics. Optional
measurement cannot change the operation result. Stop at implemented; independent verification
owns Evidence. All new paths and named tests below are implementation deliverables.

## Interface contract

One provider instance owns one bounded memo and keyed in-flight table. Keys reflect the full
resolved credential scope, not merely a role or repository string; do not put secret bytes in
keys, telemetry or diagnostics. At most one equivalent lookup runs at a time; unrelated keys
proceed independently. Bound successful reuse by actual remaining credential lifetime and the
existing conservative ceiling. Failed outcomes are shared with current waiters only and never
become a persistent negative cache. Fresh/invalidation advances a generation so older in-flight
work cannot repopulate it. A waiter's cancellation does not cancel other admitted callers.

## Task

1. Implement per-key in-flight request sharing in the provider from 58; remove the racy duplicate memo rather than placing a second cache in front of it. Bound idle entry retention and cleanup for long-lived executor use.
2. Add barrier-based concurrency tests: six identical cold lookups perform exactly one lookup; two different admitted identities each perform one and overlap. Use counters and channels, not timing-only assertions or sleeps.
3. Enforce expiry from the credential's original metadata. Test a disk-cache hit near expiry followed by a long-lived process lookup, clock movement and a fresh mint. Unknown expiry takes the documented conservative path and never renews itself on a hit.
4. Make fresh refresh, permission/custody revision and installation change invalidate the relevant generation. A completion from an older generation cannot overwrite the refreshed result. Do not coalesce destructive GitLab rotation as ordinary read reuse; preserve its dedicated serialization.
5. Test failed lookups followed by recovery and mixed waiter cancellation; no stuck in-flight entries, global-lock convoy or cross-identity value. Extend the actual concurrent deskboard path test, not only a sequential resolver loop. Publish deterministic call-count evidence and measured reference workload timings without fleet promises.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestConcurrentCredentialLookupSingleFlight$' ./internal/credential/... > "$out" && grep -F -- '--- PASS: TestConcurrentCredentialLookupSingleFlight ' "$out")` | exit 0; named PASS; six barrier-synchronized equivalent calls perform exactly one lookup and two identities progress independently; disabling keyed sharing fails |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCredentialExpiryAndRefreshGeneration$' ./internal/credential/... > "$out" && grep -F -- '--- PASS: TestCredentialExpiryAndRefreshGeneration ' "$out")` | exit 0; named PASS; near-expiry cached credentials expire at their original boundary and an old in-flight completion cannot overwrite --fresh; planted age reset fails |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCredentialFailureCancellationRecovery$' ./internal/credential/... > "$out" && grep -F -- '--- PASS: TestCredentialFailureCancellationRecovery ' "$out")` | exit 0; named PASS; failed flight wakes waiters, the next lookup recovers and cancellation of one waiter preserves others; planted persistent negative cache fails |
| 4 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestConcurrentBoardCredentialFlow$' ./cmd/deskboard/ > "$out" && grep -F -- '--- PASS: TestConcurrentBoardCredentialFlow ' "$out")` | exit 0; named PASS; production concurrent repository sweep with fake providers shares only equivalent custody; a role/scope-only key fails the distinct-identity cases |
| 5 | check:ci | `cd tools/desk && GOWORK=off go test -race -count=1 ./internal/credential/...` | exit 0; no data races or stuck keyed-flight state on the concurrency suite. |
| 6 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/59` | exit 0; actual implementation diff corroborates declared routing; out-of-scope sufficiency remains review, and no unavailable check is recorded as a pass. |

## Pre-mortem and detection

- Row 1: six barrier-synchronized equivalent calls perform exactly one lookup and two identities progress independently; disabling keyed sharing fails.
- Row 2: near-expiry cached credentials expire at their original boundary and an old in-flight completion cannot overwrite --fresh; planted age reset fails.
- Row 3: failed flight wakes waiters, the next lookup recovers and cancellation of one waiter preserves others; planted persistent negative cache fails.
- Row 4: production concurrent repository sweep with fake providers shares only equivalent custody; a role/scope-only key fails the distinct-identity cases.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Perf-v3 consumer routing — 19 October 2026

[The attributed performance contract](../../performance-attribution-spec.md) governs future
local measurement. Preserve this brief's existing behavior, tests and legacy v1/v2 privacy;
new qualified work/agent linkage requires the selected v3 workflow profile. /63 owns the
schema/resolver/readers, /64 the shared CLI/env and launch propagation, /65 own-request
cost/timing, /62 desk producer qualification and statusgen/26 standalone phase integration.
Reuse those seams when available and record missing coverage otherwise; no duplicate
writer/parser, speculative ID inference or additional measurement request. Existing credential,
cache/discovery and CLI work remains independently deliverable and is not held for v3.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: human; review the demonstrated custody/expiry contract and the lower-layer bypass proof. Approval grants no new identity or deployment.
