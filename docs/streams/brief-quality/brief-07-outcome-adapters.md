---
brief: "assay:assay:brief-quality:07"
title: "Collect execution outcomes and adjudicated authoring discoveries"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 4
depends: ["brief-quality/05"]
unblocks: ["brief-quality/08"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
consumers:
  - "qualgen/briefquality/adapters.go: follow-up brief-quality/07"
  - "qualgen/briefquality/adapters_test.go: follow-up brief-quality/07"
  - "qualgen/telemetry/: follow-up brief-quality/07"
  - "qualgen/attribution/: follow-up brief-quality/07"
  - "statusgen/verifyoutcomes.go: follow-up brief-quality/07"
issues: []
schema: "brief-v2"
id: "b2185667-fe74-43bc-84f4-e645eae20250"
version: 1
authored: "2026-09-26 by the authoring session recorded in commit 6a7d90b97"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 07 — Collect execution outcomes and adjudicated authoring discoveries

## Context

files:
- `qualgen/briefquality/adapters.go (NEW)`
- `qualgen/briefquality/adapters_test.go (NEW)`
- `qualgen/telemetry/`
- `qualgen/attribution/`
- `statusgen/verifyoutcomes.go`

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
1. Join existing witnesses/history, approved cost exports and findings to stable brief and attempt identities.
2. Capture discoveries, dispositions, amendments, failed/abandoned attempts and late defects with separate introduced/caught stages.
3. Require source lineage; do not infer actual model, active time, cost or causal attribution from missing data.
4. Read operator-supplied offline exports only; add producer contract documentation and a fixture from each adapter.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd qualgen && go test -run '^TestOutcomeAdapterUnknownLineageDistinct$' -v ./briefquality/... > "${TMPDIR:-/tmp}/bq07a.out" 2>&1 && grep -q -F -- '--- PASS: TestOutcomeAdapterUnknownLineageDistinct' "${TMPDIR:-/tmp}/bq07a.out" && go test -run '^TestOutcomeAdapterMissingCostDistinct$' -v ./briefquality/... > "${TMPDIR:-/tmp}/bq07b.out" 2>&1 && grep -q -F -- '--- PASS: TestOutcomeAdapterMissingCostDistinct' "${TMPDIR:-/tmp}/bq07b.out" && go test -run '^TestOutcomeAdapterChangedRequirementsDistinct$' -v ./briefquality/... > "${TMPDIR:-/tmp}/bq07c.out" 2>&1 && grep -q -F -- '--- PASS: TestOutcomeAdapterChangedRequirementsDistinct' "${TMPDIR:-/tmp}/bq07c.out"` | exit 0; a missing test fails the row instead of passing vacuously |
| 2 | check | `cd qualgen && go test ./telemetry/... ./attribution/... -count=1` | exit 0; existing telemetry and stage attribution contracts preserved |
| 90 | check | `statusgen --root . --consumers --brief brief-quality/07` | exit 0 on the implementation branch after dispositions are updated to match the actual diff; inherited/out-of-scope claims remain explicitly unchecked |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
