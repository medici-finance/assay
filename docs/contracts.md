# Assay tooling contracts — schema-first, three-part

Assay's load-bearing tooling seams are held by a pinned binary and prose. The pin proves
*which* build runs; on its own it proves nothing about whether that build and its consumer
still agree on the *contract* between them. When the two drift — a validator grows a field
the spec never documents, a consumer wires a key the tool no longer reads — nothing fails
until someone notices by hand, later.

This document is the pattern that turns "the maintainer discovers drift eventually" into
"CI fails the day it drifts." It is the Brokk/Anvil OpenAPI idea — a published,
machine-readable contract with a conformance check on both sides — applied to Assay's own
seams. It is written once over the seam that already has it (the brief-v1 frontmatter
contract) and parameterized so the second seam (a consumer repo's install contract) can
follow the same shape.

## The three parts

Every Assay tooling contract has the same three parts. A contract missing any one of them
is not a contract — it is a convention with a version number.

1. **A versioned, machine-readable artifact.** The contract is a file, checked into the
   source repo, that a machine can read: a JSON Schema, a pin-file grammar, a manifest.
   It carries its own version in a stable identifier so a consumer can tell which contract
   it is reading, and so a deliberate contract change is a visible version bump rather than
   a silent shape change.

2. **A source-side coverage gate.** A test in the source repo's own CI derives the
   contract's surface *from the reference implementation's own tables* and asserts the
   committed artifact covers them exactly — plus a negative case proving the gate reddens
   on a doctored artifact. This is the layer that makes the artifact and the code unable to
   drift apart without the source repo's CI failing at author time. Without it the artifact
   is hand-maintained documentation that rots at the first change nobody remembers to
   mirror.

3. **A consumer-side conformance run.** A read-only, three-state check the consumer runs in
   its *own* CI — shipped in the pinned binary and picked up on the consumer's next routine
   pin bump, so no per-consumer wiring is needed after the first. It fails on the consumer's
   side of the seam (a file the consumer owns no longer satisfies the contract) with a
   different signal, in a different repo, than the source-side gate. This is the layer that
   turns drift into a red check in the place drift actually happens.

The three parts are independent by construction — a different check, in a different
component, tripping on a different signal — which is what makes them defense-in-depth rather
than one check written three times. The source-side gate catches the maintainer editing the
validator without the artifact; the consumer-side run catches a consumer editing its own
files away from the artifact; the versioned identifier catches a deliberate migration and
reports it *as* a migration, not as a field error.

Each part reports three-state, never two: `checked-clean` / `checked-failed` /
`could-not-check`, per [`three-state-instrument-rule.md`](three-state-instrument-rule.md). A
check that could not run has cleared nothing, and must never render the same result as one
that ran and found nothing.

## Instance #1 — the brief-v1 frontmatter contract (built)

The seam is `statusgen` ↔ the `schema: brief-v1` frontmatter every brief carries. The
reference implementation validates a brief's frontmatter shape — required keys, field types,
closed value sets — but until this contract shipped, that shape lived only in the validator's
Go tables and a prose spec that had already drifted eleven releases behind it.

| Part | Realization |
|------|-------------|
| Versioned machine-readable artifact | [`../schemas/brief-v1.json`](../schemas/brief-v1.json), [`../schemas/brief-v2.json`](../schemas/brief-v2.json), [`../schemas/brief-v3.json`](../schemas/brief-v3.json) and [`../schemas/brief-v4.json`](../schemas/brief-v4.json) — one JSON Schema (draft 2020-12) **per brief-schema version**, each describing that version's frontmatter exactly as the reference validator enforces it: required keys, the four `risk` booleans, and the closed value sets (`effort`, `gate`, `exec-tier`, `domain`, `blocked-by`, `measures`, …), plus, for brief-v2, the hierarchical `brief:` id, the `version:` counter and the reserved `id` / `supersedes` / `gates` / `feathers` / `verify` keys, and, for brief-v3, the withdrawn-status keys `withdrawn-date` / `withdrawn-reason` / `superseded-by` and optional `withdrawal-context`, and, for the negotiated brief-v4 (milestone-support/04), the colon-only prefixed `<cell>:<repo>:<stream>:b-<NN>` definition id, required `version` and `id`, `requires-milestones` and the `reader-requirements` envelope (`TestBriefV4SchemaCoverage` holds its patterns to the parser). brief-v4 is read by `statusgen` only; `statusgen newbrief --schema brief-v4` writes it after `readercaps.Assess` reports every supplied reader advertisement supported (`TestNewbriefCaps`). Each `$id` carries the schema version, so the artifact is self-identifying. All are **descriptive of the validator's current behaviour first** — the on-file brief corpus is the de-facto contract, and a schema the corpus fails is a wrong schema, not a wrong corpus. A brief-schema version the validator recognizes but no artifact describes is the failure mode this seam is most exposed to: the consumer-side run below then reports could-not-check for every brief in a migrated tree, so `TestEveryRecognizedBriefSchemaHasAContract` holds the two sets equal. |
| Source-side coverage gate | `statusgen/`'s `TestBriefV1SchemaCoverage` and `TestBriefV2SchemaCoverage` (each with its `_RejectsDrift` negative case, plus `TestValidatorCoversSchemaKeywords`, `TestEmbeddedSchemaMatchesCommitted` and `TestEveryRecognizedBriefSchemaHasAContract`) derive the required-key and value sets from the validator's own tables and assert the committed schemas cover them, so validator and schemas cannot drift without the assay repo's CI going red. |
| Consumer-side conformance run | `statusgen conform --root <repo>` validates every brief file under a consumer's tree against the embedded contract that file's own `schema:` marker declares — so a tree mid-migration, holding brief-v1 and brief-v2 files side by side, has each validated against the version it actually declares. It reports three-state (`checked-clean` / `checked-failed` naming file and field / `could-not-check`, fail-closed), exit 1 on any failure. It is distinct from `--lint`: lint enforces methodology rules, `conform` enforces the schema contract. `statusgen conform --emit-schema [--schema brief-v2]` prints an embedded schema to stdout, so every artifact is reproducible from any pinned build. A file whose `schema:` marker is newer than every contract the binary embeds is reported as a **version mismatch**, not a field error — the deliberate-migration signal on a pin bump, not a false failure across every consumer at once. |

The consumer-side run rides the existing distribution lane, not a new one: it ships in a
`statusgen` release, a consumer picks it up when it bumps its [`.assay-versions`](distribution.md)
pin, and it is invoked from the shared lint action after the existing `--lint` step. Wiring
the run into that action is gated on the pinned binary actually carrying the `conform`
subcommand — a call to a verb a not-yet-bumped pinned binary lacks would redden every
consumer at once, so the wiring is sequenced *after* the release that ships `conform` and the
pin bump that adopts it, never before.

## Instance #2 — the consumer install contract (pattern ready, not yet built)

The second seam is `desk-tools` ↔ a consumer repo: what a repo must wire to run the desk
tooling. Today that contract is prose plus example configs plus a pinned binary, with one
partial machine-readable piece already in place — `deskpins`, which validates the
`.assay-versions` pin-file grammar against the contract in
[`distribution.md`](distribution.md) and is the proto-pattern this document generalizes
(a published contract plus a read-only, three-state validator). The remaining surfaces of
the seam have no artifact and no consumer-side conformance run.

Instantiating the pattern here means naming each surface and giving it the same three parts:

| Surface of the seam | Versioned artifact | Source-side coverage gate | Consumer-side conformance run |
|---------------------|--------------------|---------------------------|-------------------------------|
| The pin file (`.assay-versions`) | the pin-file grammar (already the `deskpins` contract in `distribution.md`) | a test deriving the grammar the validator accepts from its own tables | `deskpins --check` (already exists) |
| The roster known-set (the `ASSAY_*` configuration keys a consumer may set) | a machine-readable list of the known keys, versioned, emitted from the loader's own registry | a test asserting the emitted list covers the loader's registry exactly, so a new key cannot be added without the artifact moving | a consumer-side run that rejects an unknown `ASSAY_*` key against the published set (the loader already fails closed on one; the contract makes the *set itself* a published, checkable artifact) |
| The hook set (the pre-push and related client-side guards a consumer installs) | a versioned manifest of the expected hooks and their identifying content | a test asserting the manifest matches the hooks the installer actually ships | a consumer-side run reporting three-state whether the installed hooks match the manifest |
| The binary manifest (the installed tool set and its checksums) | the checksum manifest already published per release | a test asserting the manifest lists exactly the shipped assets | a consumer-side run verifying installed binaries against the manifest |
| The CI wiring (the lint action a consumer references) | the versioned action contract — the steps a consumer is expected to run | a test asserting the action runs the expected steps | the consumer's own CI *is* the run — a green lint job is the conformance signal |

Building instance #2 is a follow-up: each row above is a unit of work, and the pattern's
value is that each unit is the *same shape* as instance #1 — a descriptive-first artifact, a
source-side gate that derives its surface from the implementation's own tables, and a
consumer-side three-state run that rides the pin/release lane the consumer already follows.
The rule that keeps every instance honest is the one instance #1 states first: the artifact
is descriptive of current behaviour, and the on-file corpus is the arbiter. A contract the
existing consumers fail is a wrong contract, not a wrong fleet.

## What the pattern is not

- **Not a second linter.** `conform` and `--lint` are deliberately distinct verbs: the schema
  contract is per-file frontmatter *shape*; methodology rules (id-versus-filename, wave-versus-
  README, dependency resolution, the verifier floor, the risk/gate coupling) need the corpus or
  the README tables and stay in `--lint`. Merging them would make a reviewer unable to tell a
  shape violation from a methodology violation.
- **Not a prescriptive redesign.** The artifact never leads the implementation. It is written
  from the reference implementation's behaviour and held there by the source-side gate. A
  contract authored from the *spec* rather than the *validator* produces a false-positive storm
  on the first real corpus run — which is exactly the failure the descriptive-first rule and the
  pre-merge corpus run exist to prevent.
- **Not a replacement for the pin.** The pin still proves which build runs; this pattern adds
  the orthogonal proof that the build and the consumer still agree on the contract. Both layers
  are load-bearing, and neither subsumes the other.

## Semantic owners — one meaning, one home

A shared meaning — readiness, delivery, a claim, an identity — should have exactly one module
that computes it, so every consumer asks the same question of the same code instead of each
recomputing its own answer. Without that record nobody can tell "one meaning, several call
sites" from "one meaning, several *owners* that can quietly disagree" until two of them
diverge and the fix becomes another classification rule bolted onto one of them. This brief's
starting premise was that readiness alone has been computed this way about seven times across
this tree; the sweep below did not confirm that count for the readiness/eligibility meaning
itself — the eligibility row's own duplicates cell finds one evaluator, no full duplicate —
though most of the *other* seeded meanings below do carry a real duplicate owner or a
colliding implementation, and each row's duplicates column says which. (The prose here names
no row id on purpose: the brief's presence check counts ids across this whole section, so an
id named in prose would keep it passing after that id's row was deleted.) This section is the
record: it names today's owner for each seeded meaning,
the duplicates that exist alongside it, and — where one exists — the decision record that
chose the owner.

A row here is an **ownership record**, not a seam contract: it usually has none of the three
parts above (a row that later grows a versioned artifact, a source-side gate and a
consumer-side run stops being a bare ownership record and becomes an Instance in this
document, the way Instance #1 and #2 above did). Recording an owner is a prerequisite for a
seam contract, not a substitute for one.

**Ownership versus enforcement.** A second *owner* of one meaning is a duplicate — two
modules independently deciding the same question, free to drift apart. A second
*enforcement point* checking the same fact at a **different trust boundary**, failing for a
**different reason**, is a legitimate defense-in-depth layer, not a duplicate — the same
distinction this document already draws between the source-side gate and the consumer-side
run. This section keeps the two in separate columns so a reviewer can tell them apart at a
glance rather than re-deriving it from the diff.

**The amendment rule.** Changing a row's owner or meaning amends its decision record in the
same PR. A local exception carved out in another module — "this one caller gets to keep its
own copy" — is not a silent edit to this table; it is a `design-fit` review finding, to be
argued and recorded, not assumed. Moving a path out of *enforcement points* (or into
*duplicates*) is the edit that exposes a layer to retirement, so a PR that removes or
reclassifies an enforcement point carries a security review in the same PR.

Every path and line number below was re-verified against `origin/main` at `8ad26beab`
(2026-09-25). A cell that
found nothing names the search that found nothing, per the three-state instrument rule —
"not found" and "not checked" are never the same cell.

| id | meaning | owner | decision record | duplicates (path: note) | enforcement points | contract parts (artifact/source-gate/consumer-run) |
|---|---|---|---|---|---|---|
| S-local-performance | Local observation timing, coverage, profile privacy and invocation identity across performance schemas. Summed invocation work is neither human elapsed time nor wasted CPU. | `perfrec` (shared module): historical `schema.go`, `context.go`, `v3.go`, `reduce.go`; additive v4 `v4.go`. Desk wrappers and statusgen retain their existing effect seams. | desktools-v2/63, /67; selected producer migration remains /68. | No second recorder, raw extension producer API or alternate observation model. | Consent, file custody, request observers, online read authority and verification admission keep their owners. | `schemas/perf-v4.json`, `perfrec/v4_schema_test.go`, exact named /67 wire/bounds/external consumer/report tests and `perfrec/v4-mutations.json`; internal qualification is not independent verification. |
| S-performance-collection | Bounded, optional collection of performance observations: performance-event-v1 identity (source/epoch/sequence), consent and profiles, aliased correlation, the local spool's coverage, retention and loss accounting, and the batch/receipt admission semantics a collector answers with. Observations confer no work, acceptance, budget or effect authority. | `perfdata` (separate module): `event.go`, `payload.go`, `context.go`, `alias.go`, `consent.go`, `batch.go`, `snapshot.go`, `dist.go`; effects in `perfdata/local`; offline reduction in `perfdata/report`. | performance-collection/01 (gate: human; acceptance pending) | Collection owner only. It does not redefine measurement meanings: S-local-performance keeps perfrec's local timing and coverage; continuing-operations O02 keeps metric definitions; work, usage and acceptance keep their domain owners. Reusing the audit or dispatch journal as a prunable transport was considered and rejected. | Consent (`ASSAY_TELEMETRY=1` plus an explicit profile) and strict typed validation are enforced in `perfdata` on both the write path and the admission path; the optional database, collector and sender (performance-collection/02–05) are planned consumers, none built. | `spec/performance-event-v1.md`, `schemas/performance-event-v1.json`, `schemas/performance-batch-v1.json`, `perfdata/testdata/api/` conformance cases; the named brief tests, `TestSchemaFiles` and `perfdata/mutations.json`. |
| S-component-activation | Complete component manifest membership, deterministic discovery and current required-provider activation. A discovery snapshot is never a grant. | `tools/desk/internal/deskkit/activation.go` (`discoverManifests`, the implementation behind `DiscoverManifests`, and `ComputeActivation`), `activationindex.go` (bounded process-local reuse) | desktools-v2/60 | `tools/desk/cmd/deskmanifest/lint.go:discover` separately enumerates component membership for declaration lint; the runtime owner documents this existing duplication. It is unchanged here; `CheckVerbActivation` callers delegate to the runtime owner. | Existing direct activation, trust, stop and custody boundaries retain their authority. | `tools/desk/README.md` activation snapshot contract; the four named activation/startup tests and production-owner mutation runner. Every reused result still requires a full membership/content validation traversal; no persistent startup speed claim. |
| S-effect-recovery | Stable logical operation identity, mandatory durable intent and authoritative receipt reconciliation for every declared external effect. | `drainloop/effect.go`, `drainloop/handoff.go`; the engine owns Apply and consumes its configured Journal/TransactionalJournal. | graph-execution/04 | No competing effect journal: controller /21, publication /25 and PostgreSQL outbox /19 consume this seam in their own follow-ups. | Provider-specific idempotency/fencing and the coordination store's atomic ownership check remain independently enforced; unknown outcomes and stale pending generations hold. No activation or exactly-once claim. | `drainloop/effect_test.go`, `drainloop/handoff_test.go`, `drainloop/testdata/effect_mutations.py`; real database/provider qualification remains with consumers. |
| S-verification-domain | Qualified work, exact candidate/acceptance binding, required-fact completeness, reevaluation generation and separate stage/outcome records. Assessment is advice, never acceptance or effect authority. Evidence provenance (evidence-provenance-v1) adds the accountable principal, delegation lineage and separately corroborated assurance dimensions per evidence item, and duty applicability over them: independence is compared on responsible/delegating principals, never on keys, Apps or issuers, and a declared proof class never exceeds what is corroborated. | `statusgen/domain` composes `contexthandoff.ResolveQualifiedID` and `statusgen/evaluation`; instance/node/attempt identity stays with workflow-instance-v1. `provenance.go` (`Provenance.Validate`, `EffectiveProof`, `Corroborate` over a trusted `Selector` port, `EvaluateDuty`). Authenticating actors and principals stays with S-identity and trusted adapters. | graph-execution/28, /30 (evaluation-context-v1) and /31 (evidence-provenance-v1) | Complete derivation receipts, canonical fingerprints and bounded applicability belong to this same pure owner; current authorization stays at admission. Planned candidate workspace /23 and regression adapters /29 consume these types; admission /14, publication bridge /25 and control exports /15 are planned provenance consumers, none wired. | Existing revision-bound coverage independently holds foreign witnesses; future workspace snapshot checks remain /23. `EvaluateDuty` corroborates every evidence item through `Corroborate` against the `Selector` it is given before reading any field, so a claim that differs from the selector's record (a caller-chosen principal, delegation or proof label) is refused below any upper validation, and a missing selector is could-not-check. The selector is the trust root: composing a trusted one, never from model text, is the consumer's obligation (/14, /25), this package cannot tell a trusted selector from one that echoes its input, and none is installed. No credential, human decision, review gate or waiver moves here; decision [#4800](https://github.com/medici-finance/assay/issues/4800#issuecomment-6481737472) selected required corroboration for admission/publication, ratified in [DR-evidence-provenance](streams/decisions/DR-evidence-provenance.md); consumer wiring and activation remain separately scoped. | `spec/verification-domain-v1.md`, `schemas/verification-domain-v1.json`, `spec/evaluation-context-v1.md`, `schemas/evaluation-context-v1.json`, `spec/evidence-provenance-v1.md`, `schemas/evidence-provenance-v1.json`; named TestContextInputs/Expiry/AdmissionFlow/Membership/Schema and mutation controls; `TestDomainSchemaSurface`, `TestDomainSchemaFixtures`, `TestProvenanceSchema`, the four /31 duty tests and mutation controls; callers validate schema plus normalized records with the pure package (no CLI or runtime activation). |
| S-coordination-store | Explicit PostgreSQL compatibility and one forward-only migration/checksum ledger. | `workflow/store`; Goose owns migration execution/session locking and the project Store adapter owns checksum/version policy in the same table. | graph-execution/36 foundation; graph-execution/19 domain follow-up | No competing migration runner, online engine or mirror ledger; the bounded forge-neutral/52 mirror Store and forge-neutral/54 read-only refresh Gate and forge-neutral/55 explicit operator provisioning extend this manifest. | Open never migrates; missing/future/drifted schemas and ambient PostgreSQL configuration refuse. Operator provisioning creates only absent known accounting with immutable replay provenance; the declared restricted runtime cannot initialize/known-upgrade/reset profiles or use administrative ownership, membership or SECURITY DEFINER paths. The runtime's required fill eviction has an independent eligible-terminal SQL guard. No execution, claims, outbox, retention or activation authority is delivered. | `TestPGFoundationMigrations`, `TestPGFoundationConcurrent`, `TestPGFoundationCompatibility`, `TestPGFoundationSchema`, `TestPGFoundationNoFallback`, `TestPGFoundationFixture`, `TestPGFoundationBoundary`, `TestPGFoundationDirectivePolicy`; mirror SQL conformance, atomic coverage, process CAS, restart/rebuild, scope/presence and schema checks are owned by forge-neutral/52. forge-neutral/54 binds current generation, the transaction-bound Store and completion in this same owner; source/admission/caller budgets and individual retry observations have no local fallback. `TestPGRefreshProvision`, `TestPGRefreshProvisionReplay`, `TestPGRefreshProvisionProcesses`, `TestPGRefreshRuntimePrivileges` and `TestPGRefreshProvisionSchema` qualify the bounded operator/schema/role seam under forge-neutral/55. Gate construction requires an explicit trusted refresh.Clock shared with its source/coordinator for data-bound validation and ApplyCompletion evaluation; nil/typed-nil/zero clocks refuse and no default or skew policy is selected. DB time alone owns leases/windows/retry floors. Held/done bookkeeping fences only scope identity/version without decoding private observations. NativeLimits.Reservations is a lifetime retained-ledger cap, including observed entries; window rollover never clears it. Full parent acceptance remains owed. |
| S-forge-mirror | Admission-bound provider observations, coverage/freshness and deterministic scan/query semantics; complete-empty requires current admission and fresh complete coverage. | `forgeread/mirror` (`PlanRefresh`, `PlanScan`, `QueryScope`, guarded `ReadScopeSnapshot`, pure `EvaluateSnapshot`); trusted deskkit `NewMirrorIngestSource` implements its Source port over existing admitted readers and a composition-bound admission ID, checked against current Admission and Scope before/after each read. This consistency check does not independently qualify actual credential-installation selection. The frozen root API/envelope remain unchanged.  `forgeread/refresh` composes those ports through an injected shared gate. | forge-neutral/46, /50, /51 and /53, extracted from forge-neutral/37 | No second kernel, provider walker or runtime store. Production refresh ownership/accounting and failure cooldown belong only to the canonical R2 adapter; no local fallback. | Observations preserve BodyKnown and nil-versus-known-empty collections. PlanRefresh binds PriorObservedUpTo to the selected snapshot; PlanScan independently matches it with scope/version and retains all raw temporal-regression checks. Only a qualified complete source walk may carry forward the prior observed maximum; partial/failing source results have zero ObservedUpTo. Status-only 403/404 is not confirmed permission loss or retention authority. Store adapters independently enforce current admission, version and complete-scan watermark constraints. Permission-loss holds release only on a complete full inventory under current matching admission, with LastComplete and LastInventory strictly newer than the held LastAttempted; failed, partial, incremental, not-modified and replayed updates cannot release rows. `mirror/mirrortest` remains transport-neutral; workflow/store applies the same conformance suite to PostgreSQL under forge-neutral/52, without production shared-refresh generation fencing. Custody, joined persistence, retention selection and deployment remain forge-neutral/37 obligations.  The coordinator rechecks trusted caller epochs, repository admission and deadline after store, observer and Leave I/O; canonical guarded acquisition is shared, and snapshot evaluation/baseline planning sample trusted time after store I/O. Explicit-time QueryScope and actual future-timestamp refusal remain unchanged. Base fill leases and bounded requirement failures are separate; the canonical ReviewOnlyPartial class prevents a stronger owner from installing weaker joined-query cooldown without upgrading coverage; checked post-query Leave outcomes bind current generation and exact snapshot version, with no sliding cooldown or stale-outcome poisoning. Its fake gate/ref-store cases prove no SQL or cross-process behavior.  Native attempt receipts in forge-neutral/53 require a qualified decoded-data completion bound captured before authority observation I/O; source stamps and coverage use that bound, while query time stays current and canonical R2 budget/lease clocks remain independent. Missing production native bounds hold; legacy fake zero is unknown. The existing native leaf independently binds the closed method/origin/encoded repository/page query and current admission before canonical AttemptAuthority reservation and base access. Each reservation has one bounded typed observation; unknown/custom cache coverage holds. Local retry floors and unknown evidence refuse repeated work; canonical cross-fill budgets and durable floor publication remain with R2. No raw metadata, new walker or retry engine crosses the pure port. | `TestMirrorKernelAdmission`, `TestMirrorKernelCoverage`, `TestMirrorKernelScanPlanning`, `TestMirrorStoreConformance`, `TestMirrorPermissionHold`, `TestMirrorKernelFlow`, `TestMirrorKernelBoundaries`, `TestMirrorFieldPresence`, `TestMirrorPlanBaseline`; deskkit `TestMirrorSourceFlow`, `TestMirrorSourceScope`, `TestMirrorSourceUncertainty`; `TestMirrorNativeAttemptReceipt`, `TestMirrorNativeAttemptGrant`, `TestMirrorNativeRetryReceipt`, `TestMirrorNativeReceiptBoundary`, `TestMirrorNativeObservationBoundary`, `TestMirrorNativeFreshnessBoundary`; production-path mutation controls.  `TestRefreshSharedGate`, `TestRefreshFailureBackoff`, `TestRefreshGateFence`, `TestRefreshReleaseAuthority`; compiling production mutations and encoded-import controls. |
