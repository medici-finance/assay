# Extensible JSON contracts and compatibility transitions

**Requested future-architecture amendment, 21 October 2026.** The driver selected
versioned extensions for perf-v4 and requested the same direction for other Assay
JSON contracts. This document defines that destination and its migration obligations.
It is not a claim that these codecs are shipped, that old evidence has changed, or
that the earlier whole-target ratification covered this revision.

## One compatibility policy, separate semantic owners

Durable records, inter-component APIs and machine-readable CLI outputs should use a
stable, explicitly versioned core plus independently versioned, named extensions.
A format marker alone is insufficient: consumers need rules for optional additions,
required capabilities, meaning changes and historical reads. Keep each domain's
codec and invariants with its existing semantic owner; do not build another universal
policy engine or allow arbitrary maps to become an alternative command channel.

The stable core owns identity, provenance, units, coverage and any authorization or
signature boundary. Additive observations belong in extensions. Each extension has
an explicit name, integer version, typed payload and owner. A semantic change to a
core field requires a new core major; a semantic change within an extension requires
an extension version. Neither a binary release number nor a PostgreSQL row revision
substitutes for a wire-contract version.

Optional unsupported extensions can be skipped only if independent core meaning
survives. Report the missing capability as unavailable; never synthesize zero, PASS,
a complete snapshot, a grant or an effect acknowledgement. A request marks required
capabilities explicitly. Unsupported required versions refuse before any effect.
Unknown policy, acceptance requirements, operation verbs and authorization scopes
remain refusals. An extension cannot override a core field or remove a requirement.

Producers emit only registered, typed, validated extensions. Reader tolerance does
not authorize a generic data exporter. Every new extension receives privacy review
and explicit data/consent treatment. Bound total bytes, depth, extension count and
identifiers per contract before materializing payloads. Reject duplicate keys,
ambiguous aliases, malformed supported payloads and trailing documents. Refusal
messages must not echo unknown payloads or untrusted extension names.

Signed, hashed and persisted records define whether extension bytes participate in
canonicalization and identity. Verify the complete signed representation before
projecting a supported subset. Never drop unknown bytes and recompute a historical
signature/digest or accept a stripped record as the original. Lossless relays and
stores preserve the original bounded bytes; observational reducers may discard
unsupported payloads after checking the envelope. Those are different interfaces.

## Observed source inventory

Inspected public Assay at `af897c3c112bd426cbbc794839dca6d41df911d6`.
The census covers Go source and committed schemas in this repository, including
shared modules outside tools/statusgen. It groups contracts, not individual struct
fields or executable counts. Searches for schema/version markers and strict decoder
calls are discovery aids, not proof of every JSON consumer. Fixtures, third-party
APIs, private consumers, shell-generated JSON and deployed installations require
additional discovery when their owning migration is admitted. No fleet-wide
completeness or deployment claim follows from this inventory.

Paths below are source witnesses at that revision. Each row assigns a target and
implementation owner; none declares the migration already complete.

| Contract family and source witnesses | Observed compatibility | Future target / owning migration |
|---|---|---|
| Performance: `perfrec/schema.go`, `perfrec/v3.go`, `tools/desk/internal/deskkit/perfread.go`, `tools/desk/cmd/deskperf/report.go` | Whole-record perf-v1/v2/v3 dispatch; strict known fields; v3 attribution is not independently versioned | **First delivery:** perf-v4 core and typed extensions; desktools-v2/67 readers then /68 producers. Retain historical readers and distinct v1/v2 units. |
| Aggregate metrics: `statusgen/telemetry.go`, `tools/desk/cmd/opmetrics/emit.go`, `tools/metrics-harvest/cost.go` | telemetry-v1 fixed aggregate struct; opmetrics/2 explicitly tolerates additive fields; cost-per-row-input-v1 whole-input marker | Quality/metrics owners migrate optional metric groups to versioned extensions on the next contract revision. Keep enumerated aggregate vocabulary, opt-in and unknown-versus-zero. Preserve opmetrics additive compatibility; do not claim it currently rejects every addition. |
| Qualification artifacts: `qualgen/store.go`, `qualgen/artifacts.go`, `qualgen/m2source.go` | qualgen-mine-v1 / attribution-v1 and strict fix-source parsing; version is attached to the artifact as a whole | Qualification owner versions optional analyses separately from source/horizon/coverage and defect identity. Retain tombstone amendments, immutable source bindings and old artifact readers. |
| Verdicts, scan deltas and review observations: `statusgen/transcribeverdict.go`, `statusgen/transcribescan.go`, `tools/desk/internal/deskkit/verdict.go`, `tools/desk/internal/reviewflow/observation_json.go` | Explicit single accepted schema; strict observation decode; acceptance consumers depend on exact meaning | Evidence/review owners introduce an extensible evidence envelope with **required** evaluator/claim versions. Unknown diagnostic attachments may be skipped; unknown required acceptance semantics hold the claim. Never rewrite old attestations or treat extension availability as acceptance. |
| Worker/verifier attestations and dispatch records: `tools/desk/internal/deskkit/{workerattestation,verifierattestation,dispatchrecord}.go` | Typed records and strict decode or exact-schema validation | Existing admission/evidence owners retain a closed authority core and signed/digested byte rules; add versioned optional diagnostics and explicit required-proof capabilities. Preserve independence and subject/model/source binding. |
| Fact bundles and policy requests: `deskcore/facts/facts.go`, `deskcore/domain/domain.go`, `deskcore/policy/policy.go` | desk-facts-v1 and request schema 1; strict exact field/schema checks | deskcore owners migrate optional observations separately from required policy/evaluator inputs. Unknown required facts or grant semantics refuse; extension negotiation cannot widen a grant or complete missing coverage. |
| Domain, product and planning records: `statusgen/domain/records.go`, `statusgen/productintent/decode.go`, `statusgen/projectobligations.go`, `schemas/{verification-domain,product-intent,project-obligations}-v1.json` | Named versioned schemas and closed/strict decoders in several paths | Existing definition/planning owners add versioned metadata facets while requirement identity, approval revision, obligation coverage and lifecycle remain mandatory core semantics. Amend unfinished plans through their normal revision process. |
| Context handoffs and amendment manifests: `statusgen/contexthandoff/{records,decode}.go`, `schemas/context-handoff-v2.json`, `schemas/amendment-manifest-v1.json` | Multiple whole-format handoff versions already exist; rule-specific validation | Context/planning owner separates optional discovery facets from required coverage, approval and reconciliation obligations. An unsupported required discovery scope is incomplete, never an empty consumer set. |
| Forge observations and CLI projections: `forgeread/{envelope,heads}.go`, `tools/desk/cmd/deskread/main.go`, `tools/desk/cmd/deskboard/{delta,prfacts}.go`, `tools/desk/cmd/desksupervise/status.go`, `statusgen/briefinfo.go` | Forge issues envelope already ignores additive members but rejects unknown schema; CLI shapes vary and some expose no dedicated output-format version | Forge/projection owners publish explicit output-contract versions and optional observation extensions, preserving requested-repository coverage, partial results, freshness and source identity. Do not confuse a brief's schema field with its enclosing CLI output version. |
| Cell/workspace operations and configuration: `workspace/contract.go`, `tools/desk/internal/cellop/{plan,result}.go`, `tools/desk/internal/celllaunch/spec.go`, `tools/desk/internal/cellcontainer/config.go` | Explicit workspace/cell schema versions with closed operations and strict config paths | Cell/workspace owners negotiate optional backend/diagnostic extensions while required isolation, scope, generation and operation capabilities fail closed. Next-version readers precede writers; allocation never grants execution authority. |
| Communications: `comms/contract/envelope.go`, `tools/desk/internal/comms/{envelope,laneacl}.go` | Versioned authenticated message envelope; closed verb/class and ACL vocabulary | Communications owner defines versioned content/transport extensions with signature coverage and required capabilities. No unknown verb/class/ACL may be ignored. Coordinate with the communications future-state amendment rather than creating a second broker contract. |
| Runner requests: `loopadmin/runner/request.go`, `schemas/loop-admin-runner-v1.json` | **Existing precedent:** optional `ext` map and `require` capabilities, strict core, recursive credential rejection | Runner owner retains this mechanism and adds independently versioned payload contracts as needed; do not replace it with an incompatible generic envelope. Required extension versions must be negotiated before launch. |
| Local state, caches and installation manifests: `tools/desk/internal/{cellcache/cache,cellscratch/store,cursorbundle/archive,cursorbundle/managed,gitmaintenance/maintenance}.go` | Several independently versioned files and strict readers; cache rebuild is not the same as evidence migration | Respective resource/install owners define explicit compatibility per persisted file. Version optional metadata; retain closed authoritative configuration. Rebuild a disposable cache only from its verified source; do not treat unknown durable state as disposable. |
| External protocols: `tools/desk/internal/acp/types.go`, provider JSON adapters | Upstream protocol/version ownership, not Assay-owned wire schemas | Retain upstream negotiation and translate at adapters into the owned extensible contracts. Do not fork an external protocol or pass unknown provider fields into authority decisions. |

The first four rows demonstrate the perf-style upgrade problem directly. The other
rows show where the same future policy must be applied with stronger capability,
authority or persistence rules. This is not a count of formats already lacking all
compatibility: opmetrics, forge reads and runner requests provide partial precedents.

## Migration and reconciliation

1. Deliver the perf-v4 reader/producer transition below first. It supplies a real
   compatibility matrix and establishes the extension pattern without demanding a
   simultaneous platform rewrite.
2. At each family's next contract-changing delivery, its existing owner enumerates
   actual producers, readers, storage/relay paths, CLI/report consumers and unfinished
   briefs at pinned revisions. Publish the core/extension schemas, capability rules,
   byte/signature treatment, fixtures and consumer-routing dispositions together.
3. Ship readers/adapters before selecting new writers. Record the supported
   old/new combinations and the exact ordinary release/checksum pins needed by each
   consumer. Missing external-consumer coverage blocks that consumer's cutover.
4. Reconcile unfinished plans with dated amendments and new Verify obligations;
   preserve their source accounts and completed evidence. No blanket amendment to
   every live brief is performed by this architecture document. For performance,
   statusgen/26 and desktools-v2/62 retain snapshot, phase and workload obligations
   but future format delivery targets v4; /63–65's completed source/evidence remains
   about v3. Performance-collection must later add a v4 reader
   explicitly before consuming it.
5. Retire an old writer only after all selected consumers are proven compatible.
   Retain historical readers for the declared retention period. Roll back writer
   selection if needed; never downgrade or relabel stored records in place.

This policy becomes a design/authoring obligation for the target's R1 records,
R2 coordination, R3/R4 execution and effects, R6 observation APIs and R7 projections.
It does not add unadmitted implementation work to their current owners. Separate
wire compatibility from SQL storage migrations, authority rotation and deployment.

## Qualification required before cutover

Use a cross-product matrix: old writer/new reader, new writer/older reader that
supports the new core, supported/unsupported optional extension versions, missing
required capabilities, malformed supported extensions and unknown core majors.
An old pre-envelope binary may refuse v4: reader-first rollout is still necessary.
"Backward compatible" means retained historical reads, not magic support in old code.

Fail-first controls must prove refusal of duplicate keys, oversized/deep payloads,
unknown required capabilities, core overrides and stripped signature/coverage fields.
Exercise the real producer → transport/store → reader → reducer/evaluator → report
path and an external consumer. Prove no effect on a required-capability refusal and
no payload disclosure in diagnostics. Unknown optional diagnostics must leave usable
core observations and explicit unavailable coverage; authoritative results must still
require every applicable supported proof. Snapshot fixtures for historical formats
must remain unchanged. Independent verification records results after merge.
