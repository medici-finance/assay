---
brief: "assay:assay:brief-quality:06"
title: "Bind independent acceptance approval to the dispatch contract"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 4
depends: ["brief-quality/04", "brief-quality/05"]
unblocks: ["brief-quality/08"]
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
gate-why: "Dispatch admission changes authority checks; human confirms review identity and stale-approval refusal cannot weaken existing gates."
decision-trigger: "spec"
consumers:
  - "tools/desk/internal/deskkit/briefacceptance.go: follow-up brief-quality/06"
  - "tools/desk/internal/deskkit/briefacceptance_test.go: follow-up brief-quality/06"
  - "tools/desk/cmd/deskdispatch/: follow-up brief-quality/06"
  - "tools/desk/internal/deskkit/briefschemagate.go: follow-up brief-quality/06"
issues: []
schema: "brief-v2"
id: "e8f154d7-c8a8-46ad-a4b4-5d9e72aa0649"
version: 1
authored: "2026-09-26 by design author"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 06 — Bind independent acceptance approval to the dispatch contract

## Context

files:
- `tools/desk/internal/deskkit/briefacceptance.go (NEW)`
- `tools/desk/internal/deskkit/briefacceptance_test.go (NEW)`
- `tools/desk/cmd/deskdispatch/`
- `tools/desk/internal/deskkit/briefschemagate.go`

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
1. Implement reviewer-originated attestations and two-stage exposure record using existing role identity controls.
2. Require exact contract digest, non-author identity and ratified opt-in policy at admission; record requested and actual model/effort.
3. Invalidate approval on semantic amendment and refuse unknown compatibility; preserve existing human and capability gates.
4. Keep legacy/unselected streams unchanged; test gate-to-dispatch flow with offline fake adapters.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `cd tools/desk && go test ./internal/deskkit ./cmd/deskdispatch -count=1` | exit 0; self-review, stale approval and missing policy refuse; valid reviewed contract admits |
| 2 | check | `cd tools/desk && go test ./cmd/deskdispatch -count=1` | exit 0; neighboring dispatch controls still pass |
| 90 | check | `statusgen --root . --consumers --brief brief-quality/06` | exit 0 on the implementation branch after dispositions are updated to match the actual diff; inherited/out-of-scope claims remain explicitly unchecked |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: human. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
