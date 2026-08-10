---
brief: methodology/38
title: 'Cadence-compression research — re-clock every weekly/monthly loop against measured velocity (weekly→daily candidates, per-loop decision rule)'
wave: 0
depends: []
unblocks: []
effort: M
gate: human
gate-why: >-
  The recommendations rewire process rituals human:<name> personally participates in — the retro
  cadence is his attention and his time, and the WBR/exec artifacts are read on his clock.
  A model can generate the analysis, but ratifying a cadence change to human-attended
  loops is the human's call (the same generation-vs-ratification split the research
  itself proposes).
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
decision-issue: 729
schema: brief-v1
authored: 2026-07-12 by Fable session (human:<name> direction; fact-check computed at authoring)
sources: ["human:<name> 2026-07-12 (verbatim): \"one of the key insights i'm looking at is all the 'weekly' jobs that have been historically been done can get their cadence reviewed to a daily cycle due to the output of work going on, and our velocity speeding up 2-3x (you should fact check this number). if your research indicates this to be true, we should be adjusting this accordingly. this should be a research brief, as it will affect a lot of the methodology we have.\"", "measured 2026-07-12 @ d0222490 (baked into Context facts): git log origin/main per-day split, gh pr list merged per-day, statusgen --dora, statusgen --trend, R-01's --dora --since 2026-07-08 record", "docs/streams/RETRO.md (weekly retro; R-01 ran 2026-07-10; one-change-per-retro budget + rule-change-rate decision)", "methodology-metrics/18 (daily bottleneck report, todo)", "methodology-metrics/22 (daily artifact harvest — brief merged via PR #370, todo; the generation/ratification split already modeled)", "methodology-metrics/25 (weekly WBR deck + monthly exec summary — authored on OPEN PR #381, unmerged at authoring)", "docs/streams/methodology/scada-ooda-lineage.md (sample-rate-tracks-process; Observe ∝ Act)", "INTAKE [I-20](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-standing-daily-code-review-cadence-demonstrated-value.md) (standing daily code-review cadence, scoped) + [I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md) (watermarked incremental review sweeps, new)", "freshness-checked 2026-07-12 @ d0222490"]
why: >-
  The weekly/monthly clocks were inherited from human-org calendars (meetings cost
  coordination; data changed slowly) while this pipeline now moves 2.6-10x faster than
  when they were set: a weekly ritual now spans ~8 median brief lifecycles and ~800
  commits, so weekly-cadence decisions ride on stale data and the constraint moves
  between samples (it shifted twice in 3 days). Sample rate should track the process's
  rate of change, not the calendar — this research produces the evidence and a per-loop
  re-clock instead of a vibes-based blanket "make everything daily".
---

# Brief 38 — Cadence-compression research (weekly→daily under measured velocity)

## Context

files: deliverable = `docs/streams/methodology/cadence-review-2026-07.md` (planned) — a
cadence-review doc containing the fact-check, the per-loop recommended-cadence table, and
the amendment list. Amendment targets are OUT of this brief's write scope (enumerated,
not edited): methodology-metrics/18, methodology-metrics/22, methodology-metrics/25,
RETRO.md conventions (owner methodology/08), INTAKE [I-20](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-standing-daily-code-review-cadence-demonstrated-value.md)/[I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md) dispositions.

facts:
- **Fact-check of the 2-3x (measured 2026-07-12 @ d0222490) — accurate-to-understated;
  no measured metric is below 2x:**
  - Commits-to-main/day, the cleanest velocity series: last 7 days = 47, 115, 81, 123,
    146, 176, 81, 28-partial (797 total ≈ 114/day) vs ≈43/day over the prior 3 weeks of
    the 28-day DORA window ((1708−797)/21) → **≈2.6x week-over-prior, peak day ≈4x**.
    human:<name>'s 2-3x is RIGHT on this baseline.
  - Merged PRs/day (deploy frequency): 7 (07-08) → 55 (07-09) → 75 (07-10) → 43 (07-11)
    vs a 7.54/day 28-day average → **6-10x**, but CONFOUNDED: the PR review loop only
    became the default path 2026-07-09, so part of that jump is workflow migration
    (direct-commit → PR), not pure velocity. Quote it with the confound stated.
  - Lead time: median implemented→done 21.4h (28-day --dora); PR open→merge median 1.0h
    (0.7h in R-01's 3-day cut). **A calendar week now spans ≈8 median brief lifecycles**
    — the amount of state change weekly rituals were sized to review arrives in ~a day.
  - Done-throughput concentration: 18 briefs reached done in the 3 days after 2026-07-08
    vs 20 in the entire 28-day window (R-01 record vs --dora).
  - Counterweight the doc must carry: change-failure 49% (partial, bug-issue slice only)
    — compression must not let Act out-run Observe (invariant "Observe ∝ Act",
    `docs/streams/methodology/invariants.md`).
- **Cadenced jobs enumerated at authoring** (seed table — the research re-enumerates
  freshly with `grep -rniE 'weekly|daily|monthly|cron|schedule:' docs/streams
  .github/workflows` and diffs against this; in-repo only, `~/.claude` skills out of
  scope):

  | Loop / job | Current clock | Home (typed ID) | State 2026-07-12 |
  |---|---|---|---|
  | Retro R-NN | weekly ("weekly initially") | RETRO.md, methodology/08 | live; R-01 ran 2026-07-10 |
  | DORA system read | at-retro (= weekly) | RETRO.md conventions, methodology/18 | live |
  | Bottleneck report | daily (spec) | methodology-metrics/18 | todo |
  | Artifact harvest | daily (GH cron) | methodology-metrics/22 | brief merged (#370), todo |
  | Roadmap deck (daily grid) | daily | methodology-metrics/23 + /24 | authored on OPEN PR #381 |
  | WBR deck | weekly Mon 06:00 UTC | methodology-metrics/25 | authored on OPEN PR #381 |
  | Exec summary | monthly, 1st 06:00 UTC | methodology-metrics/25 | authored on OPEN PR #381 |
  | Standing code-review sweep | daily (proposed; no durable owner) | INTAKE [I-20](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-standing-daily-code-review-cadence-demonstrated-value.md) | intake, scoped |
  | Incremental review sweeps | ad-hoc (watermark register) | INTAKE [I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md) | intake, new |
  | Cluster dailies (fleet-health / image-refresh / log-triage) | daily k8s CronJobs | k8s, diagnostics stream | live — already daily; enumerate, don't touch |
- **The theoretical frame (apply, don't re-derive):** human-org cadences price
  meeting/coordination cost and slow data half-life; with agent throughput, generation is
  ~free and the binding constraints move to (a) human attention, (b) data half-life,
  (c) the decision latency the loop feeds. Batch-size/Little's-law reasoning: smaller
  batches + faster feedback beat calendar ritual; dwell = WIP/throughput. The SCADA
  framing is already in-house (`docs/streams/methodology/scada-ooda-lineage.md`): sample
  rate tracks the process's actual rate of change — a scan slower than the process misses
  transients (the pipeline constraint moved 2026-07-08 dispatch → 2026-07-10 verification;
  a weekly sample sees neither).
- **The decision rule the doc must produce, per loop:** cadence = f(decision latency it
  feeds, data half-life, human attention cost). The standard move for human-attended
  loops is the generation/ratification SPLIT — analysis/artifacts compress to daily
  (free), the human decision stays on the human's clock. methodology-metrics/22 (harvest)
  already models exactly this split; the research generalizes it rather than inventing a
  new mechanism.
- **Concrete candidates to answer with evidence (recommend or reject EACH, no blanket
  rule):** (1) retro → daily generated inputs + weekly human decision — NOT daily
  meetings; (2) review sweeps → daily watermark diffs (the [I-20](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-standing-daily-code-review-cadence-demonstrated-value.md) × [I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md) composition; [I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md)
  carries the cost model); (3) WBR weekly deck stays, but is a daily mini-trend needed or
  does --trend/harvest already cover it; (4) monthly exec summary unchanged (audience
  clock is calendar-set, data half-life slow) — confirm or refute.
- **Tension to resolve explicitly:** R-01 decided rule-change RATE should trend DOWN
  toward the one-change-per-retro steady state; compressing the retro clock would
  multiply the change budget. The doc must reconcile (likely: analysis cadence
  compresses, the change-budget clock does not — say so or argue otherwise).
- Goodhart guardrail carries over verbatim from --dora/RETRO: cadence and velocity
  numbers are diagnostic, never targets; a cadence chosen to flatter a metric is a retro
  finding.
- Recommendations are CANDIDATES routed through the desk/retro (the one-change budget
  applies to ENACTMENT); this brief delivers the doc + amendment list — it enacts
  nothing.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- Do NOT edit methodology-metrics/18/22/25, RETRO.md, or the intake entries here —
  amendments are follow-on work the doc enumerates.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Re-enumerate** every cadenced job (grep set in facts + `.github/workflows`
   `schedule:` triggers); diff against the seed table; carry anything new into the doc's
   inventory with its typed ID and current clock.
2. **Refresh the fact-check** at implementation time (`go run ./tools/statusgen --root .
   --dora`, `--trend`, the git/gh per-day splits) in a `## Fact-check` section: human:<name>'s
   verbatim quote, the multiplier PER METRIC with its baseline named, the PR-loop
   confound, and the plain verdict (understated / right / overstated). The authoring
   numbers above are the reference point — supersede, don't delete them.
3. **Apply the decision rule per loop** and produce the per-loop cadence table — header
   literally: `| Loop | Current clock | Recommended cadence | Rule inputs | Evidence |`
   — one row per inventory entry, each recommendation citing its rule inputs (decision
   latency / data half-life / human attention) and evidence, including the four named
   candidates and the retro-budget tension.
4. **Amendments section**: each accepted recommendation mapped to its owning artifact by
   typed ID (methodology-metrics/18, methodology-metrics/22, methodology-metrics/25,
   RETRO.md conventions via methodology/08, INTAKE [I-20](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-standing-daily-code-review-cadence-demonstrated-value.md)/[I-36](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-incremental-review-sweeps-watermark-diff-scope-tiered-models.md) dispositions) — one line
   each: what changes, who ratifies. Rejected candidates get one line of why.
5. README row status; lint green.

## Verify (executable — presence gates per [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md); quality is owned by the human gate)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/methodology/cadence-review-2026-07.md` (planned) `; echo $?` | 0 (the research doc exists) |
| 2 | `grep -c "Recommended cadence" docs/streams/methodology/cadence-review-2026-07.md` (planned) | ≥ 1 (per-loop cadence table present) |
| 3 | `grep -cE "^## Fact-check" docs/streams/methodology/cadence-review-2026-07.md` (planned) | ≥ 1 (fact-check section present) |
| 4 | `grep -c "2-3x" docs/streams/methodology/cadence-review-2026-07.md` (planned) | ≥ 1 (the claim under test quoted with the measured verdict) |
| 5 | `grep -oE -e "methodology-metrics/18" -e "methodology-metrics/22" -e "methodology-metrics/25" docs/streams/methodology/cadence-review-2026-07.md \| sort -u \| wc -l` (planned) | 3 (amendment list carries all three typed IDs) |
| 6 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence

### Non-implementer verifier run — VERIFY: PASS (stays `implemented`, `gate: human`) — glm-5.2-verifier, 2026-07-23

Isolated worktree off `origin/main` `37704530` (`.claude/worktrees/agent-aaf0e2cae5dbc1210`); shared checkout not touched.

| # | Command | Exit | Key output | Result |
|---|---|---|---|---|
| 1 | `test -f docs/streams/methodology/cadence-review-2026-07.md; echo $?` | 0 | exit 0 | PASS |
| 2 | `grep -c "Recommended cadence" …cadence-review-2026-07.md` | 0 | 1 (≥1) | PASS |
| 3 | `grep -cE "^## Fact-check" …` | 0 | 1 (≥1) | PASS |
| 4 | `grep -c "2-3x" …` | 0 | 2 (≥1) | PASS |
| 5 | `grep -oE "methodology-metrics/(18\|22\|25)" … \| sort -u \| wc -l` | 0 | 3 (all three typed IDs) | PASS |
| 6 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | PASS |

**VERIFY: PASS — all 6 rows.** Deliverable `docs/streams/methodology/cadence-review-2026-07.md` exists on merged main with the `## Fact-check` section, the per-loop `Recommended cadence` table, the `2-3x` claim quoted with its measured verdict, all three amendment typed IDs (methodology-metrics/18, /22, /25), and `statusgen --lint` exits 0.

**Status: stays `implemented`.** `gate: human` (risk answers all `no`) — a model verifier records Evidence but **cannot** flip to `verified`; ratifying cadence changes to human-attended loops (retro, WBR) is human:<name>'s call (decision-issue #729). The flip is human:<name>'s.

## Review
Gate: human — human:<name> ratifies (or strikes) each row of the recommended-cadence table; the
ratified rows become the amendment follow-ons. Reviewer confirms the fact-check numbers
regenerate from the stated commands, every recommendation cites its rule inputs rather
than asserting, and no enactment (edits to mm/18/22/25, RETRO.md, workflows) leaked into
this PR.

### Human sign-off artifact — verify-desk, 2026-07-27

`gate: human` sign-off artifact: **#729, Option A, 2026-07-18 human:ian** — human:<name> ratified the cadence re-clock. Verify Evidence above (glm-5.2-verifier PASS, all 6 rows, 2026-07-23). Row flipped `implemented → verified` by the verify-desk on the strength of that artifact.
