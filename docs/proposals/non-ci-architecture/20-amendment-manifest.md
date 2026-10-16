# Amendment manifest: first affected work set

**Reconciliation record, 13 October 2026.** Delivery-sequence step 3 ("Reconcile the first
affected work set") for the target the maintainer ratified on
[medici-finance/assay#2500](https://github.com/medici-finance/assay/issues/2500) at revision
`23f655f5d` (whole target, packages R1–R8). Approval activates nothing: each package's
activation stays gated by the [delivery sequence](16-delivery-sequence.md), and after R8
activation is a separate operator act. Plan state was inspected on main at `a435badbd`.

This PR carries actual brief patches for **R1** and the **step-2 package** (shared context
handoff and authoring support). For R2–R8 it records the owner and amendment scope only; their
brief patches are follow-up PRs. The progressive-adoption lane ([doc 19](19-progressive-adoption-and-oversight.md))
is recorded as owner rows only. No Evidence section or dated source account is edited; brief
amendments are additive dated sections.

## Owner manifest

Dependencies follow the ratified path R1 → R2/R3 → R4 → R5 → R7 → R8, with R6 feeding R7.
"Patched here" means the brief file is in this PR. Effort uses the closed S/M/L scale per
brief; a package estimate is a brief count, not elapsed time.

| Package | Owning stream and brief(s) | Delivery paths | Contract version | Depends on | Test commands | Active-work treatment | Effort |
|---|---|---|---|---|---|---|---|
| Step 2: context handoff and authoring support | statusgen: `statusgen/19`, `statusgen/20`, `statusgen/21` (new, patched here) | `spec/context-handoff-v1.md`, `schemas/context-handoff-v1.json`, `schemas/amendment-manifest-v1.json`, `statusgen/contexthandoff/`, `statusgen/context.go`, `statusgen/main.go`, `docs/context-handoff.md`, `plugins/assay/skills/author-spec/SKILL.md`, `plugins/assay/skills/author-brief/SKILL.md` | `context-handoff-v1`, `amendment-manifest-v1` | Approved target only; 19 → 20 → 21 | Named Verify rows in the three briefs (`TestContextHandoff*` in `./contexthandoff/`, `TestContext*` resolver cases, `TestContextSpecToBriefJourney`, `TestSkillsCiteShippedContextVerbs`, `cd tools/skillslint && go run . --root ../..`) | No open claim or PR touches these paths. Manual bootstrap packet stays the documented path until 20 ships and is pinned. | M + L + M |
| R1: domain contracts and regression floor | graph-execution: `graph-execution/28`, `graph-execution/29` (new, patched here); `graph-execution/09` and `graph-execution/23` amended here | `spec/verification-domain-v1.md`, `schemas/verification-domain-v1.json`, `statusgen/domain/`, `statusgen/testdata/domain/`, `statusgen/regressionfloor_test.go`, `statusgen/testdata/regressionfloor/`, `docs/contracts.md` | `verification-domain-v1` | `graph-execution/09` (which needs `graph-execution/02` verified); consumes merged `statusgen/evaluation` | Named Verify rows in 28 and 29; the six baseline cases in `statusgen`; `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/` | 09 and 23 are todo with no claim; amendments add dated sections, Task/Verify unchanged. statusgen/15's open Evidence PR is untouched; R1 consumes its merged code. | M + M |
| R2: PostgreSQL coordination and reconstruction | graph-execution: amend `graph-execution/19` (store), `graph-execution/16` (incarnation/fencing), `graph-execution/04` (effect intents), `graph-execution/26` (journal) and the stream routing | `workflow/store/` (as planned by 19), `loopadmin/supervisor/` (as planned by 26), schema migrations per follow-up | follow-up names it (store schema v1) | R1 | Real PostgreSQL restart/deletion/backup-rollback cases (doc 15 layer 2); claim-test intent from the regression inventory | 19, 16, 04, 26 are todo, unclaimed. forge-neutral/22–32 (file/served claim store) is a competing coordination owner; the follow-up dispositions it. | 3–4 briefs, each ≤ L |
| R3: isolated runner and cell authority | graph-execution: amend `graph-execution/22` (first real adapter), `graph-execution/16` (exclusive assignment), `graph-execution/26`; implemented `graph-execution/20` is consumed, not edited | `loopadmin/adapters/localharness/` (as planned by 22), `loopadmin/process/` (as planned by 26), `loopadmin/runner/` | existing `loop-admin-runner-v1` (spec and schema) | R1; beside R2 | `cd loopadmin && GOWORK=off go test -count=1 ./runner/conformance/` against the real adapter, plus capability positive/negative cases | 20 is implemented with an open Evidence PR; leave it. medici-finance/assay#2459 (runtime adapters) is reconciled by the follow-up. | 2–3 briefs, each ≤ L |
| R4: role-bound publication and reconciliation | graph-execution: amend `graph-execution/04` (effect identity, lost acknowledgment) and `graph-execution/25` (evidence-PR bridge) | `drainloop/`, `workflow/publication/` (as planned by 25) | follow-up names it | R1–R3 | Stale-capability refusal at the effect boundary; ambiguous-effect reconciliation (doc 15 new obligations) | Legacy publication briefs desk-supervision/23, /26, /27 and desk-tools/30 stay on the supported legacy path with applicability noted by the follow-up; urgent fixes continue there. | 2 briefs, each ≤ L |
| R5: complete verifier use case | graph-execution: amend `graph-execution/21` (controller host, verifier use case), `graph-execution/14` (admission), `graph-execution/18` (integrated proof) | `workflow/controller/` (as planned by 21) | consumes R1–R4 contracts | R1–R4 | Complete disposable journey with crashes at durable transitions (doc 15 layer 3) | 21, 14, 18 are todo, unclaimed. | 2–3 briefs, each ≤ L |
| R6: forge mirror and shared observations | forge-neutral: new brief(s) by the follow-up, building on implemented `forge-neutral/33` and `forge-neutral/36` and verified `desktools-v2/09` | `forgeread/`, mirror store paths per follow-up | follow-up names it | R1–R2; beside R3–R5 | `cd forgeread && GOWORK=off go test -count=1 ./...` plus pagination, lost/reordered event and permission-loss cases | forge-neutral/18 is in progress; the follow-up must not rebind it silently. | 2 briefs, each ≤ L |
| R7: public API and thin cockpit | No existing owner. Proposed: a new stream `cell-api`, `status: parked` (see the cell operator surface rows below; ratified when medici-finance/assay#2890 is merged by the maintainer). The manifest's original options were extending derived-board (projection semantics) or a new stream; a new stream ships `status: parked` and its PR names the swap. | follow-up | follow-up | R5–R6 (interface work may begin earlier) | Mounted-route authorization and cross-cell isolation cases | None active. | 2–3 briefs, each ≤ L |
| R8: shadow, migration and retirement | composability: amend `composability/03` (desired-state reconcile, migration notices); build-less-brittle/11 testledger consumed as the departure report | `tools/desk/cmd/deskmigrate/` (per composability/03), `migrations/` (adopter migration notices), `tools/desk/internal/testledger/` (consumed) | follow-up | R5–R7 | Old-capability refusal, repository-wide handover and rollback in a disposable environment; classified old/new differences | composability/03 is todo, unclaimed. | 2 briefs, each ≤ L |
| Workspace lifecycle completion (continuing-operations slice) | continuing-operations (parked, no authored briefs) for the slice; desk-tools deskwt briefs for current-tool flags | `tools/desk/cmd/deskwt/`, sweep and maintenance owner paths per follow-up | retirement and maintenance contracts (R1/R2) | R1 retirement identities; R2 retirement progress | Sustained-work workload retiring disposable resources with held reasons explainable | medici-finance/assay#2525 (offline repository maintenance) is open; the follow-up reconciles against it, never in parallel. | 2 briefs, each ≤ M |

The real head of R1 is the verification of `graph-execution/02` (implemented, Evidence PR open;
verification is blocked on a Linux network-off sandbox, medici-finance/assay#2665), then
`graph-execution/09`. `statusgen/19` has no prerequisite and is dispatchable once this PR merges.
Independent runtime work may use a bounded manual packet against approved contracts meanwhile.

## R2–R8 amendment scope

**R2.** Amend graph-execution/19 from a single-controller embedded SQLite store to PostgreSQL
coordination for attempts, leases and effect intents with versioned migrations and reconstruction
from Git definitions, published evidence and forge reads; SQLite remains only an offline read-only
projection. Amend 16 for operator incarnation binding (a fresh authority incarnation after
deletion or rollback), 04 for transactional intent/outbox handoff, and 26 so the supervisor
journal records process facts against the shared store rather than its own engine. Rewrite the
2026-10-02 stream routing that excludes a shared store, keeping it as history. Disposition
forge-neutral/22–32's claim store against PostgreSQL coordination. Carry durable pending feedback
and retirement progress (operating obligations).

**R3.** Amend graph-execution/22 so the first real adapter runs disposable execution under the
implemented loop-admin runner protocol with role separation and process containment; amend 16 for
exclusive repository assignment (duplicate cell assignment refused before capabilities issue) and
26 for guard events, budgets, resource capacity and custody. The conformance kit runs against the
real adapter while fake and hostile controls stay.

**R4.** Amend graph-execution/04 for stable effect identity and lost-acknowledgment handling, and
25 so evidence-PR submission checks current authority at the effect boundary and publication
cannot grant acceptance or a human decision. Record the legacy publication briefs (desk-supervision/23,
/26, /27; desk-tools/30) as the supported legacy path until R8 retirement.

**R5.** Amend graph-execution/21 to host the complete verifier use case (resolve, admit, run,
assess, publish/reconcile, project) with cancellation and recovery; 14 binds admission at the
dispatch boundary to R1 records; 18 becomes the integrated disposable-journey proof including
resumable retirement and reevaluation without bookkeeping-triggered reruns.

**R6.** Author forge-neutral brief(s) for scoped PR/MR/issue mirroring with persisted freshness
and coverage, conditional refresh, polling repair and an authenticated event adapter where enabled,
reusing the importable fact reader (forge-neutral/36) and the tuned snapshot queries
(desktools-v2/09). Translate refreshed feedback into durable reevaluation; partial scans stay
incomplete.

**R7.** Owner proposed as the new `cell-api` stream (see the cell operator surface rows below); then author the public API and thin cockpit brief(s): the
same work, evidence and hold semantics for UI and clients; freshness and recovery visibility;
execution, acceptance, pending feedback, resource holds and maintenance health shown
independently through one authorized API.

**R8.** Amend composability/03 for migration notices and adapters with removal conditions, and
author the shadow-comparison, handover-rehearsal and legacy-publisher-inventory brief(s). Map the
recent hygiene regressions into the migration inventory through the testledger contract. Qualify
one maintenance owner per common Git store and the release/pin migration. Retirement of legacy
paths follows the installed-consumer inventory, never brief counts.

## Progressive-adoption lane (owner rows only)

| Slice | Owner | Note |
|---|---|---|
| Pure adoption/oversight contracts | graph-execution/28 (R1 meanings) plus composability (install-contract owner) | Follow-up extends 28's fixtures and composability's component contract; no brief patch here |
| Inspect and advisory Assist | statusgen (offline definition/evidence adapters, existing CLI surfaces) | Uses the bounded manual bootstrap; no R2–R8 prerequisite |
| Selected installation and actionable output | composability (config/cell operation) and derived-board (projection) | Versioned dependency closure, preview/resume/no-clobber |
| Managed oversight | R1 policy, R2 pending decisions, R3/R4 authority, R5/R7 result flow (owners above) | Keeps the R1–R8 dependency and activation sequence |
| Sampling/batch delegation and scale | decisions stream plus the runtime, effect and operations owners above | Needs a separately approved scoped contract |

## Cell operator surface (amendment 21)

Owner rows for the [cell operator surface amendment](21-cell-operator-surface.md), proposed in medici-finance/assay#2890; ratified when that PR is merged by the maintainer. These rows record where its work lands and apply no brief patch to any owner below. The verb briefs themselves are in `desk-containers` on today's tools.

| Work | Owner | Brief(s) / treatment | Status |
|---|---|---|---|
| Result / plan contract (`cell-op-v1`) | desk-containers for the schema and CLI adapter now (apply only by interactive confirm); the R7 owner serves it read/plan-only | `desk-containers/15` | Authored |
| Approval-based apply (deferred) | R7 (served route) and R3 (isolation from the desk account); evidence-and-decisions shapes the record | Gated on amendment 21's five approval properties; no brief | Deferred; the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge) |
| Interim verbs: up, status, doctor, logs, gc, upgrade | desk-containers (interim bridges) | `desk-containers/17`, `/20`, `/21`, `/22`, `/24`, `/23` | Authored; each names its target owner |
| Lifecycle transitions (pause, resume, drain, stop, start; restart as a sequence) and handoff record | graph-execution: **R2 records the requested transition and the handoff record; R3 executes it on processes** | Interim adapters `desk-containers/16`, `/18`, `/19`; R2/R3 owner amendments follow ratification | Split is the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge) |
| Cell API status queries and scoped operator requests (from `send`) | R7: new stream `cell-api` (`status: parked` at creation; scaffolding is a follow-up PR) | No brief; requirements recorded in amendment 21 | Owner is the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge) |
| Workspace retirement behind `gc` | continuing-operations workspace-lifecycle slice (row above) | gc is a trigger only | Reconciled with that slice's follow-up |
| Upgrade / release-pin migration behind `upgrade` | cell operation; R8 (composability/03) | upgrade is a trigger only | Reconciled with the R8 follow-up |

## Conflicts and follow-ups

- **Storage routing.** graph-execution's 2026-10-02 routing permits an embedded SQLite store (19)
  and a per-supervisor journal engine (26) and excludes a shared store. The ratified target
  places runtime coordination in PostgreSQL. The R2 follow-up resolves it; until then 19's store
  and 26's journal engine choice need that amendment before work starts.
- **Claim store.** forge-neutral/22–32 move claims to a file/served claim store; R2 owns the
  coordination record. The R2 follow-up dispositions them.
- **Legacy publication.** Evidence-landing briefs on desk-supervision and desk-tools stay on the
  supported legacy path; R4 and R8 decide applicability and retirement.
- **Follow-up PRs.** One per package or tightly coupled pair, in dependency order: R2, R3, R4,
  R5, R6, R7 (owner proposed: `cell-api`, see below), R8, then the workspace-lifecycle slice. Each carries actual
  brief patches; an "amend" row here does not fulfil it.

Adopters make their corresponding local amendments separately; nothing in this manifest edits an
adopter repository or names private repositories, credentials or deployment details.

## Portfolio/drift follow-through — 13 October 2026

The [product-intent and portfolio amendment](26-product-intent-and-portfolio.md) adds a concrete owner/dependency table and six brief files,
not just owner rows. Statusgen/19's merged work is unchanged; /23 extends its reviewed
result after /20. Statusgen/20–21 amendments are limited to declared coverage and supported
product-binding consumption. Continuing-operations remains parked pending explicit admission.
R2 and R6 are covered by the merged PRs #2750 and #2743; this amendment changes neither.
R7 must consume statusgen/25 and map independent product/architecture/binding/allocation/
observation dimensions; its `cell-api` owner stream is named above, and its scaffolding and
serving brief remain owed.

## Forge-service follow-on — amendment of 14 October 2026

The new [bounded design](22-forge-service-and-source-queries.md) extends R6/R4 and R7's
read interface. The original dated manifest above remains its source account. PR 2743
(merged as `6b479a573`) authored forge-neutral/37–38 (mirror and event adapter) and the follow-on
forge-neutral/39–45. Reconcile its reciprocal edges and waves
against the landed R2 amendments before delivery. Do not duplicate the mirror or silently
rebind in-progress forge-neutral/18, populated Evidence or historical decisions.

| Extension | Owner/delivery | Dependencies and active-work treatment |
|---|---|---|
| Typed service contract and SDK | forge-neutral/39; `forgeread/service/`, separate `forgeread/client/` | Pure contract first; root/offline reader and implemented 36 are preserved |
| Authenticated serving and executor custody | forge-neutral/40; `tools/desk/internal/forgeservice/`, private role executor, thin commands | 39 and R2/R3 incarnation/ownership contract; preserve role-specific custody and guards |
| Write-through receipt/projection | forge-neutral/41; service composition plus `workflow/store/` adapters | 37, 40, graph-execution/04 and /19; no second intent/outbox or issue authority |
| SHA-bound source queries | forge-neutral/42; pure query contract plus trusted Git/provider adapters | 39–40; consume existing gitcore and exact-candidate meanings |
| Incremental indexes and invalidation | forge-neutral/43; mirror-derived indexes and application query composition | 37 and 42; domain evaluators unchanged; explicit access/time/input dependencies |
| Shared Git reuse trial | forge-neutral/44; gitcore test harness and public experiment report | 42; reuse existing maintenance/retirement owners, no daemon or live fetch required |
| Served flow, recovery and cutover | forge-neutral/45; disposable integration harness and operator runsheet | 38, 41, 43, 44 and R3/R4 proofs; write activation independent of read release |

The existing R7 owner choice still governs full remote ingress/cockpit work. This amendment
supplies a narrow reusable local forge surface, not an alternative application framework.
The source baseline and the merged forge-neutral/37–45 state are pinned in doc 22; implementation workers
refresh current source/claims and resolve existing human decisions before starting.


## Public runtime alignment — 14 October 2026

Controlling delta: [one public runtime](23-runtime-contract-alignment.md). Existing R1–R8
rows remain owners; no separate implementation programme is opened. Actual patches here:

| Gap/consumer | Applied authoring amendment | Qualification |
|---|---|---|
| Evaluation context and expiry | new graph-execution/30 after /28; complete inputs, time-bound validity and conservative reuse | pure context/schema and real evaluator flow; cached result grants no authority |
| Accountable principals/proof provenance | new graph-execution/31 after /28; shared issuer versus delegation, proof classes and missing corroboration | synthetic independent-principal and trusted-selector bypass cases; no crypto/custody deployment claim |
| Dispatch/admission | /14 depends on /30–31, version 4; dated consumption and two Verify rows | current context and corroborated independence checked at actual admission |
| PostgreSQL bindings | /19 depends on /30, version 4; dated context persistence and real-PG Verify row | restore cannot renew validity or source coverage |
| Scoped control exports | /15 depends on /31, version 2; proof-provenance consumption and Verify row | truthful assurance and missing/denied coverage in independently read exports |
| Publication/review | /25 depends on /31, version 2; dated provenance consumption and Verify row | existing exact-candidate, role and disabled-profile controls retained |

Historical Evidence is byte-preserved. /28 receives reciprocal unblocks only, not Task/Verify
changes. The merged forge-neutral/39–45 and performance-collection/01–06 are reused unchanged.
R3/R4 custody and R5/R7/R8 integration/migration remain their existing proof obligations.
