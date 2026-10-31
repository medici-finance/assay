# Delivery milestone contract

**Version:** v1.0-draft
**Status:** draft — proposed contract; no runtime, schema or reader support is shipped by this document.

## 1. Purpose and boundary

A delivery milestone is a useful, demonstrable outcome with a bounded support promise.
It lets another stream depend on that promise without depending on its internal task list.
It is not a wave, a drive, a new brief lifecycle, or a deployment authorization.
The word “milestone” in `brief-flow-event-v1` continues to mean an event boundary;
that event vocabulary is unchanged.

Normative requirements below apply only to a future implementation advertising this
contract. Existing streams and `brief-v1` through `brief-v3` remain valid unchanged.
Do not place the proposed declarations in operational streams before the capability
negotiation and consumers in §10 exist. A draft MUST NOT veto approved work.

### Controlling sources and discovery

This revision is based on Assay main `866a169aecab730e1acdaad7cbe57f9c41af8553`:
`spec/brief-v1.md`, `spec/lifecycle-v1.md`, `spec/registers-v1.md`,
`spec/context-handoff-v1.md`, `statusgen/parse.go`, `statusgen/model.go`, and the
`author-spec`, `author-brief` and `trace-path-to-live` skills. The current stream parser
has no milestone declaration; brief dependency grammar names briefs, not milestones.
The initiating stream review found that waves often group implementation order rather
than independently useful outcomes, including broad maintenance streams. That review
is a proposal, not acceptance evidence or authority to reclassify those streams.
No runtime quality or performance measurements were made for this documentation change.

## 2. Identity, location and revisions

Each `(milestone ID, contract-version)` MUST have exactly one canonical record under
`docs/streams/<stream>/milestones/` in its owning repository. Version-one records use
`m-<NN>-<slug>.md`; later versions MAY use a `-v2` suffix before `.md`. Identity and
version come from the record's keys, never by parsing a slug or filename suffix.
The owning stream is accountable for the promise; contributing briefs MAY live elsewhere.

The record's `milestone` key is the full colon-only ID
`<cell>:<repo>:<stream>:m-<NN>`, for example `example-cell:tools:build:m-03`.
The containing cell/repository/stream MUST agree with that key. `NN` is a positive
decimal number padded to at least two digits; `m-3` is rejected. The `m-` prefix
identifies a milestone, not its execution wave. IDs MUST NOT be reused after retirement.

A milestone dependency pins an exact positive contract version. Its accepted forms are:

| Written milestone reference | Resolution |
|---|---|
| `<cell>:<repo>:<stream>:m-<NN>@<version>` | Explicit owning cell/repository/stream. |
| `<repo>:<stream>:m-<NN>@<version>` | Declaring cell; explicit repository/stream. |
| `<stream>:m-<NN>@<version>` | Declaring cell/repository; explicit stream. |

Names and repo-alias lookup follow
[`context-handoff-v1.md` §2](./context-handoff-v1.md#2-qualified-brief-ids).
`<repo>` is an alias from the existing **graph alias registry**, never an owner/name pair.
Expansion uses the declaration's owning cell/root; unknown or ambiguous identities
are unknown, never a search across roots by slug. New writes use colon-only forms.
Earlier draft slash references (`<stream>/m-<NN>@<version>` or
`<cell>:<repo>:<stream>/m-<NN>@<version>`) remain compatibility inputs to the future
negotiated adapter and MUST normalize to the same identity/version. They do not create
another milestone or imply current parser support. Record-key migration likewise
preserves the original owning root and identity.

`contributes` writes full colon-only brief references
`<cell>:<repo>:<stream>:b-<NN>[@<version>]`, with the optional exact authored-brief
revision selector defined in brief §3.8. That is distinct from this milestone's
contract version. Unversioned contributions bind the brief revision in the assessed
source snapshot; an unavailable explicitly requested revision is unknown, never
substituted with another revision. Existing unprefixed fullIds
and other legacy aliases remain readable through that compatibility adapter, not new writes.

Both typed adapters reuse the existing graph alias registry. Neither the `b-` nor
`m-` grammar is already accepted by the brief-only `qualifiedRef` or verification-domain
parser: those contracts and their internal fullIds remain unchanged. Resolve the new
reference through the negotiated adapter before invoking existing APIs. A rename MUST
preserve identity or retain an unambiguous alias through the existing identity mechanism;
duplicate `(owning cell, repository, ID, version)` records are an error.

Three different versions MUST remain distinct:

- `schema: milestone-v1` identifies the record format.
- `contract-version: <positive integer>` identifies the externally relied-on promise.
  Dependency references MUST pin an exact version, e.g.
  `example-cell:tools:build:m-03@1`; no implicit latest or version ranges in v1.
- A source revision (repository commit and path) identifies the exact plan and evidence
  snapshot. Edits to contributing briefs without changing the promise do not require a
  new contract version. Git history preserves those edits; a second manual edit counter
  is unnecessary.

The public promise includes outcome, supported scope, exclusions, acceptance criteria
and prerequisite capability versions. A change to any of those MUST create a new
contract version and retain the old version as an addressable record. The filename is only a locator; an apparent `-v2` in a slug does not establish version 2.
A typo or a relocated evidence link with unchanged meaning MAY preserve the version.
Acceptance records MUST still bind the exact source revision they assessed.

## 3. Required information

The proposed frontmatter has the following closed key set. Missing, unknown or wrongly
typed keys MUST be diagnosed, rather than silently ignored.

| Key | Type | Meaning |
|---|---|---|
| `schema` | string | Exactly `milestone-v1`. |
| `milestone` | string | Full colon-only ID `<cell>:<repo>:<stream>:m-<NN>` from §2; owning root must agree. |
| `contract-version` | integer | Positive promise version. |
| `title` | string | A plain-language useful outcome, not only “m-03” or a wave label. |
| `owner` | string | Accountable role or team; confers no approval authority. |
| `sources` | array[string] | Nonempty canonical spec/decision references; pin material revisions in the body. Reuse existing REQ IDs where applicable. |
| `contributes` | array[string] | Full colon-only `:b-<NN>[@<version>]` references from §2, unique by logical brief identity within this record; required completion work. Empty is permitted for an existing capability, with justification and evidence. |
| `requires-milestones` | array[string] | Exact milestone-version references in one of the three forms defined in §2, needed to accept this outcome. Empty explicitly means none. |

The body MUST contain:

| Section | Required content |
|---|---|
| Outcome | Who can do what once accepted; business or operational value. |
| Supported scope | Environment, user/workload profile, interfaces and relevant quality limits. State exclusions and remaining manual steps. Unknown limits are explicit unresolved decisions. |
| Acceptance | Stable criterion IDs; observable pass/fail conditions, method, expected evidence and responsible verifier. Include relevant recovery or failure cases. Link existing Verify rows rather than copying their results. |
| Coverage | Map each criterion to contributing brief deliverables and/or existing evidence. Expose uncovered criteria. Explain empty contributions; do not manufacture work. |
| Prerequisites | For each required milestone, explain the capability used and whether it is only needed for final acceptance. Actual start requirements belong on the affected briefs. |
| Acceptance authority | Existing policy/decision reference identifying who may accept this exact outcome and required separation of duties. An unresolved authority prevents acceptance; a team owner or report author cannot grant themselves that authority. |
| Change and open decisions | Material source revisions, version rationale, unresolved decisions and their owners. Reuse existing decision/finding records, not a new queue. |

A target date, estimate, priority and wave summary MAY be displayed as projections from
existing planning sources. They are not required milestone fields or separate scheduling
authority. “Later” work MUST be identified as excluded, not quietly required for acceptance.

## 4. The three dependency relationships

All authored lists mean AND: every listed requirement is required. Alternatives require
separate explicitly scoped milestones; v1 has no implicit OR or optional dependency.

| Relationship | Authored once in | Effect |
|---|---|---|
| Brief needs brief | Existing `depends` on the consuming brief | Existing implementation dependency semantics. |
| Brief needs milestone | Proposed `requires-milestones` on the consuming brief (§10) | The capability must have a qualifying acceptance before this brief can start. |
| Milestone includes briefs and needs milestones | `contributes` and `requires-milestones` on the milestone | All contributors must be `done` and all prerequisite promises accepted before this outcome may be accepted. |

Contributing to a milestone MUST NOT make a brief inherit all that milestone's
prerequisites. Adding a milestone dependency MUST NOT silently add a start gate to every
contributor. A genuinely necessary implementation dependency MUST be declared on the
specific brief. Reverse membership and reverse milestone dependencies MUST be derived;
there is no authored `milestones:` list on every brief or milestone `unblocks` mirror.
A brief MAY contribute to several milestones. Counts deduplicate by canonical brief identity.

### Example: “Build changes are less brittle”

In a hypothetical record owned by cell `example-cell`, repository alias `tools`,
`milestone: example-cell:tools:build:m-03` and `contract-version: 1` identify
`example-cell:tools:build:m-03@1`.
Its `contributes` list is `["example-cell:tools:build:b-11", "example-cell:tools:build:b-12@2"]`.
The latter pins brief 12's authored revision 2; it does not refer to milestone contract 2.
Its `requires-milestones` list may be `["stream-y:m-01@1", "stream-z:m-02@1"]` in
that same root. A cross-root dependency uses, for example,
`example-cell:other-tools:stream-y:m-01@1`, with `other-tools` registered there.
Legacy brief aliases `build/11`, `build:11` and `example-cell:tools:build:11`
continue to identify the same logical brief as `example-cell:tools:build:b-11`.
If only brief 12 needs Y's capability to start, only brief 12 declares that requirement.
Brief 11 can proceed independently. Z may be needed only for joined acceptance.
Splitting Y's internal brief into three updates Y's contribution list and coverage;
consumers still refer to Y's same promise version. Changing Y's supported promise
requires a new version and explicit consumer adoption.

## 5. Graphs, waves and readiness

A consumer MUST distinguish the execution graph from the acceptance graph:

- Execution contains existing brief dependencies and explicit brief-to-milestone start
  requirements. Existing brief `wave` remains derived from its brief `depends` only.
  Milestone gates are separately shown; a wave number alone cannot mean ready.
- Acceptance contains milestone-to-contributing-brief completion edges and
  milestone-to-prerequisite-milestone edges, as well as each brief's execution needs.
  Cycle detection MUST run over this combined graph before claiming a viable path.

In these graphs an edge points from a consumer to what it requires. A brief requiring
its own containing milestone forms a cycle and MUST be rejected, including indirect
cross-stream cycles. A milestone MUST NOT be assigned a hand-authored wave. Reports
MAY derive a span of its contributing briefs' waves, clearly labelled a summary rather
than an outcome sequence. A milestone can cross waves; a wave can serve several milestones.

Unresolved identities, unreadable roots, inaccessible acceptance records and unsupported
versions MUST produce `unknown` coverage/readiness, never “no dependency” or “done”.
A known unsatisfied requirement is `blocked`; missing information is not proof of a block
or proof of readiness. Reports MUST retain known blockers alongside unknown coverage.
Existing parked/paused dispatch policy remains in force: a required parked-stream item
is a visible obstacle for a decision, not automatic permission to dispatch it.

## 6. Completion, acceptance and present availability

The delivery view MUST distinguish:

1. Work remaining: contributing/prerequisite briefs not yet `done`.
2. Work complete, acceptance outstanding: completion requirements met but no qualifying
   acceptance for this promise and subject.
3. Accepted for a named subject: an authorized acceptance with evidence exists.

These are derived views, not manually advanced brief statuses. `implemented` and `verified`
remain unfinished; withdrawn work MUST NOT count as completed. A withdrawn contributor
requires an explicit scope-preserving replacement or a new promise version.
An accepted prerequisite qualifies only for the consuming milestone's stated environment,
interface and workload profile. Historical acceptance in staging does not prove production
availability. The acceptance criteria MUST say which current checks or freshness bounds
are required when readiness depends on live state; absent observations remain unknown.

Acceptance MUST reference an existing authorized decision/evidence mechanism, not create
an automatic approval or new desk role. Its immutable receipt MUST identify:

- Milestone ID/version and exact contract source commit/path.
- Assessed subject: relevant repository/artifact revisions, deployment/environment and
  configuration identity, without secret material.
- Source revisions and identities of completed contributors, required milestone acceptance
  receipts, criterion results and retrievable evidence.
- Observation time, verifier identity and the authorized acceptance decision reference.

An incomplete or failed criterion cannot be accepted by a report. Evidence unavailable
at read time MUST be reported as could-not-check. A later incident, withdrawal or changed
subject MUST NOT erase an earlier acceptance; record a superseding/revocation decision
through the same existing mechanism and show current availability separately. A revoked
acceptance MUST NOT satisfy new readiness checks. New versions are not accepted by inheritance.
Retirement and identity withdrawal are recorded through existing authorized decision
records bound to the milestone ID/version, not an undeclared frontmatter status key.
Consumers MUST consult those decisions; an unavailable retirement decision is unknown,
never permission to erase identity or infer satisfaction.

## 7. Change propagation

| Change | Required effect |
|---|---|
| Split/add/replace implementation work, same promise | Update canonical contributions and criterion coverage; recheck dependencies and cycles. Reverse views update automatically; consumers do not edit their references. |
| Add work to an already accepted version | Preserve its receipt. Report new plan work separately; do not silently invalidate past acceptance or extend what its evidence proved. Assess the changed subject afresh if relied on. |
| Change scope, criteria or prerequisite promise versions | Create a new contract version. Enumerate affected consumers; adopt through an explicit reviewed change. Never silently redirect old refs. |
| Remove/withdraw a required brief | Show completion unmet until replacement or scope decision; preserve withdrawal and completed evidence history. |
| Retire a milestone | Preserve addressable versions and acceptances; enumerate live dependants. Do not treat deletion as satisfaction. |

Acceptance of a changed plan MUST bind its new source revision. Existing brief version,
Task/Verify amendment, risk conservation, design approval and human decision requirements
continue to apply; milestone edits provide no exemption. Use the existing context-handoff
amendment mechanism to identify affected unfinished briefs and consumers.

## 8. Stream and report views

A stream README SHOULD show each milestone's title, support scope, work remaining,
acceptance state, blockers and evidence links. Membership, completion and reverse views
MUST be generated from canonical records, not a second hand-maintained table.
A report MUST let the reader select an outcome and see its direct contributors, additional
prerequisites, deduplicated remaining work and uncovered criteria. Broad maintenance
streams MAY have independent outcome tracks; no default cumulative m-01→m-02→m-03 chain.

The prerequisite graph MUST display only unfinished briefs (`status != done`) and
unsatisfied milestone requirements, with completed prerequisite evidence accessible by link.
Omitting done nodes MUST NOT omit a still-required execution/acceptance gate. Known
required work from parked streams remains visible with its hold explained; unrelated
parked work is excluded. Paths with unknown coverage MUST NOT be called definitive critical paths.

At the top, show six clearly named brief-count lines: Total work, All work remaining,
Not started, Being implemented, Awaiting verification, Awaiting final approval.
Total work counts all non-withdrawn briefs in the selected expanded plan, including done.
Map the last five to non-done/non-withdrawn, todo, in-progress, implemented and verified
respectively. Unknown lifecycle values make the affected reading incomplete.
Blocked briefs count in remaining and retain a separate visible explanation; withdrawn
briefs are disclosed separately and are not unfinished executable work. The four status
lines therefore need not sum to remaining. Counts are brief counts, not effort or percentages
of a usable outcome. Acceptance outstanding stays visible even when remaining work is zero.

History covers the last seven calendar days individually plus the preceding three
seven-day periods at their period ends. Label dates and observation timezone. Use the
membership/dependency graph and statuses recorded at each historical point, show scope
changes beside total-work growth, and disclose missing samples as unknown. Never project
today's membership backwards or replace missing readings with zero. Distinguish historical
scope from any optional fixed-scope comparison. Deduplicate shared prerequisites at each
reading; always disclose archive/root coverage and source revisions.

## 9. Acceptance cases for the future implementation

A conforming milestone implementation MUST demonstrate at least:

- Brief 11 starts while its containing milestone's final-only prerequisite is outstanding;
  brief 12 with an explicit capability requirement remains blocked.
- Internal decomposition changes without editing downstream capability references;
  an incompatible promise change does not satisfy a consumer pinned to the old version.
- Direct and cross-root indirect cycles, including own-milestone start dependencies, refuse.
- Unknown roots/refs/versions, inaccessible evidence and revoked acceptances cannot appear ready.
- All contributors `done` with missing joined acceptance shows acceptance outstanding.
- An accepted staging outcome does not satisfy a production-only criterion.
- Withdrawal does not finish a requirement; shared briefs are counted once; historical
  scope changes and missing samples remain visible.
- Existing non-opted-in streams keep their lifecycle, dependency and dispatch behavior.

## 10. Adoption and follow-on work

This is a specification draft, not a rollout. First implement explicit capability
advertisement/refusal for milestone records, stream opt-in and brief start requirements.
Old readers currently tolerate unknown keys in several places: inserting new YAML keys
alone is unsafe because they may silently ignore a required gate. A future negotiated
brief schema (provisionally `brief-v4`, subject to allocation at implementation) MUST
be used for `requires-milestones`; supported brief schemas remain v1–v3 today.
No JSON schema, release support entry or live declaration is added by this draft.

The implementation handoff MUST enumerate parser/schema/lint, graph resolution, eligibility,
Next-up/dispatch, board/deskboard, historian/export and trace report consumers. Unsupported
required semantics MUST stop the affected operation with a diagnostic. Legacy operations
remain unchanged where no milestone capability is required. Wire the existing graph alias
registry for repository identity rather than introducing another cross-root registry;
the new milestone-reference syntax still needs its own negotiated typed resolver.

Author-spec would author outcome, scope, criteria and authority; author-brief would map
contributors and start requirements and validate cycles; trace-path-to-live would select
outcomes and explain evidence and remaining work. Stream scaffolding and generated README
views must agree with these contracts. Authoring skills may capture clearly labelled
scoping proposals while support is absent. Operational authoring and consuming behavior
must follow coordinated implementation after the contract is ruled; this draft alone
does not establish operational support.

Pilot one stream with multiple useful outcomes and one cross-stream capability dependency
before any broader adoption. Kubernetes is an illustrative candidate: a runnable test cell,
operationally qualified service, and migration/consumer retirement are different promises.
Those names do not assert existing brief membership, evidence or approved milestone order.
The pilot must demonstrate §9, reader refusal and readable history. No compulsory backfill
of existing streams, no new task queue and no additional universal approval desk.

Open implementation decisions: exact capability-negotiation envelope and release/schema
allocation; acceptance-receipt adapter to existing decision/evidence records; eligible pilot
and its outcome-specific acceptance authority. These need concrete decisions before rollout;
they do not prevent reviewing this proposed semantic contract.

## Reader negotiation implementation boundary

The envelope/stream opt-in allocation from §10 is now defined by
[reader-capabilities-v1](reader-capabilities-v1.md). The shared pure assessment and
named legacy consumer preflights do not implement milestone loading, assessment,
dispatch or acceptance. Those capabilities remain unadvertised. Operational milestone
declarations and adoption remain held until coordinated readers are qualified.
