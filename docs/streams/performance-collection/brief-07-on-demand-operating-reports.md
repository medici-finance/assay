---
brief: assay:assay:performance-collection:07
title: "On-demand three-report bundle and thin skill"
why: "Operators need one reproducible view of completions, stream movement and tool costs from collected data, without more provider calls."
wave: 5
depends: ["performance-collection/02"]
unblocks: ["performance-collection/08"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Report collection, exposure and recurring effect boundaries need acceptance of the concrete implementation; no live permission is granted by this draft."
decision-trigger: spec
issues: []
schema: brief-v2
version: 5
id: 29c71b10-6ca6-4594-bc10-1f294a5d1463
authored: "2026-10-15 by reporting authoring session"
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "2026-10-16 wave reconciliation at origin/main 9f9bb35de: performance-collection/02 consumes desktools-v2/61; reporting scope and historical Evidence unchanged"
  - "medici-finance/assay#3353 — cell startup scratch scan; measurements on v1.0.48"
  - "medici-finance/assay#3099 — remaining parent launch telemetry coverage"
  - "freshness-checked 2026-10-20 @ eb82d5b62 (merged main, installed v1.0.51); the 2026-10-18 v1.0.49 install recorded source 3cb57291e"
  - docs/streams/performance-collection/README.md
  - docs/streams/continuing-operations/spec.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - "freshness-checked 2026-10-15 at public main d430e8f18"
  - "docs/desk-workflow-efficiency-spec.md"
  - "freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d; approved spec and consolidation amendment"
exec-tier: strong
exec-tier-why: "Cross-source identity, coverage, authority and migration must agree."
domain: complicated
outcome: none
consumers:
  - "perfdata/report/: follow-up performance-collection/07"
  - "perfdata/cmd/perfdata/: follow-up performance-collection/07"
  - "perfdata/testdata/operating-reports/: follow-up performance-collection/07"
  - "plugins/assay/skills/operating-report/SKILL.md: follow-up performance-collection/07"
  - "docs/performance-collection.md: follow-up performance-collection/07"
  - "docs/contracts.md: follow-up performance-collection/07"
  - "changelog/operating-reports.md: follow-up performance-collection/07"
  - "perfdata/report/phasecost_test.go: follow-up performance-collection/07"
---

# Brief 07 — On-demand three-report bundle and thin skill

## Context

files:
- `perfdata/report/` (planned)
- `perfdata/cmd/perfdata/` (planned)
- `perfdata/testdata/operating-reports/` (planned)
- `plugins/assay/skills/operating-report/SKILL.md` (planned)
- `docs/performance-collection.md` (planned)
- `docs/contracts.md`
- `changelog/operating-reports.md` (planned)
- `perfdata/report/phasecost_test.go` (planned)
- `perfdata/report/cellstart_test.go` (planned) (additional startup scope)

facts:
- This is authored todo scope, not deployed capability. Collection briefs 01/02 produce the bounded source contract; no database or remote endpoint is needed by local reports.
- Existing work/acceptance, runner usage and operational audit retain their owners. `statusgen/flow.go` is package main, not an importable library; consume its versioned saved export, not an invented cross-module import.
- perf-v1 has seven-day raw retention, own-process CPU and partial forge/child counters. Preserve source limits; absent history stays unknown.
- Source pin is in frontmatter; refresh actual source, claims and prerequisites at pickup.

layering: pure validation/aggregation and typed snapshot consumers; adapters own I/O and effects. Future cell coordination and effect execution remain with R2/R3/R4; optional reporting cannot grant authority.

design-fit:
  owner: perfdata report/proposal consumer over existing source semantics
  contract: performance-event-v1 and bounded snapshots from 01/02; register the consumer in docs/contracts.md
  retires: [temporary local perf-v1 triage reader after released parity qualification]
  weight: "Measure added verbs, flags, refusals and cost at implementation; do not invent savings."
  why-add: "Existing source owners lack this composed operator journey. Reuse their outputs rather than a second historian, scheduler or acceptance evaluator."

single-point-of-failure: source coverage and publication scope — independent expected fixtures and lower-boundary authorization tests reject unsupported complete/authorized claims.

## Read first

Read the stream README, docs/library-first.md, docs/report-packs.md, continuing-operations spec sections 5/9, and runtime-contract-alignment. All required outcomes are restated in Tasks and Verify.

## Human decision

Review the exact report audience, permitted source profiles, output allowlist and retention evidence. The on-demand command must remain useful offline; this acceptance does not authorize upload or recurring publication.

Options:
1. Accept the demonstrated scope, source coverage and access/effect boundaries; select the concrete operating profile before activation.
2. Hold the unsupported boundary and continue offline fixture work only.

Default if no answer: none — blocks until answered.

Acceptance/activation remains held; offline implementation can proceed.

## Ground rules

- Own isolated branch and draft PR; no main push, merge, live deployment or workflow dispatch.
- Synthetic sources and fake effect boundaries until the applicable operating policy is recorded.
- Stop at implemented; independent verifier fills Evidence.
- Missing coverage, authority or effect outcome never becomes zero, success or permission to retry.

## Task

1. Extend the perfdata report entrypoint created by performance-collection/01 with an operating bundle selector, date/window/timezone and permitted repository/stream/tool filters. JSON is the common result; self-contained HTML includes all three historical charts and accessible data tables. A thin operating-report skill invokes the command and interprets cited facts; it does not recompute metrics or contact a forge. Report saved source cutoffs and collection health first.
2. Compose completion and stream-flow results from existing lifecycle/flow definitions and versioned saved exports. Stable work identity deduplicates imports; first-observed done is opening balance. Show first completions and separate reopens/recompletions, a daily chart and seven-calendar-day rolling average. Unknown days break the line/rolling coverage, never silently enter as zero. Added/removed/renamed scope and paused states remain visible.
3. Render per-stream movements since an explicit previous cutoff, stage counts, age, last meaningful movement and stalled work. A versioned stage/priority policy nominates stalls. Show recorded blocker reason, accountable role and source/as-of when present; otherwise reason unknown. Do not infer a causal explanation from inactivity or current labels.
4. Aggregate per-tool/verb/role/build/platform runs and total/Avg/nearest-rank P95 for invocation wall, child wall, own-process CPU and forge requests. Pool samples or declared mergeable distributions, not daily percentiles/means. Show per-field n, unavailable values, lower bounds, malformed/lost samples and build/workload breaks. Do not sum nested times as elapsed brief time. Tokens/cost are optional qualified workflow-profile inputs, never fabricated from shared totals.
5. Prove local report operation with no cell database, network, credentials or hosted runtime. Compose over public typed export contracts; if reuse needs extraction from package main, record it as an explicit owner change before importing. Changing output filters cannot bypass current source authorization. No raw prompt, title (issue, PR, brief, workpad, session or window title), error text or credential appears in any report output, public or not; free-text source fields such as recorded blocker reasons are reduced to bounded typed values before rendering.
6. Document command examples and retirement parity for the temporary perf-v1 triage aid. A released local triage selector must reproduce recorded total-wall/CPU/forge focus ranking, sample counts and unknown/lower-bound behavior before the temporary reader is retired.


### Approved workflow-efficiency amendment — 17 October 2026

Extend the existing tool panel, not the number of reports, with bounded phase/endpoint/caller
breakdowns and matched complete-cycle views. Show per-field coverage, own vs descendant CPU,
overlapping elapsed intervals, attempts vs points, retries/304s, actual/estimated basis and
unknowns. Reconstruct complete dispatcher-plus-agent cohorts using source-qualified correlation;
never sum parent aggregates with children or compare pooled builds/workloads as controlled wins.
Keep baseline/subject and tool-only/supervision-only/combined changes separate when #3325 is
adopted; tokens are not a substitute for CPU or forge points. Allow local qualification exports
consumed by desk-tools/44 without DB/API/upload; performance collection remains optional to early
qualification. Pending #3305 owns reusable scorecard/experiment policy if landed, not this report
renderer.

This is an amendment to unfinished implementation acceptance. Preserve every prior Task, Verify
obligation, risk decision and Evidence entry. All tests named here are planned deliverables; this
authoring change claims no implementation or operational adoption.

### Cell startup report acceptance — 2026-10-18

Extend the existing tool panel with cell time-to-first and time-to-all requested confirmed
harness starts, shared preparation, per-role admission and lock waits from /02. Separate
startup from session lifetime and subsequent deskboot. A refused, failed, unknown or missing
requested role prevents a successful all-started sample; show its outcome and coverage rather
than silently shrinking the requested set. Never sum concurrent role wall times or repeat a
shared parent cost. Compare cold/warm and 1x/10x retained-history workloads at fixed role count,
with exact measured build/source, sample counts and uncertainty. Keep installed-version metadata
separate from sample version; upgrading to v1.0.49 cannot relabel #3353's v1.0.48 sample.

Use saved local observations with DB/API absent. Missing phase/correlation data is could-not-check,
never proof of faster startup. This reports the existing launch owner's result; it introduces
neither a latency SLO nor another runtime owner and does not change this stream's admission state.

## Verify

Named commands below are implementation deliverables. Each must run the named test (not succeed with no matching tests).

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./report/... -run '^TestOperatingReportsGolden$' > "$o" && grep -F -- '--- PASS: TestOperatingReportsGolden ' "$o")` | exit 0; named PASS; hand-checked completion, stream and tool panels match one snapshot, including opening balance/reopen and unknown blocker cases |
| 2 | check:ci | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./report/... -run '^TestOperatingDistributionPooling$' > "$o" && grep -F -- '--- PASS: TestOperatingDistributionPooling ' "$o")` | exit 0; named PASS; unequal daily cohorts yield correct Avg/P95; missing CPU and missing days stay unknown |
| 3 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./report/... -run '^TestOperatingReportOfflineAndScope$' > "$o" && grep -F -- '--- PASS: TestOperatingReportOfflineAndScope ' "$o")` | exit 0; named PASS; no provider/socket attempt; unauthorized sources excluded before aggregates and stale coverage visible |
| 4 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./report/... -run '^TestOperatingReportReplayAndTriageParity$' > "$o" && grep -F -- '--- PASS: TestOperatingReportReplayAndTriageParity ' "$o")` | exit 0; named PASS; same snapshot/config has identical JSON and deterministic chart data; legacy triage ranking/coverage agrees |
| 5 | check:ci +flow | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v ./report/... -run '^TestOperatingReportRedaction$' > "$o" && grep -F -- '--- PASS: TestOperatingReportRedaction ' "$o")` | exit 0; named PASS; credential-, prompt-, title- and error-shaped strings seeded into fixture sources (including blocker reasons) are absent from the JSON, the HTML and the manifest |
| 6 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/07` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |
| 7 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPhaseReportFlow$' ./report/... > "$o" && grep -F -- '--- PASS: TestPhaseReportFlow ' "$o")` | exit 0; named PASS. Nested/failed/missing-child fixture produces correct component/cycle tables and unknowns; hidden downstream work or double-counted parent/child makes the expected report fail. |
| 8 | check:ci +flow +mutation | `(cd perfdata && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellStartReport$' ./... > "$o" && grep -F -- '--- PASS: TestCellStartReport ' "$o")` | exit 0; named PASS. Hand-computed overlapping-role fixture yields correct first/all-start times and one shared preparation cost; failed/missing role makes all-start coverage incomplete. Mutation: omit failed roles, sum overlapping walls or relabel historical builds; test fails. |

## Perf-v3 delivery amendment — 19 October 2026

Apply the [perf-v3 contract](../../performance-attribution-spec.md). Existing Tasks/Verify/Evidence and consent/retention controls remain owed.

The tool panel can filter/group selected aliased work, session, agent, originating desk and
assigned role. Show each process's own forge attempts, request-duration samples and actual,
estimated and unknown GraphQL points separately by unit/resource. Show missing agent/parent
coverage and unattributed/shared overhead; do not manufacture historical attribution for
v1/v2. Deduplicate source/invocation, never sum parent inclusive wall with child/phase/request
wall, and do not sum rate gauges. Extend TestPhaseReportFlow (existing report Verify
obligation) with these mixed-version/parent-child and false-zero negative fixtures; preserve
all three reports and the offline/no-provider-read contract.

## Query/cache measurement amendment — 19 October 2026

Preserve the query/cache boundary contract from docs/performance-attribution-spec.md:
client logical service calls and transport attempts, handler timing/cache/coalescing outcomes,
and actual direct/passthrough/refresh/poll/repair provider sends/cost remain different kinds.
Import correlation only under the selected aliased workflow policy; never forward raw local
locators or infer missing server coverage as zero. Shared refresh/background cost remains
shared/unallocated and one provider observation cannot be copied to all waiting work items.
Do not sum client wait, handler/provider spans or concurrent process CPU as elapsed savings.
Extend the existing local/served/replay/report Verify flow for this owner with the shared
50-query/three-page fixture, warm hit, passthrough retry, background poll and absent server
data. Wrongly combining the two sides of one service call or multiplying upstream points
must fail the independent expected result; existing selectors and all Evidence remain intact.

## Evidence

<!-- Independent verifier: revision, environment, command, exit, observed evidence/date/runner. -->

## Review

Gate: human. Verify evidence and lower-boundary failure cases; acceptance does not activate a destination, cadence, credential, policy change or merge permission.
