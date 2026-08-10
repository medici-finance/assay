---
brief: methodology-metrics/03
title: statusgen --trend — the SCADA historian view over time
wave: 1
depends: ["methodology-metrics/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping)
sources: ["[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA/OODA — statusgen --trend historian)", "docs/streams/methodology/scada-ooda-lineage.md", "methodology-metrics/01 (historian)"]
---

# Brief 03 — `statusgen --trend`

## Context
files: `../assay-toolkit/statusgen/` (new `--trend` subcommand), reads `methodology-metrics/01`'s history log.
facts:
- [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA) names a `statusgen --trend` **historian view** — the process-control "historian" that
  shows state over time, not just now. The 01 log is the raw historian; this is the read/rollup.
- Useful trends: briefs per status over time (the todo→...→done funnel moving), throughput per week,
  the Awaiting-verification backlog size over time (the lead-time debt curve), findings opened vs
  resolved. Terminal-friendly (sparkline/ascii) since STATUS.md is text.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add `statusgen --trend [--since <date>] [--weekly|--daily] [--history <path>]`: roll up the 01
   history log into a time series — status counts per period, throughput (transitions into
   `done`/merged), and the Awaiting-verification backlog curve. Render as compact ascii
   (sparklines/small tables); text only. `--history` overrides the default
   `docs/streams/.history.jsonl` (points at any snapshot; also lets Verify exercise a short log
   without mutating the live one). Default period is weekly.
2. Surface the lead-time debt explicitly: the standing count of briefs at `implemented`/`verified`
   (merged-but-not-done) over time — the curve the verify-desk is meant to bend down.
3. Empty/short history degrades gracefully (says "insufficient history", not a crash).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Trend` | exit 0 (rollup tests over a multi-week testdata history) |
| 2 | `statusgen --trend --since 2026-07-01 --weekly` | prints per-week status/throughput series + the Awaiting-backlog curve, exit 0 |
| 3 | `statusgen --trend --history tools/statusgen/testdata/trend/onerow.jsonl` | prints "insufficient history" (no crash), exit 0 |
| 4 | `go vet ./tools/statusgen/ && statusgen --lint` | exit 0 |

<!-- amend (methodology-metrics/03 impl): item 3 gained an explicit --history fixture path. The
     original `--trend`-with-no-path can only yield "insufficient history" against a ≤1-entry log,
     but the live docs/streams/.history.jsonl already holds 150+ transitions; the --history override
     (Task item 1) points the check at a 1-row fixture so it is deterministically runnable without
     touching the single-writer live log. TestTrendRunInsufficientFixture covers the same path. -->


## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run Trend` | 0 | 9 Trend tests PASS | 2026-07-10 | opus-verifier |
| 2 | `go run ./tools/statusgen --trend --since 2026-07-01 --weekly` | 0 | per-week status/throughput series + awaiting-backlog curve, 230 transitions | 2026-07-10 | opus-verifier |
| 3 | `--trend --history <onerow.jsonl>` | 0 | graceful `insufficient history — need at least two…` (no crash) | 2026-07-10 | opus-verifier |
| 4 | `go vet && --lint` | 0 | clean | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — trend series render; insufficient-history handled gracefully.

## Review
Gate: model. Reviewer records verdict + date in the stream README.
