---
stream: statusgen
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
---

# statusgen Stream

The planning board for `statusgen` itself — the tool that reads these streams and
generates the `STATUS.md` board. `statusgen`'s source lives in this repo (`statusgen/`),
so its own planning lives here too, alongside the code it plans.

Briefs are self-contained `brief-NN-*.md` files; each carries its own scope, rules,
task, and an executable Verify table. The opening set of briefs is statusgen's own
open planning work — issue and self-improvement metrics, the adoption-ladder indicator,
the drives anti-starvation floors + critical tier, the lint-firing audit, and the
findings-register state machine.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [30-day lint-firing audit — retire cold rules](brief-01-lint-firing-audit.md) | 1 | S | implemented | — | — |
| 02 | [issue metrics (`--issues`)](brief-02-issue-metrics.md) | 1 | L | implemented | — | — |
| 03 | [self-improvement metric (self-healed vs human-touched)](brief-03-self-improvement-metric.md) | 2 | M | todo | — | — |
| 04 | [ladder-position indicator (`--ladder`)](brief-04-ladder-position-indicator.md) | 1 | S | implemented | — | — |
| 05 | [drives phase 3 — anti-starvation floors + critical tier](brief-05-drives-phase3-floors-critical-tier.md) | 1 | L | implemented | — | — |
| 06 | [findings register — corroborated state machine](brief-06-findings-register-state-machine.md) | 1 | L | todo | — | — |

## Critical path
statusgen/02 (issue metrics) → statusgen/03 (self-improvement metric). The
self-improvement classifier extends the `--issues` infrastructure, so 02 leads 03. Every
other brief is independent and self-contained.

## Dependency waves
- **Wave 1** — statusgen/01, statusgen/02, statusgen/04, statusgen/05, statusgen/06
  (all independent; parallelizable).
- **Wave 2** — statusgen/03 (depends on statusgen/02).

## Conventions
- `statusgen --lint-audit` reports 30-day per-rule firing counts; COLD (0-firing,
  un-tested) rules are retirement candidates — retirement stays a human call. Implemented
  by statusgen/01, PR #78.
