---
brief: "assay:assay:brief-quality:01"
title: "Define the brief assessment and outcome contract"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 0
depends: []
unblocks: ["brief-quality/02"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
id: "4a742172-c319-44ef-985d-48fcb1efdd0b"
version: 1
authored: "2026-09-26 by design author"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 01 — Define the brief assessment and outcome contract

## Context

files:
- `docs/streams/brief-quality/spec.md`

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
1. Define the six anchored dimensions, independent review contract and separate outcome record.
2. Specify data ownership, reuse boundaries, three-state metrics, migration and open rulings.
3. Author dependent briefs and demonstrate the dependency graph is acyclic.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `python3 -c "from pathlib import Path; s=Path('docs/streams/brief-quality/spec.md').read_text(); assert all(x in s for x in ['Work size','Specification completeness','Reasoning difficulty','Coupling','Verification strength','Failure consequence'])"` | exit 0; six named dimensions present; adequacy remains review-only |
| 91 | check +dereference | `python3 -c "from pathlib import Path; assert Path('statusgen/briefv2.go').is_file(); assert 'parseBriefV2Keys' in Path('statusgen/briefv2.go').read_text(); assert Path('qualgen/telemetry/source.go').is_file()"` | exit 0; cited parser and telemetry seams resolve in this checkout; this does not prove document adequacy |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

Author checks on 2026-09-26 (not independent verification):

- `cd statusgen && go run . --root .. --lint`: exit 0, `LINT: PASS`; whole-corpus legacy notices remain, no brief-quality notices after correction.
- Loaded all nine YAML frontmatters and checked every dependency has its inverse unblock and strictly earlier wave: passed.
- Cited parser/telemetry paths and `parseBriefV2Keys` resolve at the recorded baseline.
- Independent editorial review found active-stream/draft-spec mismatch; fixed to parked with a spec pointer. That bounded review is not a full specification approval.

No verified/done claim. Proposed implementation tests are unrun because their deliverables do not exist yet.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
