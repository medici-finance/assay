---
brief: methodology-metrics/42
title: 'Ladder-position indicator — one computed adoption-step number (behavioral axes, never tooling) on the board + roadmap deck'
wave: 3
depends: ["methodology-metrics/40", "methodology-metrics/41"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [766]
schema: brief-v1
authored: 2026-07-22 by Opus 4.8 session (human:<name> direction, from #766)
sources: ["#766 (section (b) 'Ladder position' row + acceptance — this brief completes the set and closes the issue)", "#766 2026-07-18 addendum point 1 (behavior defines the step, not tooling)", "#766 2026-07-20 sweep item 3 (step position is behavioral — dated author signal)", "methodology-metrics/23 (roadmap deck page 1 — the render surface, done)", "freshness-checked 2026-07-22 @ ef1de62a"]
why: >-
  Steering by intent needs one number to watch: which adoption step the desk system is actually
  AT, computed from behavior (autonomy ratio, relay ratio, verification lag, gate share) rather
  than from what's installed. Today the rating is a hand-argued paragraph in an issue; #766's
  acceptance is that the daily artifact carries it with a 7-day trend so ladder movement — or
  stall — is visible without anyone re-arguing it.
---

# Brief 42 — Ladder-position indicator

## Context
files:
- `../assay-toolkit/statusgen/` — `--ladder` emit + tests; a panel line on the roadmap deck page 1 (mm/23)
- `tools/daily-harvest/` — include the indicator in the day dir
- `docs/streams/methodology-metrics/README.md` — convention line (authored in this brief set)

facts:
- **Axes (all behavioral, per addendum point 1)**: autonomy ratio + deterministic-gate share
  (mm/41 emit), relay ratio + intervention rate + session hygiene (mm/40 day-file when present),
  implemented→verified P50 (mm/01 historian via existing helpers), agents-in-flight proxy
  (per-day dispatch-claim count from the mm/40 day-file).
- **Step mapping is a documented lookup, not model judgment**: a small table in code mapping axis
  thresholds → step 0–4 (e.g. step 3 requires loop-initiated share above threshold AND relay
  ratio below threshold AND verification lag bounded), thresholds as named consts with the ladder
  citation. Missing axes (no day-file) → the indicator renders a RANGE ("2–3, operator axes
  unmeasured today"), never a false-precision single number.
- **Render**: one line — step (or range) + the binding constraint axis ("held at 2–3 by: relay
  ratio") — on `--ladder`, the daily artifact, and the mm/23 roadmap overview panel. The
  constraint phrasing mirrors mm/18's one-line verdict style (addendum point 2: the binding
  constraint IS the progression metric).
- Anti-gaming: the indicator is diagnostic; thresholds move only via a reviewed change (they are
  the whole semantics — a drive-by threshold edit is how the number gets gamed).
- Closing #766: mm/41 covers (b) rows 1+3+5 (autonomy, token-efficiency, gate-share, rework);
  this brief lands the last acceptance item (daily artifact carries (a)+(b)+trend +indicator);
  the landing PR carries `Closes #766` per the close-PR flow.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: fixture axis inputs → expected step; missing-day-file → range
   render with named unmeasured axes; constraint-axis naming.
2. Implement `--ladder` + harvest inclusion + the roadmap panel line.
3. README convention line.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run Ladder -v` | exit 0; covers step mapping, missing-axis range render, constraint naming |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --ladder` | exit 0; output contains `step` and a named constraint axis or `unmeasured` |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no). Reviewer confirms thresholds are named consts with
citations and the missing-axis path renders a range, not a fabricated point value. Verdict +
date in the README table.
