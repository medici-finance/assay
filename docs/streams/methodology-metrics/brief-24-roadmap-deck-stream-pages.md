---
brief: methodology-metrics/24
title: 'Roadmap deck pages 2..N — one identical-skeleton page per stream: delta panel first, wave ladder, computed blockers'
wave: 2
depends: ["methodology-metrics/23", "methodology-metrics/26"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (human:<name> direction; design basis researched same day)
sources: ["human:<name> 2026-07-12: 'b) for each stream show the briefs in them' (same directive as mm/23)", "docs/streams/methodology-metrics/roadmap-deck-research.md (design basis — REQUIRED READING; per-stream skeleton is its §'two-page-tier structure')", "methodology-metrics/23 (page 1 + the shared roadmap.go/health-rule table this extends)", "methodology-metrics/01 (historian — the since-yesterday panel's source)", "freshness-checked 2026-07-12 @ post-#365 main"]
why: >-
  The overview answers "where is the portfolio constrained"; the exec/PM follow-up is always
  "show me THAT stream". Research (exec reading order) says the second thing read after
  status is what-materially-changed — so each stream page leads with its delta panel, and
  every page shares one skeleton so daily readers build the WBR familiarity that makes
  anomalies pop. Waves render as an ordinal ladder, never a time axis (Gantt fakes precision
  our dependency tiers don't carry).
---

# Brief 24 — Roadmap deck: per-stream pages

## Context

files: `../assay-toolkit/statusgen/` (extends mm/23's roadmap.go), output pages under
`docs/reports/roadmap/` (planned) — either anchored sections of index.html or
`<stream>.html` siblings; implementer picks ONE and records it in Evidence.

facts:
- Skeleton per page, identical across streams (order fixed, matching page-1 grid order):
  1. header band: stream · owner · health pill + printed rule (SAME computation as page 1 —
     one Go table, never two) · one-line outcome (first sentence of the stream README's
     opening paragraph) · `x done / y total` + stage bar;
  2. "since yesterday" delta panel FIRST: stage transitions (historian, 24h), new briefs,
     merged PRs touching the stream, findings opened/resolved; empty state = one line
     "no changes";
  3. blockers & asks: computed rows from the typed graph (brief X blocked-by stream/NN not
     done; unresolved FINDING affecting a brief) in issue → effect → action form; any
     hand-authored ask from the stream README renders visually distinct (asserted ≠ computed);
  4. next wave gate: "Wave N unlocks when <briefs> reach verified" — from the depends graph;
  5. brief table grouped by wave: id/title linked, effort, lifecycle token, days-in-stage,
     Δ badge, blocked-by refs; fully-done waves collapse to one summary line so 25-brief
     streams hold a page;
  6. per-stream DORA tile (mm/26 `--dora --by stream --json`): lead time median/p90,
     deploy freq, CFR proxy, each with its n= annotation — small-n honesty rendered, not
     hidden;
  7. footer legend generated from the mm/23 rule table.
- Completed/archived streams (docs/archive/) are excluded; the overview's stream set is the
  page set — no page without a grid row, no row without a page (tested invariant).
- Same brand/self-containment/determinism constraints as mm/23.
- consumers (rule 6): mm/23's index (cross-links row → page), mm/22 harvest (ships the whole
  docs/reports/roadmap/ directory once wired), the-desk boot read.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Extend `--roadmap` to emit the per-stream pages per the skeleton above; page-1 grid rows
   link to their stream page.
2. Row↔page invariant test; delta-panel fixture test (historian log → rendered panel);
   collapsed-done-wave test on a large synthetic stream.
3. Keep output deterministic (byte-identical re-run on same state).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --roadmap && grep -c 'stream-page' docs/reports/roadmap/index.html` | ≥ 1 (page-1 rows link to stream pages) |
| 2 | active-stream count equals rendered page count: `diff <(count of active streams from docs/streams/*/README.md) <(count of stream pages)` — implementer scripts the literal form | exit 0 |
| 3 | `grep -c "since yesterday\|no changes" <one stream page>` | ≥ 1 (delta panel present with explicit empty state) |
| 4 | `go test ./tools/statusgen/ -run Roadmap -count=1` | exit 0 |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
