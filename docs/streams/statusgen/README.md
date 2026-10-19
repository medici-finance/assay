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

## Portfolio extension path — 13 October 2026

See the [contract and delivery amendment](../../proposals/non-ci-architecture/26-product-intent-and-portfolio.md).
The merged /19 contract and its Evidence are unchanged. /22 supplies product records; /23 extends /20 with bindings;
/24 supplies capacity accounting; /25 composes their views with continuing-operations/02.
/21 consumes /23 after both contracts ship. Dependency waves are 1: /22, 2: /24 and
operations/01, 3: /23 and operations/02, 4: /21 and /25 (alongside the existing /19→/20 path).
Operations admission remains a blocker for /25; the verifier runtime does not wait on this lane.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [30-day statusgen check-firing audit — retire cold --lint rules](brief-01-lint-firing-audit.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 02 | [Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk](brief-02-issue-metrics.md) | 1 | L | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 03 | [Self-improvement metric — loops that self-diagnose AND self-resolve (agent-raised + agent-fixed, no human touch) vs human-touched](brief-03-self-improvement-metric.md) | 2 | M | done | 2026-09-15 sonnet-5-verifier | 2026-09-15 assay-reviewer-app[bot] (approved PR #1137 @ b9aed2c3fe00690c94043a16e30a9d6b5e96125e) |
| 04 | [Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck](brief-04-ladder-position-indicator.md) | 1 | S | done | 2026-08-26 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #156 @ 112b206fee74b470016be325dc7c2dfeff670931) |
| 05 | [Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable)](brief-05-drives-phase3-floors-critical-tier.md) | 1 | L | implemented | — | — |
| 06 | [Findings register becomes a corroborated state machine — bounded shelving (parked) + transition guard on resolved/affects/parked](brief-06-findings-register-state-machine.md) | 1 | L | implemented | — | — |
| 07 | [New brief-flow metrics in statusgen](brief-07-brief-flow-metrics.md) | 1 | L | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #306 @ 4f37b243efb70e1b1d3e726bc4019967ad64ad99) |
| 08 | [Composite AssayScore computation](brief-08-assayscore-computation.md) | 2 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-05 assay-reviewer-app[bot] (approved PR #403 @ 8c10c90a0ed62049f45f647e7dad20aedcb212e1) |
| 09 | [Opt-in statusgen telemetry — anonymized fleet-drift corpus (off by default)](brief-09-optin-telemetry.md) | 1 | M | implemented | — | — |
| 10 | [statusgen graph export — derived-only DOT + JSONL from the existing parse tree, evaluated on real multi-hop questions](brief-10-graph-export.md) | 1 | M | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #318 @ 6ab8de53a40c1a4f71fa6c0a0ddccb4b27a000c8) |
| 11 | [DORA/insights hybrid — Apache DevLake for commodity metrics, our methodology metrics retained](brief-11-devlake-hybrid-metrics-split.md) | 1 | L | done | 2026-09-11 opus-4.8[1m]-verifier (assay 553dc2ae; rows 1-7 PASS, docs landed #574; 2026-09-01 FAIL superseded) | 2026-09-11 assay-reviewer-app[bot] (approved PR #860 @ 4e50f17e2722421e97f6880606baf6f8fadccc5e) |
| 12 | [`homed-in: <owner/repo>` brief field — exclude a brief whose deliverable lives in another repo from THIS board's Next-up, keep its tracking row, carry the target repo](brief-12-homed-in-field.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #404 @ 894d5e5f73ce417aa49c55134d10db3dc3675cfb) |
| 13 | [Cadenced roadmap artifacts — `--cadence weekly\|monthly` window computation reusing the roadmap renderer, a `theme:` render rule, config-driven priority order and brand](brief-13-cadenced-roadmap-artifacts.md) | 1 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #409 @ 3b022c17ea158700be8cfab679d1719c75afb7a4) |
| 14 | [`--lint`: flag a Verify row whose `go test -run` selector can pass on "no tests to run" (vacuous-selector rule)](brief-14-vacuous-gotest-run-lint.md) | 1 | M | done | 2026-09-30 assay-verifier-app[bot] @ 43420f7ecd74 (claude-opus-5-5) | 2026-09-30 assay-reviewer-app[bot] (approved PR #1659 @ f0355f1b57794105b59a1408522a44c7de59eb93) |
| 15 | [Extract canonical eligibility and coverage evaluation API](brief-15-extract-canonical-eligibility-and-coverage-evaluation-api.md) | 1 | L | implemented | — | — |
| 16 | [Hold an issue out of dispatch when a non-human removed its excluded label: both scanners hold the placeholder, the issue board holds the un-briefed row](brief-16-label-removal-hold.md) | 1 | L | implemented | — | — |
| 17 | [`--scan-issues`: a `risk:high` label derives `gate: human` on the issue's placeholder (`risk:med` does not)](brief-17-risk-high-label-gate.md) | 1 | S | implemented | — | — |
| 18 | [`--scan-issues`: print a notice when an open issue derives `gate: human` and its placeholder reads `gate: model` (detect only, no write)](brief-18-gate-mismatch-notice.md) | 2 | S | todo | — | — |
| 19 | [Context handoff contracts and synthetic fixtures](brief-19-context-handoff-contracts.md) | 1 | M | implemented | — | — |
| 20 | [Offline future-state resolver with receipt and amendment validation](brief-20-offline-context-resolver.md) | 2 | L | todo | — | — |
| 21 | [Author skills consume the shared context handoff](brief-21-author-skill-handoff.md) | 4 | M | todo | — | — |
| 22 | [Product target records and compatible requirement parsing](brief-22-product-target-records-and-compatible-requirement-parsing.md) | 1 | M | implemented | — | — |
| 23 | [Product target binding and context drift assessment](brief-23-product-target-binding-and-context-drift-assessment.md) | 3 | M | todo | — | — |
| 24 | [Portfolio allocation and capacity accounting](brief-24-portfolio-allocation-and-capacity-accounting.md) | 2 | M | todo | — | — |
| 25 | [Read-only portfolio and drift projections](brief-25-read-only-portfolio-and-drift-projections.md) | 4 | M | todo | — | — |
| 26 | [Extend statusgen phase measurements through the shared performance recorder](brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md) | 2 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path
statusgen/02 (issue metrics) → statusgen/03 (self-improvement metric). The
self-improvement classifier extends the `--issues` infrastructure, so 02 leads 03.
statusgen/07 (brief-flow metrics) → statusgen/08 (composite AssayScore): the score rolls up
the brief-flow metrics, so 07 leads 08. statusgen/13 (cadenced roadmap artifacts) reuses the
landed `--roadmap` renderer over a computed window — independent, no new critical-path edge.
statusgen/14 (the vacuous `go test -run` Verify-row rule, issue #1581 part 5) adds one
advisory rule to the existing Verify-row lint — independent, no new critical-path edge.
statusgen/17 (`risk:high` gates a placeholder at first write) → statusgen/18 (the scan's notice
for a placeholder whose gate the issue has outgrown): 18 reports on every label and title word
that derives `human`, the label 17 adds among them, and extends the skill sentence 17 writes,
so 17 leads 18.
statusgen/19 (context handoff contracts) → statusgen/20 (offline resolver with receipt and
amendment validation) → statusgen/21 (author skills consume the handoff): the resolver
produces the 19 records and the skills may call only the shipped 20 verb, so 19 leads 20.
The portfolio extension adds typed edges (see the
[contract and delivery amendment](../../proposals/non-ci-architecture/26-product-intent-and-portfolio.md)):
statusgen/22 (product-target records) → statusgen/24 (allocation and capacity ledger), and
22 → continuing-operations/01 → continuing-operations/02; statusgen/23 (product binding)
needs 22 and 20; statusgen/21 now needs 20 and 23; statusgen/25 (read-only projections)
needs 23, 24 and continuing-operations/02. With /19 merged, the head of the
`20 → 23 → 25` path is statusgen/20 (plus /19's independent verification).
Every other brief is independent and self-contained.

## Dependency waves
- **Wave 1** — statusgen/01, statusgen/02, statusgen/04, statusgen/05, statusgen/06,
  statusgen/07, statusgen/09, statusgen/10, statusgen/11, statusgen/12, statusgen/13,
  statusgen/14, statusgen/15, statusgen/19, statusgen/22 (all independent; parallelizable).
- **Wave 2** — statusgen/03 (depends on statusgen/02), statusgen/08 (depends on statusgen/07),
  statusgen/18 (depends on statusgen/17), statusgen/20 (depends on statusgen/19),
  statusgen/24 (depends on statusgen/22; cross-stream: continuing-operations/01 is also wave 2).
- **Wave 3** — statusgen/23 (depends on statusgen/20 and statusgen/22; cross-stream:
  continuing-operations/02, depending on continuing-operations/01, is also wave 3).
- **Wave 4** — statusgen/21 (depends on statusgen/20 and statusgen/23), statusgen/25 (depends on
  statusgen/23, statusgen/24 and continuing-operations/02).

## Conventions
- `statusgen --lint-audit` reports 30-day per-rule firing counts; COLD (0-firing,
  un-tested) rules are retirement candidates — retirement stays a human call. Implemented
  by statusgen/01, PR #78.

## Library-first work — 2026-10-08

New work: `statusgen/15` extracts the evaluation API independently of forge reads.
`graph-execution/19` consumes it; no workflow-store dependency flows back into extraction.
See [the component contract](../../library-first.md). Existing completed history is unchanged.

## Non-CI architecture reconciliation — 2026-10-13

The maintainer ratified the non-CI verifier target (medici-finance/assay#2500, revision
23f655f5d); approval activates nothing. This stream owns the delivery-sequence step-2 package
(shared context handoff and authoring support): `statusgen/19` contracts, `statusgen/20`
resolver and receipt/amendment validation, `statusgen/21` skill integration with release and
adoption notes. Until 20 ships and is pinned, the bounded manual bootstrap packet remains the
documented path. Owner rows for every package: the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md).
Existing completed history is unchanged.

## Performance reuse — 2026-10-16

Statusgen/26 extends the already-shared perfrec recorder after desktools-v2/61 (wave 2).
It measures discovery, parsing, evaluation, rendering and the existing deskread child;
invocation-local snapshot reuse must match fresh evaluation. It has no dependency on
the online mirror, perfdata storage or a database and acquires no credential. Context
resolver /20 retains independent membership enumeration and exact receipt semantics.
The desktools-v2 README holds the cross-stream performance priority and critical path.
