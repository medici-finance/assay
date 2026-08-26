---
brief: statusgen/04
title: 'Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck'
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-20 (authored clean for the statusgen board)
sources:
  - "A maintainer directive: the ladder step must be defined by BEHAVIOR (a dated author signal), not by what tooling is installed"
  - "The roadmap-deck overview page — the render surface this panel line joins"
  - "The one-line-verdict style used elsewhere (the binding constraint IS the progression metric)"
why: >-
  Steering by intent needs one number to watch: which adoption step the desk system is actually
  AT, computed from behavior (autonomy ratio, relay ratio, verification lag, gate share) rather
  than from what's installed. Today the rating is a hand-argued paragraph; the acceptance is that
  the daily artifact carries it with a 7-day trend so ladder movement — or stall — is visible
  without anyone re-arguing it.
---

# Brief 04 — Ladder-position indicator

> **Self-contained by design — no typed cross-repo dependency.** Some of this indicator's
> behavioral axes are read from an OPTIONAL operator-metrics day-file produced by a separate
> operator-machine collector that is NOT part of this public tool. That input is a **soft,
> fail-neutral** one: when the day-file is absent — the default for any adopter — the operator axes
> are simply *unmeasured*, and the indicator renders an explicit RANGE, never a false-precision
> single number and never a silent zero. There is therefore **no typed `depends:` edge**: the
> degrade is handled at render time, exactly like the autonomy-axis degrade the same design already
> uses.

## Context
files:
- `statusgen/` — the `--ladder` emit + tests; a panel line on the roadmap-deck overview page
- the daily artifact (day directory) — include the indicator
- `docs/streams/statusgen/README.md` — convention line (authored in this brief set)

facts:
- **Axes (all behavioral)**: autonomy ratio + deterministic-gate share (statusgen-native board
  signals); relay ratio + intervention rate + session hygiene (from the OPTIONAL operator-metrics
  day-file, when present); implemented→verified P50 (from statusgen's own history helpers);
  agents-in-flight proxy (a per-day dispatch-claim count, also from the operator day-file when
  present).
- **Step mapping is a documented lookup, not model judgment**: a small table in code mapping axis
  thresholds → step 0–4 (e.g. step 3 requires loop-initiated share above threshold AND relay
  ratio below threshold AND verification lag bounded), thresholds as named consts with a citation.
  Missing axes (no operator day-file) → the indicator renders a RANGE ("2–3, operator axes
  unmeasured today"), never a false-precision single number.
- **Render**: one line — step (or range) + the binding constraint axis ("held at 2–3 by: relay
  ratio") — on `--ladder`, the daily artifact, and the roadmap overview panel. The constraint
  phrasing mirrors the one-line-verdict style: the binding constraint IS the progression metric.
- Anti-gaming: the indicator is diagnostic; thresholds move only via a reviewed change (they are
  the whole semantics — a drive-by threshold edit is how the number gets gamed).

## Ground rules
- NEVER git push to main / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: fixture axis inputs → expected step; missing-day-file → range
   render with named unmeasured axes; constraint-axis naming.
2. Implement `--ladder` + daily-artifact inclusion + the roadmap panel line.
3. README convention line.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run Ladder -v` | exit 0; covers step mapping, missing-axis range render, constraint naming |
| 2 | `go test ./statusgen/ && go vet ./statusgen/` | exit 0 |
| 3 | `statusgen --root . --ladder` | exit 0; output contains `step` and a named constraint axis or `unmeasured` |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-08-26 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `ea7fea5`
Runner != implementer. Own isolated worktree off `origin/main`, OFFLINE (`KUBECONFIG=/dev/null`). gate: model, all risk no. `statusgen/` module; test rows ran from inside the module dir; binary built locally.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `go test ./statusgen/ -run Ladder -v` | exit 0; step map, missing-axis range, constraint naming | exit 0 — StepMappingExact (all-fail/topped-out/gap-ignored), RangeWhenAxisUnmeasured, MeasuredFailCapsRange, DegradesWithoutDayFile PASS | 2026-08-26 | opus-4.8[1m]-verifier |
| 2 | `go test ./statusgen/ && go vet ./statusgen/` | exit 0 | exit 0 — ok 33.9s; vet clean | 2026-08-26 | opus-4.8[1m]-verifier |
| 3 | `statusgen --root . --ladder` | exit 0; step + named constraint or unmeasured | exit 0 — emitted a step RANGE named by constraint axes; unmeasured rungs widen to a range, never a fabricated point | 2026-08-26 | opus-4.8[1m]-verifier |
| 4 | `statusgen --root . --lint; echo $?` | 0 | exit 0 — LINT: PASS | 2026-08-26 | opus-4.8[1m]-verifier |

**RISK-VALUE: NAMED (citation-authority), reversible** — the four rung thresholds ladderLoopShareStep1=25.0, ladderGateShareStep2=50.0, ladderDispatchShareStep3=60.0, ladderNoopRateStep4Max=20.0 @ statusgen/ladder.go — reversible DIAGNOSTIC-display thresholds (per-project, never a target/gate); semantic authority is the methodology-metrics/42 citation the source consts carry, which the brief's Review clause has a reviewer confirm rather than re-derive. Rank last by the reversibility rule; no derivation owed.

## Review
Gate: model (all four risk answers no). Reviewer confirms thresholds are named consts with
citations and the missing-axis path renders a range, not a fabricated point value. Verdict +
date in the README table.
