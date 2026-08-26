---
brief: statusgen/07
title: New brief-flow metrics in statusgen
wave: 1
depends: []
unblocks: ["statusgen/08"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 (re-authored clean for the statusgen board)
sources:
  - "The settled metric-definitions spec — per-metric compute recipes + the AssayScore roll-up these numbers feed"
  - "A three-angle synthesis of per-metric compute feasibility"
why: >-
  These are the objective, business-value numbers that replace commits/merges/PRs and feed the
  AssayScore — weighted throughput, lead time by size, flow efficiency, first-pass yield,
  review-rework, decision latency, per-stream stall. They must be computed by the same statusgen
  binary as the existing instruments so a published page has one provenance-checked source.
---

# Brief 07 — New brief-flow metrics in statusgen

## Context
files: `statusgen/` (new metric functions beside `bottleneck.go`, `trend.go`, `dora.go`,
  `awaitage.go`), `docs/streams/.history.jsonl`, brief frontmatter, `gh` PR reviews.

Builds on prior statusgen instrument work — the existing DORA lead-time machinery, the trend
series, and the bottleneck WIP model — which this extends rather than forks.

facts:
- weights S=1·M=3·L=8; issue-loop segmented from authored streams
- lead time uses `authored:` (day precision) + historian `to:"done"`
- review-rework computes from the `gh pr view --json reviews` ARRAY (not the laundered latest view)
- flow efficiency needs a work-start event for honest touch/wait; ship a proxy if that event is thin
- decision-latency uses the canonical decision-queue definition (the same one the anti-starvation
  floor work relies on)
- three-state discipline: thin/absent data → could-not-check, never 0

## Ground rules
- NEVER git push. Stop at `implemented`.
- EXTEND existing statusgen instruments; do not fork parallel tooling. Reuse the `--dora` lead-time,
  the `--trend` series, the `--bottleneck` WIP.
- Every metric emits a `--json` mode with a three-state `state` field for the page.
- Report NEEDS_CONTEXT if a metric's data source is absent rather than emitting a fabricated 0.

## Task
Implement, each as a statusgen subcommand/flag with `--json`:
1. **Weighted brief throughput** — Σ effort-points of `to:"done"` in window; issue-loop segmented.
2. **Lead time authored→done, by size** — median + p85 per S/M/L; report `n` alongside.
3. **Flow efficiency** — touch ÷ (touch+wait) from historian dwell + PR commit span (+ work-start
   events where present); emit `could-not-check` until enough post-instrumentation data exists.
4. **First-pass yield** — brief merged with 0 CHANGES_REQUESTED ∧ no verify-fail ∧ no finding names it.
5. **Review-rework rounds** — CHANGES_REQUESTED cycles/PR distribution (from the reviews array).
6. **Decision latency + WIP + oldest** — per the canonical decision-queue definition.
7. **Per-stream net flow + stall flag** — arrivals − completions; stall = active ∧ backlog>0 ∧ no
   transition ≥14d.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go run . --root .. --throughput --json 2>/dev/null \| python3 -c "import json,sys;d=json.load(sys.stdin);print(d.get('state'))"` | exit 0; prints `ok` or `could-not-check` |
| 2 | `cd statusgen && go run . --root .. --leadtime --by-size --json 2>/dev/null \| grep -Eq -e 'S' -e 'M' -e 'L'` | exit 0 (size-split present) |
| 3 | `cd statusgen && go run . --root .. --review-rework --json 2>/dev/null \| head -c1` | exit 0 (emits JSON) |
| 4 | `cd statusgen && go run . --root .. --decision-latency --json 2>/dev/null \| head -c1` | exit 0 |
| 5 | `cd statusgen && go test ./... 2>&1 \| tail -1` | contains `ok` (unit tests for each new metric, incl. a could-not-check case) — DEREFERENCES that the compute is correct, not just present |
| 6 | `cd statusgen && go run . --root .. --lint` | exit 0 (`LINT: PASS`) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model. Reviewer confirms each metric reuses existing instruments where possible, emits three-state
JSON, and has a unit test including a thin-data (`could-not-check`) case. Verdict + date in the README.
