<!-- GENERATED from plugins/assay/references/milestone-authoring.md by deskinstall --harness cursor. Do not edit. -->
# Capture and decompose delivery milestones

<!-- assay:harnesslint non-matrix-reference — harness-neutral milestone authoring procedure shared by author-spec and author-brief, not a per-harness capability binding; it defines no harness capability or per-skill degradation cells -->

Use this procedure for stream scoping, staged delivery and milestone changes. A useful
milestone is an observable supported outcome, not a wave label or a renamed task group.
A small standalone fix need not acquire an artificial milestone; preserve adequate
existing scope and explain absent outcome stages where material.

## Controlling contracts and support check

Read the relevant source revision of the
[milestone contract](https://github.com/medici-finance/assay/blob/main/spec/milestone-v1.md),
[stream integration contract](https://github.com/medici-finance/assay/blob/main/spec/stream-v1.md)
and [brief integration](https://github.com/medici-finance/assay/blob/main/spec/brief-v1.md#37-proposed-delivery-milestone-integration-not-shipped).
In an Assay checkout, use those `spec/` files from the selected source revision; elsewhere
retrieve the corresponding versioned canonical source. Record revisions and unavailable
sources. The `blob/main` links are discovery pointers; use the selected checkout or
pinned release revision for the controlling contract, not an unrecorded moving main. This procedure is not a second schema: the contracts own fields and semantics.

Before operational declarations, establish both the contract's approval state and the
actual pinned readers' advertised support for milestone records, stream discovery,
brief start requirements and affected dispatch/report consumers. Record what was checked;
a successful legacy lint or tolerance of unknown YAML keys does not prove support.
Do not guess capability commands, introduce a `brief-v4` declaration, or claim an
unshipped parser/gate exists. Follow milestone §10 for coordinated reader adoption.

While approval or support is absent/unknown, still capture the requested planning work:
put clearly labelled milestone proposals and mappings in the scoping document's authored
prose, outside operational discovery paths and generated regions. Use the intended IDs
and name missing information, but do not present an incomplete proposal as a conforming
record. Leave milestone-dependent operational YAML and eligibility unchanged; continue normal
supported brief authoring and preserve approved work. If that source
is unavailable, report the coverage gap and continue independent scoping; do not invent
a contract from memory. No automatic bulk backfill of existing streams.

## Colon-only references, versions and compatibility

Read milestone §2 and brief §3.8 before choosing identifiers. Write full record IDs as
`<cell>:<repo>:<stream>:m-<NN>` for milestones and
`<cell>:<repo>:<stream>:b-<NN>` for briefs once the generator and all consumers support
them. References allow the corresponding repo/stream and stream-only colon elisions
against the declaration's actual context. Milestones require `@<contract-version>`;
brief references optionally select `@<brief-version>`, their existing authored Task/Verify
revision. A brief definition keeps its version in its separate `version` field.

An exact brief pin cannot fall back to another revision. An unversioned reference binds
the brief at the selected source snapshot; record observed version/commit in the handoff.
Do not create separate logical tasks/claims for two spellings or version selectors.
Unknown or incompatible revision requirements stay explicit. Prefixes are distinct types.

Unprefixed colon and slash forms, plus earlier prefixed slash draft references, remain
compatibility inputs under the contracts' rules. Normalize old/new aliases to the same
UUID/work identity; reject collisions and deduplicate views. Preserve the existing internal
numeric fullId representation at old library boundaries. Those APIs do not understand
prefixes or version selectors until adapted. New authoring emits colon-only prefixed forms;
legacy forms are deprecated for new writes after negotiated support, not invalid history.
Before support, use the new forms in scoping proposals and retain supported legacy IDs
for executable briefs. Do not invent generator flags or write new IDs under brief-v2.
No automatic filename changes, bulk migration, UUID reminting or Evidence rewrite.

## Outcomes delivered with simulated dependencies

Use this section when an outcome relies on a stub, mock, fake, simulator, recorded
response, manual stand-in or temporarily absent service. A small standalone change
without such a boundary needs no extra profile or document.

Identify what is real, substituted or absent at every material consumer/provider
boundary. State the interface/contract version, behaviour and failure cases represented,
limitations, selected environment and the evidence each substitute can and cannot supply.
A stub returns canned answers; a mock can check interactions; a stateful fake implements
selected behaviour. None establishes fidelity merely by its label or by passing its own
fixture tests. A delivery profile describes this supported composition; it is distinct
from a model/harness execution profile and confers no execution or acceptance authority.

A simulated outcome can be useful and independently accepted when simulation is part
of its explicit promise. A criterion requiring a real component must exercise that
component. Never replace the component whose integration is the outcome under test with
a test double and retain the same claim. Check actual APIs, module/version compatibility
and owner delivery before choosing the critical-path head. Missing real orchestration
remains a prerequisite, not an invitation to build a demo-only controller.

Use the spec's existing supported-scope, acceptance, coverage and change sections to
record repeatable setup/reset/teardown, scenario and artifact/configuration identity,
resource/credential isolation, and who verifies the integrated journey. Require relevant
failure cases and observable evidence through the actual consumer seam; a frontend
fixture or mocked request cannot stand in for that path. Missing scope metadata and
unavailable observations stay unqualified, not implicitly real or passed.

For replacement by a real service, name its owner, consumer-contract checks and the
fresh integrated evidence required. Shared cases against fake and real providers help
detect drift, but prove only their declared behaviour; they do not establish model
quality, deployment or untested durability. Keep prior simulated acceptance bound to its
original subject. A changed promise follows the existing milestone version rule; a new
artifact/environment subject needs its own applicable evidence even when the promise
version is unchanged. No automatic promotion from simulated to real acceptance.

Carry these obligations into brief Task/Verify and consumer routing, with profile
lifecycle and joined qualification work assigned explicitly. Reuse current launch,
test/conformance and acceptance owners; do not create a mock platform, profile schema,
parallel approval store or extra service unless the product needs one. If runtime
profile/milestone readers are unavailable, use authored prose and ordinary supported
briefs; this procedure introduces no required schema keys, new gate or operational opt-in.

For example, a real operator client → real supervisor → fake runner can establish a
scripted launch/observe/cancel journey. It cannot establish real model execution or
cluster deployment. Replacing that runner preserves the old fake evidence and requires
new provider-contract and end-to-end evidence for the real subject.

## Author-spec: establish the promises

1. Identify independently useful outcomes and owning streams from the requested scope.
   For each, capture full `<cell>:<repo>:<stream>:m-<NN>` identity, title, accountable
   owner and promise
   version. Brief references use `:b-<NN>[@<version>]`; apply the support boundary above. Preserve existing IDs and accepted history;
   numbering implies no order. Independent maintenance outcomes can remain independent.
2. Define the user/operator capability, supported environment/workload/interfaces,
   relevant quality limits, exclusions and manual steps. Capture existing REQ/decision/spec
   provenance at its material revision. Do not invent quantitative limits or approval authority.
3. Give acceptance criteria stable IDs with observable pass/fail conditions, method,
   evidence and verifier responsibility, including relevant failure/recovery conditions.
   Identify the existing authorized acceptance mechanism and exact subject/profile being
   accepted. Expose unresolved criteria or authority as decisions with owners.
4. Identify prerequisite capability promises and why they are needed, pinning exact versions
   as `<stream>:m-<NN>@<version>` within the declaring root, or
   `<repo>:<stream>:m-<NN>@<version>` within the declaring cell or
   `<cell>:<repo>:<stream>:m-<NN>@<version>` for explicit qualification. Use graph alias
   registry repository aliases; these are milestone refs, not brief `qualifiedRef` inputs. Separate final
   acceptance needs from known implementation start needs; leave brief scheduling to
   author-brief. Do not insert a dependency on an entire stream because its details are unknown.
5. Hand off proposed criterion-to-deliverable coverage, existing evidence and known briefs
   where established. Mark unmapped work explicitly; an unmapped contribution is not an
   empty list asserting an already-delivered capability. Do not invent brief IDs or split
   work merely to populate the record. Once support exists, use the canonical milestone
   record and link it from the source rather than maintain a duplicate definition.

For revisions, classify a changed promise separately from internal decomposition.
Changing supported scope, criteria or prerequisite promise versions requires a new
contract version with explicit consumer adoption. A brief split preserving the promise
can keep its version. Preserve historical acceptance and completed Evidence. Carry
affected unfinished work and consumers through the existing context-handoff mechanism.

## Author-brief: map the work and test the dependencies

1. Read the source promises and existing briefs/boards across relevant configured roots.
   Record actual source revisions, archived work and unavailable coverage. Map criteria
   to existing deliverables and Verify rows before creating work; expose uncovered
   criteria and any joined outcome check. Do not re-author completed work or fabricate evidence.
2. Maintain `contributes` only in the canonical milestone record once operational support
   exists, writing full brief refs `<cell>:<repo>:<stream>:b-<NN>[@<version>]`; until then maintain the proposed mapping in the scoping document. A brief can
   contribute to multiple milestones; reverse membership is derived. Do not add a
   `milestones:` field or hand-maintained inverse list to every brief.
3. Keep brief-to-brief start needs in `depends`/`unblocks`. Only a brief that actually
   needs another capability to start gets a milestone start requirement, using the supported
   negotiated brief schema when available. Record its capability use/profile in Context.
   Never put milestone IDs in the existing brief dependency or requirement-ID grammar.
4. Keep milestone-to-milestone final acceptance needs on the milestone. Do not inherit
   them onto all contributors. Existing brief waves continue to derive from brief dependencies;
   summarize their span per milestone rather than assign milestone waves. Validate cycles
   over completion and start relationships, including a brief requiring its own milestone.
5. Check each outcome's actual blocking head and readiness, including parked-stream holds,
   external evidence and unknown roots. A proposed dependency graph is a manual assessment
   until implemented readers exist; do not report it as machine-validated. Deduplicate shared
   work in projections; unknown and blocked remain distinct. A required parked stream is a
   decision to surface, not permission to activate it.
6. Review criterion coverage, contribution changes and affected consumers together. Splitting
   a prerequisite's internal work should not rewrite its consumers' promise refs. A changed
   promise must not silently retarget them. Brief Task/Verify versioning, risk conservation,
   source approval and existing human gates continue to apply.
7. Hand off the source/contract revisions, milestone IDs/versions, criterion coverage,
   remaining work, acceptance outstanding, blockers and unknowns. All contributors `done`
   does not establish milestone acceptance or current availability. Link the existing
   acceptance/evidence mechanism; authoring does not execute it or grant approval.

For example, `build:m-03@1` includes briefs `build:b-11` and `build:b-12` and requires
`stream-y:m-01@1` and `stream-z:m-02@1` for acceptance. If only brief 12 needs Y to start,
record that need on brief 12; brief 11 can proceed. Z can remain a final acceptance need.
Use fully qualified refs when crossing roots. Check that splitting Y's internal brief
preserves this behavior, and that making brief 11 require its own milestone is rejected.
In the record's `contributes`, write full refs such as `example-cell:tools:build:b-11`
and `example-cell:tools:build:b-12@2`; the latter pins authored brief revision 2.
Do not emit an unprefixed internal fullId as a new authoring reference. These are
reasoning checks, not claims that current tooling enforces them.
