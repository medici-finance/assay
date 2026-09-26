---
brief: "assay:assay:brief-quality:05"
title: "Add portable brief-quality events and immutable snapshots"
why: "Make brief authoring measurable and correctable while preserving independent evidence and human authority."
wave: 3
depends: ["brief-quality/03"]
unblocks: ["brief-quality/06", "brief-quality/07"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
consumers:
  - "qualgen/briefquality/events.go: follow-up brief-quality/05"
  - "qualgen/briefquality/store.go: follow-up brief-quality/05"
  - "qualgen/briefquality/events_test.go: follow-up brief-quality/05"
  - "qualgen/briefquality/testdata/: follow-up brief-quality/05"
  - "qualgen/telemetry/source.go: follow-up brief-quality/05"
issues: []
schema: "brief-v2"
id: "84fefd89-9fb1-4b8a-90e0-cf6f1a3f7a56"
version: 1
authored: "2026-09-26 by design author"
sources: ["docs/streams/brief-quality/spec.md", "freshness-checked 2026-09-26 @ 7aa3835d7"]
exec-tier: "strong"
exec-tier-why: "Cross-contract design and attribution errors can survive happy-path tests."
outcome: "none"
---

# Brief 05 — Add portable brief-quality events and immutable snapshots

## Context

files:
- `qualgen/briefquality/events.go (NEW)`
- `qualgen/briefquality/store.go (NEW)`
- `qualgen/briefquality/events_test.go (NEW)`
- `qualgen/briefquality/testdata/ (NEW)`
- `qualgen/telemetry/source.go`

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
1. Implement versioned envelopes and authored/approved/completed snapshots with recoverable content and canonical digest vectors.
2. Use atomic per-event writes, idempotency, conflict detection, corrections and explicit unavailable fields.
3. Define role/attempt IDs, policy/model/effort provenance and data minimization; store no prompts or raw sessions.
4. Prove concurrent writes, replay, conflicting ID, missing content and redaction tombstones with synthetic fixtures.

## Verify

Run from the repository root unless the command changes directory. Proposed test names are deliverables, not claims that tests exist today. Doc-only checks establish presence; review owns semantic adequacy.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check | `cd qualgen && go test ./briefquality/... -count=1` | exit 0; seeded missing/conflicting records rejected or visibly incomplete |
| 2 | check | `cd qualgen && go test -race ./briefquality/... -count=1` | exit 0; concurrent writers retain all unique records |
| 90 | check | `statusgen --root . --consumers --brief brief-quality/05` | exit 0 on the implementation branch after dispositions are updated to match the actual diff; inherited/out-of-scope claims remain explicitly unchecked |

## Pre-mortem

A plausible implementation could report missing data as success or change the original contract after seeing results. The negative fixtures in the spec must demonstrate those cases remain visible. For document-only work, independent review checks that commands discriminate behavior and that recommendations are not recorded as rulings; text-presence checks cannot establish either.

## Evidence

No independent implementation evidence yet.

## Review

Gate: model. Reviewer confirms the spec contract and the declared limitations. No hand-authored verified/done claim.
