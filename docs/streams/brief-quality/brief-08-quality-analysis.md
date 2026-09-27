---
brief: "assay:assay:brief-quality:08"
title: "Report brief-authoring quality and cost with coverage"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 5
depends: ["brief-quality/06", "brief-quality/07"]
unblocks: ["brief-quality/09"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
consumers:
  - "qualgen/briefquality/reduce.go: follow-up brief-quality/08"
  - "qualgen/briefquality/render.go: follow-up brief-quality/08"
  - "qualgen/briefquality/reduce_test.go: follow-up brief-quality/08"
  - "qualgen/main.go: follow-up brief-quality/08"
  - "qualgen/README.md: follow-up brief-quality/08"
issues: []
schema: "brief-v2"
id: "3ff3f138-36fd-4d8a-8725-0aa4bc58ef5d"
version: 1
authored: "2026-09-26 by the authoring session recorded in commit 6a7d90b97"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 08 — Report brief-authoring quality and cost with coverage

## Context

files:
- `qualgen/briefquality/reduce.go (NEW)`
- `qualgen/briefquality/render.go (NEW)`
- `qualgen/briefquality/reduce_test.go (NEW)`
- `qualgen/main.go`
- `qualgen/README.md` (planned)

facts:
- Read-first: `docs/streams/brief-quality/spec.md`; this is a proposed design, not active fleet policy.
- Current source was inspected at `7aa3835d7` on 2026-09-26; recheck before implementation.
- Existing brief-v2 IDs/version and qualgen telemetry/attribution are the integration seams.
- No live infrastructure, automatic publication, or raw session collection is needed.

layering: Extend existing tools; pure assessment/reduction rules in qualgen/briefquality, file/identity/dispatch effects at adapters. Test both separately and one full offline join.

## Ground rules
- Preserve human gates and capability minima. Stop at implemented; a non-implementer verifies.
- If facts conflict with current source, record NEEDS_CONTEXT rather than guessing.
- Follow the ruled pilot policy; unknown evidence is never a passing result.

## Task
1. Implement the spec metric definitions in a pure reducer with denominator, coverage, censoring and policy/cohort versions.
2. Expose qualgen briefs offline command with JSON, Markdown and self-contained HTML from one result; include evidence drill-down.
3. Keep tokens/currencies separate and incomplete cost partial; include unsuccessful attempts in fixed-cohort cost.
4. Golden-test complete, sparse, zero, late-corrected and abandoned cohorts; outputs label synthetic samples and advisory recommendations.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `cd qualgen && go test -run '^TestQualityAnalysisDenominatorsMatchGolden$' -v ./briefquality/... > "${TMPDIR:-/tmp}/bq08a.out" 2>&1 && grep -q -F -- '--- PASS: TestQualityAnalysisDenominatorsMatchGolden' "${TMPDIR:-/tmp}/bq08a.out" && go test -run '^TestQualityAnalysisMissingStatesMatchGolden$' -v ./briefquality/... > "${TMPDIR:-/tmp}/bq08b.out" 2>&1 && grep -q -F -- '--- PASS: TestQualityAnalysisMissingStatesMatchGolden' "${TMPDIR:-/tmp}/bq08b.out"` | exit 0; a missing test fails the row instead of passing vacuously |
| 2 | check | `cd qualgen && go test ./... -count=1` | exit 0; neighboring quality commands preserved |
| 90 | check | `statusgen --root . --consumers --brief brief-quality/08` | exit 0 on the implementation branch after dispositions are updated to match the actual diff; inherited/out-of-scope claims remain explicitly unchecked |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
