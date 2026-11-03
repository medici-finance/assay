---
brief: assay:assay:milestone-support:07
title: "Evaluate milestone readiness through the canonical eligibility API"
why: "Dispatch and reporting need one deterministic account of remaining work, outstanding acceptance and unknown coverage."
wave: 5
depends: ["milestone-support/05", "milestone-support/06"]
unblocks: ["milestone-support/10", "milestone-support/11", "milestone-support/12"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: fdbf54d5-7f9d-40c6-93a7-e85458b9d814
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes readiness consumed by dispatch; missing or revoked prerequisite checks could admit work with irreversible external effects."
decision-trigger: spec
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/evaluation/: follow-up milestone-support/07 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/nextup.go: follow-up milestone-support/07 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/eligibility.go: follow-up milestone-support/07 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/testdata/milestone-support/07/: follow-up milestone-support/07 (this delivery; replace with fixed-here in implementation)"
  - "docs/library-first.md: follow-up milestone-support/07 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md: follow-up milestone-support/10 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-11-project-milestone-state-into-stream-views-and-deskboard.md: follow-up milestone-support/11 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-12-preserve-milestone-scope-and-identity-in-history-and-exports.md: follow-up milestone-support/12 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 07 — Evaluate milestone readiness through the canonical eligibility API

## Context

files: `statusgen/evaluation/`, `statusgen/nextup.go`, `statusgen/eligibility.go`, `statusgen/testdata/milestone-support/07/` (planned), `docs/library-first.md`, `changelog/milestone-support-07.md` (planned), `statusgen/testdata/milestone-support/07-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen/evaluation owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen/evaluation"
  contract: S-eligibility
  retires: ["duplicated interpretation or implicit state inference at the named existing consumer boundary"]
  weight: "New binaries 0; new services 0; required pure API/schema/fixture changes only. Measure actual rule/flag/refusal deltas at implementation review; no current weight reduction is claimed."
  why-add: "Reuse the existing semantic owner and public module. This records a missing shared meaning; moving it into a helper process or copying another parser would add authority or drift without benefit."

single-point-of-failure: incorrect shared identity or capability interpretation; retain independent current-subject, authority and consumer checks and plant faults with the outer check bypassed where Verify names that boundary.

## Read first

- [Implementation scope and consumer census](README.md#implementation-scope).
- [Milestone contract](../../../spec/milestone-v1.md).
- [Brief contract and compatibility](../../../spec/brief-v1.md).
- [Stream contract](../../../spec/stream-v1.md).
- [Library-first direction](../../library-first.md).
- [Existing semantic owners](../../contracts.md).
- [Dependency waves and adoption holds](README.md).

## Ground rules

- Own isolated branch and draft implementation PR. Do not merge, flip status/spec/stream/PR readiness, dispatch work, cut a tag, change installed pins or contact a cluster.
- Scope approval and stream activation take effect on the maintainer's merge of PR #4182; this brief has no completion evidence. A non-implementer owns post-merge verification; never fill Evidence from authorship or a fixture expectation.
- Refresh main, active claims/PRs, module imports, source contracts and consumer inventory at pickup. Record brief/source revisions, actual public API availability, coverage/unknowns and continue/adapt/hold in the implementation handoff. No obsolete active claim is overwritten.
- Preserve existing credentials, independent human/risk gates, UUIDs, archived evidence and exact-subject meanings. No ambient-auth fallback or automatic bulk migration.
- Shared resolution and capability assessment MUST be direct library calls, not subprocess calls to statusgen, desk-tools or a new helper executable. External I/O remains in existing adapters.

## Human decision

Before changing the governed boundary, present the concrete implementation/design through the existing decision mechanism. Confirm the preserved actor, subject, revocation and effect-admission rules. Options: accept the bounded implementation; request corrections; hold. No answer supplies no approval. Do not infer this ruling from the planning request.

Default if no answer: none — blocks until answered

## Task

1. Extend the existing EvaluateEligibility/coverage owner rather than create a second planner. Consume normalized graph and acceptance observations; return work remaining, work complete/acceptance outstanding, accepted-for-subject and current availability as distinct fields with typed holds.
2. Apply explicit brief capability start requirements without copying final-only milestone requirements onto contributors. Retain existing risk, design, human decision, verification and parked/paused admission rules. Eligibility is not permission, capacity or a claim.
3. Compute per-outcome deduplicated work counts and relevant blocking heads with incomplete coverage flagged. Keep verified and implemented unfinished, withdrawal unsatisfied, and required parked work visible but not dispatchable. Legacy no-milestone fixtures retain their current eligibility.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneEligibility$' ./evaluation/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneEligibility ' "$milestone_verify_out")` | exit 0 and named PASS; shared evaluator permits independent contributor, holds actual capability consumer and preserves all legacy/human/parked holds; bypassing a required hold fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNextupMilestoneCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNextupMilestoneCoverage ' "$milestone_verify_out")` | exit 0 and named PASS; actual Next-up adapter consumes canonical results and never promotes incomplete roots, withdrawn prerequisites or missing acceptance as ready. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
