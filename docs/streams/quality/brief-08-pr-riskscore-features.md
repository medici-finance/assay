---
brief: quality/08
title: "`pr <n>` mode — per-file risk features (generic riskscore feed)"
why: >-
  The whole point of mining hotspots and defect density is to get the numbers to the agents
  working the code at the moment they touch it. `qualgen pr <n>` is that delivery point: for
  the files a PR touches, emit the brittleness features (hotspot percentile, traced defect
  density, ownership concentration, missing coupling partners) as a generic JSON feed any
  riskscore consumer can weight. It ships no thresholds and no verdict — thresholds live in
  the consumer's config — so it stays a neutral instrument, not a gate.
wave: 3
depends: ["quality/02", "quality/03", "quality/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §9.1 — PR riskscore feed (per-touched-file features; consumers weight; thresholds in consumer config)"
  - "docs/streams/quality/spec.md §4.3–4.5 — hotspot, ownership/SPOF, change coupling (the M1 features consumed)"
  - "docs/streams/quality/spec.md §5.3 — per-file defect density (consumed from brief 07)"
  - "docs/streams/quality/spec.md §3.2 — three-state `measured` flag per feature"
---

# Brief 08 — `pr <n>` mode — per-file risk features (generic riskscore feed)

## Context

files:
- NEW `qualgen/pr.go` (planned) (+ `qualgen/pr_test.go` (planned)) — the `pr <n>` mode: resolve the file set a
  PR touches, join each file to its M1 features + M2 defect density, emit a per-file feature
  record as JSON.
- NEW `qualgen/features.go` (planned) (+ `qualgen/features_test.go` (planned)) — the shared feature-assembly the
  `pr` and `check` (brief 09) modes both call: given a file path, return its
  `FileFeatures` (hotspot percentile, defect density, ownership, coupling partners, measured
  flags). Factor it here so brief 09 reuses it rather than forking.
- CONSUMES `qualgen/hotspot.go` (planned) / ownership / coupling from briefs 02–03 (M1 aggregates in
  `docs/quality/metrics.jsonl`) and `qualgen/szz.go` (planned)'s `DefectDensity` from brief 07.
- READS the target PR's touched-file set through the same repo/history access the miner uses
  (a merged PR's file set, or an open PR's head-vs-base diff).
- OUTPUT is JSON to stdout (a feed, not a committed artifact) — a consumer captures it.

facts:
- feature set per touched file (spec §9.1): `hotspot_percentile` (this file's rank in the
  repo's decayed hotspot distribution), `defect_density` (traced inducing-commits/file from
  brief 07, WITH its trace-rate), `ownership_top` (top-identity surviving-line share, spec
  §4.4), `coupling_missing` (historical coupling partners of this file NOT also touched by
  this PR, spec §4.5), and a three-state `measured` flag PER feature (spec §3.2).
- GENERIC feed, not a scorer: this mode emits features only. It assigns NO weights, NO
  aggregate score, NO pass/fail. Weighting and thresholds live in the CONSUMER's config
  (spec §9.1) — putting any threshold here is out of scope and a design error.
- three-state per feature: a feature that could not be computed (file too new for a hotspot
  percentile, blame unreachable for density) emits `measured: could-not-measure`, never a
  silent 0. A genuine zero (no coupling partners) emits `measured: measured-zero`.
- `coupling_missing` is the inverse signal (spec §4.5): the feed lists the coupling partners
  the PR did NOT touch, so the consumer can flag "changed A, historically coupled to B, B
  untouched" — the strongest cheap brittleness predictor.

single-point-of-failure: none load-bearing — this mode is a read-only feature join with no
control standing between a fault and damage; a wrong feature value degrades a consumer's
advisory score, and the three-state `measured` flag is the layer that keeps an uncomputable
feature from masquerading as a real zero.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- This mode is READ-ONLY against the repo and writes NO artifact — it prints a feed to
  stdout. Do not commit its output anywhere.
- Emit NO threshold, weight, or verdict — features only (spec §9.1). If a design pull toward
  "just flag the risky ones here" appears, that belongs in the consumer; report NEEDS_CONTEXT
  rather than adding it.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `qualgen/features.go` (planned): implement `FileFeatures(path)` — join the file to its M1 hotspot
   percentile (brief 03), ownership top-share (brief 03), coupling partners (brief 03), and
   M2 defect density (brief 07). Each field carries its own `measured` state. This is the
   shared assembly brief 09 reuses.
2. `qualgen/pr.go` (planned): implement `qualgen pr <n>` — resolve the PR's touched-file set, call
   `FileFeatures` per file, compute `coupling_missing` as (historical coupling partners) minus
   (files this PR touched), and emit a JSON object `{pr, files: [{path, hotspot_percentile,
   defect_density, defect_trace_rate, ownership_top, coupling_missing, measured:{...}}]}`.
3. Wire the three-state `measured` map per feature: `measured` | `measured-zero` |
   `could-not-measure`, sourced from the underlying M1/M2 record's own state — never
   fabricate a value for an unmeasured feature.
4. Handle the empty/edge cases explicitly: a PR touching only new files (no history) emits
   every file with `could-not-measure` hotspot/density and a `measured-zero` coupling set —
   a valid, honest feed, not an error.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run PR && go test ./... -run Features` | exit 0; feature-assembly + pr-mode tests pass |
| 3 (DEREFERENCING — a specific feature value on a fixture) | `cd qualgen && go test ./... -run TestPR_KnownHotspotFeatureValue -v` | exit 0. The test builds a fixture repo where one file is churned across many commits (a planted hotspot) and another is touched once, then runs `pr` mode on a fixture PR touching both. Assert the churned file's `hotspot_percentile` is strictly greater than the quiet file's, and both carry `measured: measured`. |
| 4 (DEREFERENCING — missing coupling partner surfaced) | `cd qualgen && go test ./... -run TestPR_MissingCouplingPartnerFlagged -v` | exit 0. Fixture: files A and B are co-changed in many historical commits (coupled); the fixture PR touches A but NOT B. Assert A's `coupling_missing` list contains B. |
| 5 (DEREFERENCING — defect density carries trace-rate) | `cd qualgen && go test ./... -run TestPR_DefectDensityCarriesTraceRate -v` | exit 0. Fixture with a traced planted defect on file A (via the brief-07 corpus). Assert A's feed record has a non-empty `defect_density` AND a non-empty `defect_trace_rate` beside it — density without its trace-rate fails the test (honest-claims). |
| 6 (three-state — new file is could-not-measure, not zero) | `cd qualgen && go test ./... -run TestPR_NewFileIsCouldNotMeasure -v` | exit 0. Fixture PR adds a brand-new file with no history; assert its `measured.hotspot == "could-not-measure"`, NOT a `0` percentile. |
| 7 (no thresholds leaked) | `cd qualgen && grep -icE -e threshold -e verdict -e 'pass.?fail' -e 'score >' qualgen/pr.go` | prints `0` (this mode emits features only; any threshold/verdict/score belongs in the consumer). |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (all four risk answers no — a read-only, write-nothing feature feed over any git
repo; emits no verdict and no threshold). Reviewer confirms the feed is genuinely generic
(no weighting/threshold has crept in), each feature carries an honest three-state `measured`
flag, and defect density never ships without its trace-rate. Records verdict + date in the
stream README table.
