---
brief: assay:assay:performance-collection:07
title: "On-demand three-report bundle and thin skill"
why: "Operators need one reproducible view of completions, stream movement and tool costs from collected data, without more provider calls."
wave: 3
depends: ["performance-collection/02"]
unblocks: ["performance-collection/08"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Report collection, exposure and recurring effect boundaries need acceptance of the concrete implementation; no live permission is granted by this draft."
decision-trigger: spec
issues: []
schema: brief-v2
version: 2
id: 29c71b10-6ca6-4594-bc10-1f294a5d1463
authored: "2026-10-15 by reporting authoring session"
sources:
  - "2026-10-16 wave reconciliation at origin/main 9f9bb35de: performance-collection/02 consumes desktools-v2/61; reporting scope and historical Evidence unchanged"
  - docs/streams/performance-collection/README.md
  - docs/streams/continuing-operations/spec.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - "freshness-checked 2026-10-15 at public main d430e8f18"
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
---

# Brief 07 — On-demand three-report bundle and thin skill

## Context

files: `perfdata/report/` (planned), `perfdata/cmd/perfdata/` (planned), `perfdata/testdata/operating-reports/` (planned), `plugins/assay/skills/operating-report/SKILL.md` (planned), `docs/performance-collection.md` (planned), `docs/contracts.md`, `changelog/operating-reports.md` (planned)

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

Default if no answer: none — acceptance/activation remains held; offline implementation can proceed.

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

## Evidence

<!-- Independent verifier: revision, environment, command, exit, observed evidence/date/runner. -->

## Review

Gate: human. Verify evidence and lower-boundary failure cases; acceptance does not activate a destination, cadence, credential, policy change or merge permission.
