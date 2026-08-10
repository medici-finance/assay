---
stream: methodology-metrics
status: active
priority: P1
track: platform
serves: assay
tiering: implement=cheap verify=strong
issues: [156, 217, 282]
---

# Methodology-Metrics Stream — instrument the methodology's own outcomes

**Status**: Plan · **Date**: 2026-07-09 (oit origin) · merged into this repo 2026-08-01 by
`assay-selfcontain/03`
**Scope**: Measure the methodology, don't just run it. Two convergent lenses on the same substrate:
**DORA** (deployment frequency, change lead time, change failure rate, recovery time, rework rate) and
the **SCADA/OODA** observability analysis (I-08) — point quality, alarm KPIs, span-of-control, a trend
historian, and two written invariants, plus the anti-falsification checks that decide whether a
recorded sign-off is real. The 2026-07-09 velocity read exposed why the DORA half matters: we measure
**output** (35 PRs merged / 69 commits in a day) while the **outcome** — lead time to `done` — and all
stability signals are invisible (35 merged, 5 `done`). Almost all of it is Go work in this repo's
`statusgen/` — **canonical here**. That statement is now the decided one, not a local assumption:
the root README carried "Statusgen home" as a still-open decision while this line treated it as
settled, which is the contradiction [#274](https://github.com/medici-finance/assay-toolkit/issues/274)
calls the distribution inversion. human:<name> settled it on #274 (2026-08-02) — this repo is canonical,
consumers take pinned release binaries, nobody vendors the source. Both docs now say that; the
channel rules live in docs/distribution.md.

**Scoping sources**: `oit:docs/streams/INTAKE.md` I-08 (SCADA/OODA,
`../oit/docs/streams/methodology/scada-ooda-lineage.md`), `../methodology/brief-18-dora-metrics.md`
(DORA retro consumption — same-repo reference now that `methodology` moved here alongside this
stream), the `verify-desk` skill (change-failure/rework feed), https://dora.dev/ (DORA Core).

## Merge note (assay-selfcontain/03, 2026-08-01)

This stream previously existed in **two places**: briefs 01–42 developed in
`oit` (oit), and brief 43 authored directly here ahead of the
move — this directory's own seed note (above the fold in git history) explained why: brief 43's
implementation is `statusgen/` code, canonical in this repo, so authoring it in oit would have
created migration work for an already-decided destination. This file is that seed note's
prescribed merge: this repo's frontmatter conventions (`tiering:`) are kept, oit's
`issues: [156, 217, 282]` folded in, oit's Scope/Scoping-sources/Shared-conventions prose carried
over wholesale (cross-repo paths re-pointed to local where the referenced stream moved here too),
and the two brief tables concatenated — the numbering was already disjoint (oit 01–42, here 43)
by design, so no renumbering was needed. See brief 43's own file for the full ruling history on
why it kept its number.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Status-transition historian — statusgen logs every status change with a timestamp](./brief-01-status-historian.md) | 0 | M | done | 2026-07-09 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 02 | [DORA metrics emitter — `statusgen --dora` (5 metrics from the historian + git/gh + verify-desk)](./brief-02-dora-emitter.md) | 1 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 03 | [`statusgen --trend` — the SCADA historian view over time](./brief-03-statusgen-trend.md) | 1 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 04 | [Point-quality rendering — an unverified `done*` visibly distinct from evidence-backed `done`](./brief-04-point-quality.md) | 0 | S | done | 2026-07-09 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 05 | [FINDINGS alarm KPIs — rate, standing-alarm age, flood detection (ISA-18.2)](./brief-05-findings-alarms.md) | 1 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 06 | [Next-up span-of-control + overflow-as-alarm (EEMUA-191)](./brief-06-nextup-span.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 07 | [Two written invariants — "Observe ∝ Act" and "Orient integrity is paramount"](./brief-07-invariants.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 08 | [Next-up claim-aware — exclude briefs with an open branch/PR (#156)](./brief-08-nextup-claim-aware.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-09 reviewer-app[bot] |
| 09 | [Dependency-precise Next-up eligibility — gate on typed depends, not whole-wave completion](./brief-09-dependency-precise-eligibility.md) | 0 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 10 | [Verification-debt as a first-class alarm — Awaiting-queue depth/ratio on the board](./brief-10-verification-debt-alarm.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 12 | [Surface the verified-stage human gate — irreversible briefs get a sign-off issue at implemented (#231)](./brief-12-irreversible-gate-surface.md) | 1 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 14 | [Next-up value term — explicit value field + held-up-by count in the score (F-09, R-01)](./brief-14-nextup-value-term.md) | 1 | M | done | 2026-07-10 opus-verifier | 2026-07-10 reviewer-app[bot] |
| 15 | [Human-stamp corroboration — human:<name> additions need that account's on-PR action (#237)](./brief-15-human-stamp-corroboration.md) | 1 | M | done | 2026-07-13 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 11 | [Gate-queue prioritization — review and verify loops drain by brief priority, not arrival order](./brief-11-gate-queue-priority.md) | 1 | M | done | 2026-07-19 glm-5.2-verifier | 2026-07-18 assay-reviewer-app[bot] |
| 13 | [Per-stream max-concurrent — Next-up offers at most N in-flight briefs from a serialized stream (#217)](./brief-13-nextup-max-concurrent.md) | 0 | S | todo | — | — |
| 16 | [DORA time series — --dora --series buckets CFR/frequency/lead-time per ISO week](./brief-16-dora-time-series.md) | 1 | S | done | 2026-07-13 opus-verifier | 2026-07-12 reviewer-app[bot] |
| 17 | [Verify-desk lands results as verifiers return — continuous drain, not wave-end batches (#282)](./brief-17-verify-land-as-you-go.md) | 0 | S | done | 2026-07-10 opus-verifier | 2026-07-11 reviewer-app[bot] |
| 19 | [Code-efficiency metrics — --code: SLOC/churn/defect-density/review-depth, ledger-sourced](./brief-19-code-efficiency-metrics.md) | 1 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 20 | [Approved-idle alarm + merge-when-green — approval is perishable when main outruns the merge](./brief-20-approved-idle-alarm.md) | 1 | M | done | 2026-07-25 glm-5.2-verifier | 2026-07-24 assay-reviewer-app[bot] |
| 18 | [Daily factory-floor bottleneck report — --bottleneck: per-stage WIP, constraint, shift](./brief-18-daily-bottleneck-report.md) | 1 | M | verified | 2026-07-24 glm-5.2-verifier | — |
| 21 | [Consumer-enumeration gate — shared-value briefs carry a consumers: list whose routing claims are corroborated against the diff](./brief-21-consumer-enumeration-gate.md) | 1 | M | implemented | — | — |
| 23 | [Roadmap deck page 1 — --roadmap: all-streams portfolio grid (computed health, fixed order)](./brief-23-roadmap-deck-overview.md) | 1 | L | done | 2026-07-19 glm-5.2-verifier | 2026-07-18 assay-reviewer-app[bot] |
| 24 | [Roadmap deck pages 2..N — identical-skeleton per-stream pages (delta panel, wave ladder)](./brief-24-roadmap-deck-stream-pages.md) | 2 | M | todo | — | — |
| 25 | [Cadenced exec artifacts — weekly WBR deck + monthly exec summary on the shared schedule](./brief-25-cadenced-exec-artifacts.md) | 2 | M | todo | — | — |
| 26 | [DORA breakdowns — --dora --by stream/goal: the four metrics per stream and per product goal](./brief-26-dora-breakdowns.md) | 1 | M | done | 2026-07-19 glm-5.2-verifier | 2026-07-18 assay-reviewer-app[bot] |
| 22 | [Daily artifact harvest — scheduled AI-free collector commits the day's metrics/board artifacts](./brief-22-daily-artifact-harvest.md) | 1 | M | done | 2026-07-18 glm-5.2-verifier | 2026-07-20 human:ian |
| 28 | [Issue metrics — `--issues`: counts + age/sitting-time + internal-vs-external + by-raising-desk](./brief-28-issue-metrics.md) | 3 | L | todo | — | — |
| 29 | [`raised-by:<desk>` stamping — the filing desks label issues they raise (makes 28's by-desk cut real)](./brief-29-raised-by-stamping.md) | 4 | S | todo | — | — |
| 30 | [Self-improvement metric — self-diagnosed + self-resolved (agent-raised + agent-fixed, no human touch) vs human-touched](./brief-30-self-improvement-metric.md) | 5 | M | todo | — | — |
| 31 | [Path-scope the repo-infra lint checks — a finance-app deploy check must not red-gate an unrelated doc PR](./brief-31-statusgen-infra-path-scope.md) | 0 | M | done | 2026-07-19 glm-5.2-verifier | 2026-07-18 assay-reviewer-app[bot] |
| 32 | [Product-scoped statusgen — reuse `serves:` + `--scope <product>` so each product lints only its own streams](./brief-32-statusgen-product-scope.md) | 1 | M | done | 2026-07-20 opus-verifier | 2026-07-18 assay-reviewer-app[bot] |
| 33 | [Trim the always-unknown DORA metrics from the retro-facing emit](./brief-33-retro-dora-trim.md) | 1 | S | todo | — | — |
| 34 | [Segment the Awaiting board by blocker owner (desk / human-gate / rework / paused / env)](./brief-34-await-board-segment.md) | 0 | S | implemented | — | — |
| 35 | [30-day statusgen check-firing audit — retire cold `--lint` rules](./brief-35-lint-firing-audit.md) | 1 | S | todo | — | — |
| 36 | [Drain-before-instrument — gate new metric/alarm briefs while the queue they measure is over threshold](./brief-36-drain-before-instrument.md) | 0 | S | todo | — | — |
| 37 | [Make UNRUN a first-class board state — block `done` on an unrun risk-bearing Verify row](./brief-37-unrun-board-state.md) | 0 | M | implemented | — | — |
| 38 | [Daily human-gate sign-off digest + per-stream age-at-gate metric](./brief-38-signoff-digest.md) | 1 | S | todo | — | — |
| 39 | [Auto-flip verified→done for gate:model briefs from the reviewer-App approval](./brief-39-auto-flip-model-done.md) | 1 | M | todo | — | — |
| 40 | [opmetrics — local operator-layer collector: relay ratio, intervention rate, decision latency, session hygiene → aggregates-only day-file](./brief-40-opmetrics-operator-collector.md) | 1 | M | todo | — | — |
| 41 | [Autonomy ratio + token efficiency + deterministic-gate share + rework — the step-3 gauges in statusgen/harvest](./brief-41-autonomy-token-metrics.md) | 2 | M | todo | — | — |
| 42 | [Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck](./brief-42-ladder-position-indicator.md) | 3 | S | todo | — | — |
| 43 | [verify-gate-close becomes the SOLE writer of a human:&lt;name&gt; stamp — `--lint` rejects it anywhere else](./brief-43-sole-human-stamp-writer.md) | 1 | M | todo | — | — |

Status legend: `todo` (unclaimed) · `in-progress` · `implemented` (implementer stops here) ·
`verified` (a non-implementer ran the Verify table + filled Evidence) · `done` (+ recorded
review; a `gate: human` brief needs a human sign-off, not a model one).

## Critical path

```
01 (status-transition historian) ──► 02 (DORA emitter) ──► methodology/18 (DORA in the retro)
                                └───► 03 (--trend view)
                                └───► 05 (FINDINGS alarm KPIs)
```

**Smallest unblocking move: brief 01 (the historian).** **Verified as the REAL head, not assumed:**
statusgen today renders **point-in-time** state only — it records *what* a brief's status is, never
*when* it changed (checked `statusgen/` — no transition log, no timestamps beyond the free-text
Verified/Reviewed cells). So **Change Lead Time (implemented→done), `--trend`, and the age-based alarm
KPIs are all uncomputable until 01 exists.** Everything time-series in this stream sits behind it.
Point-quality (04), span-of-control (06), the invariants (07), and claim-awareness (08, #156) are
the exceptions — they read current state (git/streams) only, so they're wave 0 alongside 01.

## Dependency waves

**For brief-v1 briefs** (those with `schema: brief-v1` frontmatter): the `depends:` list is the
scheduler's gate — a `todo` brief-v1 is eligible when every referenced dep is `done` or `verified`;
wave is for organization and rendering only. **Legacy briefs** (no brief-v1 marker) keep the old
whole-wave rule (every lower-wave brief in the stream must be `done` or `verified`). See brief 09.

```
Wave 0: [01, 04, 06, 07, 08]      (01 is the head; 04/06/07/08 read current state, no historian needed)
        [09, 10]                  (assay-review-1, 2026-07-09: eligibility + queue-depth read current state too)
Wave 1: [02←01, 03←01, 05←01]     (everything time-series)
```

Critical path: `01 → 02 → methodology/18`. This stream's **02 unblocks `methodology/18`** (the retro
consumes the emitted metrics); brief-18 stays in the `methodology` stream as the retro-process brief.
`methodology` and `methodology-metrics` both live in this repo as of this merge, so the reference is
same-repo (no repo qualification needed).

## Shared conventions

- **verify-gate-close is the sole writer of a `human:<name>` sign-off** in a stream-README
  Verified/Reviewed cell, and `statusgen --lint` rejects that stamp added on any PR (brief 43,
  `gate: human` — not yet implemented). The writer workflow itself lives in oit
  (`../oit/.github/workflows/verify-gate-close.yml`); this repo has no such
  workflow and therefore currently has **no** permitted writer and **zero** stamps in its own
  stream READMEs. Brief 43 covers what that means for this repo.
- **Next-up score formula (brief 14, F-09 / R-01)**: an eligible brief's Next-up
  rank is `priorityWeight(stream) + staleness×stalenessPerDay + valueWeight(value)
  + unblocksWeight×blockedCount` (`statusgen/nextup.go`). `value` is the
  optional brief-v1 field `value: low | med | high` (absent = med, the neutral
  zero point — an opt-out brief scores exactly as before this brief). `blockedCount`
  is the reverse transitive typed-`depends:` walk — the number of not-done briefs
  this one holds up (shared with the gate-score, brief 11). **Staleness clock**: a
  brief ages from its OWN last recorded status transition (the historian,
  `.history.jsonl`), NOT the stream's git touch, so a sibling brief's activity no
  longer resets an unrelated item's aging; a brief with no history row falls back
  to the stream LastTouch. Aging is uncapped-until-the-cap, so value/blockedCount
  never starve an old brief — it eventually floats regardless (the OS-scheduler
  rationale). **These weights are an evolving heuristic, not truths** (F-09
  discipline): staleness still only rewards age, value is a coarse three-way knob,
  blockedCount carries no effort term — documented as tunables at their `const`
  declarations, and they move when I-13 lands a better score.
- **Gate-score formula (brief 11)**: the Awaiting-verification/review queue uses the
  same formula as Next-up — `priorityWeight(stream) + staleness×stalenessPerDay
  + valueWeight(value) + unblocksWeight×blockedCount` (`statusgen/nextup.go`,
  `gateScores`). The board renders Score + BlockedCount columns ordered by score
  descending so the verify-desk and pr-review-desk drain top-first by brief
  priority rather than arrival order. `blockedCount` is the number of not-done
  briefs transitively gated on this brief reaching `done` — it reuses `buildRevDeps`
  and `blockedCount` from brief 14, never re-derived. The score is GUIDANCE: a human
  request to review/verify a specific item legitimately jumps the queue with no
  ceremony. JSON export at `--gate-scores` for deskboard consumption.
- **Next-up span-of-control (brief 06)**: Next-up caps the total items shown at a **span-of-control**
  limit (default **20**, configurable via `--span N`) on top of the
  per-stream cap (4). EEMUA-191's 7±2 is a *human* operator's cognitive band; this queue is worked by
  agents, so the cap was raised to 20 (human:<name> 2026-07-16) — overflow still alarms on a genuinely huge
  backlog, without throttling the fanout to human scale. When the eligible backlog exceeds the cap,
  STATUS renders an explicit overflow line and `--lint` emits a
  matching `NOTICE` (WIP-pressure alarm); overflow is never a silent truncation. The overflow threshold
  defaults to the span cap and is separately tunable via `--overflow-threshold N`.
- **UNRUN is a board state, and it is DERIVED (brief 37)**: a Verify row counts as RUN only when the
  brief's `## Evidence` section carries a row for it with a date and a runner. Silence reads as unrun —
  no token an author writes (or declines to write) can assert that a row ran. An UNRUN **risk-bearing**
  row (the row carries a `live`/`mutating`/`risk-bearing`/`end-to-end` tag, or the brief's `risk.*` has
  any `yes`) renders `done‡`/`verified‡` and **blocks the `done`/`verified` transition** unless the row
  is run or ROUTED to a named follow-up brief/issue in its Evidence row (brief-23 live-verify pattern,
  mandatory). The gate is scoped to the transition: briefs already closed at the merge-base with
  `origin/main` are grandfathered to a `--lint` NOTICE, so the backlog stays visible without creating an
  incentive to edit a closed brief's Evidence until the board goes green. The same derivation at
  `implemented` (0 rows corroborated, past 72h) is the stale-implemented NOTICE — F-impl-claims-unproven.
- **One brief = one PR** (draft while worked; the reviewer App approves; the desk flips ready — see
  CLAUDE.md PR review loop). Implementers stop at `implemented`; the `verify-desk` runs the Verify table
  post-merge and fills Evidence.
- **Never commit STATUS.md on a branch** — single-writer is main's CI. These briefs edit `statusgen/`
  and add its outputs; STATUS.md regenerates on main.
- **Anti-gaming (DORA's core warning) is a stream-wide rule, not per-brief decoration:** these metrics are
  diagnostic, per-project, for continuous improvement — never targets, individual scorecards, or
  cross-team comparison. A metric that starts driving perverse behavior is itself a retro finding.
- **Verification-debt alarm**: depth/ratio NOTICE fires when Awaiting > threshold (currently 10) or
  Awaiting > total done (methodology-metrics/10). Age/trend behind mm/01 via mm/03 — out of scope here.
- Go tooling: run from this repo's `statusgen/` module; the pinned release binary
  (`.assay-versions`) is what actually gates other repos' CI, but `go build . && go test ./...`
  and `go run . --root .. --lint` must stay exit 0 here too.
- **Daily artifact harvest**: `../oit/docs/reports/daily/<YYYY-MM-DD>/` — generated by
  oit's `../oit/.github/workflows/daily-harvest.yml` (06:00 UTC + `workflow_dispatch`); desks read, never
  regenerate. (oit-side machinery; `daily-harvest` itself is mid-move to this repo under a separate,
  concurrent brief.)
- **Adoption-ladder set (#766, briefs 40→41→42)**: operator + autonomy-loop layers. The operator
  collector (mm/40) runs on the OPERATOR machine — its sources (`~/.claude` transcripts, roster,
  claims) do not exist on CI runners; its `opmetrics.json` day-file is the interface the CI-side
  axes (mm/41) and the ladder indicator (mm/42) consume, degrading to "unmeasured" when absent.
  All three inherit the anti-gaming rule: diagnostics, never targets or per-person scorecards.
