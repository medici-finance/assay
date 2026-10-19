---
brief: assay:assay:desktools-v2:61
title: "Version shared performance boundaries and keep legacy reports comparable"
why: "Current desk wall time excludes entrypoint setup while CPU includes process lifetime, and readers cannot distinguish several command modes. A shared versioned measurement boundary makes the local records usable for comparing optimizations across desk tools and statusgen without treating missing measurements as zero."
wave: 1
depends: []
unblocks: ["desktools-v2/62", "statusgen/26", "performance-collection/02"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 1
id: c01a7fef-514d-47ac-b39e-ede3f4e9d63a
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
  - "perfrec/: follow-up desktools-v2/61"
  - "tools/desk/internal/deskkit/perf.go and perfread.go: follow-up desktools-v2/61"
  - "tools/desk/cmd/deskperf/: follow-up desktools-v2/61"
  - "tools/desk/cmd/cellctl/perf.go and existing boot/launch step readers: follow-up desktools-v2/61 (compatibility, no duplicate invocation totals)"
  - "tools/desk/cmd/deskboard, deskdispatch and desktoken: follow-up desktools-v2/62"
  - "statusgen/perfrecord.go and perfrecord_test.go: follow-up statusgen/26"
  - "qualgen and standalone tools: out-of-scope (document the credential-free perfrec API and owner adoption checklist; no speculative wrappers)"
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "docs/telemetry.md and tools/desk/README.md: follow-up desktools-v2/61"
---

# Brief 61 — Version shared performance boundaries and keep legacy reports comparable

## Context

files:
- `perfrec/perfrec.go`
- `perfrec/perfrec_test.go`
- `perfrec/cpu_unix.go`
- `perfrec/cpu_windows.go`
- `perfrec/deps_test.go`
- `perfrec/testdata/` (planned)
- `tools/desk/internal/deskkit/perf.go`
- `tools/desk/internal/deskkit/perfread.go`
- `tools/desk/internal/deskkit/perfread_test.go`
- `tools/desk/cmd/deskperf/`
- `docs/telemetry.md`
- `tools/desk/README.md`
- `changelog/performance-boundaries.md` (planned)

facts (source and date in frontmatter):
- Fresh main eba941721 already has the stdlib-only perfrec module. deskkit aliases its record/writer, and statusgen/perfrecord.go records through it from exitProcess. This brief extends that owner; it does not re-extract logging.
- Desk PerfRun starts wall timing inside run's wrapper after config echo and activation; CPUMillis currently returns process-lifetime RUSAGE_SELF. A compiled --help benchmark recorded wallMs=0 despite 92–101 ms external elapsed startup in checkouts.
- perfrec.Schema explicitly requires a version bump when fields change. The current reader filters perf-v1 and step records by a verb prefix; preserve existing files and legacy step views.
- performance-collection/01 owns the separate optional correlated event/spool contract, not local perf-v1 semantics. This work needs no perfdata, database, network or runtime activation.

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

Extend perfrec with a documented invocation/session measurement API and a deliberately new
local schema for changed boundaries or fields. Capture wall and CPU baselines at the same
declared Go entrypoint boundary, before configuration/activation, and finish before best-effort
append. State excluded runtime startup and writer/flush costs; benchmark executable elapsed time
separately. Typed fixed-roster phases/counters have explicit availability; no argv/path/repo,
credential, raw error or new cross-run identifier. Reader APIs retain legacy perf-v1 semantics,
surface schema/coverage differences and never combine inclusive child/phase times into additive
elapsed time. perfrec remains credential-, network-, subprocess- and database-free.

## Task

1. Specify the new local schema and timing/counter meanings in perfrec and docs/telemetry.md. Keep old fixtures readable; do not reinterpret historical wallMs or CPU fields. Distinguish unsupported counter coverage from measured zero, failed/missing CPU reads and timed phases that overlap.
2. Deliver a narrow shared begin/phase/finish API with injected clocks and CPU samples for deterministic tests. Fixed tool-owned command/phase rosters determine labels after successful parsing; unknown input maps to constants, never substring matches over arbitrary argv. Bound per-invocation phases/counters and disabled-path overhead.
3. Adapt deskkit recorder and deskperf readers to both versions, with explicit legacy/new groups or clearly stated incompatible coverage. Keep boot and cellctl launch step records out of invocation totals; do not count a parent's inclusive elapsed time plus its children as recoverable minutes.
4. Provide low-cardinality counters for subprocess, direct HTTP, cache lookup/hit/miss/refresh and coalesced waiter outcomes with measured/unavailable coverage. Measurement adapters observe existing effects and cannot initiate credential or provider work. Provider hooks and command phase placement land in 62; statusgen uses this same API in statusgen/26.
5. Prove off creates nothing and cannot change stdout, stderr or exit code; failed recording is best effort. Keep the dependency-closure privacy tests, restrictive local storage and seven-day retention. Document module build/consumer resolution with GOWORK=off and a synthetic external consumer; no source reimplementation in another module.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationBoundaryAndCPUParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationBoundaryAndCPUParity ' "$out")` | exit 0; named PASS; injected setup delay and CPU deltas share one boundary; absent CPU is unavailable and a planted late start fails |
| 2 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceSchemaAndPrivacyCompatibility$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceSchemaAndPrivacyCompatibility ' "$out")` | exit 0; named PASS; old and new fixtures retain meanings, hostile labels never serialize, disabled mode writes nothing and sink failure preserves result; silent schema reuse fails |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMixedSchemaReportBoundaries$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestMixedSchemaReportBoundaries ' "$out")` | exit 0; named PASS; mixed perf-v1/new records and boot/launch steps report coverage and distributions without double-counting inclusive durations; naive summed totals fail |
| 4 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPublicRecorderConsumerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPublicRecorderConsumerBoundary ' "$out")` | exit 0; named PASS; synthetic external GOWORK=off consumer uses the actual library; network, exec, credential and desk imports are planted negative controls |
| 5 | check +dereference | `statusgen --consumers --root . --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899` | exit 0 and the summary line reads `summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`; actual implementation diff (`2013b8dc` against its first parent `613cce8e`) corroborates declared routing. Report that summary line as printed, all three counts; exit 0 alone does not show corroboration, it only says nothing was disproved. Out-of-scope sufficiency remains review: the one UNCHECKED entry (the out-of-scope qualgen and standalone tools entry) is not a pass, and the reviewer weighs its stated reason. No unavailable check is recorded as a pass: missing history or an unreadable delivery exits 2 (could-not-check), never a pass. |

## Pre-mortem and detection

- Row 1: injected setup delay and CPU deltas share one boundary; absent CPU is unavailable and a planted late start fails.
- Row 2: old and new fixtures retain meanings, hostile labels never serialize, disabled mode writes nothing and sink failure preserves result; silent schema reuse fails.
- Row 3: mixed perf-v1/new records and boot/launch steps report coverage and distributions without double-counting inclusive durations; naive summed totals fail.
- Row 4: synthetic external GOWORK=off consumer uses the actual library; network, exec, credential and desk imports are planted negative controls.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
