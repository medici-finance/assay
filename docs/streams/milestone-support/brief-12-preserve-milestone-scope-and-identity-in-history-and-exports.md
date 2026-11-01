---
brief: assay:assay:milestone-support:12
title: "Preserve milestone scope and identity in history and exports"
why: "Changing brief membership must explain a growing plan rather than rewrite the past or reset existing work history."
wave: 6
depends: ["milestone-support/07", "milestone-support/09"]
unblocks: ["milestone-support/13"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: add56e3a-27ae-4e62-b766-8c67ac074934
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes durable history and import identity interpretation; migration must not rewrite accepted history, duplicate claims or manufacture evidence."
decision-trigger: spec
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/history.go: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/backfill.go: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/briefflow.go: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/flow.go: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/testdata/milestone-support/12/: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "spec/brief-flow-event-v1.md: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/statusgen/README.md: follow-up milestone-support/12 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-13-trace-and-report-a-selected-milestone-through-canonical-readers.md: follow-up milestone-support/13 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 12 — Preserve milestone scope and identity in history and exports

## Context

files: `statusgen/history.go`, `statusgen/backfill.go`, `statusgen/briefflow.go`, `statusgen/flow.go`, `statusgen/testdata/milestone-support/12/` (planned), `spec/brief-flow-event-v1.md`, `docs/streams/statusgen/README.md`, `changelog/milestone-support-12.md` (planned), `statusgen/testdata/milestone-support/12-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen historian/export adapters owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen historian/export adapters"
  contract: none — existing index does not yet name this exact typed record/projection; update its existing owner row in the implementation, not a second registry.
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

1. Extend historian/export/import through versioned adapters, recording source snapshot, effective membership, identity aliases and milestone acceptance references. Preserve append-only historical observations; a delivery milestone does not rename the existing brief-flow event milestone vocabulary.
2. Deduplicate mixed old/new refs by permanent identity, keep version/subject distinct, and preserve prior accepted history on scope or retirement changes. Unsupported import schemas/required semantics refuse before any write; incomplete roots/periods remain missing observations.
3. Provide historical count inputs for seven daily observations plus the preceding three seven-day period ends with explicit timezone/coverage. Count the membership existing at each observation, not today's membership projected backwards. Withdrawal/blocked work retain their distinct meaning and no fake zero samples are inserted.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneHistory$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneHistory ' "$milestone_verify_out")` | exit 0 and named PASS; same brief keeps one trajectory across aliases, scope growth changes total without erasing past acceptance; present-membership backfill fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneImportExport$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneImportExport ' "$milestone_verify_out")` | exit 0 and named PASS; actual adapters round-trip legacy/current observations and refuse unsupported or mismatched subjects before writing; missing period is unknown, not zero. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
