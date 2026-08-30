---
brief: quality/02
title: M1 line-operation taxonomy + churn / rework rate (GitClear-aligned)
why: >-
  The industry's headline AI-code-quality finding is that churn (code revised or deleted
  within two weeks) roughly doubled and copy/paste overtook refactoring as models started
  writing more code. Those are exactly the signals that tell us whether our own output is
  durable or throwaway. This brief computes them the published way, so our numbers are
  comparable to public baselines — and, unlike the proprietary tools, inspectable line by
  line.
wave: 1
depends: ["quality/01"]
unblocks: ["quality/05", "quality/08", "quality/09", "quality/11", "quality/13"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §4 — M1 aggregation grains (per commit / file / package / PR / stream / author-identity / window) and the honest-claims discipline"
  - "docs/streams/quality/spec.md §4.1 — line-operation taxonomy (added/deleted/updated/moved/copied/churned), block-level move-vs-copy at N>=4, copy/paste ratio + duplicate-block rate"
  - "docs/streams/quality/spec.md §4.2 — churn / rework rate (14-day window, per stream + author-identity class; second published anchor: Faros rework-rate, 14–30-day window — report against both)"
exec-tier: strong
exec-tier-why: >-
  (c) taxonomy correctness is a class of subtle implementation error that survives naive
  tests — moved-vs-copied and the block-match threshold pass a happy-path test while being
  quietly wrong, so it needs a strong tier and adversarial fixtures.
---

# Brief 02 — M1 line-operation taxonomy + churn / rework rate

## Context

files:
- NEW `qualgen/taxonomy.go` (planned) (+ `taxonomy_test.go`) (planned) — the per-changed-line
  classifier: `added` / `deleted` / `updated` / `moved` / `copied` / `churned`, with
  block-level move-vs-copy detection.
- NEW `qualgen/churn.go` (planned) (+ `churn_test.go`) (planned) — the churn / rework computation:
  a landed line revised or deleted within the churn window, as churned-lines / new-lines.
- NEW `qualgen/m1agg.go` (planned) (+ `m1agg_test.go`) (planned) — the M1 aggregator: rolls the
  per-line classifications up to the spec §4 grains and emits `metrics.jsonl`, including the
  headline copy/paste ratio and duplicate-block rate.
- READ-ONLY, from quality/01: `qualgen/measure.go` (planned) (`Measure[T]`), `qualgen/commit.go` (planned)
  (`Commit`), `qualgen/diff.go` (planned) (`FileDiff`/`Hunk`/`LineChange`), `qualgen/store.go` (planned)
  (`Store` reader + append-only writer). This brief consumes that seam; it does not
  redefine it.

facts:
- consumes quality/01's interface contract: reads the commit + diff tables through the
  `Store`, wraps every emitted number in `Measure[T]`, and appends aggregates via the same
  single-writer `Store`. Never parses the JSONL by hand.
- taxonomy (spec §4.1): each changed line is exactly one of `added`, `deleted`, `updated`,
  `moved` (relocated, content-identical/near-identical), `copied` (duplicated — identical to
  a line that REMAINS), `churned` (added/updated by this same identity within the churn
  window and revised again now).
- block granularity: `moved` vs `copied` is decided at BLOCK level — a run of `>= N` similar
  lines, `N` configurable, DEFAULT 4 (GitClear's published duplicate-block granularity). A
  3-line identical run below the threshold is NOT a block move/copy.
- headline ratios (spec §4.1): copy/paste ratio = `copied / (moved + copied)`;
  duplicate-block rate = duplicated blocks over the window. Reported beside the industry
  baseline where §4/§9.3 pair one.
- churn window (spec §4.2): a line revised or deleted within **14 days** of landing
  (configurable; 14d is the comparable default). Reported as churned-lines / new-lines per
  window, per stream, and per author-identity class.
- author-identity classes (spec §3.1): a CONFIGURABLE partition — `human` / `agent` /
  `automation` — supplied per target as author patterns; any author matching no pattern
  falls into an explicit `unclassified` class, never silently dropped or merged.
- honest-claims (spec §10): label numbers "computed per GitClear's *published* definitions,"
  never "GitClear-equivalent"; where a window/threshold differs from the published one, the
  artifact states both.

single-point-of-failure: none — this is read-only aggregation over quality/01's artifacts,
writing only to the tracking root; no funds/auth/ledger surface, so the core-system
defense-in-depth obligation does not apply. Taxonomy correctness is guarded by adversarial
fixtures in the Verify table rather than by a runtime control.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch
  + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit a generated view / `STATUS.md`-class artifact on a branch (single writer =
  main's CI). Write artifacts only under a temp/test tracking root.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `taxonomy.go`: classify each `LineChange` (from quality/01's diff table) into
   the six categories. Implement block-level move-vs-copy: detect runs of `>= N` matching
   lines (N configurable, default 4); a matched block whose source lines are GONE is `moved`,
   a matched block whose source lines REMAIN is `copied`. Lines with no measurable diff
   (the `could-not-measure` line data from quality/01) propagate as `could-not-measure`,
   never counted as any category.
2. Implement `churn.go`: for each landed line, determine whether it is revised or deleted
   within the churn window (default 14d) by the SAME author-identity class; emit
   churned-lines / new-lines. A window with data but zero churn is `measured-zero`; a window
   whose inputs could not be read is `could-not-measure`.
3. Implement the author-identity partition: load the configurable class map (`--identity-map`
   or config); classify each `Commit.author`; unmapped authors form the explicit
   `unclassified` class. Aggregate churn per class.
4. Implement `m1agg.go`: aggregate the per-line taxonomy and per-line churn to the spec §4
   grains (per file / package / PR / stream / author-identity / window). Emit the headline
   copy/paste ratio (`copied / (moved + copied)`) and duplicate-block rate. Append all
   aggregates to `docs/quality/metrics.jsonl` via the `Store`, each value a `Measure[...]`,
   each labelled per the honest-claims discipline. Wire this aggregation into the `mine`
   pipeline so `qualgen mine` produces `metrics.jsonl`.
5. Add adversarial fixtures: a relocated block (expect `moved`), a duplicated-while-original-
   remains block (expect `copied`), a 3-line vs 4-line identical run (threshold boundary), a
   line re-edited at day 13 vs day 15 (churn boundary), and a commit by an author matching no
   class (expect `unclassified`).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run TestTaxonomyMovedVsCopied` | exit 0; a relocated block classifies `moved` (source gone) and a duplicated block classifies `copied` (source remains) — the two are not conflated |
| 3 | `cd qualgen && go test ./... -run TestBlockMatchThreshold` | exit 0; a 3-line identical run is NOT a block move/copy at the default N=4, a 4-line run IS — the configurable threshold is honored |
| 4 | `cd qualgen && go test ./... -run TestChurnWindowBoundary` | exit 0; a line re-edited at day 13 counts as `churned`, at day 15 does not (14-day window); a window with zero churn emits `measured-zero`, not `could-not-measure` |
| 5 | `cd qualgen && go test ./... -run TestUnclassifiedIdentityClass` | exit 0; an author matching no configured class lands in an explicit `unclassified` class and is never silently merged into human/agent/automation |
| 6 | `cd qualgen && go test ./... -run TestCopyPasteRatioValue` | exit 0; on a fixture with exactly 1 copied block and 1 moved block, the emitted copy/paste ratio is the `measured` value 0.5 — DEREFERENCES the computed number against a known-answer fixture |
| 7 | `TMP=$(mktemp -d); cd qualgen && go run . mine --repo .. --out "$TMP" && test -f "$TMP/docs/quality/metrics.jsonl" && grep -q '"copy_paste_ratio"' "$TMP/docs/quality/metrics.jsonl" && grep -q '"published definitions"' "$TMP/docs/quality/metrics.jsonl"` | exit 0; the mine pipeline emits M1 aggregates including the copy/paste ratio, labelled per the published-definitions honest-claims discipline |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). "verified" in the stream
     README requires this section filled by someone who did NOT implement. -->

Independently verified 2026-08-30 by opus-4.8[1m]-verifier (verify-desk, non-implementer) against merged head b506390d (Merge PR #240). Offline envelope (KUBECONFIG=/dev/null). VERIFY: PASS 7/7 as-written (no re-baseline needed); zero could-not-check. Deliverables present: qualgen/taxonomy.go, churn.go, m1agg.go, identity.go, mine.go + tests.

| # | Command | Exit | Output | Date · Runner |
|---|---------|------|--------|---------------|
| 1 | cd qualgen && go build ./... && go vet ./... | 0 | build 0, vet 0 | 2026-08-30 · opus-4.8[1m]-verifier |
| 2 | go test -run TestTaxonomyMovedVsCopied | 0 | PASS — relocated (source deleted)=moved, duplicated (source remains)=copied; not conflated | 2026-08-30 · opus-4.8[1m]-verifier |
| 3 | go test -run TestBlockMatchThreshold | 0 | PASS — 3-line run not a block at N=4, 4-line run is | 2026-08-30 · opus-4.8[1m]-verifier |
| 4 | go test -run TestChurnWindowBoundary | 0 | PASS — day-13 churned, day-15 not; zero-churn window emits measured-zero | 2026-08-30 · opus-4.8[1m]-verifier |
| 5 | go test -run TestUnclassifiedIdentityClass | 0 | PASS — unmapped author lands in explicit unclassified | 2026-08-30 · opus-4.8[1m]-verifier |
| 6 | go test -run TestCopyPasteRatioValue | 0 | PASS — ratio == 0.5 (1 copied / (1 moved + 1 copied)), state measured (known-answer) | 2026-08-30 · opus-4.8[1m]-verifier |
| 7 | go run . mine --repo .. --out TMP; assert metrics.jsonl + copy_paste_ratio + published-definitions | 0 | mine: 607 commits extracted, tip b506390d; both greps matched | 2026-08-30 · opus-4.8[1m]-verifier |

RISK-VALUE: DERIVED — DefaultBlockMin = 4 @ qualgen/taxonomy.go:40 — GitClear's published duplicate-block granularity (a run of >=4 similar lines counts as a moved/copied block; spec §4.1). Right value: the brief's contract is comparability to GitClear's published definitions, and 4 is that published threshold. Reversible via --block-min.
RISK-VALUE: DERIVED — DefaultChurnWindowDays = 14 @ qualgen/churn.go:22 — GitClear's published 14-day churn window (spec §4.2); the comparable published default (Faros rework 14-30d is target-selectable). Reversible via --churn-window-days.
gate: model, all risk no, irreversible: no — desk flips implemented→verified; verified→done stays CI's on the reviewer approval.

## Review
Gate: model (all four risk answers no — read-only aggregation over quality/01's artifacts,
writing only to the tracking root; no funds/auth/ledger surface). exec-tier: strong —
moved-vs-copied and the block-match threshold are subtle-error-prone and must be pinned by
adversarial fixtures. Reviewer records verdict + date in the stream README table.
