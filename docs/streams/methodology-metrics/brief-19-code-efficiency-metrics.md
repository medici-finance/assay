---
brief: methodology-metrics/19
title: 'Code-efficiency metrics — statusgen --code: SLOC delta, churn, defect density, review depth (ledger-sourced, [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)-safe)'
wave: 1
depends: ["methodology-metrics/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: old-school code-efficiency metrics are available, we don't have energy to dig them out — let the tool compute them going forward", "human:<name> 2026-07-10: the [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) review on the Assay teaser (assay-toolkit PR #3) — ledger-sourced numbers are quotable where the engineer-equivalent is not", "methodology-metrics/02 (--dora emitter this extends), mm/16 (--dora --series time-series)", "docs/streams/methodology/brief-34-article-model-mix.md (the article that can quote these)", "freshness-checked 2026-07-10 @ post-#312 main"]
why: >-
  Two needs converge. (1) The pre-DORA code-efficiency lineage (SLOC/day, churn, defect
  density, review depth) is the fourth metric family alongside human/DORA/ToC — computable
  from git + the issue register, so no one has to dig it out by hand. (2) The [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) lesson
  from the teaser: numbers RECOMPUTED FROM A LEDGER ARTIFACT are quotable in Assay copy where
  the engineer-equivalent (a leverage figure with no defensible denominator) is not. These
  metrics are ledger-sourced by construction, so they give the teaser/article defensible
  figures the leverage number can't.
---

# Brief 19 — Code-efficiency metrics

## Context
files: `../assay-toolkit/statusgen/` (new `--code` block, composing with `--dora`; reuse the git/gh
data collection mm/02 already has), tests
facts:
- Metrics (all from git log + the `bug`-labelled issue register — no external analyzer, no
  new dependency):
  - **SLOC delta/day**: added / removed / net, non-merge non-regen commits (the same
    exclusions --dora uses). Windowed via `--since`.
  - **Churn ratio**: lines touched again within N days ÷ lines added (rework signal — a
    high churn ratio means code is being rewritten shortly after landing). N default 7,
    documented.
  - **Defect density**: `bug` issues filed ÷ KSLOC-changed in the window (the ledger-sourced
    quality-per-volume figure — distinct from --dora's CFR which is per-merge).
  - **Change spread**: median files-touched per change (a diffuseness signal).
  - **Review depth**: PR review comments ÷ merged PR (from gh review data — how much
    scrutiny each change drew).
- Composes with `--series` (mm/16): `--code --series` buckets per ISO week like the DORA
  series; the article/teaser quotes the trend.
- **[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) posture (state it in the output header):** every figure here is recomputed from a
  repo ledger artifact (git history, the issue register) — which is the [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) EXEMPTION
  (methodology/09). The header says so, so a consumer knows these are quotable where a
  leverage estimate is not. It does NOT emit any leverage/person-day/tier-mix figure (those
  stay forbidden — this brief adds ledger metrics, not the forbidden class).
- Same Goodhart header as --dora (diagnostic, never a target); small-n honesty (a window
  with <N commits prints `–`, not a misleading ratio).
- NOT wired into the offline `--lint` gate (it's a diagnostic emitter; may read gh).

## Ground rules
- NEVER git push / trigger workflows / mutating kubectl. Leave commits per task only.
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Implement `--code` (+ `--series`, + `--since`) per facts; reuse mm/02's collection.
2. Tests: SLOC delta on a fixture history; churn ratio on a re-touched-file fixture; defect
   density = bugs ÷ KSLOC; small-n suppression; header states the [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)-exempt (ledger-sourced)
   posture; no forbidden-class figure emitted.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-2 cases |
| 2 | `statusgen --root . --code --since 2026-07-08 \| head -20` | SLOC delta / churn / defect density / review depth, with the ledger-sourced header |
| 3 | `statusgen --root . --code --series --json \| head -c 200` | valid JSON per-period array |
| 4 | `statusgen --root . --lint; echo $?` | 0 (lint never invokes --code) |

## Evidence
Non-implementer verifier run (glm-5.2-verifier, merged main `ab8f5dfa`, 2026-07-18). **VERIFY: PASS — all 4 rows.**

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen 2.313s`; all Task-2 cases present in `codeefficiency_test.go` (SLOCDelta, ChurnRatio + ChurnRatioNoReTouch, DefectDensity, ChangeSpread, ReviewDepth, SmallNSuppression + SuppressForReviewDepth, F12HeaderPresent, NoForbiddenClassEmitted, JSONHasFiveKeys, SeriesWeeklyBuckets/JSONShape/TextRender, RunRejectsBadSince/FutureSince, ChurnDaysBoundary) |
| 2 | `go run ./tools/statusgen --root . --code --since 2026-07-08 \| head -20` | 0 | header `Code-efficiency metrics — 2026-07-08 … 2026-07-18`; **[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) ledger-sourced posture line present verbatim**; Goodhart header present; all 5 metrics rendered — SLOC/churn/defect/change-spread = `–` (small-n suppression fired correctly on a shallow clone; math proven by row 1's fixtures), review depth `0.0 comments/PR`. The ONLY line matching `leverage\|person-day\|tier-mix` is the [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) negation disclaimer — **no forbidden figure emitted**. |
| 3 | `go run ./tools/statusgen --root . --code --series --json \| head -c 200` | 0 | valid JSON `array`, 5 weekly periods; per-period keys include added/removed/churn_lines/churn_ratio/defect_density/files_touched/review_depth/num_prs |
| 4 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 — lint never invokes `--code` (stdout empty; stderr = normal NOTICEs) |

**VERIFY: PASS.** Caveats (environmental, not defects): `--code` latency ~3 min/run (review-depth shells out to
`gh` across hundreds of merged PRs — by-design per the brief); the shallow clone masks live SLOC cells (the
fixture tests in row 1 are the math proof, not the live `–` cells); review-depth numerator `0 / 388 merged PRs`
is a numerator question (line-level comments vs review submissions) for the stream owner to sanity-check — not a
windowing bug (denominator correctly windowed). `gate: model`, all four risks `no` → model flip permitted.
`verified → done` awaits the review-gate record.

## Review
Gate: model. Reviewer confirms every metric is git/issue-register-sourced (no external
analyzer smuggled in), the header states the [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)-exempt posture, no forbidden leverage/
person-day figure is emitted, and small-n suppresses rather than misleads.
