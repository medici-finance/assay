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
| 01 | [30-day lint-firing audit — retire cold rules](brief-01-lint-firing-audit.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 02 | [issue metrics (`--issues`)](brief-02-issue-metrics.md) | 1 | L | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 03 | [self-improvement metric (self-healed vs human-touched)](brief-03-self-improvement-metric.md) | 2 | M | implemented | — | — |
| 04 | [ladder-position indicator (`--ladder`)](brief-04-ladder-position-indicator.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 05 | [drives phase 3 — anti-starvation floors + critical tier](brief-05-drives-phase3-floors-critical-tier.md) | 1 | L | implemented | — | — |
| 06 | [findings register — corroborated state machine](brief-06-findings-register-state-machine.md) | 1 | L | implemented | — | — |
| 07 | [new brief-flow metrics](brief-07-brief-flow-metrics.md) | 1 | L | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #306 @ 4f37b243efb70e1b1d3e726bc4019967ad64ad99) |
| 08 | [composite AssayScore computation](brief-08-assayscore-computation.md) | 2 | M | implemented | — | — |
| 09 | [opt-in telemetry — anonymized fleet-drift corpus (off by default)](brief-09-optin-telemetry.md) | 1 | M | implemented | — | — |
| 10 | [graph export (`--graph` DOT + JSONL)](brief-10-graph-export.md) | 1 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #318 @ 6ab8de53a40c1a4f71fa6c0a0ddccb4b27a000c8) |
| 11 | [DORA/insights hybrid — DevLake commodity split](brief-11-devlake-hybrid-metrics-split.md) | 1 | L | implemented | — | — |
| 12 | [`homed-in: <owner/repo>` — exclude a re-homed brief from THIS board's Next-up, keep its tracking row, carry the target repo](brief-12-homed-in-field.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #404 @ 894d5e5f73ce417aa49c55134d10db3dc3675cfb) |
| 13 | [cadenced roadmap artifacts (`--cadence weekly/monthly`)](brief-13-cadenced-roadmap-artifacts.md) | 1 | M | implemented | — | — |

## Critical path
statusgen/02 (issue metrics) → statusgen/03 (self-improvement metric). The
self-improvement classifier extends the `--issues` infrastructure, so 02 leads 03.
statusgen/07 (brief-flow metrics) → statusgen/08 (composite AssayScore): the score rolls up
the brief-flow metrics, so 07 leads 08. statusgen/13 (cadenced roadmap artifacts) reuses the
landed `--roadmap` renderer over a computed window — independent, no new critical-path edge.
Every other brief is independent and self-contained.

## Dependency waves
- **Wave 1** — statusgen/01, statusgen/02, statusgen/04, statusgen/05, statusgen/06,
  statusgen/07, statusgen/09, statusgen/10, statusgen/11, statusgen/12, statusgen/13 (all
  independent; parallelizable).
- **Wave 2** — statusgen/03 (depends on statusgen/02), statusgen/08 (depends on statusgen/07).

## Conventions
- `statusgen --lint-audit` reports 30-day per-rule firing counts; COLD (0-firing,
  un-tested) rules are retirement candidates — retirement stays a human call. Implemented
  by statusgen/01, PR #78.
