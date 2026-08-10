---
brief: metrics-harvest/02
title: Cross-domain aggregator — per-domain totals + all-products roll-up over the harvest tree
wave: 1
depends: ["metrics-harvest/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-20 by Opus session (human:<name> direction via desk)
sources: ["docs/streams/INTAKE.md I-07 (human:<name> direction 2026-07-20: 'once that''s done we need something to aggregate it')", "metrics-harvest/01 (produces the reports/daily/<date>/<domain>/<repo>/ tree this brief rolls up)", "docs/desk-console-design.md (Measures pane — the eventual consumer of the cross-domain roll-up)"]
---

# Brief 02 — Cross-domain aggregator

## Context
files:
- `tools/metrics-harvest/` (extends the brief-01 tool — a subcommand/flag, e.g.
  `metrics-harvest aggregate` or a sibling `--aggregate`, in the same Go module)
- `reports/daily/<YYYY-MM-DD>/` (reads brief-01's `<grouping>/<repo>/` artifacts; writes the
  roll-up beside them)

facts:
- **Input = brief-01's tree, unchanged.** The aggregator is a pure reader/reducer over
  `reports/daily/<date>/<grouping>/<repo>/{prs.json,issues.json,git.txt,throughput.json}` for
  all four groupings (`assay`/`plumb`/`lending`/`org`). It adds NO new `gh`/`git` reads — it is
  AI-free arithmetic over already-harvested data (the generation-vs-analysis split). It must
  tolerate a grouping/repo whose capture failed (brief-01 fail-loud artifact) by counting it
  as a gap, not crashing.
- **Auth (human:<name> ruling 2026-07-20)**: the aggregator uses the **all-covering `metrics-harvest-all`
  App** — the only identity that spans every product + the org-wide repos. It needs read only
  to fill gaps (its input is normally the already-committed tree); still no product-scoped App
  can aggregate cross-product.
- **Two output tiers**, written under the same dated dir:
  - `reports/daily/<date>/<grouping>/rollup.json` — per-grouping totals (for each of the four
    groupings incl. `org`): open-PR count (+ draft vs ready, + review-decision breakdown),
    open-issue count (+ label breakdown), commits-to-main for the day, derivable
    velocity/throughput summed across the grouping's repos.
  - `reports/daily/<date>/rollup.json` (+ a human-readable `rollup.md`) — the cross-domain
    roll-up: the **three products side by side plus an all-products total**, and the **`org`
    grouping surfaced at the top level as a distinct section** (org-wide, NOT summed into the
    product total — human:<name> ruling), plus **trends** (day-over-day deltas) where a prior day's
    roll-up exists.
- **Trends need history.** Day-over-day deltas require the previous day's roll-up; when it is
  absent, emit the absolute figures and mark the trend fields `null`/`n/a` (never fabricate a
  delta). A ≥2-day window is what makes the trend columns meaningful — noted, not asserted as
  a hard precondition.
- **The Desk Console Measures pane is the eventual consumer** (`desk-console/12`, cross-
  stream). The aggregator's JSON is the render source; keep it a stable, documented shape.
- consumers (shared value = the roll-up JSON shape): `desk-console/12` Measures pane
  (follow-up, cross-stream — renders it); the harvest workflow (brief-01) may call the
  aggregate step in the same scheduled run (fixed-here: brief-01's workflow gains one
  aggregate invocation + the roll-up files in its commit).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Aggregate step** in the brief-01 tool: read a date's `<grouping>/<repo>/` artifacts,
   compute per-grouping `rollup.json` (all four groupings) and the top-level `rollup.json` +
   `rollup.md` — three products side by side + an all-products total + a distinct `org`
   section (org-wide, not summed into the product total). Uses the `metrics-harvest-all` App.
   AI-free, deterministic, gap-tolerant.
2. **Trends**: when the prior day's roll-up exists, add day-over-day deltas; otherwise emit
   `null` trend fields — never a fabricated delta.
3. **Wire into the schedule**: the brief-01 workflow runs the aggregate step after the
   harvest and includes the roll-up files in the same `[skip-status-regen]` commit.
4. **Document the roll-up shape** in tools/metrics-harvest/README.md (the Measures-pane
   render contract) and link from the README docs/ index.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/metrics-harvest && go build ./...` | exit 0 |
| 2 | run the aggregate step for a date that has a harvested tree (from metrics-harvest/01), then `jq -e '.products \| length == 3' reports/daily/<that-date>/rollup.json` (or the equivalent product-keys check) | exit 0 (three products in the roll-up) |
| 3 | `jq -e '.products.assay and .products.plumb and .products.lending and .org' reports/daily/<that-date>/rollup.json` (or equivalent keys) | exit 0 (three products + a DISTINCT top-level `org` section, org not summed into the product total) |
| 4 | `jq -e '.openPRs and .openIssues and .commitsToMain' reports/daily/<that-date>/assay/rollup.json` | exit 0 (a per-grouping rollup carries PR flow, issue backlog, velocity fields) |
| 5 | FLOW (trends): run aggregate for two consecutive harvested dates; `jq -e '.trend != null' reports/daily/<later-date>/rollup.json` | exit 0 (day-over-day delta present when history exists) |
| 6 | run aggregate for a date whose prior day is absent; `jq -e '.trend == null' reports/daily/<that-date>/rollup.json` | exit 0 (no fabricated delta without history) |
| 7 | `test -f reports/daily/<that-date>/rollup.md` | exit 0 (human-readable roll-up exists) |
| 8 | `cd statusgen && go run . --root .. --lint` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
