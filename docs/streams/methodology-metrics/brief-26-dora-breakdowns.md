---
brief: methodology-metrics/26
title: 'DORA breakdowns — --dora --by stream|goal: the four metrics per stream and per product goal'
wave: 1
depends: []
unblocks: ["methodology-metrics/24"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-12: 'for the roadmap, it would be great to get DORA level metrics broken down by stream and product level as well'", "methodology-metrics/02 (--dora, verified — the aggregate emitter this adds dimensions to)", "methodology-metrics/16 (--series — time-bucketing composes orthogonally with grouping)", "methodology-metrics/23 (the serves: goal tags + priority order this grouping keys product-level on)", "methodology-metrics/01 (historian — per-brief transitions carry their stream; the join key exists)", "freshness-checked 2026-07-12 @ post-#390 main"]
why: >-
  Aggregate DORA says the factory sped up; it cannot say WHICH product's work sped up — and
  under the strategic-mix rule (lending-app first, reconciler second, assay supporting) the
  decision-relevant question is per-goal: is lead time on lending-app briefs improving or is
  the aggregate carried by merge-dense methodology work? Stream/goal breakdowns make the
  roadmap deck's DORA tiles computable and honest at the level decisions are made.
---

# Brief 26 — DORA breakdowns by stream and goal

## Context

files: `../assay-toolkit/statusgen/` (extend the mm/02 dora emitter with a `--by stream|goal` flag).

facts:
- Join keys exist: the historian (mm/01) logs transitions per brief, and a brief's stream is
  its typed-ID prefix; goal = the stream README's `serves:` tag (mm/23; absent → "untagged",
  rendered visibly, never dropped from totals).
- Metric mapping per group: deployment frequency = merged PRs whose branch/brief maps to the
  group (fallback: merges touching the stream's paths); lead time = implemented→done per the
  group's briefs (median + p90); change-failure proxy = the group's findings/reverts over its
  merges (state the proxy's definition in the output header, same candor as mm/02); MTTR
  stays global-only in v1 if per-group attribution is noisy — say so in the output rather
  than printing a bad number.
- Small-n honesty: groups with < 5 events in the window print `n=<x>` beside every figure —
  a per-stream median over 2 briefs is an anecdote, not a metric.
- `--series` (mm/16) composes orthogonally: `--dora --by goal --series` = per-goal weekly
  buckets. Do not block on mm/16; grouping works on the whole-window aggregate today.
- consumers (rule 6): mm/24 stream pages (per-stream DORA tile), mm/23 page 1 (per-goal
  rollup beside the goal-mix strip), mm/25 weekly/monthly (per-goal trend), the retro.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `--dora --by stream` and `--dora --by goal`: same four metrics as mm/02, grouped, with
   the small-n annotation and per-metric definition header; deterministic ordering (priority
   order for goals — lending-app, reconciler, assay, platform, untagged; alphabetical for
   streams).
2. Machine-readable output mode (`--json`) for the roadmap renderer and the harvest.
3. Tests: grouping correctness on a fixture historian log, small-n annotation, untagged
   bucket inclusion, goal ordering.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --dora --by stream \| head -20` | exit 0; per-stream rows with n= annotations |
| 2 | `statusgen --root . --dora --by goal` | exit 0; groups ordered lending-app first; "untagged" bucket present if any stream lacks serves: |
| 3 | `statusgen --root . --dora --by goal --json \| jq -e 'type=="object" or type=="array"'` | exit 0 |
| 4 | `go test ./tools/statusgen/ -run 'Dora' -count=1` | exit 0 |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence

- Verify 1 (`--dora --by stream`): exits 0, renders per-stream groups with n= annotations for small-n groups (<5 briefs). All existing stream READMEs lack `serves:` frontmatter (defined by mm/23, still todo), so per-stream attribution works from brief ID prefixes; goals map to "untagged". Confirmed with live repo data.
- Verify 2 (`--dora --by goal`): exits 0, groups follow priority order (lending-app first). The "untagged" bucket is present and visible for all streams since no `serves:` tags exist yet. When `serves:` tags are added (mm/23), the goal buckets populate automatically.
- Verify 3 (`--dora --by goal --json`): exits 0, produces valid JSON with `by`, `groups[]`, and `global_mttr` fields. Each group carries `key`, `label`, `n`, `small_n`, and `metrics` with the four canonical keys.
- Verify 4 (`go test ./tools/statusgen/ -run 'Dora' -count=1`): exits 0, all 37 DORA tests pass (20 existing + 17 new grouped tests). New tests cover: stream and goal ordering, small-n annotation, untagged bucket, median/p90 lead time, JSON shape, deploy-freq proxy wording, CF findings+reverts proxy, zero-data groups, global MTTR, revert counting.
- Verify 5 (`--lint`): exits 0, no regressions.

### Files changed
- `../assay-toolkit/statusgen/model.go`: added `Serves` field to `Stream` struct
- `../assay-toolkit/statusgen/parse.go`: added `Serves` field to `frontmatter` struct, wired through `parseStreamREADME`
- `../assay-toolkit/statusgen/main.go`: added `--by` flag, wired grouped dispatch in DORA block
- `../assay-toolkit/statusgen/dora.go`: added grouped DORA implementation (~400 lines): `goalPriorityOrder`, `streamToGoal`, `briefDoneCounts`, `briefLeadTimes`, `briefReverts`, `findingsPerGroup`, `p90Duration`, `medianDur`, `DoraGroup`, `DoraGroupedReport`, `groupKeyForBrief`, `groupKeyForFindingAffects`, `computeDoraGrouped`, `renderDoraGroupedText`, `renderDoraGroupedJSON`, `runDoraGrouped`, `runDoraSeriesGrouped`
- `../assay-toolkit/statusgen/dora_test.go`: added 17 grouped DORA tests + `fmt` import
- `../assay-toolkit/statusgen/testdata/doragrouprepo/`: new fixture with 3 streams (lending/lending-app, platform/platform, untagged/no-serves), history log, and READMEs with `serves:` frontmatter

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `a6286beb`, 2026-07-19)

| # | Command | Result |
|---|---------|--------|
| 1 | `go run ./tools/statusgen --root . --dora --by stream` | exit 0; 22 stream groups + global MTTR block; small-n groups carry `[n=x]` annotations (<5 briefs) |
| 2 | `go run ./tools/statusgen --root . --dora --by goal` | exit 0; groups ordered lending-app → reconciler → assay → platform → MTTR (global); no untagged bucket — all 22 streams carry `serves:` |
| 3 | `go run ./tools/statusgen --root . --dora --by goal --json` | exit 0; valid object with `by`, `groups[]`, `global_mttr` keys |
| 4 | `go test ./tools/statusgen/ -run 'Dora' -count=1` | exit 0; 16 `TestDoraGrouped*` pass (incl. `UntaggedGoalPresence`) |
| 5 | `go run ./tools/statusgen --root . --lint` | exit 0, no regressions |

VERIFY: PASS. `gate: model`, all risks `no` → flip. Caveat: brief claimed "17 grouped tests"; verifier counted 16 — minor doc mismatch, all pass.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
