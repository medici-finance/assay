---
stream: statusgen
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
board: generated
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

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [30-day statusgen check-firing audit — retire cold --lint rules](brief-01-lint-firing-audit.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 02 | [Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk](brief-02-issue-metrics.md) | 1 | L | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 03 | [Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched](brief-03-self-improvement-metric.md) | 2 | M | implemented | — | — |
| 04 | [Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck](brief-04-ladder-position-indicator.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 05 | [Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable)](brief-05-drives-phase3-floors-critical-tier.md) | 1 | L | implemented | — | — |
| 06 | [Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked](brief-06-findings-register-state-machine.md) | 1 | L | implemented | — | — |
| 07 | [New brief-flow metrics in statusgen](brief-07-brief-flow-metrics.md) | 1 | L | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #306 @ 4f37b243efb70e1b1d3e726bc4019967ad64ad99) |
| 08 | [Composite AssayScore computation](brief-08-assayscore-computation.md) | 2 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-05 assay-reviewer-app[bot] (approved PR #403 @ 8c10c90a0ed62049f45f647e7dad20aedcb212e1) |
| 09 | [Opt-in statusgen telemetry — anonymized fleet-drift corpus (off by default)](brief-09-optin-telemetry.md) | 1 | M | implemented | — | — |
| 10 | [statusgen graph export — derived-only DOT + JSONL from the existing parse tree, evaluated on real multi-hop questions](brief-10-graph-export.md) | 1 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #318 @ 6ab8de53a40c1a4f71fa6c0a0ddccb4b27a000c8) |
| 11 | [DORA/insights hybrid — Apache DevLake for commodity metrics, our methodology metrics retained](brief-11-devlake-hybrid-metrics-split.md) | 1 | L | verified | 2026-09-11 opus-4.8[1m]-verifier (assay 553dc2ae; rows 1-7 PASS, docs landed #574; 2026-09-01 FAIL superseded) | — |
| 12 | [`homed-in: <owner/repo>` brief field — exclude a brief whose deliverable lives in another repo from THIS board's Next-up, keep its tracking row, carry the target repo](brief-12-homed-in-field.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #404 @ 894d5e5f73ce417aa49c55134d10db3dc3675cfb) |
| 13 | [Cadenced roadmap artifacts — `--cadence weekly\|monthly` window computation reusing the roadmap renderer, a `theme:` render rule, config-driven priority order and brand](brief-13-cadenced-roadmap-artifacts.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #409 @ 3b022c17ea158700be8cfab679d1719c75afb7a4) |
<!-- statusgen:briefs:end -->

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
