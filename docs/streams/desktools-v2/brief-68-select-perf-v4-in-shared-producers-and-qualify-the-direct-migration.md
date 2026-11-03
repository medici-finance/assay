---
brief: assay:assay:desktools-v2:68
title: Select perf-v4 in shared producers and qualify the direct migration
why: "Future producer upgrades should select the extensible format after its reader is qualified."
wave: 4
depends: ["desktools-v2/67", "desktools-v2/64", "desktools-v2/65"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Selected workflow diagnostics retain identifiable local work references; compatibility and privacy need human acceptance."
issues: []
schema: brief-v2
version: 1
authored: 2026-10-21 by statusgen newbrief
sources:
  - 'desktools-v2: Select perf-v4 in shared producers and qualify the direct migration'
  - freshness-checked 2026-10-21 @ af897c3c1 (origin/main)
exec-tier: strong
exec-tier-why: "Cross-contract compatibility and consumer closure require design judgment."
domain: complicated
outcome: none
consumers:
  - "tools/desk/internal/deskkit/perf.go and tools/desk/internal/cli/: follow-up desktools-v2/68"
  - "statusgen/perfrecord.go: follow-up desktools-v2/68"
  - "docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md: follow-up desktools-v2/68"
  - "docs/streams/desktools-v2/brief-62-instrument-desk-hot-paths-and-qualify-credential-and-startup-savings.md: follow-up desktools-v2/68"
  - "consumer binary pins: out-of-scope (ordinary published release/checksum rollout after implementation)"
  - "performance-collection/: out-of-scope (no activation in this brief)"
---

# Brief 68 — Select perf-v4 in shared producers and qualify the direct migration

## Context

files: tools/desk/internal/deskkit/perf.go; tools/desk/internal/cli/; tools/desk/cmd/{desktoken,verifyloop,deskperf}; statusgen/perfrecord.go — and actual recorder call sites; maintained performance inventory.
facts: At af897c3c112bd426cbbc794839dca6d41df911d6 source has 68 desk invocation producers plus statusgen. Two tools in retained local v3 history do not imply only two capable producers. /64 and /65 independent verification remain prerequisites, not waived by the v4 successor.

Design-fit: implement the producer transition specified by desktools-v2/69; change the shared selector and adapters using /67's codec; reuse context resolution, recorder isolation and observation reducers. Preserve all per-tool phase/counter obligations. /60 snapshot implementation is already merged and is not reimplemented here.

## Human decision

Accept selected-profile v4 emission after /67 and /64–65 qualify, with the same
local workflow disclosure boundary and default v2 behavior. Recommended: shared
selector plus full producer census and two compiled canary paths. Alternative:
retain selected v3 writing until consumers can read v4. No option grants telemetry
upload, live execution, release dispatch or a check waiver.

## Ground rules

Use normal native worker admission, one isolated worktree/PR and role-bound publication.
No workflow triggers, releases, installs, uploads or live infrastructure. Preserve other
owners and completed evidence. Stop at implemented; independent verification owns Evidence.

## Task

1. At implementation, replace authoring-time self-routing with fixed-here for actual changed paths. Re-enumerate actual shared producer call sites independently from inventory/registry; reconcile missing, stale and unsupported entrypoints with explicit coverage. Select v4 only for explicit tool/workflow profile. Telemetry-disabled writes nothing; opted-in no-profile behavior remains v2; no dual-write or automatic upload.
2. Qualify compiled desktoken and verifyloop plus statusgen through success, refusal and early-exit paths. Use fixture/offline provider seams only. Prove invocation identity is fresh, child/context isolation holds, tool profile carries no workflow linkage, and underlying command results remain unchanged on recorder errors.
3. Exercise the file reader/reporter with new and historical records; record a truthful producer capability census, not an installed-fleet claim. Preserve unsupported/missing coverage and immutable v1-v3 history.
4. Add dated amendments to unfinished statusgen/26 and desktools-v2/62 future format tasks, reader dependencies and relevant Verify cases while retaining their original snapshot/benchmark/phase requirements and prerequisites. Re-read active owners first; stop and hand off an owned conflict. Do not alter completed /63–65 Verify/Evidence or manufacture verification.
5. Document ordinary release/checksum reader-before-writer consumer sequencing and reversible profile selection. Do not tag, install local builds, repin unpublished artifacts or activate performance-collection. Measure matched cold/warm overhead honestly, including full membership-validation cost; no cross-process startup-savings inference.

## Verify

| # | Command | Expect |
|---|---|---|
| 1 | `cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestV4SelectedProducerFlow$' ./internal/deskkit` | exit 0 and named PASS; compiled desktoken/verifyloop → files → compiled report, explicit v4 profiles and preserved default v2 |
| 2 | `cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestV4ProducerCensus$' ./internal/deskkit` | exit 0 and named PASS; independent source enumeration equals classified inventory; deliberately omitted producer fails |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -v -run '^TestV4StatusgenExitFlow$' .` | exit 0 and named PASS; success/refusal/early-exit real producer/report paths and separate invocation identities |
| 4 | `cd tools/desk && GOWORK=off go test -race -count=1 -v -run '^TestV4ConsentIsolation$' ./internal/deskkit` | exit 0 and named PASS; no-write disabled, default v2, no workflow leakage in tool profile, parent/child isolation and recorder-failure result parity; mutations removing these guards fail |
| 5 | `cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestV4CutoverCost$' ./internal/deskkit` | exit 0 and named PASS; matched fixture p50/p95, counts and cold/warm distinction, no asserted unmeasured savings |
| 6 | `statusgen --root . --lint` | exit 0; reconciled dependency links and no new failures |

Pre-mortem: two-tool-only migration hides stale producers → row 2; opt-in/default
regression → rows 1/4; missed statusgen early exits → row 3; misleading cost claim →
row 5. Named tests must run, not just compile with no matching cases.

## Evidence

<!-- Independent postmerge verifier records actual commands, source and outcomes here. -->

## Review

Independent correctness and security review of the exact head; normal ready/merge gates.
