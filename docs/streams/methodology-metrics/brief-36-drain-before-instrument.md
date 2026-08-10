---
brief: methodology-metrics/36
title: Drain-before-instrument — gate new metric/alarm briefs while the queue they measure is over threshold
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [885]
schema: brief-v1
authored: 2026-07-20 by Opus 4.8 authoring session (intake Tier-2, #885)
sources: ["[I-drain-first](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-17-drain-before-instrument-rule.md)", "2026-07-17 adversarial methodology review (overhead-ratio)", "docs/streams/methodology/verify-desk-bottleneck-2026-07-17.md", "tools/statusgen/nextup.go eligible()", "methodology-metrics/10 (the verification-debt threshold this reuses)", "#885 (tracking)"]
why: >-
  Observed reflex: when a process queue backs up, the response is often to build an alarm or
  metric ABOUT the queue rather than drain it. Verification debt sat at 28-40 while metric
  briefs (mm/18 bottleneck report, mm/19 code metrics) stayed Next-up-eligible, and the mm/10
  prose NOTICE was ignored. Instrumentation is not service. Make "drain before you measure the
  drain" mechanical: a metric brief about a breached queue is not eligible until the queue drains.
---

# Brief 36 — Drain-before-instrument eligibility gate

## Context
files:
- `../assay-toolkit/statusgen/nextup.go` — `eligible()` (the Next-up gate) + the pick loop
- `../assay-toolkit/statusgen/parse.go` / `model.go` — brief-v1 frontmatter (`Brief` struct)
- `../assay-toolkit/statusgen/emit.go` — `debtCounts()` (the measured-queue depth + threshold, mm/10)
- `../assay-toolkit/statusgen/{nextup,parse}_test.go` — tests
- `docs/streams/methodology-metrics/README.md` — one convention line

facts:
- Design: an OPT-IN brief-v1 field `measures: <queue-name>` (optional `*string`, absent = not
  an instrumentation brief — the neutral default; mirror the `Tiering *string` present/absent
  pattern already in parse.go/model.go). A `todo` brief with `measures: <q>` is **excluded from
  Next-up eligibility while queue `<q>` exceeds its own alarm threshold**.
- The only queue wired at authoring time is `verification-debt` — its depth + threshold already
  live in `debtCounts`/`verificationDebtThreshold` (emit.go, mm/10). So a brief with
  `measures: verification-debt` is ineligible while the Awaiting queue is over threshold (or,
  post-mm/34, over the desk-actionable threshold — prefer the desk-actionable count if mm/34 has
  landed, else the total; do NOT hard-depend on mm/34). An unknown queue name = a hard lint
  PROBLEM (typo protection), never a silent no-op.
- Boundary (F-09 discipline): ZERO score-input change — this is an eligibility exclusion, exactly
  like claim-awareness (mm/08). Excluded briefs count as held-back in the mm/06 overflow line, the
  same as claimed/serialized ones. When the queue is under threshold the field is inert.
- The gate is a mechanical expression of "drain before you measure the drain" — the fix for a
  breached verification queue is a verify-desk sprint, not another metric brief. It directly
  serves the meta-budget concern (a separate intake item).
- Reads current state (queue depth + brief frontmatter) — wave 0.
- Out of scope: retroactively tagging existing metric briefs with `measures:` (a desk decision,
  done per-brief later); any queue other than verification-debt (additive later — the mechanism
  is generic, only the wired queue is one).

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: parse populates `Brief.Measures`; `eligible()` excludes a
   `measures: verification-debt` todo brief when the queue is over threshold and includes it
   when under; a brief with no `measures:` is unaffected; an unknown queue name is a lint
   PROBLEM.
2. Implement `Measures *string` in parse.go/model.go; the eligibility exclusion in nextup.go
   keyed off `debtCounts` vs threshold; the unknown-queue-name lint check.
3. README: one line under conventions — `measures: <queue>` gates a metric/alarm brief out of
   Next-up while the named queue is over its alarm threshold (drain-before-instrument); only
   `verification-debt` is wired today.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Measures -v` | exit 0; `TestMeasures*` covers over-threshold-excludes, under-threshold-includes, no-field-unaffected, and unknown-name-PROBLEM |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint; echo $?` | 0 (no existing brief carries `measures:`, so nothing is newly excluded or flagged) |
| 4 | `statusgen --root .` (regen local-only, NOT committed) | Next-up unchanged from current tree (field is opt-in; no brief uses it yet) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling; an opt-in eligibility gate,
zero score-input change, inert until a brief adopts the field). Reviewer records verdict + date
in the stream README table.
