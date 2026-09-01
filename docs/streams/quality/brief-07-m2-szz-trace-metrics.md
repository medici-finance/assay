---
brief: quality/07
title: M2 B-SZZ inducing-commit trace + derived defect metrics
why: >-
  Fix identification (brief 06) names WHICH changes were bug fixes; it does not name
  the change that introduced each bug. Tracing fix back to inducing commit is what turns
  a pile of fixes into a defect-lineage corpus: a bug-churn trend line, per-file defect
  density, and fix latency. Every downstream measurement — per-PR risk features, stage
  attribution, the traced-CFR refinement, the learned risk model — reads the record this
  brief emits, so its output types are the contract the rest of M2/M3 is built on.
wave: 2
depends: ["quality/06"]
unblocks: ["quality/08", "quality/10", "quality/11", "quality/13", "quality/14", "quality/15"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
exec-tier: strong
exec-tier-why: >-
  subtle SZZ correctness survives naive presence tests (question c) and the blame walk is
  cross-artifact reasoning over fix-parent trees, refinement filters, and report dates
  (question b).
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §5.2 — inducing-commit trace (B-SZZ with standard refinements)"
  - "docs/streams/quality/spec.md §5.3 — derived metrics (defect-inducing change rate, per-file defect density, fix latency, traced CFR)"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant"
  - "docs/streams/quality/spec.md §10 — honest-claims discipline (SZZ ships trace-rate + evidence tier, never bare)"
why-critical-path: >-
  On the stream critical path (01 → 06 → 07 → 10 → 12 → 14). This is the item that mints the
  defect-lineage records the whole of M2/M3 consumes; if its record shape is wrong, every
  downstream brief inherits the error.
---

# Brief 07 — M2 B-SZZ inducing-commit trace + derived defect metrics

## Context

files:
- NEW `qualgen/szz.go` (planned) (+ `qualgen/szz_test.go` (planned)) — the B-SZZ trace engine: for each
  `DefectFix` from brief 06, blame the fix's deleted/modified lines at the fix's PARENT,
  apply the standard refinements, and emit a `DefectTrace` record.
- NEW `qualgen/blame.go` (planned) (+ `qualgen/blame_test.go` (planned)) — the blame helper the trace engine
  calls: resolve, for a set of (file, line-range) at a given commit, the commit that last
  touched each line. Uses `go-git` blame; MAY shell to the `git` binary for blame at scale
  where a benchmark (recorded in Evidence) shows go-git is materially slower (spec §3).
- NEW `qualgen/refine.go` (planned) (+ `qualgen/refine_test.go` (planned)) — the three refinement filters:
  cosmetic/format-only exclusion, postdating-the-report exclusion, and the confidence
  scorer.
- CONSUMES `qualgen/defect.go` (planned) (brief 06) — the `DefectFix` record (fix PR/commit, closed
  issue, evidence tier, report date). This brief reads it; it does not modify it.
- WRITES `docs/quality/defects.jsonl` (append-only, relative to the tracking root, spec §9.4)
  — one `DefectTrace` per fix, carrying tier (from 06) + confidence + trace state.
- The derived-metric rollups land in the M1 aggregate artifact family
  (`docs/quality/metrics.jsonl`, spec §9.4) under a defect-metrics section keyed the same
  way M1 aggregates are (file, package, window).

facts:
- algorithm: B-SZZ with standard refinements (spec §5.2) — blame deleted/modified lines at
  the fix's parent; exclude cosmetic/format-only inducers; exclude inducers that postdate
  the defect report; output `(fix_pr, inducing_commit(s), inducing_pr(s), confidence)`.
- three-state trace outcome (spec §3.2): every fix resolves to exactly one of `traced`
  (≥1 inducing commit after refinement), `traced-none` (blame ran, all candidates filtered
  out — a real measured-zero), or `could-not-trace` (blame unreachable: blameless/omission
  bug, multi-hop history, squash-merge floor). Never a silent zero, never a silent drop.
- disclosed limit (spec §5.2): ~40% of cases are not reachable by blame alone per the SZZ
  literature; the run PUBLISHES its trace-rate and every derived number ships it (spec §10).
- confidence is a recorded field, not a gate: it scores blame agreement (single vs multiple
  inducers, refinement survival), and travels on the record for consumers to weight.
- interface contract (see below): the `DefectTrace` type and the `per-file defect density`
  rollup are the seam consumed by briefs 08, 10, 11, 14, 15 — freeze their field names here.

single-point-of-failure: the blame walk is the one control that names an inducing commit —
but the three-state rule is the backstop layer: a blame that fails or returns nothing is
recorded as `could-not-trace`/`traced-none`, never as "no defects," so a blame miss degrades
the published trace-rate visibly rather than silently zeroing a metric.

## Interface contract (frozen here — consumed by 08/10/11/14/15)

The two output types below are the seam the rest of M2/M3 reads. Their field names are the
contract; downstream briefs bind to these names, so changing them later is a breaking change.

- `DefectTrace` (one per `DefectFix`, emitted to `defects.jsonl`):
  - `fix_pr`, `fix_commit` — from the consumed `DefectFix`.
  - `inducing_commits: []string`, `inducing_prs: []string` — resolved inducers (empty when
    `traced-none` or `could-not-trace`).
  - `evidence_tier` — carried through from brief 06 (1/2/3), never re-derived here.
  - `confidence` — this brief's blame-agreement score.
  - `trace_state` — one of `traced` | `traced-none` | `could-not-trace` (spec §3.2).
  - `could_not_trace_reason` — enum when `could-not-trace` (`blameless` | `multi-hop` |
    `squash-floor` | `blame-error`); empty otherwise.
- `DefectDensity` (per-file rollup, emitted to the metrics artifact) — inducing-commit count
  per file over a window, WITH the file's trace-rate beside it (a density computed over a
  low trace-rate is a floor, not a count). This is the field brief 08 reads as
  `defect_density` and brief 09 reads for the coupling/coverage advice.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit a rendered single-writer view (`QUALITY.md`) on a branch (CI is its only
  writer). The JSONL artifacts this brief writes are diffable and branch-safe.
- Honest-claims (spec §10): NO derived number is emitted without its trace-rate and evidence
  tier alongside it. A bare rate is a bug, not a formatting choice.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `qualgen/blame.go` (planned): implement the blame helper — given (file, line-ranges, commit), return
   the last-touching commit per line. Prefer go-git; if a recorded benchmark shows the git
   binary is materially faster at scale, route through the brief-01 fallback seam and record
   the benchmark in Evidence. The helper is three-state at its own boundary: a blame that
   errors returns a typed `blame-error`, never an empty-but-successful result.
2. `qualgen/szz.go` (planned): for each `DefectFix`, compute the deleted/modified line set from the
   fix diff, blame those lines at the fix's PARENT commit, and collect candidate inducing
   commits + their PRs. Resolve each candidate through the fix→parent tree so the blame is
   against the state the fix changed, not the tip.
3. `qualgen/refine.go` (planned): apply, in order — (a) drop cosmetic/format-only candidates
   (whitespace/formatting-only diffs contribute no inducer); (b) drop candidates whose
   commit date POSTDATES the defect report date from the `DefectFix` (a change made after
   the bug was reported cannot have induced it); (c) score `confidence` from how many
   candidates survive and whether they agree. Emit the `DefectTrace` with its `trace_state`.
4. Assign `trace_state` per the three-state rule: ≥1 surviving inducer → `traced`; blame
   ran and every candidate was filtered → `traced-none`; blame unreachable → `could-not-trace`
   with a `could_not_trace_reason`. Compute and record the run's overall `trace_rate`
   (traced / total fixes) in the artifact header.
5. `qualgen/szz.go` (planned) (derived metrics, spec §5.3): from the `DefectTrace` corpus compute
   **defect-inducing change rate** (inducing-PRs / merged-PRs per window),
   **per-file defect density** (the `DefectDensity` rollup above), **fix latency**
   (inducing-merge → fix-merge time), and the **traced-CFR** input (traced defect-inducing
   rate, for brief 11's DORA join). Every one carries its trace-rate + tier composition.
6. Emit `defects.jsonl` (append-only) and the defect-metrics rollup, matching the frozen
   field names in the interface contract above.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run SZZ && go test ./... -run Blame && go test ./... -run Refine` | exit 0; trace-engine, blame, and refinement tests pass |
| 3 (DEREFERENCING — planted bug traced end-to-end) | `cd qualgen && go test ./... -run TestSZZ_PlantedBugTracedToInducingCommit -v` | exit 0. The test builds a throwaway fixture repo: commit A introduces a defect on a line, commits B/C are unrelated, commit F is a `fix:` PR that rewrites A's defective line and closes a bug-labeled issue. Assert the emitted `DefectTrace.inducing_commits` contains A's SHA and NOT B/C, `trace_state == "traced"`. |
| 4 (DEREFERENCING — postdating refinement) | `cd qualgen && go test ./... -run TestSZZ_ExcludesInducerPostdatingReport -v` | exit 0. Fixture: the blamed line was last touched by a commit dated AFTER the defect-report date on the fix. Assert that candidate is filtered and the trace resolves `traced-none` (not `traced` on the postdating commit). |
| 5 (DEREFERENCING — three-state, no silent zero) | `cd qualgen && go test ./... -run TestSZZ_UnreachableBlameIsCouldNotTrace -v` | exit 0. Fixture: a squash-merged fix whose pre-image is unreachable by blame. Assert `trace_state == "could-not-trace"` with `could_not_trace_reason == "squash-floor"`, and that this fix is EXCLUDED from the numerator/denominator of defect-inducing rate, not counted as zero. |
| 6 (honest-claims — trace-rate travels) | `cd qualgen && go test ./... -run TestSZZ_DerivedMetricsCarryTraceRate -v` | exit 0. Assert every derived-metric record (`defect_inducing_rate`, `DefectDensity`, `traced_cfr_input`) serializes a non-empty `trace_rate` and `tier_composition` field beside the number — a record with a bare rate fails the test. |
| 7 (cosmetic exclusion) | `cd qualgen && go test ./... -run TestRefine_CosmeticInducerExcluded -v` | exit 0. Fixture: the blamed line's last-touching commit was a whitespace/format-only change; assert it is dropped and blame falls through to the prior real inducer. |
| 8 (contract field names) | `cd qualgen && grep -cE -e inducing_commits -e inducing_prs -e trace_state -e could_not_trace_reason -e evidence_tier -e confidence qualgen/szz.go` | exit 0; count >= 6 (the frozen `DefectTrace` fields the downstream briefs bind to are all present). |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). Include the blame
     benchmark result if the git-binary fallback was chosen. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-09-01 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `09de1a1`

Runner ≠ implementer. Own temporary worktree off `origin/main` at `09de1a1`, offline (`KUBECONFIG=/dev/null`); no live/cluster dependency in any row. Files present at SHA: `qualgen/szz.go`, `qualgen/blame.go`, `qualgen/refine.go` (+ their `_test.go` twins).

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | 0 | build + vet clean |
| 2 | `cd qualgen && go test ./... -run SZZ && … -run Blame && … -run Refine` | 0 | all three suites `ok` |
| 3 | `go test ./... -run TestSZZ_PlantedBugTracedToInducingCommit -v` | 0 | `--- PASS` — planted bug traced to inducing commit A, `trace_state==traced` |
| 4 | `go test ./... -run TestSZZ_ExcludesInducerPostdatingReport -v` | 0 | `--- PASS` — postdating inducer filtered, resolves `traced-none` |
| 5 | `go test ./... -run TestSZZ_UnreachableBlameIsCouldNotTrace -v` | 0 | `--- PASS` — `could-not-trace` + `squash-floor`, excluded from numerator/denominator |
| 6 | `go test ./... -run TestSZZ_DerivedMetricsCarryTraceRate -v` | 0 | `--- PASS` — every derived record carries a non-empty `trace_rate` + `tier_composition` |
| 7 | `go test ./... -run TestRefine_CosmeticInducerExcluded -v` | 0 | `--- PASS` — cosmetic inducer dropped, blame falls through |
| 8 | `cd qualgen && grep -cE -e inducing_commits -e inducing_prs -e trace_state -e could_not_trace_reason -e evidence_tier -e confidence szz.go` | 0 | count = 11 (≥ 6) — all frozen `DefectTrace` fields present. (brief command names `qualgen/szz.go` while already in `qualgen/` — a brief-side path typo; run as `szz.go`) |

`RISK-VALUE: N/A` — no risk-bearing literal gates any irreversible/regulated/funds action. The nearest candidate, `confidence`, is by design a recorded (non-gating) field — `scoreConfidence` @ qualgen/refine.go:214-235; brief and code both state confidence is a recorded field, not a gate. The `len(survivors) >= 1` three-state classification @ qualgen/szz.go:289 is a measurement branch, not a guard. Read-only git-history miner writing only diffable JSONL.

**VERIFY: PASS** — all eight Verify rows checked-clean by a non-implementer. Advancing `implemented → verified`.

## Review
Gate: model (all four risk answers no — a read-only history miner over any git repo; no
regulated/customer/irreversible/sensitive surface, writes only diffable JSONL artifacts to
an operator-chosen tracking root). `exec-tier: strong` because SZZ correctness is exactly the
kind of subtle logic that passes naive presence tests while being wrong (question c) and the
blame walk is cross-artifact reasoning (question b) — the reviewer should confirm the
planted-bug and postdating fixtures actually exercise the refinement logic, not just presence.
Reviewer records verdict + date in the stream README table.
