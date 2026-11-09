---
brief: assay:assay:milestone-support:06
title: "Bind milestone acceptance to existing evidence and decisions"
why: "All contributors being done is not evidence that the joined outcome was accepted for the relevant environment and version."
wave: 4
depends: ["milestone-support/05"]
unblocks: ["milestone-support/07"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: c883588a-7bd6-4f32-9a2e-30588e5f6183
authored: 2026-10-22 by assay-worker-app[bot]
exec-tier: strong
exec-tier-why: Cross-module identity, version, evidence and compatibility behavior must agree at independent boundaries.
domain: complicated
outcome: none
gate-why: "Changes which human decision and verification receipts satisfy milestone acceptance; incorrect actor, subject or revision binding could accept unapproved scope."
decision-trigger: spec
design: DR-milestone-support-06
decision-issue: 5705
ruling: "https://github.com/medici-finance/assay/issues/5705#issuecomment-6715755830"
sources:
  - docs/streams/milestone-support/README.md
  - spec/milestone-v1.md
  - spec/stream-v1.md
  - spec/brief-v1.md
  - docs/library-first.md
  - "freshness-checked 2026-10-22 @ a631d47ea71b3270ab33ba08af47d3dde8c9e7d5"
consumers:
  - "statusgen/milestoneacceptance.go: fixed-here milestone-support/06 (receipt adapter implementation)"
  - "statusgen/milestoneacceptance_test.go: fixed-here milestone-support/06 (receipt adapter implementation)"
  - "statusgen/decisionruling.go: fixed-here milestone-support/06 (receipt adapter implementation)"
  - "statusgen/domain/: fixed-here milestone-support/06 (receipt adapter implementation)"
  - "statusgen/testdata/milestone-support/06/: fixed-here milestone-support/06 (receipt adapter implementation)"
  - "spec/milestone-v1.md: fixed-here milestone-support/06 (receipt adapter implementation)"
  - "docs/streams/milestone-support/brief-07-evaluate-milestone-readiness-through-the-canonical-eligibility-api.md: follow-up milestone-support/07 (consumes this delivery; exact runtime readers enumerated at pickup)"
---

# Brief 06 — Bind milestone acceptance to existing evidence and decisions

## Context

files: `statusgen/milestoneacceptance.go`, `statusgen/milestoneacceptance_test.go`, `statusgen/decisionruling.go`, `statusgen/domain/`, `statusgen/testdata/milestone-support/06/`, `spec/milestone-v1.md`, `changelog/milestone-support-06.md`, `statusgen/testdata/milestone-support/06-mutations.json` — The receipt adapter and its tests are delivered in the draft implementation; independent verification and Evidence remain pending.

facts:
- Source baseline: merged main `a631d47ea71b3270ab33ba08af47d3dde8c9e7d5`, inspected 22 October 2026.
- Existing reference owners: `statusgen/contexthandoff/ids.go`, `statusgen/briefv2.go` and `tools/desk/internal/deskkit/trailer.go` currently implement different legacy grammars; do not erase those boundary contracts.
- Module boundary: `statusgen` and `tools/desk` are separate Go modules. Public `statusgen/workref` and `statusgen/readercaps` are planned; neither exists at this baseline. `tools/desk/internal` is not importable by statusgen.
- Support baseline: brief schemas v1-v3 are advertised; milestone, prefixed-reference and negotiated reader-capability support is not advertised. The installed plugin or a permissive YAML parser is not evidence of support.
- Availability: `statusgen/evaluation`, `statusgen/domain`, `statusgen/contexthandoff`, `statusgen/streamview` and `loopadmin` exist; the root `workflow` store/controller module does not. Preserve its future integration obligations without inventing a working store.

layering: statusgen acceptance adapters owns this slice. Use in-process public Go library calls for shared parsing, reference resolution and capability assessment. No new binary, daemon or subprocess-RPC seam; host adapters own filesystem, transport, authentication and effects. Existing online forge reads remain behind their current authorized deskread boundary. Task and Verify exercise the actual consumer seam.

design-fit:
  owner: "statusgen acceptance adapters"
  contract: S-verification-domain
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

1. Choose and implement the §10 receipt adapter over existing authorized decision, verification and Evidence mechanisms; document its precise record pointers/subject binding. Do not introduce a parallel approval store, signer role or unverified self-acceptance stamp.
2. Bind milestone identity/contract version, exact source commit/path, contributor revisions, prerequisite receipts, criterion results, environment/profile and authorized decision. Reuse existing verifier/actor separation and authority checks; owner metadata grants none. Brief @revision and milestone @contract-version remain distinct.
3. Derive historical acceptance separately from present availability. Missing, inaccessible, incomplete, stale, wrong-subject or revoked receipts cannot satisfy current requirements. Preserve old accepted history and revocation decisions; new versions/subjects do not inherit acceptance.

## Interface and completion contract

Consume only delivered prerequisite APIs at reviewed revisions. Shared core functions accept explicit values and return typed results; they do not fetch, claim, approve, launch or mutate the environment. Unknown coverage and unsupported required semantics remain explicit. Update affected canonical docs and the per-delivery changelog. Enumerate newly discovered readers with fixed-here/follow-up/out-of-scope dispositions before changing shared values. No new stateful store, task queue or acceptance authority is introduced.

## Verify

The following named tests, fixtures and executable mutation maps are FUTURE DELIVERABLES. They are not authoring-time results. Each test must exercise the production entrypoint/API named in Task and fail under its stated wrong-but-plausible mutation. A named PASS guard prevents an absent `-run` target from appearing green. Record concrete mutation commands in this brief's declared mutation manifest; no test may prove completion by file existence alone.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneAcceptSubject$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneAcceptSubject ' "$milestone_verify_out")` | exit 0 and named PASS; done work without receipt stays acceptance outstanding; wrong profile/version/actor or missing criterion cannot accept; substituting latest evidence fails. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && milestone_verify_out=$(mktemp "${TMPDIR:-/tmp}/milestone-verify.XXXXXX") && trap 'rm -f "$milestone_verify_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMilestoneRevocation$' . > "$milestone_verify_out" && grep -q -- '--- PASS: TestMilestoneRevocation ' "$milestone_verify_out")` | exit 0 and named PASS; historical acceptance remains visible while revoked/stale/unavailable evidence prevents present readiness; rewriting history or accepting a revoked receipt fails. |

## Pre-mortem and dispatch check

The wrong-but-plausible delivery is the mutation named in each Verify row. Each must turn that production-path test red and the correct implementation green; do not merely inspect imports or fixture shapes. The facts, public-module visibility, derived waves, risk answers, sizing and consumers were inspected while authoring. API adequacy, completeness of the reader census and human acceptance remain review obligations. Implementation pickup rechecks them.

## Evidence

<!-- Empty by design. Independent verifier records actual commit, environment, command/exit, named test and mutation result, coverage/unknowns and evidence location. -->

## Review

Gate: human, derived from the declared risks. Independent review checks compatibility and consumer-flow evidence. No reviewer/worker stamp, release permission or acceptance receipt is created by this authoring change.


## Receipt adapter design and recorded ruling

**Design approved; draft implementation delivered, independent verification pending.** The driver accepted
`DR-milestone-support-06` and the bounded proposal at
`ffaaf82ff0ab3e09e779ca7fa34ef00ac91804ea` in the
[unedited human ruling](https://github.com/medici-finance/assay/issues/5705#issuecomment-6715755830).
[DR-milestone-support-06](../decisions/DR-milestone-support-06.md) records the
approval, including protected-default-branch source checks and authorized
revocation/supersession with unauthorized comments quarantined. No implementation,
Task/Verify change, Evidence or lifecycle credit is claimed. The proposal below
records the reviewed design; its pending-ruling statements are superseded only by
this exact approval. Independent review and implementation checks remain required.

### Architecture pickup and source coverage

Manual bootstrap at source commit `5142af8c66f4671d7cb3c575d5b65d5eadd6275b`:
brief `assay:assay:milestone-support:06`, authored revision 1. The dispatch packet
records the same commit, and the local worktree head matched it before editing.
The following are inspected source contracts, rather than new approvals:

- [Milestone identity, receipt and history contract](../../../spec/milestone-v1.md),
  especially §§2, 6 and 10 and the combined graph implementation boundary.
- [Brief compatibility](../../../spec/brief-v1.md#compatibility-and-canonical-writes)
  (the section titled “Compatibility and canonical writes”) and §4.5 Evidence;
  [stream ownership](../../../spec/stream-v1.md); [design approval](../../../spec/lifecycle-v1.md#44-design-approval-gate).
- [Decision records and ruling corroboration](../../../spec/registers-v1.md#7-decisions-register),
  §§7.4–7.5; [verification domain](../../../spec/verification-domain-v1.md);
  [evidence provenance](../../../spec/evidence-provenance-v1.md), especially duty separation.
- [Semantic owners](../../contracts.md), rows S-verification-domain,
  S-evidence-coverage, S-eligibility and S-decision-acceptance;
  [library direction](../../library-first.md) and [engineering catalogue](../../go-engineering.md).
  The large contract/index documents were inspected in relevant sections; this is
  affected-context coverage, not a whole-repository architecture audit.

Observed implementation: `statusgen/workref` and `statusgen/readercaps` now exist;
`statusgen/evaluation/milestonegraph.go` exposes `BuildMilestoneGraph`, and
`statusgen/milestonegraph.go` maps source snapshots to it. The graph deliberately
never satisfies a milestone start gate from completion alone. The reader advertisement
in `statusgen/briefschemaversions.go` lists syntax/preflight only, with no milestone
acceptance capability. The board records /05 as verified; that is dependency status,
not evidence of this brief's acceptance behavior.

`statusgen/domain/records.go`, `assess.go` and `provenance.go` expose `Candidate`,
`BindAcceptance`, `Assess`, `Assessment.CheckCurrent`, `Corroborate` and `EvaluateDuty`.
A domain acceptance binding is a definition binding, not a human acceptance.
`WorkIdentity` is brief-only; it must not receive an `m-` reference or a fabricated
workflow instance. `EvaluateDuty` requires trusted selected provenance, and no
production selector is installed by that package. A selected claim's shape or digest
alone cannot establish its authenticity.

`statusgen/decisionruling.go:resolveRulingOn` checks same-repository issue/comment
binding, mapped human author, bot exclusion, unedited text and record identity.
Its successful result intentionally does not distinguish approval from rejection.
It is package-main code, not an importable public SDK. `tools/desk/internal` is also
unavailable to statusgen. The current `workflow/store` exists (unlike the brief's
older authoring baseline); its existence grants no acceptance-store integration and
it remains outside this delivery.

**Disposition: continue authoring; hold implementation.** Existing public libraries
cover identity, candidate binding, coverage and duty comparisons. The aggregate
milestone receipt and decision-content adapter still require the ruling below.
No dependency, binary, service or new credential holder is proposed. Before
implementation, refresh the current source and claim/PR overlap and obtain the ruling
on [the existing decision issue](https://github.com/medici-finance/assay/issues/5705).

### Approved bounded design — historical proposal

Use existing repository Evidence artifacts and the existing human decision-issue
mechanism. Add a pure aggregate adapter in the statusgen module, with host mapping
in the planned `statusgen/milestoneacceptance.go` (planned). Proposed aggregate types belong
to the S-verification-domain owner; they are separate from brief `WorkIdentity` and
from its existing wire schema. Names and a wire-format allocation remain proposed
until the ruling; no operational declaration or support advertisement is added here.

A receipt is a content-addressed evidence manifest plus an exact human comment
pointer, not a new approval database. The manifest is a repository-tracked Evidence
artifact selected explicitly by the existing evidence producer, linked from the
milestone's Coverage evidence. Its path is an explicit repository-relative locator;
there is no directory scan for “latest acceptance.” Existing brief Evidence sections
and execution witnesses remain the verification records; a manifest references them
without rewriting their results or granting the producer acceptance authority.

| Bound value | Proposed exact pointer/comparison |
|---|---|
| Promise | Canonical typed milestone ID and positive contract-version, resolved by `workref` using the declaring root's registry; retain original spelling for diagnostics. |
| Contract source | Owning repository identity, full source commit, repository-relative milestone path and content digest. Contract-version and source commit are separate values. |
| Assessed subject | Explicit list of repository/artifact candidates using `domain.Candidate` where applicable, artifact digests, environment/configuration manifest identity, interface and workload/profile identity. No branch, ambient HEAD or default environment substitutes for a missing value. |
| Contributors | Canonical brief identity/UUID, authored revision, owning source commit/path, and exact completed Evidence/Verify-row/witness pointers. Resolve optional brief selectors at that snapshot; never substitute a newer revision. |
| Prerequisites | Exact prerequisite milestone version, receipt manifest digest and decision comment pointer, together with the consuming environment/interface/profile compatibility required by the promise. Mere historical acceptance in another profile is insufficient. |
| Criteria | Exactly the criterion IDs in the bound contract, each with explicit result, observation time, verifier/provenance reference and immutable evidence pointer (repository, commit, path, row/anchor and digest). Missing, duplicate, extra or failed results cannot be treated as a complete pass. |
| Authority | Exact existing policy/decision source commit/path/digest identifying the outcome's acceptance authority and required duty separation; plus a same-repository issue number and comment ID. Owner/team metadata supplies no authority. |
| Currency | Supplied observation time, selected evaluator/policy identity, existing context/applicability bounds where required, and complete current decision/revocation observations. No implicit freshness duration or live probe. |

The contract and authority policy source commits must be reachable from the
declaring root's protected default branch. A producer-selected unmerged policy or
contract cannot establish authority or relax duty separation. Unavailable protected
branch/reachability observations remain unknown. Test both an unmerged authority
policy and an unmerged contract while bypassing the outer admission check.

The manifest's canonical bytes exclude the later decision pointer, avoiding a digest
cycle. The human's unedited issue comment explicitly identifies the milestone,
contract-version, source commit/path, manifest digest and disposition (`accept`,
`reject`, `hold` or `revoke`). The normalized receipt pairs that manifest with the
comment's repository/issue/comment identity and observed content digest. A revision
of either requires a new artifact/comment; it cannot edit the accepted history.
This is a proposed payload over an existing mechanism, not a minted signature or a
claim that issue comments themselves are permanently immutable.

Reuse the ruling resolver's authentication/binding checks through its existing
host seam; any necessary extraction preserves all legacy DR and brief behavior.
Milestone issue binding is explicit and distinct from a DR design-decision marker.
Do not widen `resolveRulingOn` and then presume that any passing DR comment accepts a
milestone. The new content adapter must match the explicit disposition and every
subject field against the selected manifest. Free prose, issue closure, PR approval,
`decided-by`, completed contributors and a corroboration PASS are each insufficient.
An unknown payload version remains could-not-check.

Host adapters establish trusted observations and authority, using existing admitted
forge reads only; the pure aggregate receives normalized explicit values and returns
history plus current applicability, reasons and coverage. It performs no I/O, token
lookup, claim, lifecycle advance, acceptance write or effect admission. Existing
coverage and current-subject checks remain independent of duty/authentication checks.
Use `domain.Assess`/`EvaluateDuty` for actual brief-bound evidence only when their
required selected records exist. Missing trusted lineage is unknown, not an assertion
of independence, and no milestone-as-brief coercion is allowed. Aggregate decisions
must retain the outcome's existing human/risk and verifier separation policy.

### Historical acceptance and present availability

Keep the original accepted receipt addressable. A later authorized revocation points
to that exact receipt and subject, through a new unedited comment in the same decision
mechanism. Supersession also names the replaced receipt; ordering by timestamp alone
must not silently choose among conflicting decisions. Missing decision pages, missing
revocation coverage or an unresolved conflicting successor makes current applicability
unknown. A known revocation prevents current satisfaction while preserving history.
A revocation for another receipt cannot erase this one.

Revocation and supersession use the same bound authority policy as acceptance.
Authenticate and authorize each candidate decision before it can count as a
revocation, successor or conflict. Quarantine unauthorized comments; they cannot
force an accepted receipt to unknown merely by being present. Test an unauthorized
revocation and forged conflicting successor against a valid accepted receipt.

Expose historical acceptance independently from present `supported`, `blocked` and
`unknown` applicability. Incomplete work or an observed failed criterion is a known
blocker; inaccessible evidence or unsupported semantics is unknown. Retain both lists
when both occur. All work done with no qualifying receipt remains acceptance outstanding.
New contract versions, changed source/subject or a different environment/profile require
a fresh applicable assessment and decision; no acceptance is inherited. Evidence that
was accepted but is now unavailable is historical acceptance with current could-not-check.
Retirement/withdrawal decisions are consulted without deleting identity or history.

/06 returns these typed observations only. /07 owns readiness integration; /10 owns
current dispatch/pickup admission. Neither this proposal nor a future pure “supported”
result can authorize execution, merge, release or deployment.

### Acceptance cases for the post-ruling implementation

These are proposed case details for the existing two Verify rows, not new executable
checks or reported results. The original commands and Evidence remain unchanged.

| Existing future test | Case and expected discrimination |
|---|---|
| `TestMilestoneAcceptSubject` | All contributors done, no receipt: acceptance outstanding. An exact authorized accept with complete independent evidence is the positive control; a reject/hold comment passing actor corroboration never accepts. |
| `TestMilestoneAcceptSubject` | Independently change promise version, source commit/path, candidate, brief revision, prerequisite receipt, criterion result, environment or profile: no current acceptance; unavailable observations remain unknown. Plant “latest evidence” substitution at a second contributor and require the production adapter to reject it. |
| `TestMilestoneAcceptSubject` | Wrong human, bot, edited/deleted/unrelated comment, unauthorized policy, missing trusted selector, shared accountable lineage or forged selected provenance: no acceptance. Bypass an outer check to show the inner subject/provenance check still refuses. |
| `TestMilestoneRevocation` | Preserve accepted history after an exact authorized revocation while denying present satisfaction. Wrong-target revocation cannot revoke another receipt; a missing revocation page cannot prove absence. |
| `TestMilestoneRevocation` | Preserve history after evidence becomes stale/unavailable, a subject/profile changes, or a new contract version appears. Unknown current applicability and known blockers remain distinct; history is never rewritten to show current acceptance. |
| Both tests | Unrequired legacy streams retain behavior; unsupported required receipt semantics remain unknown/refused at the consumer boundary. No test fixture counts as a human acceptance or a live observation. |

After the ruling, implement these through the actual adapter and record compiling
mutations in the already declared `06-mutations.json`. Show fail-first results before
opening the implementation PR. No such runtime test or mutation is claimed by this
proposal; the named tests are still absent at this source revision.

### Consumer census and transition

| Consumer | Disposition |
|---|---|
| This brief's decision/design appendix and changelog | fixed-here: authoring material only; no shared runtime value changes. |
| `statusgen/milestoneacceptance.go` (planned), its tests/fixtures and `statusgen/domain` aggregate types | follow-up /06 after ruling; preserve existing brief wire/identity contracts. |
| `statusgen/decisionruling.go` and existing DR/brief ruling consumers | follow-up /06 only for shared authentication extraction needed by the adapter; preserve all checks and explicit no-content-verdict semantics. Alias/workflow subject expansion remains /09. |
| `statusgen/milestones.go`, `evaluation.BuildMilestoneGraph`, reader capability advertisement | out-of-scope for this authoring change; delivered loader/graph remain held and acceptance remains unadvertised. Any newly required seam is inventoried before implementation. |
| `spec/milestone-v1.md` receipt boundary | follow-up /06: update canonical semantics only after the ruling; this appendix is a proposal under the existing contract. |
| `evaluation.EvaluateEligibility`, statusgen Next-up/eligibility and milestone graph gate mapping | follow-up /07; consume one typed receipt assessment without re-deriving acceptance. |
| Desk dispatch/pickup and trailer/claim readers | follow-up /10 and /08 respectively; no active worker scope change here. |
| Board/deskboard, historian/export, selected-outcome trace | follow-up /11, /12 and /13; preserve history and unknown coverage, consume the canonical result. |
| `workflow/store`, verifier scheduling, existing Evidence publication/role custody | out-of-scope; reuse their records/ports where delivered, without activating a new store or changing admission. |
| `docs/streams/delivery-profiles/spec.md` and its /07 real-runner qualification | follow-up under that stream: profile qualification evidence must bind the exact profile and cannot accept itself; no fake-to-real acceptance inheritance. |

### Decision block

**Owner:** the maintainer authorized by the existing /06 decision issue, linked above. This design gate approves how /06 will be implemented;
it does not accept any milestone outcome.

1. **Accept the bounded adapter proposal (recommended):** authorize the explicit
   manifest/comment binding, aggregate types under S-verification-domain, distinct
   accept/reject/hold/revoke content check, unchanged actor and duty boundaries,
   immutable history/current-applicability separation, and no effect admission.
2. **Request corrections:** name the pointer, subject, authority, revocation or
   consumer rule to change; implementation remains held for a revised proposal.
3. **Hold:** leave /06 at its current lifecycle state and specify what evidence or
   ownership resolution is needed before another proposal.

Exact ruling needed: an unedited human comment on the existing decision issue naming
`milestone-support/06`, the proposal PR and its head, and `DR-milestone-support-06`, selecting one option and stating
any corrections. The reserved record id is `DR-milestone-support-06`; naming it in the human comment
satisfies the existing record-naming condition without changing the resolver. If
accepted, transcribe the ruling into that decision-v1 DR record with
alternatives, consequences and its corroboratable ruling link, and cite it via the
brief's `design:` before governed implementation. Do not create an approved DR,
fill `decided-by`, record Evidence or advance a lifecycle token from this proposal.


### Implementation pickup — 9 November 2026

Manual bootstrap at `aae4fbf71bca3419f1ac7ece8e83e8e5a651983e`, brief revision 1.
The live unedited ruling and approved DR above were refreshed at pickup; main matched
this source revision and no overlapping milestone implementation PR was observed.
Inspected current `workref`, `readercaps`, `domain` record/assessment/provenance APIs,
`MilestoneRecord` loader, graph adapter and `resolveRulingOn`; module/import boundaries,
`docs/library-first.md`, `docs/go-engineering.md` and S-verification-domain,
S-evidence-coverage, S-eligibility and S-decision-acceptance remain controlling.
Disposition: **continue** within the approved bounded adapter. No dependency, binary,
store, signer, installed pin, capability advertisement or effect authority is added.

Updated implementation census:

| Reader or producer | Disposition |
|---|---|
| `domain.AssessMilestone`, `MilestoneManifest` and normalized receipt observations | fixed-here: aggregate meaning under S-verification-domain, distinct from brief wire records. |
| `readMilestoneAcceptance`, existing `MilestoneRecord` loader, `resolveRulingOn`, domain Assess/EvaluateDuty/Corroborate | fixed-here: direct production consumer flow; existing authentication and verification owners retained. |
| Canonical milestone receipt documentation, named tests, fixture and `06-mutations.json` | fixed-here; no independent verification credit is recorded. |
| Eligibility, Next-up and combined graph start gates | follow-up /07: consume the typed receipt assessment; existing holds remain. |
| PR trailers/claims; workflow/human-ruling aliases; dispatch/pickup | follow-up /08, /09, /10 respectively. |
| Board/deskboard, historian/export and selected-outcome trace | follow-up /11, /12, /13; preserve history and unknown coverage. |
| Reader qualification, release/adoption | follow-up /14; no runtime support advertisement here. |
| Trusted source/provenance composition, workflow store and evidence publication | out-of-scope for activation; hosts must establish selected records, protected default-branch provenance and acceptor lineage. Missing trusted observations remain unknown. |

Coverage is the affected receipt boundary, not a whole-repository architecture audit.
Tests use frozen source and authenticated forge fixtures; no cluster/live-state check,
new production selector, outcome acceptance, lifecycle update or Evidence is claimed.


### Review repair pickup — 9 November 2026

Continued the same draft implementation after live ownership/review/claim checks at
`67d9cd121`, then merged main `a7e2f320f` into the owned feature branch. That upstream
delta contains only the separately held statusgen/36 proposal and its changelog;
it changes none of the receipt APIs, approved authority, semantic contracts or Go
engineering guidance. Disposition: **continue** under the same DR and consumer census.
Review repairs include the real `readercaps` pure dependency in the hermetic domain
snapshot, restrictive treatment of uncertain authorized successors and overlapping
revocation lineage, pointer-path validation, canonical reason lists and host mutation
controls. Original Task, Verify definitions, Evidence and lifecycle remain unchanged.
