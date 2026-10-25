---
brief: assay:assay:milestone-support:10
title: "Enforce milestone capabilities at dispatch and pickup"
why: "A correct graph is not enough if the dispatcher or resumed worker executes a stale or unsupported work packet."
wave: 6
depends: ["milestone-support/07", "milestone-support/08", "milestone-support/09"]
unblocks: ["milestone-support/14"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: db2d526f-c30d-4771-b889-54213ce9482b
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes worker start/resume admission; stale readiness or unknown capabilities must not launch unsupported work with possible irreversible effects."
decision-trigger: spec
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "tools/desk/cmd/deskdispatch/: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskboard/admission.go: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/deskkit/admission.go: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/loopengine/: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/internal/dispatchkit/: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up milestone-support/10 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-14-qualify-the-two-binary-rollout-and-prepare-release-adoption.md: follow-up milestone-support/14 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 10 — Enforce milestone capabilities at dispatch and pickup

## Context

files: `tools/desk/cmd/deskdispatch/`, `tools/desk/cmd/deskboard/admission.go`, `tools/desk/internal/deskkit/admission.go`, `tools/desk/internal/loopengine/`, `tools/desk/internal/dispatchkit/` (planned), `plugins/assay/skills/worker-desk/SKILL.md`, `plugins/assay/skills/the-desk/SKILL.md`, `changelog/milestone-support-10.md` (planned), `statusgen/testdata/milestone-support/10-mutations.json` (planned). New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: deskdispatch and worker admission owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "deskdispatch and worker admission"
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

## Task

1. Consume the canonical eligibility output and readercaps assessment at selection and immediately before claim/start/resume. Bind packet source revisions and required capability/brief versions; re-read current holds at the existing admission boundary so a stale eligible snapshot cannot authorize launch.
2. Keep requirement failure before claim or execution; absent/unknown peer support, changed revision, missing acceptance and parked streams hold explicitly. An unrelated final-only prerequisite does not block independent contributors. A milestone remains an outcome, never a worker task.
3. Update dispatcher packets and existing worker/coordinator procedures to consume reader results rather than independently parse the milestone graph. Preserve role identity, capacity, claim generation, human gates and no-ambient-auth behavior. All tests use fake transports; no live dispatch or role-loop activation occurs.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneDispatch$' ./cmd/deskdispatch/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneDispatch ' "$milestone_verify_out")` | exit 0 and named PASS; actual packet/claim/start path launches only eligible compatible work; missing capability, changed revision or parked source records zero claims/starts. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestonePickupRecheck$' ./internal/deskkit/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestonePickupRecheck ' "$milestone_verify_out")` | exit 0 and named PASS; resume/pickup catches changed acceptance/version after queue read; bypassing the outer selector still leaves the admission check effective. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
