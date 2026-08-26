---
brief: statusgen/08
title: Composite AssayScore computation
wave: 2
depends: ["statusgen/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 (re-authored clean for the statusgen board)
sources:
  - "The settled metric-definitions spec §AssayScore — the fixed composite formula"
---

# Brief 08 — Composite AssayScore computation

## Context
files: `statusgen/` (new `assayscore.go` (planned) beside the brief-flow metrics of statusgen/07).
single-point-of-failure: the score is a derived summary — the SPOF is a wrong normalization silently
  skewing it; mitigated by (a) always emitting the four sub-scores + raw inputs beside the composite so
  a wrong score is visibly inconsistent with its parts, and (b) a golden-value unit test.
facts:
- AssayScore = geometric mean of four 0–100 sub-scores: Speed, Flow, Quality, Value (formula in the
  settled metric-definitions spec)
- sub-score normalization is self-relative to a trailing-90-day baseline until cross-org benchmarks exist
- ALWAYS emit composite + 4 sub-scores + raw inputs together (transparency is the product)
- a dimension in `could-not-check` must NOT silently count as 0 — the composite reports partial with the
  missing dimension named (geometric mean over available dims + an explicit `incomplete` flag)

## Ground rules
- NEVER git push. Stop at `implemented`.
- The formula is FIXED by the settled metric-definitions spec — do not invent an alternative here;
  implement the settled one.
- Report NEEDS_CONTEXT if a sub-score's normalization band is unspecified rather than choosing one.

## Task
1. Implement `statusgen --assayscore --json`: compute the four sub-scores from the statusgen/07 metrics,
   normalize each to 0–100 per the settled spec, take the geometric mean.
2. Emit `{score, subscores:{speed,flow,quality,value}, inputs:{...}, baseline_window, state, incomplete}`.
3. Handle `could-not-check` dimensions explicitly: exclude from the geometric mean, set `incomplete:true`,
   list the missing dimension(s) — never coerce to 0.
4. Golden test: a fixture of known metric inputs → an asserted score value, plus an `incomplete` case.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go run . --root .. --assayscore --json 2>/dev/null \| python3 -c "import json,sys;d=json.load(sys.stdin);assert 'score' in d and 'subscores' in d;print('ok')"` | prints `ok` |
| 2 | `cd statusgen && go run . --root .. --assayscore --json 2>/dev/null \| python3 -c "import json,sys;d=json.load(sys.stdin);print(len(d['subscores']))"` | `4` |
| 3 | `cd statusgen && go test ./ -run AssayScore 2>&1 \| tail -1` | contains `ok` (golden value + incomplete case) — DEREFERENCES the formula against a known-input fixture |
| 4 | `cd statusgen && go run . --root .. --assayscore --json 2>/dev/null \| grep -q '"incomplete"'` | exit 0 (partial-data flag present) |
| 5 | `cd statusgen && go run . --root .. --lint` | exit 0 (`LINT: PASS`) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model. Reviewer recomputes the golden fixture by hand and confirms a `could-not-check` dimension
sets `incomplete` rather than dragging the score toward 0. Verdict + date in the README.
