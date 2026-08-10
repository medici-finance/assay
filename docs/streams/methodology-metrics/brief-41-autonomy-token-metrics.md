---
brief: methodology-metrics/41
title: 'Autonomy ratio + token efficiency + deterministic-gate share + rework — the step-3 gauges in statusgen/harvest'
wave: 2
depends: ["methodology-metrics/40"]
unblocks: ["methodology-metrics/42"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
value: high
authored: 2026-07-22 by Opus 4.8 session (human:<name> direction, from #766)
sources: ["#766 (adoption-ladder metrics, section (b) rows 1+3+5 — this brief implements them)", "#766 2026-07-20 sweep comment item 1 (deterministic external validator is the 3→4 unlock — the gate-share axis)", "#766 2026-07-18 addendum (markers: PRs/day per operator, % loop-initiated)", "tools/desk/cmd/deskroster/roster.go (Claim type — dispatch records are machine-local)", "tools/statusgen/dora.go:261 (doraRework placeholder — the issue definition IS automatable, replacing it)", "freshness-checked 2026-07-22 @ ef1de62a (statusgen has no autonomy/token/gate-share axis; grep = empty)"]
exec-tier: strong
exec-tier-why: (b) correctness depends on cross-artifact provenance reasoning — joining gh authorship, workflow-authored actions, harvest artifacts, and the mm/40 day-file into one ratio without double-counting.
why: >-
  The ladder's step-3→4 gauge is "most agents are kicked off by Claude, not the operator" — and we
  cannot compute it: nothing distinguishes loop-initiated work from human:<name>-prompted work, nothing
  tracks what a merged PR cost in subagent tokens, nothing measures how many merges are
  guarded by code (deterministic gates) vs model judgment, and the existing doraRework axis is
  a hardcoded "unknown" placeholder with a rationale that doesn't match the issue's automatable
  definition (CHANGES_REQUESTED count + re-review cycles per merged PR, from gh review data).
  These four numbers are the difference between claiming step 3 and demonstrating it.
---

# Brief 41 — Autonomy ratio, token efficiency, deterministic-gate share

## Context
files:
- `../assay-toolkit/statusgen/` — NEW `--autonomy` emit (+ tests); wire the summary line into the daily
  bottleneck/board surfaces where they already render
- `tools/daily-harvest/` — extend the collector to persist the three axes into the day dir
- `docs/streams/methodology-metrics/README.md` — convention line (authored in this brief set)

facts:
- **Autonomy ratio** = loop-initiated units / all units, computed two ways and emitted separately
  (never blended): (1) **CI-computable proxy** — actions authored by scheduled/event workflows
  (daily-harvest, status-regen, verify-gate-open/close, needs-decision-close, bugs-gc) and by the
  assay Apps (`assay-*-app[bot]`) count as loop-initiated; `the-org`-authored actions are
  AMBIGUOUS (agents and desks share the account — record them as their own bucket, do not guess);
  `human:<name>` = human. (2) **dispatch-level ratio** — from the mm/40 day-file's claim counts
  (dispatches filed by routine/loop vs operator-prompted session), present only on days the
  operator collector ran. Emit both with their denominators.
- **Token efficiency** = subagent tokens per merged PR + no-op dispatch rate (workers exiting
  "already done"). Source: dispatch records are machine-local (roster/claims — see mm/40), so
  the CI side READS the mm/40 day-file fields; absent file → axis reported "unmeasured today",
  never zero. Alarm/gate policy per #766 sweep item 2: TREND cost per merged PR always;
  ALARM only on workflows past PMF — encode as a config list of alarmed workflows, default empty.
- **Deterministic-gate share** = % of merged PRs whose merge path was guarded by at least one
  code gate that ran at head (statusgen --lint PR gate, daml-ci upgrade-check) vs model-judgment
  gates only (reviewer-App verdicts). Source: `gh` statusCheckRollup per merged PR (harvest
  `prs.json` already carries PR state). Per the 07-20 sweep: a deterministic gate is the stronger
  3→4 signal — render the split, don't collapse it.
- **Rework** = CHANGES_REQUESTED count + re-review cycles per merged PR, the #766 (b) row 5
  definition. Source: gh review data per merged PR (adjacent to the statusCheckRollup fetch
  already done for gate-share). This replaces `../assay-toolkit/statusgen/dora.go:261`'s hardcoded
  `doraRework = "unknown"` placeholder — the old rationale (post-merge-defect classification
  not automatable) is for a different definition; the issue's definition IS automatable, and
  this brief's implementer wires it into the existing DORA emit in-place.
- Day-file interface: `docs/reports/daily/<date>/opmetrics.json` schema is owned by mm/40
  (tools/desk/README.md) — consume it, never re-derive; missing fields degrade to "unmeasured".
- Stream anti-gaming rule applies: diagnostics, never targets or per-person scorecards; token
  budgets stay at the WORKFLOW level (addendum point 6).

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: fixture gh/harvest/day-file inputs → autonomy ratio (both variants,
   ambiguous bucket separate), tokens-per-merged-PR + no-op rate (and the "unmeasured" degrade),
   deterministic-gate share from fixture statusCheckRollup, rework (CHANGES_REQUESTED count +
   re-review cycles per merged PR from fixture review data).
2. Implement `--autonomy` in statusgen + the harvest persistence; document the four axes and
   their honest limits (the-org ambiguity, day-file dependency) in the emitted output itself.
   Wire the rework axis into the existing `--dora` emit, replacing the `doraRework` placeholder
   in `../assay-toolkit/statusgen/dora.go`.
3. README convention line.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Autonomy|TokenEff|GateShare|Rework' -v` | exit 0; covers both autonomy variants, ambiguous bucket, unmeasured degrade, gate-share split, rework (CHANGES_REQUESTED count + re-review cycles from fixture review data) |
| 2 | `go test ./tools/statusgen/ ./tools/daily-harvest/ && go vet ./tools/statusgen/ ./tools/daily-harvest/` | exit 0 |
| 3 | `statusgen --root . --autonomy` | exit 0; output contains `autonomy` and either a ratio or `unmeasured` — never a silent zero for a missing source |
| 4 | `statusgen --root . --dora` | exit 0; `rework` axis is computed (CHANGES_REQUESTED + re-review cycles), not `unknown` |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go metrics tooling over already-committed
artifacts). Reviewer confirms the ambiguous-authorship bucket is never folded into either side of
the ratio and the day-file schema is consumed, not re-derived. Verdict + date in the README table.
