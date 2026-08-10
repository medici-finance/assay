---
brief: methodology-metrics/16
title: 'DORA time series — --dora --series buckets CFR/frequency/lead-time per ISO week'
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction, for methodology/34's lead number)
sources: ["human:<name> 2026-07-10: track change-failure rate over time — better than a snapshot", "tools/statusgen --dora (mm/02, the emitter being extended)", "tools/statusgen --trend (mm/03 — the ISO-week bucketing to reuse)", "methodology/34 (the article that quotes the series)", "freshness-checked 2026-07-10 @ post-#226 main"]
why: >-
  A single change-failure-rate snapshot (69%, partial proxy) is dominated by the ungated
  greenfield era and reads as a static indictment; the TREND — failure rate per week across
  the pre-methodology and gated eras — is the thesis in one picture and the honest way to
  quote the number publicly (methodology/30 leads with it). The emitter has the window
  start (--since) but no bucketing; --trend already buckets by ISO week.
---

# Brief 16 — DORA time series

## Context
files: `../assay-toolkit/statusgen/` (the --dora emitter, mm/02; reuse --trend's ISO-week bucketing,
mm/03), tests alongside the existing dora/trend tests
facts:
- New flag composition: `--dora --series` (respects the existing `--week`/`--day` bucket
  flags and `--since`) emits per-period rows instead of one aggregate: period, merged PRs,
  commits, new bug issues, CFR proxy (bugs÷merged, with the same "partial" label the
  aggregate carries), and median PR open→merge lead time for PRs merged in that period.
  `--json` composes (array of period objects).
- Same data sources as the aggregate (gh PR mergedAt, issue createdAt, git log) — bucketed,
  not re-derived; the Goodhart header prints once above the series, unchanged.
- Brief lead-time (implemented→done) per period comes from the historian log where
  available; periods with <3 data points print `–` rather than a misleading median (small-n
  honesty — same discipline as the aggregate's "unknown" rows).
- Rendering: aligned table like --trend's, plus the existing spark-bar row style for CFR so
  the era contrast is visible in a terminal. No graphing dependencies.
- Consumer: methodology/34's article quotes the series (regenerated at draft time); the
  [I-28](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-loop-monitoring-dashboard-a-wip-website-over-the-standing.md) dashboard later renders it as a widget (note only, no dashboard work here).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `--series` per facts; reuse (do not duplicate) the trend bucketer and the
   dora data collection.
2. Tests: bucket boundary correctness (ISO week edges), small-n `–` suppression, --json
   shape, --since respected, aggregate mode unchanged.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-2 cases |
| 2 | `statusgen --root . --dora --series \| head -20` | per-week rows incl. a CFR column; Goodhart header once |
| 3 | `statusgen --root . --dora --series --json \| head -c 200` | valid JSON array of period objects |
| 4 | `statusgen --root . --dora \| head -5` | aggregate mode byte-identical in shape to pre-change |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verified 2026-07-13 by `opus-verifier` (non-implementer, verify desk) against merged main `a7f48277`.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok github.com/medici/statusgen 1.737s` — Task-2 cases present and passing (`TestDoraSeriesWeeklyBuckets`, `…SmallNSuppress`, `…JSONShape`, `…SinceRespected`, `…AggregateUnchanged`, `…TextRenderHasSparkBar`, `…RunRejectsBadSince`; 10 series tests) |
| 2 | `go run ./tools/statusgen --root . --dora --series \| head -20` | 0 | 5 per-week rows with a CFR column, e.g. `2026-07-06  227  815  74  33% (partial)  1.2h  21.4h`; `grep -c Goodhart` = `1` (header once); spark-bar `▁▁█▁▁` renders |
| 3 | `go run ./tools/statusgen --root . --dora --series --json \| head -c 200` | 0 | valid JSON array; `jq -e 'type=="array" and (.[0]\|type=="object")'` → `true`; keys `period, merged_prs, commits, bug_issues, cfr, pr_lead_time, brief_lead_time` |
| 4 | `go run ./tools/statusgen --root . --dora \| head -5` | 0 | **byte-identical** to pre-change, not merely shape-identical: built the binary at `fe9df685^` (implementing commit `fe9df685`, PR #319) and diffed old vs new against the same worktree — no output, 16 lines each |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | `0` |

Substance: buckets are ISO Mondays, contiguous, ascending, no gaps; the current partial week appears
exactly once. Series sums reconcile exactly to the aggregate (merged 242 = 242, commits 1724 = 1724,
bugs 104 = 104), so no double-count and no drop. `--since` correctly truncates the leading bucket.

**VERIFY: PASS** — all 5 rows exit 0 on merged main.

Advisory (recorded as INTAKE [I-40](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-13-dora-cfr-renders-above-100-percent-unpaired-numerator.md), not a verify failure): the CFR can render **362%**, which is not a
rate — week `2026-06-29` divides 29 bugs created by 8 PRs merged, an unpaired numerator/denominator
left over from the direct-to-main era. Small-n suppression guards divide-by-zero but not
small-nonzero denominators, and the spark-bar caption drops the `(partial)` proxy label the brief's own
Review gate requires in *every* rendering. In-spec as written; a publication hazard for a tool whose
purpose is the honest public number.

## Review
Gate: model. Reviewer confirms the bucketing is reused from --trend (not re-implemented),
small-n periods suppress rather than mislead, and the partial-proxy label survives into
every rendering of the CFR column.
