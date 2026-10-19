---
brief: assay:assay:desktools-v2:62
title: "Instrument desk hot paths and qualify credential and startup savings"
why: "Aggregate timings cannot distinguish storage admission, repeated forge reads, credential lookup and worktree work. Typed phase and outcome measurements on real command flows make the prioritized optimizations reviewable and prevent a reduced subprocess count from hiding the same work elsewhere."
wave: 3
depends: ["desktools-v2/58", "desktools-v2/59", "desktools-v2/61", "desktools-v2/60"]
unblocks: ["forge-neutral/45"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 1
id: cd878998-fb23-4088-9204-0d66f539b24d
authored: 2026-10-16 by performance brief authoring
sources:
  - docs/streams/desktools-v2/README.md — Performance delivery priority, 2026-10-16
  - docs/library-first.md
  - docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - freshness-checked 2026-10-16 @ eba941721 (origin/main); installed reference e70f08e4
exec-tier: strong
exec-tier-why: Cross-consumer compatibility, concurrency and unavailable-state behavior require negative-path and flow proofs.
domain: complicated
consumers:
  - "tools/desk/cmd/deskboard, deskdispatch and desktoken: follow-up desktools-v2/62"
  - "tools/desk/internal/credential/, deskkit/runtool.go, forgetransport.go and activation.go: follow-up desktools-v2/62"
  - "tools/desk/internal/cellcache/: out-of-scope (observe its existing cheap-clean/locked admission outcomes; do not change its policy)"
  - "tools/desk/cmd/deskperf/: follow-up desktools-v2/62 (actual hotpath records read end-to-end)"
  - "statusgen/: follow-up statusgen/26"
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "tools/desk/internal/forgeservice/: follow-up forge-neutral/45"
---

# Brief 62 — Instrument desk hot paths and qualify credential and startup savings

## Context

files:
- `tools/desk/cmd/deskboard/main.go`
- `tools/desk/cmd/deskboard/sweep.go`
- `tools/desk/cmd/deskboard/readcost.go`
- `tools/desk/cmd/deskdispatch/main.go`
- `tools/desk/cmd/deskdispatch/dispatch.go`
- `tools/desk/cmd/deskdispatch/storage.go`
- `tools/desk/cmd/deskdispatch/reviewroundread.go`
- `tools/desk/cmd/deskdispatch/exec.go`
- `tools/desk/cmd/desktoken/main.go`
- `tools/desk/internal/credential/`
- `tools/desk/internal/deskkit/forgetransport.go`
- `tools/desk/internal/deskkit/runtool.go`
- `tools/desk/internal/deskkit/activation.go`
- `tools/desk/internal/deskkit/perfcoverage_test.go`
- `tools/desk/testdata/perfworkload/` (planned)
- `tools/desk/README.md`
- `changelog/desk-hotpath-measurement.md` (planned)

facts (source and date in frontmatter):
- The bounded local sample ending 2026-10-16T13:47:01Z recorded 684 dispatches (p50 17.477 s), 314 board commands (p50 21.604 s), and 8,256 token commands (p50 219 ms). Command variants and nested wall times prevent a causal savings estimate.
- Token mint calls use a direct http.Client outside the forge-counting transport, and the old token subprocess bypasses deskkit.Run. Zero counters therefore do not prove no network or no child work.
- The ordinary dispatch bucket called other has p50 25.015 s; check-verifier, stamp-only and help are separate workloads. Most board verbs also collapse to other under first-argument matching.
- Existing board snapshot/conditional reads/per-PR facts and cellcache.Admit cheap-clean are retained baseline features. Do not count them again as delivered savings.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: perfrec and existing domain producers
  contract: versioned local performance record
  retires: [incompatible timing boundaries and unclassified measured paths]
  weight: verbs +0; flags +0; no new service, credential holder or online endpoint
  why-add: Reuse the existing semantic owner and retire duplicated work; proposed measurements are local, optional and bounded.

Risk derivation: these four no answers are scoped to behavior-preserving discovery or
measurement work. No credential, private payload or new identity is exposed, and no policy,
role grant or refusal may change. Direct reference/exit parity is required. Any proposed
authority or disclosure change is outside this brief and needs a separately derived gate.

## Read first

Read the existing implementations listed in Context, the source-pinned performance priority
section of the desktools-v2 README, and docs/library-first.md. Refresh current claims and the
source revision before implementation; dated measurements do not certify current deployment.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure
contact. Use isolated temporary homes, fake providers and synthetic repositories. Preserve
existing custody, stop, trust, audit, budget and unavailable-state semantics. Optional
measurement cannot change the operation result. Stop at implemented; independent verification
owns Evidence. All new paths and named tests below are implementation deliverables.

## Interface contract

Use the measurement API from 61 on real command dispatch. Phases and modes come from validated
typed command/kit outcomes and fixed rosters, never raw argv. Invocation wall/CPU include config
and activation; scoped phases expose setup, storage/cache admission, review-scope reads, claim,
fetch/worktree, stamp/queue-label and output preparation where applicable. Count actual requests
including credential HTTP and actual child starts through every retained seam; declare omitted
paths unavailable. Counter ownership prevents transport wrappers and phases counting one effect
twice. No free-text identity or remote telemetry is added.

## Task

1. Route every terminal path of the three commands through one measurement finalizer, starting before setup and retaining the original output/exit. Record parsed board verb and dispatch kit/mode rather than treating --root-first invocations as other. Preserve help/version offline behavior from their CLI owners.
2. Add the listed phase scopes around actual implementations and direct-provider typed outcomes. Expose cache hit/miss/refresh, in-flight waiter, real mint, HTTP requests and child starts. Include refused/disabled/unverifiable paths with measured or unavailable coverage; neither a lookup nor a retry is automatically a mint.
3. Observe cellcache admission's existing fast/locked reason classes through bounded enums, and separate fetch/worktree costs from credential/setup. Use the current board read-cost meter for requests/304s/skips; do not start a parallel request counter with incompatible units.
4. Commit a deterministic fake-provider/temporary-tree workload and reference report: equivalent cold/warm board sweeps, six concurrent same-identity and distinct-identity lookups, fresh/near-expiry tokens, normal/check/stamp dispatch and refused calls. Run the production adapters. Report external and recorded p50/p90/p95, CPU, requests, children, scan counts and sample count, with source/build/workload revisions and collection-on/off overhead.
5. Compare no-minter direct lookup and coalescing against an executable legacy/reference fixture, with identical fake delays and inputs; separate each changed variable. Include startup index 60 in the matched cold/warm case and label unsupported optional counters unavailable rather than silently comparing different inputs. No dependency on a live forge, database or future service. Update docs and publish limits on parent/child attribution.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestHotpathModeAndSetupCoverage$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestHotpathModeAndSetupCoverage ' "$out")` | exit 0; named PASS; actual compiled command cases include setup, classify root-first flags correctly and preserve every exit; planted late timer/argv substring labels fail |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCredentialAndChildCounterCoverage$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestCredentialAndChildCounterCoverage ' "$out")` | exit 0; named PASS; fake real mint, cached hit, shared waiter, retained child and direct HTTP each have exact counts or explicit unavailable coverage; bypassed/double counters fail |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestDeskPerformanceWorkloadFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestDeskPerformanceWorkloadFlow ' "$out")` | exit 0; named PASS; production fixture operations produce records deskperf can read; legacy/direct/concurrent controls prove request and child reductions without changing decisions, and include failed paths |
| 4 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/62` | exit 0; actual implementation diff corroborates declared routing; out-of-scope sufficiency remains review, and no unavailable check is recorded as a pass. |

## Pre-mortem and detection

- Row 1: actual compiled command cases include setup, classify root-first flags correctly and preserve every exit; planted late timer/argv substring labels fail.
- Row 2: fake real mint, cached hit, shared waiter, retained child and direct HTTP each have exact counts or explicit unavailable coverage; bypassed/double counters fail.
- Row 3: production fixture operations produce records deskperf can read; legacy/direct/concurrent controls prove request and child reductions without changing decisions, and include failed paths.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
