---
brief: assay:assay:milestone-support:13
title: "Trace and report a selected milestone through canonical readers"
why: "The report should show the path to a useful outcome with defensible coverage, using the same facts that dispatch consumes."
wave: 7
depends: ["milestone-support/11", "milestone-support/12"]
unblocks: ["milestone-support/14"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 094f43e2-de68-4149-a338-58a6a7b073e4
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "plugins/assay/skills/trace-path-to-live/SKILL.md: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/skills/trace-path-to-live/references/: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/skills/author-spec/SKILL.md: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/skills/author-brief/SKILL.md: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "plugins/assay/references/milestone-authoring.md: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "statusgen/testdata/milestone-support/13/: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "tools/desk/cmd/deskboard/: follow-up milestone-support/13 (this delivery; replace with fixed-here in implementation)"
  - "docs/streams/milestone-support/brief-14-qualify-the-two-binary-rollout-and-prepare-release-adoption.md: follow-up milestone-support/14 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 13 — Trace and report a selected milestone through canonical readers

## Context

files: `plugins/assay/skills/trace-path-to-live/SKILL.md`, `plugins/assay/skills/trace-path-to-live/references/` (planned), `plugins/assay/skills/author-spec/SKILL.md`, `plugins/assay/skills/author-brief/SKILL.md`, `plugins/assay/skills/verify-desk/SKILL.md`, `plugins/assay/references/milestone-authoring.md`, `statusgen/testdata/milestone-support/13/` (planned), `tools/desk/cmd/deskboard/`, `changelog/milestone-support-13.md` (planned), `statusgen/testdata/milestone-support/13-mutations.json` (planned) — New paths and tests are planned implementation deliverables, not current artifacts.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: trace-path-to-live and report readers owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "trace-path-to-live and report readers"
  contract: none — existing index does not yet name this exact typed record/projection; update its existing owner row in the implementation, not a second registry.
  retires: ["duplicated interpretation or implicit state inference at the named existing consumer boundary"]
  weight: "New binaries 0; new services 0; required pure API/schema/fixture changes only. Measure actual rule/flag/refusal deltas at implementation review; no current weight reduction is claimed."
  why-add: "Reuse the existing semantic owner and public module. This records a missing shared meaning; moving it into a helper process or copying another parser would add authority or drift without benefit."

risk-path disposition: deskboard changes are read-only projections/fixtures. They introduce no credential, admission, execution or ruling-policy changes; actual acceptance/dispatch remains at the independently gated existing owners. The declared model gate is intentional for this bounded slice.

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

## Task

1. Extend trace-path-to-live to select a milestone/version and consume canonical graph, eligibility, view and history readers. Do not implement a third dependency parser in a skill or report script. Update author/verify handoffs to use supported canonical records and existing acceptance mechanisms only after capability checks.
2. At the top show Total work, All work remaining, Not started, Being implemented, Awaiting verification and Awaiting final approval over seven daily observations and the preceding three seven-day period ends. Explain scope changes, blocked/withdrawn counts and missing coverage. Counts are briefs, not effort or usable capability percentages.
3. Show only unfinished brief nodes and unsatisfied milestone requirements in the prerequisite graph; keep completed evidence accessible. Report acceptance outstanding even at zero remaining briefs, parked prerequisites without activating them, and incomplete critical paths as provisional. Render a self-contained HTML artifact outside repositories; keep the operation read-only.
4. Regenerate managed Cursor skill/reference copies and validate the bundle. Use checked-in synthetic reader outputs plus the pinned cellctl source example to validate the complete report flow; no live cluster probes or claim/PR mutations occur.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneReportReaders$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneReportReaders ' "$milestone_verify_out")` | exit 0 and named PASS; production reader fixture supplies consistent identity/graph/history facts for the report; dropping a prerequisite or source-coverage field fails. |
| 2 | check:ci +flow +mutation | sh | `(cd tools/desk && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneReportContract$' ./cmd/deskboard/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneReportContract ' "$milestone_verify_out")` | exit 0 and named PASS; cross-consumer projection distinguishes done work from accepted outcome and retains historical scope/missing readings; fixture mutation exposes an inconsistent report. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: model, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.
