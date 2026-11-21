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
design: DR-milestone-support-07
decision-issue: 6114
ruling: "https://github.com/medici-finance/assay/issues/6114#issuecomment-6830782383"
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/evaluation/: fixed-here (canonical readiness implementation)"
  - "statusgen/nextup.go: out-of-scope (no reader edit is required; its existing gate/map contract consumes the changed post-scan host and is exercised by TestNextupMilestoneCoverage)"
  - "statusgen/eligibility.go: out-of-scope (the existing per-brief adapter stays unchanged; the shared EvaluateEligibility projection and post-scan host own this change, with library/CLI parity checked)"
  - "statusgen/testdata/milestone-support/07/: fixed-here (canonical readiness implementation)"
  - "docs/library-first.md: fixed-here (canonical readiness implementation)"
  - "docs/streams/milestone-support/brief-10-enforce-milestone-capabilities-at-dispatch-and-pickup.md: follow-up milestone-support/10 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-11-project-milestone-state-into-stream-views-and-deskboard.md: follow-up milestone-support/11 (consumes this delivery; exact runtime readers enumerated at pickup)"
  - "docs/streams/milestone-support/brief-12-preserve-milestone-scope-and-identity-in-history-and-exports.md: follow-up milestone-support/12 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 07 — Evaluate milestone readiness through the canonical eligibility API

## Context

files: `statusgen/evaluation/`, `statusgen/nextup.go`, `statusgen/eligibility.go`, `statusgen/milestonegraph.go`, `statusgen/milestoneacceptance.go`, `statusgen/model.go`, `statusgen/readiness_test.go`, `statusgen/testdata/milestone-support/07/`, `statusgen/testdata/milestone-support/07-mutate.py`, `statusgen/testdata/milestone-support/07-mutations.json`, `statusgen/testdata/milestone-support/05-mutations.json`, `docs/contracts.md`, `docs/library-first.md`, `spec/milestone-v1.md`, `docs/streams/decisions/DR-milestone-support-07.md`, `changelog/milestone-support-07.md` — Existing Next-up/eligibility readers retain their contracts without edits; the host and canonical owner supply the changed values.

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

The following named tests, fixtures and executable mutation map are implementation deliverables. Independent results belong in Evidence, not inferred from their presence. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneEligibility$' ./evaluation/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneEligibility ' "$milestone_verify_out")` | exit 0 and named PASS; shared evaluator permits independent contributor, holds actual capability consumer and preserves all legacy/human/parked holds; bypassing a required hold fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNextupMilestoneCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNextupMilestoneCoverage ' "$milestone_verify_out")` | exit 0 and named PASS; actual Next-up adapter consumes canonical results and never promotes incomplete roots, withdrawn prerequisites or missing acceptance as ready. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

### Verification run 2026-11-18 (independent verifier, merged main bec90193ce91)

Non-implementer verifier (claude-opus-5-5, strong tier) ran the Verify table against merged main
`bec90193ce916b2d5b11433f25970867a1b7ef95` after a successful `deskdispatch --check-verifier`
admission. The delivering change is PR #6263, merge `b066c7ac58f110a1a038c3bf9c16e85b3e90f8fa`
(implementation commit `321cdceb02683c5bc47aca6123fabec4a2f57124`). `deskpathguard rederive`
reports 2 rows, 0 author-added, and both rows run with their original Command, Expect and
class. The delivery changed only the Verify section's intro prose, never a row, and the PR
does not carry the protected-table label, so this run is an ordinary record with no
`Verification-Table:` marker.

Both rows are `check:ci`. The host is darwin, where `unshare --net` is not available, so they
ran in the closed container-netns admission bracket (rule 45): the host opened it with
`--module-cache`, the rows ran in a `--network none` container using the release-pinned
verify-runner v1.0.87 image (digest checked against that release's checksums.txt asset), and the
host closed it. `statusgen verifyrun --bracket-digest` on the closed witness reports
`verdict=match span-lines=tree-digest:1,container-image:1,attestation:1`. The witness bytes
follow unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneEligibility$' ./evaluation/... > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneEligibility ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestNextupMilestoneCoverage$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestNextupMilestoneCoverage ' "$milestone_verify_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ bec90193ce91 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:1e8f3139667c5b2860658d062ab306b15095bbf06895404673ef6a551831135d
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6911 run=4d67cdacaae5400f5282a272276ba9937f32d9f2b3af4ab46398bddf7c6bb133 source=bec90193ce916b2d5b11433f25970867a1b7ef95 brief=docs/streams/milestone-support/brief-07-evaluate-milestone-readiness-through-the-canonical-eligibility-api.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=4d67cdacaae5400f5282a272276ba9937f32d9f2b3af4ab46398bddf7c6bb133 source=bec90193ce916b2d5b11433f25970867a1b7ef95 tree=sha256:1e8f3139667c5b2860658d062ab306b15095bbf06895404673ef6a551831135d witness=sha256:1ba03025609b876a66a554cd4c6ca52780c46809063098d0dd5da22f3f43a2de opened=2026-11-18T15:23:50Z closed=2026-11-18T15:25:25Z before=receipt after=check-verifier

Supplemental observations. These are diagnostics with no acceptance credit; they ran against
throwaway source snapshots outside the admitted home, in the same network-off runner image
with `GOPROXY=off` and the bracket's module cache mounted read-only.

| Probe | Command | Observed | Date | Runner |
|---|---|---|---|---|
| S1 mutation map | python3 statusgen/testdata/milestone-support/07-mutate.py, run in a committed snapshot of bec90193ce91 | exit 0; BASELINE PASS TestMilestoneEligibility and TestNextupMilestoneCoverage; CAUGHT all 20 declared mutations (15 on TestMilestoneEligibility, including start-gates-as-feathers, absent-resolved-satisfied, final-gates-inherited, other-holds-overridden, verified-finishes-work, withdrawal-finishes-work, revocation-satisfied, subject-match-omitted and version-match-omitted; 5 on TestNextupMilestoneCoverage: host-gates-as-feathers, host-readiness-not-applied, host-subject-token-dropped, host-revocation-ignored, host-history-dropped) | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S2 fail-first row 1 | row 1 command, verbatim, in a snapshot of merge-base 0665bfb771788b07473c281b8875fc1806dab36d | exit 1; go test printed "no tests to run", so the named-PASS guard fails at base | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S3 fail-first row 2 | row 2 command, verbatim, in the same base snapshot | exit 1; go test printed "no tests to run", so the named-PASS guard fails at base | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |

An earlier S1 invocation stopped after its first baseline and before any mutation: git in the
container could not read its global config, so the script's clean-baseline check reported
"source differs from committed baseline". It was discarded and re-run with `safe.directory`
set through the environment; it credits nothing.

Risk-bearing values (risk: irreversible). Enumeration covered the production diff of PR #6263:
the evaluation package files eligibility.go, identity.go, milestonegraph.go and readiness.go,
plus the host files milestoneacceptance.go, milestonegraph.go and model.go, all under statusgen.

- RISK-VALUE: DERIVED — work-done threshold `w.Status != "done"` @ statusgen/evaluation/readiness.go:199 — spec/milestone-v1.md, delivery-view rule 1: remaining work is any contributing or prerequisite brief not yet `done`, and `implemented` and `verified` stay unfinished. Mutations verified-finishes-work and withdrawal-finishes-work are caught.
- RISK-VALUE: DERIVED — exact-match cardinality `len(matches) != 1` @ statusgen/evaluation/readiness.go:309 — DR-milestone-support-07, Subject and Revocation rules: only one exact current observation for the milestone version and subject can satisfy. Zero matches (missing) and two or more (ambiguous) both fail closed to could-not-check, never to satisfied.
- RISK-VALUE: DERIVED — accepted mapping `a.State == "supported" && !a.AcceptanceOutstanding && len(a.History) > 0` @ statusgen/milestoneacceptance.go:250 — the actor rule (DR-milestone-support-06, kept by /07) needs a recorded human act, so an assessment with zero history can never map to accepted. Revoked or superseded history then overrides the mapping (host-revocation-ignored and host-history-dropped are caught).
- RISK-VALUE: DERIVED — unknown filter literal `"acceptance-outstanding"` @ statusgen/milestoneacceptance.go:245 — it equals the literal the domain owner emits at statusgen/domain/milestone.go:617. Filtering it out maps an outstanding assessment to `outstanding`, which is unsatisfied and still a hold, never satisfied.
- RISK-VALUE: DERIVED — status vocabulary `"todo", "in-progress", "implemented", "verified", "done", "blocked", "withdrawn"` @ statusgen/evaluation/readiness.go:341 — spec/lifecycle-v1.md lists the five ordered statuses plus `blocked` (§2.0) and `withdrawn` (§2.0a). Any other status is held as could-not-check coverage (coverage-gap-cleared is caught).
- RISK-VALUE: DERIVED — `EvaluatorVersion = "statusgen-evaluation/6"` @ statusgen/evaluation/identity.go:13 — identity.go's own contract says the generation must change whenever a verdict can change for the same observations, and readiness changes start-gate verdicts. A wrong value is reversible: edit and redeploy.

The gate is human and the item is irreversible, so this is Evidence only. The verifier changes
no lifecycle status; the human gate closes it.

**VERIFY: PASS** — 2/2 original rows pass in the closed container-netns bracket (bracket-digest verdict=match); all 20 declared mutations caught; both rows fail at the merge-base; Evidence only, status left for the human gate.

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.

## Approved readiness integration — DR-milestone-support-07

The driver accepted this bounded design in the unedited ruling recorded in
[DR-milestone-support-07](../decisions/DR-milestone-support-07.md), naming PR #6119
at `01ed5b1f32c591e541562dbfa5af0faf97643546`. The design discussion below is
retained as its approved source. The ruling authorizes implementation only;
Evidence and independent lifecycle credit remain separate.

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

### Approved bounded design

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

### Acceptance cases for implementation

These cases are covered by the two named Verify entrypoints and the declared mutation
map. Local implementation checks do not supply independent Evidence.

| Existing Verify test | Case and expected discrimination |
|---|---|
| `TestMilestoneEligibility` | Brief 11 belongs to a milestone whose only prerequisite is final-only, and it stays `eligible`. Brief 12 has an explicit `requires-milestones` on an unaccepted milestone and is `held`. Mutation: copying final-only requirements onto contributors holds brief 11, so the test fails. |
| `TestMilestoneEligibility` | All contributors are `done` and every prerequisite promise has qualifying acceptance, but this milestone has no acceptance: the result reports acceptance outstanding, and the start gate stays unsatisfied or could-not-check. Exact current acceptance is the positive control. Revoked, superseded, wrong-version and wrong-subject observations never satisfy. |
| `TestMilestoneEligibility` | All local contributors are `done`, but prerequisite P has remaining briefs: report those briefs as remaining and do not report work complete with acceptance outstanding. A shared not-done brief reached directly and through multiple prerequisites counts once. Mutation: omitting prerequisite traversal or deduplication fails. When P's briefs become `done` but P lacks qualifying acceptance (or its coverage is unknown), completion requirements still are not met. Only qualifying acceptance for every prerequisite permits this milestone's acceptance-outstanding view. |
| `TestMilestoneEligibility` | `implemented`, `verified` and withdrawn contributors count as remaining or blocking. A brief shared by two milestones is counted once per outcome. Mutation: counting `verified` as done, or counting withdrawal as complete, fails. |
| `TestMilestoneEligibility` | A satisfied start gate never lifts a human, design, risk, verification, placeholder or parked hold. Mutation: returning `eligible` as soon as the start gate is satisfied fails. A nil `Milestones` reproduces the legacy fixtures byte for byte. |
| `TestNextupMilestoneCoverage` | Next-up, run through the real host adapter, never lists a brief whose milestone root is unread, whose prerequisite is withdrawn or whose acceptance is missing. The incomplete-coverage flag and blocking heads are surfaced. Required parked work is visible and non-dispatchable. |
| `TestMilestoneEligibility` | An unresolved milestone start edge remains a `Gates` hold. Two separate mutations in `07-mutations.json` must fail through the production evaluator/host flow: moving the edge into `Feathers`, and treating an absent `Resolved` value as satisfied. Neither mutation may yield `eligible` or `eligible-with-notice`. |
| `TestNextupMilestoneCoverage` | With the reader capability unadvertised, as it is on main, every start requirement is `could-not-check`, and Next-up output for non-milestone streams is unchanged. |

The cases run through the actual entrypoints. The compiling mutations are declared in
`statusgen/testdata/milestone-support/07-mutations.json`; after committing, run
`python3 statusgen/testdata/milestone-support/07-mutate.py` from the repository root.
Fail-first outcomes and any unavailable checks accompany the implementation PR.

### Consumer census and transition

| Consumer | Disposition |
|---|---|
| This brief's design appendix and design changelog fragment | fixed-here: authoring material only; no shared runtime value changes. |
| `statusgen/evaluation/eligibility.go`, a sibling readiness file in the same owner, and `docs/contracts.md` row `S-eligibility` | fixed-here: generation 6, optional readiness input and per-brief projection. |
| `statusgen/milestonegraph.go` host adapter and `statusgen/milestoneacceptance.go` mapping | fixed-here: maps /06 assessment to plain observations without re-deriving acceptance. |
| `statusgen/nextup.go`, `statusgen/drivefrontier.go`, the `--eligibility` output and its could-not-check notices | fixed-here: consume the canonical per-brief result and retain existing exclusions; no new CLI envelope. |
| `statusgen/flow.go` depends reading | out-of-scope: brief `depends` semantics are unchanged. |
| `spec/milestone-v1.md` start-gate and implementation sections, `docs/library-first.md` eligibility row | fixed-here: canonical implementation boundary updated after the recorded ruling. |
| Reader capability advertisement, dispatch admission and operational rollout | follow-up /10 and /14; nothing is advertised here. |
| Board, historian/export and selected-outcome trace | follow-up /11, /12 and /13; consume the canonical result. |
| The desk loop engine's own eligibility verdict type | out-of-scope: it has a different meaning and is not this evaluator. |

### Accepted decision block (historical proposal)

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


### Implementation architecture pickup

**Continue**, manual bootstrap against `0665bfb771788b07473c281b8875fc1806dab36d`,
brief revision 1 and the approved appendix at design head
`01ed5b1f32c591e541562dbfa5af0faf97643546`. The authorized dispatcher supplied
pickup observations of remote main, the merged/approved design, the unedited human
ruling and an empty bounded open-PR search for this item. These are dispatcher
observations, not worker forge reads; title-search coverage is not a file-level
claim census. Forge publication and its final currentness check remain with that
parent because the worker cannot access the required stored-account credential.

The source assessment read the graph, eligibility and receipt owners, host mapping,
`docs/go-engineering.md`, `docs/library-first.md`, S-eligibility and
S-verification-domain, the stream scope, milestone contract and affected brief/stream
contract sections at that base. Public workref/readercaps and /05–/06 exist; the
old Context baseline is retained as history. The approved direction is compatible:
no new package, dependency, process, selector, capability or semantic owner is needed.
The graph now retains contribution observations for the readiness owner; the host's
old graph-to-eligibility interpretation is replaced by `EvaluateReadiness`.

Additional reader census: `statusgen/domain/assess.go` remains on the nil-input
legacy projection (fixed-here compatibility); `statusgen/instance.go` and both
eligibility CLI projections keep the canonical per-brief map (fixed-here).
`statusgen/brieffile.go` calls the existing post-scan adapter (fixed-here).
`statusgen/flow.go` keeps its historical depends-only meaning (out-of-scope).
The /05 blocked-state mutation is repointed to the canonical readiness owner
(fixed-here); its test and Verify obligation are retained.
Board/deskboard, historian/export and selected-outcome trace remain /11–/13;
`Stream.MilestoneReadiness` carries the typed result for those consumers without
adding a second authored register or changing their wire formats. /10 and /14 own
admission and rollout. No overlapping active ownership was reassigned.

The original Task and human gate remain. The worker does not fill Evidence or
certify verified/done; bounded local results and sandbox limitations belong in the
PR workpad and publication handoff.
