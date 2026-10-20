---
brief: assay:assay:statusgen:26
title: "Extend statusgen phase measurements through the shared performance recorder"
why: "Statusgen already shares the local recorder but aggregate duration cannot explain discovery, parsing, evaluation, rendering or deskread overhead. Reusing the improved contract makes its costs comparable and identifies safe parse reuse without importing credential or desk internals."
wave: 4
depends: ["desktools-v2/63", "desktools-v2/64"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 2
id: 34374f41-67cd-492a-9bb3-5d3353cf0559
authored: 2026-10-16 by performance brief authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "assay:assay:desktools-v2:62 — matched desk workload; this brief supplies the statusgen consumer using the same recorder"
  - docs/streams/desktools-v2/README.md — Performance delivery priority, 2026-10-16
  - docs/library-first.md
  - docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - freshness-checked 2026-10-16 @ eba941721 (origin/main); installed reference e70f08e4
exec-tier: strong
exec-tier-why: Cross-consumer compatibility, concurrency and unavailable-state behavior require negative-path and flow proofs.
domain: complicated
consumers:
  - "statusgen/perfrecord.go, main.go, load.go, parse.go and forgeread.go: follow-up statusgen/26"
  - "statusgen/go.mod and go.sum: follow-up statusgen/26"
  - "perfrec/: follow-up desktools-v2/61 (one shared recorder implementation)"
  - "tools/desk/cmd/deskperf/: follow-up statusgen/26 (read actual statusgen records)"
  - "tools/desk/cmd/deskread: out-of-scope (existing online credential boundary is retained; child benefits from desktools-v2/58 and /59)"
  - "statusgen/contexthandoff/: follow-up statusgen/20 (reuse only explicit source-bound immutable input snapshots)"
  - "qualgen/: out-of-scope (documented consumer candidate; no measured hotpath or new wrapper claimed)"
  - "docs/telemetry.md: follow-up statusgen/26"
---

# Brief 26 — Extend statusgen phase measurements through the shared performance recorder

## Context

files:
- `statusgen/perfrecord.go`
- `statusgen/perfrecord_test.go`
- `statusgen/main.go`
- `statusgen/load.go`
- `statusgen/parse.go`
- `statusgen/forgeread.go`
- `statusgen/go.mod`
- `statusgen/go.sum`
- `statusgen/bench/perfworkload.sh` (planned)
- `statusgen/testdata/perfworkload/` (planned)
- `docs/telemetry.md`
- `changelog/statusgen-performance-phases.md` (planned)

facts (source and date in frontmatter):
- At fresh main eba941721, statusgen/perfrecord.go already calls perfrec.New/Append and CPUMillis; every main exit passes exitProcess. Preserve that coverage and its off-by-default local-only contract.
- docs/library-first.md allows only offline/frozen in-process fact readers; statusgen may not import tools/desk, deskkit, authenticated forgeread adapters or a credential provider.
- Existing telemetry.go has separate counts-only network consent. Local phase measurements do not widen that payload, enable an endpoint or add upload.
- Statusgen has distinct lint, regen, gate-scores, next-up and forge-read modes. A new file/consumer outside a previous read set invalidates completeness; HEAD equality alone does not make a dirty working-tree parse immutable.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: perfrec and existing domain producers
  contract: versioned local performance record
  retires: [incompatible timing boundaries and unclassified measured paths]
  weight: verbs +0; flags +0; no new service, credential holder or online endpoint
  why-add: Reuse the existing semantic owner and retire duplicated work; proposed measurements are local, optional and bounded.

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

Use perf-v3 from /63 and the shared context contract from /64 directly, with statusgen-owned fixed mode/phase rosters and the same
wall/CPU baseline semantics. Observe source discovery, read/parse, evaluation, rendering and
existing deskread child time. Online work remains behind the existing verb with no credentials
in statusgen. Input reuse is limited to an invocation-local explicit snapshot and versioned
parser/evaluator result; no persistent semantic cache or second parser is introduced here.
If a required source is unavailable, preserve the existing incomplete/held result.

## Task

1. Adopt the perf-v3 shared recorder API and explicitly selected profile at the current exitProcess boundary, preserving all exits, opt-out behavior and legacy mode mapping. Account for package-init versus Go-entrypoint timing explicitly; do not change historical perf-v1 semantics.
2. Add phases through existing owners for discovery/load, parse, eligibility/coverage evaluation and rendering. Observe actual deskread child starts/elapsed time without opening a forge client in statusgen; the child owns provider/credential counters. Do not sum child and parent inclusive time as elapsed savings.
3. Inventory repeated reads/parses within one invocation in lint, regen, gate-scores and next-up. Reuse the same validated source snapshot/results where those owners already support it; keep the parser/evaluation implementations canonical. A changed membership or file revision between declared snapshots must trigger fresh input, never a stale empty result. Report no-op reuse when the reference workload shows none.
4. Add credential-free synthetic small/large stream workloads for all four offline modes plus a fake deskread process. Record cold/warm p50/p95, CPU, file reads/parses and child counts with sample and source revisions. Compare output/exit/coverage byte-for-byte against the fresh reference evaluator; numerical targets require a declared workload.
5. Prove statusgen keeps zero imports of tools/desk/custody, no new network endpoint and no telemetry files when off. Document how deskperf reads statusgen records and how future context resolver 20 consumes explicit immutable snapshots without weakening independent membership enumeration. Keep module resolution working with GOWORK=off.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedPhaseRecorderFlow$' ./... > "$out" && grep -F -- '--- PASS: TestSharedPhaseRecorderFlow ' "$out")` | exit 0; named PASS; compiled offline lint/regen/gate-scores/next-up and fake deskread retain output/exit, produce correct shared records and deskperf can read them; copied writer fails parity |
| 2 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationSnapshotReuseParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationSnapshotReuseParity ' "$out")` | exit 0; named PASS; same snapshot shares parsing, while new/deleted/untracked consumers and changed contents match fresh reference results; HEAD-only or prior-readset reuse fails |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceKeepsCredentialBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceKeepsCredentialBoundary ' "$out")` | exit 0; named PASS; dependency closure contains no desk/custody/authenticated adapter imports, off creates no files and fake deskread stays the sole online route; planted direct credential getter fails |
| 4 | check +dereference | `statusgen --root . --consumers --brief statusgen/26` | exit 0; actual implementation diff corroborates declared routing; out-of-scope sufficiency remains review, and no unavailable check is recorded as a pass. |

## Pre-mortem and detection

- Row 1: compiled offline lint/regen/gate-scores/next-up and fake deskread retain output/exit, produce correct shared records and deskperf can read them; copied writer fails parity.
- Row 2: same snapshot shares parsing, while new/deleted/untracked consumers and changed contents match fresh reference results; HEAD-only or prior-readset reuse fails.
- Row 3: dependency closure contains no desk/custody/authenticated adapter imports, off creates no files and fake deskread stays the sole online route; planted direct credential getter fails.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Perf-v3 delivery amendment — 19 October 2026

Consume the [perf-v3 contract](../../performance-attribution-spec.md) directly from perfrec; do not import desk internals or authenticated
adapters. Bind the shared --perf-* / explicit env sources through statusgen's real parser;
keep help/version offline, prior opt-in/schema defaults and no-profile behavior. Current
phase/snapshot work stays in scope. A statusgen invocation has fresh identity, receives
work/session/agent/desk and assigned role through the selected workflow profile, and supplies
its invocation as the deskread child's parent. The child reports its own forge attempts,
timing and points; missing child instrumentation is a coverage gap, never parent zero-cost.

Extend TestSharedPhaseRecorderFlow (row 1) with two qualified work references, override/clear,
child causal links and mixed v1/v2/v3 records. Extend TestPerformanceKeepsCredentialBoundary
(row 3) to prove inherited workflow fields cannot appear in tool profile and malformed optional
context cannot change normal output/exit. Old privacy defaults remain binding outside v3.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
