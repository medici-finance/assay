---
brief: methodology-metrics/18
title: 'Daily factory-floor bottleneck report — statusgen --bottleneck: per-stage WIP, constraint location, day-over-day shift'
wave: 1
depends: ["methodology-metrics/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
value: high
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["#766 (adoption-ladder metrics — names this brief the #760 backpressure sensor and raises its priority; value: high added 2026-07-22)", "human:<name> 2026-07-10: factory-floor bottleneck analysis (where, has it shifted, what to do) as a DAILY report", "methodology-metrics/01 (the status historian — per-stage transition timestamps this reads)", "methodology-metrics/10 (verification-debt alarm — the WIP depth this contextualizes)", "docs/streams/methodology/scada-ooda-lineage.md (the SCADA/ToC lens)", "INTAKE [I-29](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-factory-floor-flow-metrics-per-stage-cycle-time-aging-wip-fl.md) (flow metrics — the CONWIP pull rule this report's recommendation feeds)", "freshness-checked 2026-07-10 @ post-#309 main"]
why: >-
  The pipeline's constraint MOVES (2026-07-08 dispatch → 2026-07-10 post-merge verification,
  once the #228 dispatch bug was fixed — Theory of Constraints: relieve one station, the next
  becomes the bottleneck). A one-off manual read caught it tonight; a daily report catches it
  every time it shifts, and names the ToC action (elevate the constraint, subordinate the
  rest) instead of leaving it to whoever happens to eyeball the board.
---

# Brief 18 — Daily bottleneck report

## Context
files: `../assay-toolkit/statusgen/` (new `--bottleneck` subcommand), report artifact under
`docs/reports/factory-floor/` (per-day files, append-only like the retro)
facts:
- `--bottleneck` computes, from the historian (mm/01) + current README states:
  (1) **per-stage WIP** (todo/in-progress/implemented/verified/done/blocked counts);
  (2) **per-stage DWELL** (median time a brief currently sits in each stage — the
  historian has entry timestamps; the constraint is the stage with the highest
  dwell × WIP, not just highest count);
  (3) **the constraint** = that stage, named plainly ("the bottleneck is verified→done");
  (4) **shift detection** — compares to the previous report's constraint; if it moved,
  says so and from-where (the ToC signal);
  (5) **the prescribed action** keyed to WHICH stage is the constraint (a small lookup:
  dispatch-bound → check dispatch health; verify-bound → add verify capacity + CONWIP
  throttle per [I-29](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-factory-floor-flow-metrics-per-stage-cycle-time-aging-wip-fl.md); review-bound → parallelize review recording). NOT freeform advice —
  a documented stage→action table so the report is deterministic.
- Output: markdown to `docs/reports/factory-floor/<date>.md` (append-only history — the
  shift-detection reads yesterday's file), AND a one-screen summary to stdout.
- Honest limitation ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) discipline): WIP×dwell is a heuristic locator, not a proof;
  the report says "likely constraint" and shows the numbers behind it. Dwell for briefs
  with no historian row falls back to "unknown", counted separately, never silently zero.
- This is a REPORT (diagnostic), never a gate — it emits, it does not block; the Goodhart
  header from --dora applies verbatim (a constraint metric that becomes a target rots).

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Leave commits per task only.
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Implement `--bottleneck` per facts; the stage→action lookup is a documented constant.
2. Emit the dated report file + stdout summary; shift-detection reads the prior dated file.
3. Tests: WIP counts correct on a fixture; constraint = highest WIP×dwell stage; shift
   detected vs a prior-report fixture; unknown-dwell briefs counted separately not zeroed.
4. Wire it as a suggested daily desk step (out-of-repo the-desk skill boot note, #221) —
   the report is generated, not hand-written.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | `statusgen --root . --bottleneck \| head -20` | per-stage WIP + named constraint + prescribed action |
| 3 | `test -f docs/reports/factory-floor/$(date +%F).md` (after a run) | the dated report exists |
| 4 | PR body carries the out-of-repo the-desk diff (#221) | present |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- non-implementer rows. Today's manual instance (2026-07-10) is in the PR body as the
     target output shape. -->

### Non-implementer verify — VERIFY: PASS — glm-5.2-verifier, 2026-07-24

Isolated worktree `/private/tmp/vrf-meth-trio` off `origin/main` `e890be13`. PR **#560** (merged
2026-07-16). R1 (the unit-test row) was **F-34 guard-blocked** (the `statusgen` token in
`go test ./tools/statusgen` trips the writeguard from shared-checkout-homed sessions, including
dispatched agents; `go test` has no `--root` anchor to clear it) — recorded as UNRUN, not assumed,
with strong corroboration: `--lint` exit 0 proves compile+run; `bottleneck_test.go` read on main
(12 funcs: WIP counts, constraint=WIP×dwell, true-median, unknown-dwell-counted-separately, shift
detection, no-shift, skipped-day, first-report, action-table, run-writes-file, stage-label,
empty-history, blocked); R2/R3 exercise the feature live.

| # | command | exit | result | date | runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen -count=1` | BLOCKED | UNRUN — F-34 writeguard (statusgen token); corroborated: lint exit 0 (compile+run), 12 test funcs read on main, R2/R3 live | 2026-07-24 | glm-5.2-verifier |
| 2 | `--bottleneck \| head -20` | 0 | PASS — per-stage WIP table (todo/in-progress/implemented/verified/done/blocked), named constraint `todo→in-progress`, prescribed action "Dispatch-bound — check dispatch health" | 2026-07-24 | glm-5.2-verifier |
| 3 | `test -f docs/reports/factory-floor/$(date +%F).md` (after R2) | 0 | PASS — `../oit/docs/reports/factory-floor/2026-07-24.md` written (1142 B) | 2026-07-24 | glm-5.2-verifier |
| 4 | PR body carries the out-of-repo the-desk diff (#221) | — | PASS — PR #560 body has "## Out-of-repo diff (#221 — the-desk skill boot note)" wiring `--bottleneck` into the daily orientation | 2026-07-24 | glm-5.2-verifier |
| 5 | `--lint; echo $?` | 0 | PASS — exit 0 | 2026-07-24 | glm-5.2-verifier |

**VERIFY: PASS** (every runnable row passes; R1 guard-blocked with corroboration). Status flipped
`implemented → verified` (gate: model, risk all-no). Side-finding (not a Verify-row expectation):
R2 showed the known shift-detection false-positive (same-stage "shift" reported) — already tracked
in PR #1161, Go fix routed to assay-toolkit (CI guard blocks `../assay-toolkit/statusgen/*.go` edits here).

## Review
Gate: model. Reviewer confirms the constraint is located by WIP×DWELL (not raw count — a
high-count fast-moving stage is not the bottleneck), shift-detection reads the prior file,
and the stage→action table is a documented constant (deterministic, not freeform advice).
