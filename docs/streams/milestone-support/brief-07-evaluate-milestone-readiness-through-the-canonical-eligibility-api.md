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

## Proposed readiness integration — pending human ruling

**Proposal only. No implementation, Task/Verify change, Evidence or lifecycle credit is
claimed.** This brief is human-gated, and its decision issue is open with no ruling yet.
This appendix sets out the concrete design that the Human decision section asks for, so
the maintainer can accept it, correct it or hold it. Until an unedited human ruling
names this proposal, nothing here changes a shared value, advertises a capability or
makes a milestone ready.

### Architecture pickup and source coverage

Manual bootstrap at source commit `4502966e88b19860ab06f1a2dbc1729cbcd6c766`, brief
`assay:assay:milestone-support:07`, authored revision 1. The dispatch packet records the
same commit, and the worktree head matched it before editing. Both dependencies are
delivered on main: /05 is verified, and /06 is implemented with its Evidence PASS landed.
The table lists the source contracts that were inspected. None of them is a new approval.

| Source | What the design relies on |
|---|---|
| `statusgen/evaluation/eligibility.go` (`EvaluateEligibility`, `S-eligibility`) | The canonical eligibility owner. It takes explicit observations and returns `eligible`, `held` or `eligible-with-notice`, with each edge `satisfied`, `unsatisfied` or `could-not-check`. Pre-resolved gates arrive as `Edge.Resolved`. |
| `statusgen/evaluation/milestonegraph.go` (`BuildMilestoneGraph`) | The combined graph. It holds per-brief start gates and, per milestone, the contributors, prerequisites, completion (`complete`, `blocked`, `unknown` or `cycle`), blockers and unknowns. It also lists cycles and provides the deduplicating `ContributorCount`. A start gate is never satisfied today, because this reader does not observe acceptance. Withdrawn contributors block. |
| `statusgen/milestonegraph.go` (`applyMilestoneStartGates`) | The host adapter. It writes each start gate onto the brief row as a `behavioural-gate` edge, pre-resolved to `unsatisfied` or `could-not-check`. |
| `statusgen/domain` (`AssessMilestone`, `S-verification-domain`) and `statusgen/milestoneacceptance.go` | The /06 receipt assessment. It records immutable history, a present state (`supported`, `blocked` or `unknown`), whether acceptance is outstanding, blockers, unknowns and quarantined input. No production selector is installed, and nothing calls it in production. |
| `statusgen/briefschemaversions.go` (`statusgenReaderCaps`) and `statusgen/milestones.go` | Only `syntax` and `preflight` are advertised. While `milestone-v1` is unadvertised, milestone records are not read and every start requirement is `could-not-check`. |
| `statusgen/nextup.go` (`eligibleBase`, `eligible`) and `statusgen/drivefrontier.go` | Next-up and the drive frontier read one eligibility map. Every existing exclusion is applied outside the evaluator: inactive stream, stale ref, homed-in, merged in a sibling, phantom, claimed, measures queue, placeholder and the legacy wave rule. |
| `spec/milestone-v1.md` §5, §6, §9 and its implementation sections | Unknown never means "no dependency" or "done". The three outcome views and current availability stay distinct. Revoked acceptance never satisfies, withdrawn work does not count, shared briefs count once, and legacy streams are unchanged. |

Import direction fixes the package boundary. `statusgen/domain` already imports
`statusgen/evaluation`, so the evaluator cannot import the receipt types without
creating a cycle. The /06 result therefore enters the evaluator as plain values, and the
statusgen host does the mapping.

### Bounded design to rule on

The design extends the existing owner. It does not add a second planner.

1. **One evaluator, one optional input.** `EligibilityInput` gains an optional
   `Milestones` field. It carries the `MilestoneGraph` that `BuildMilestoneGraph`
   already builds, plus a list of acceptance observations. A new
   `EvaluateReadiness(EligibilityInput)`, in the same `S-eligibility` owner, returns
   the existing per-brief `Eligibility` map plus a per-milestone readiness map and a
   coverage summary. `EvaluateEligibility` stays, as the per-brief projection of the
   same call. When `Milestones` is nil, its output is byte-identical to today's, which
   is the legacy guarantee.
2. **Plain acceptance observations.** Each observation names the exact milestone,
   promise version and subject. It carries a present state (`accepted`, `outstanding`,
   `revoked`, `superseded` or `unknown`), with reasons and a history count. The
   statusgen host maps it from `domain.MilestoneAssessment` without re-deriving
   acceptance. The evaluator never reads comments, decisions or evidence itself.
3. **Distinct per-milestone fields.** Each milestone reports four separate values, never
   folded into one verdict:
   - work remaining: contributing and prerequisite briefs not yet `done`, deduplicated
     by canonical brief identity across the milestone and its prerequisite graph.
     `implemented` and `verified` count as remaining. Withdrawn or missing work is a
     blocker and never counts as progress; unread prerequisite coverage stays unknown;
   - work complete with acceptance outstanding: every contributor is `done` and every
     prerequisite promise has qualifying acceptance for the consuming milestone's
     environment, interface and workload profile, but this milestone has no current
     acceptance for its exact version and subject. A prerequisite with remaining work,
     missing acceptance or unknown coverage cannot meet these completion requirements;
   - accepted for subject: the exact milestone version and subject that have current
     acceptance. Revoked or superseded history stays visible and never satisfies;
   - current availability: `supported`, `blocked` or `unknown`. Typed holds carry the
     blocker and unknown lists and the blocking heads, meaning the not-done contributors
     and unaccepted prerequisites nearest the root. An incomplete-coverage flag is set
     whenever any root, record, version or observation was not read.
4. **Start gates.** A brief's `requires-milestones` entry is `satisfied` only when two
   things hold: the milestone's current availability is `supported`, and an observation
   shows current acceptance for the exact version and subject. Known remaining work, a
   cycle, a withdrawn contributor or a revoked or superseded acceptance makes the entry
   `unsatisfied`. A missing observation, absent selector, unread record or unadvertised
   reader capability makes it `could-not-check`. Satisfying a start gate removes only
   that one hold and never overrides any other hold.
5. **Final-only requirements stay final-only.** A milestone's own `requires-milestones`
   holds only that milestone's acceptance. It is never copied onto contributing briefs.
   A contributor without its own start requirement therefore keeps its current
   eligibility (spec §9: brief 11 starts, and brief 12, which has an explicit
   requirement, stays held).
6. **Every existing rule is retained.** The risk, design, human-decision, verification,
   placeholder, legacy-wave, measures-queue, claimed, phantom, homed-in, merged-sibling
   and parked/paused rules all apply unchanged. Required work in a parked or paused
   stream is listed as a blocking head and stays non-dispatchable.
7. **Host wiring is kept narrow.** `applyMilestoneStartGates` stops pre-resolving the
   gate itself and passes the graph and the mapped observations through
   `EvaluateReadiness`. Next-up and the drive frontier keep reading the per-brief map.
   Start requirements remain in `Gates`, never `Feathers`; an edge with no `Resolved`
   value must still resolve fail-closed. No reader capability is advertised and no
   production acceptance selector is installed. On main, every start requirement therefore stays `could-not-check` until
   /10 and /14 turn the operational path on.

### Preserved actor, subject, revocation and effect-admission rules

The Human decision section asks for these four rules to be confirmed explicitly.

- **Actor.** Unchanged from `DR-milestone-support-06`. Only the authorized human
  identity on the protected source can accept, reject, hold or revoke. The evaluator
  holds no identity and accepts nothing. It only reads the result the host maps.
- **Subject.** Every comparison uses the exact milestone, promise version and subject.
  An acceptance for another version, subject or profile never satisfies, and a newer
  promise version does not inherit an older acceptance.
- **Revocation.** A revoked or superseded acceptance keeps its history. It never
  satisfies a start gate and never counts as accepted for the subject. If the
  revocation source cannot be read, the present state is `unknown`; an unreadable
  source never proves that no revocation exists.
- **Effect admission.** None. Eligibility and readiness are reports only. They grant no
  permission, capacity, claim, dispatch admission or acceptance. Dispatch admission
  stays with /10. No new store, queue or acceptance authority is introduced.

### Acceptance cases for the post-ruling implementation

These are proposed cases for the two existing Verify rows. They are not new executable
checks or reported results. The original commands and Evidence remain unchanged.

| Existing future test | Case and expected discrimination |
|---|---|
| `TestMilestoneEligibility` (planned) | Brief 11 belongs to a milestone whose only prerequisite is final-only, and it stays `eligible`. Brief 12 has an explicit `requires-milestones` on an unaccepted milestone and is `held`. Mutation: copying final-only requirements onto contributors holds brief 11, so the test fails. |
| `TestMilestoneEligibility` (planned) | All contributors are `done` and every prerequisite promise has qualifying acceptance, but this milestone has no acceptance: the result reports acceptance outstanding, and the start gate stays unsatisfied or could-not-check. Exact current acceptance is the positive control. Revoked, superseded, wrong-version and wrong-subject observations never satisfy. |
| `TestMilestoneEligibility` (planned) | All local contributors are `done`, but prerequisite P has remaining briefs: report those briefs as remaining and do not report work complete with acceptance outstanding. A shared not-done brief reached directly and through multiple prerequisites counts once. Mutation: omitting prerequisite traversal or deduplication fails. When P's briefs become `done` but P lacks qualifying acceptance (or its coverage is unknown), completion requirements still are not met. Only qualifying acceptance for every prerequisite permits this milestone's acceptance-outstanding view. |
| `TestMilestoneEligibility` (planned) | `implemented`, `verified` and withdrawn contributors count as remaining or blocking. A brief shared by two milestones is counted once per outcome. Mutation: counting `verified` as done, or counting withdrawal as complete, fails. |
| `TestMilestoneEligibility` (planned) | A satisfied start gate never lifts a human, design, risk, verification, placeholder or parked hold. Mutation: returning `eligible` as soon as the start gate is satisfied fails. A nil `Milestones` reproduces the legacy fixtures byte for byte. |
| `TestNextupMilestoneCoverage` (planned) | Next-up, run through the real host adapter, never lists a brief whose milestone root is unread, whose prerequisite is withdrawn or whose acceptance is missing. The incomplete-coverage flag and blocking heads are surfaced. Required parked work is visible and non-dispatchable. |
| `TestMilestoneEligibility` (planned) | An unresolved milestone start edge remains a `Gates` hold. Two separate mutations in `07-mutations.json` (planned) must fail through the production evaluator/host flow: moving the edge into `Feathers`, and treating an absent `Resolved` value as satisfied. Neither mutation may yield `eligible` or `eligible-with-notice`. |
| `TestNextupMilestoneCoverage` (planned) | With the reader capability unadvertised, as it is on main, every start requirement is `could-not-check`, and Next-up output for non-milestone streams is unchanged. |

After the ruling, these cases are implemented through the actual entrypoints. The
compiling mutations are recorded in the declared `07-mutations.json`, and fail-first
results are shown before the implementation PR opens. Neither named test exists at this
source revision.

### Consumer census and transition

| Consumer | Disposition |
|---|---|
| This brief's design appendix and design changelog fragment | fixed-here: authoring material only; no shared runtime value changes. |
| `statusgen/evaluation/eligibility.go`, a sibling readiness file in the same owner, and `docs/contracts.md` row `S-eligibility` | follow-up /07 after the ruling; the evaluator version changes wherever a verdict can change. |
| `statusgen/milestonegraph.go` host adapter and `statusgen/milestoneacceptance.go` mapping | follow-up /07; maps the /06 assessment to plain observations without re-deriving acceptance. |
| `statusgen/nextup.go`, `statusgen/drivefrontier.go`, the `--eligibility` output and its could-not-check notices | follow-up /07; consume the canonical result and keep every existing exclusion. |
| `statusgen/flow.go` depends reading | out-of-scope: brief `depends` semantics are unchanged. |
| `spec/milestone-v1.md` start-gate and implementation sections, `docs/library-first.md` eligibility row | follow-up /07; the canonical text is updated only after the ruling. |
| Reader capability advertisement, dispatch admission and operational rollout | follow-up /10 and /14; nothing is advertised here. |
| Board, historian/export and selected-outcome trace | follow-up /11, /12 and /13; consume the canonical result. |
| The desk loop engine's own eligibility verdict type | out-of-scope: it has a different meaning and is not this evaluator. |

### Decision block

**Owner:** the maintainer authorized by the existing /07 decision issue. This gate
approves how /07 will be implemented. It does not accept any milestone outcome.

1. **Accept the bounded readiness integration (recommended).** This authorizes:
   - the optional `Milestones` input and `EvaluateReadiness` in the `S-eligibility`
     owner;
   - the four distinct per-milestone fields, with typed holds;
   - start gates satisfied only by exact current acceptance;
   - final-only requirements never copied onto contributors;
   - every existing hold retained;
   - the actor, subject, revocation and no-admission rules above.
2. **Request corrections.** Name the field, gate rule, preserved rule or consumer to
   change. Implementation stays held until a revised proposal.
3. **Hold.** Leave /07 at its current lifecycle state, and say what evidence or
   ownership resolution is needed before another proposal.

Exact ruling needed: an unedited human comment on the existing decision issue. It names
`milestone-support/07`, the proposal PR and its head, and `DR-milestone-support-07`,
selects one option, and states any corrections. The reserved record id is
`DR-milestone-support-07`. If the proposal is accepted, the ruling is transcribed into
that decision-v1 record, with alternatives, consequences and a ruling link that can be
corroborated. The brief then cites the record through its `design:` field before
governed implementation starts. Nothing in this proposal creates an approved DR, fills
`decided-by`, records Evidence or advances the lifecycle status.
