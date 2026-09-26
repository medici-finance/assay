---
brief: "assay:assay:brief-quality:03"
title: "Extend brief parsing, lint and templates with six dimensions"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 2
depends: ["brief-quality/02"]
unblocks: ["brief-quality/04", "brief-quality/05"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
consumers:
  - "spec/brief-v1.md: follow-up brief-quality/03"
  - "docs/brief-template.md: follow-up brief-quality/03"
  - "statusgen/brieffile.go: follow-up brief-quality/03"
  - "statusgen/briefv2.go: follow-up brief-quality/03"
  - "statusgen/newbrief.go: follow-up brief-quality/03"
  - "statusgen/schemas/brief-v1.json: follow-up brief-quality/03"
  - "statusgen/schemas/brief-v2.json: follow-up brief-quality/03"
  - "statusgen/briefquality_test.go: follow-up brief-quality/03"
issues: []
schema: "brief-v2"
id: "eca58593-c356-4522-a609-e2667b4e2ce4"
version: 1
authored: "2026-09-26 by design author"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 03 — Extend brief parsing, lint and templates with six dimensions

## Context

files:
- `spec/brief-v1.md`
- `docs/brief-template.md`
- `statusgen/brieffile.go`
- `statusgen/briefv2.go`
- `statusgen/newbrief.go`
- `statusgen/schemas/brief-v1.json`
- `statusgen/schemas/brief-v2.json`
- `statusgen/briefquality_test.go (NEW)`

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
1. Add the versioned assessment extension and anchored vocabulary without duplicating effort or repurposing outcome.
2. Validate categories/rationales/unknown-resolution and expose not-assessed for legacy briefs.
3. Update schema, generator and canonical docs together; maintain v1/v2 compatibility tests and unknown-field migration disclosure.
4. Add negative fixtures and a generated-profile round trip through the dispatcher-facing reader.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd statusgen && go test ./... -count=1` | exit 0; malformed and contradictory fixtures fail, legacy absence stays not-assessed |
| 2 | check | `cd statusgen && go test ./... -count=1` | exit 0; existing graph, generator and requirement outcome semantics preserved |
| 90 | check | `statusgen --root . --consumers --brief brief-quality/03` | exit 0 on the implementation branch after dispositions are updated to match the actual diff; inherited/out-of-scope claims remain explicitly unchecked |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
