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
| R7: public API and thin cockpit | The `cell-api` stream owns R7, ratified by the maintainer merge of medici-finance/assay#2890. Created parked; active for implementation from 20 October 2026 on the maintainer's own unpark request (https://github.com/medici-finance/assay/issues/3669#issuecomment-6302690781). The stream retains its existing qualification and human gates. | follow-up | follow-up | R5–R6 (interface work may begin earlier) | Mounted-route authorization and cross-cell isolation cases | cell-api/01–07; declared gates apply. | Seven briefs, each M |
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
| Cell API status queries and scoped operator requests (from `send`) | R7: `cell-api` (created parked; active for implementation from 20 October 2026) | cell-api/01–07; requirements in amendment 21 | Ownership ratified by the maintainer merge of medici-finance/assay#2890; stream scheduling follows the maintainer's own unpark request (https://github.com/medici-finance/assay/issues/3669#issuecomment-6302690781), a post-ratification scheduling act |
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

### Operating-report owner routing — 15 October 2026

| Slice | Owner and delivery | Runtime boundary |
|---|---|---|
| Capture and retained coverage/distributions | performance-collection/01–02; preserve /03–06 separate database/transport qualification | Local-first optional observations, no operational authority |
| On-demand command/skill and daily three-report bundle | performance-collection/07–08; existing flow/lifecycle meanings retained | R7 consumes the same projection; local report-pack host can precede R2–R8 |
| Later issue/draft-repair proposal loop | performance-collection/09; continuing-operations assessment and existing desk consumers | Managed activation depends on R2/R3/R4 and the brief's typed graph prerequisites; no new scheduler or autonomous merge |

This is a requested specification/brief amendment, not implementation or activation evidence. It supersedes the "reused unchanged" record above for performance-collection/01, 02 and 06 only: the 15 October operating-report amendment extends those briefs and adds 07–09, with their collection/storage boundaries and runtime authority unchanged.

## Operation-count brakes and pacing — 15 October 2026

Controlling delta: [brake and pacing contract](11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed).
Counts that bound a runaway or blast radius are brakes and stay fail-closed: write
ceilings, daily and new-issue caps, the circuit breaker, the child-launch budget, ingress
and retry budgets. Only pacing counts become best effort, with a shared estimate (always
re-derived from durable admission records for effect-class counts) and a
declared drift bound for effect-class counts (cap + C, or
2 × (cap + C) after one recorded reset per window and operator incarnation); read-class
pacing counts carry no numeric bound and are unbounded on counter loss.
Unknown usage still cannot become a fresh allowance. Docs 14, 15, 19 and 23 carry the
split; doc 15 adds an operation-count qualification section. No brake is relaxed;
relaxing one is a separate human decision.

| Consumer | Applied amendment | Qualification |
|---|---|---|
| R2 store/ownership: graph-execution/16 and /19, versions 7 | Cumulative attempt budgets and brake counts keep fail-closed reservations; pacing counts need no exact budget slot and keep one shared estimate, re-derived from window intents and receipts for effect-class counts, never from a cached row | New Verify rows: brake holds on an unavailable count; restart or per-process fallback cannot multiply a pacing cap |
| R3 supervisor: graph-execution/26, version 6 | The child-launch budget is classified as a fail-closed brake; only desk/tool pacing is best effort, and launch pacing is effect-class, re-derived from recorded launches | Missing configuration and an unavailable child-launch count both refuse; new Verify row with a mutation that treats the count as under budget |

Existing Task/Verify rows and historical Evidence remain intact; each brief adds rows and
bumps `version:`. No installed limiter, default, credential, executor activation or hard
resource limit changes in this documentation PR.

## Portable execution environments — 16 October 2026

Controlling delta: [agent execution environments](27-agent-execution-environments.md).
The new agent-execution stream, parked until that amendment is approved, authors seven
bounded extensions of existing owners:

| Gap | Owning brief | Existing prerequisite/consumer | Change made in this authoring PR |
|---|---|---|---|
| Qualified harness/model/backend and explicit preservation | agent-execution/01 | implemented graph-execution/20 | New extension obligation; reciprocal edge only on /20, existing Evidence untouched |
| Prepared-environment receipt and workspace port | agent-execution/02 | /01; existing candidate and retirement owners retained | New pure port; no second Git, resource store or pruner |
| Local enforcement | agent-execution/03 | /02, graph-execution/22 and /26 | Follow-on to their existing implementation; reciprocal edges, no reassignment |
| Integrated restore/effect proof | agent-execution/04 | /03, graph-execution/21 and /25 | Production-owner integration tests; no alternative scheduler or effect ledger |
| Lifecycle projection | agent-execution/05 | /03–04, graph-execution/27 | Existing cellop/cellctl consumers; R7 retains API/cockpit ingress |
| AX source/fixture qualification | agent-execution/06 | /01–02, independent of managed integration | Offline go/no-go/unknown report; no production adapter or cluster activation |
| Governed harness refinement | agent-execution/07 | /01–02; existing improvement/promotion owners retained | Offline protocol experiment with independent holdouts, protected controls and complete cost; no live optimizer |

Source: main ec8d77e56627a7c2269f3d143ef6a31f3e41bd02, open-PR inventory refreshed
16 October. No listed implementation PR targets the new extension/receipt paths. The open
/20 Evidence PR is preserved. Refresh claims and PRs again at pickup; this bounded observation
is not an ownership transfer. Existing Task/Verify text, gates, versions and Evidence are
unchanged; only reciprocal unblocks and dated routing notes are added. /03's concrete profile
adoption decision ripens after its implementation design. Full live qualification retains
existing R1–R8 and operator decisions. No existing platform/profile is silently replaced.

## Desk-process improvement — proposed owner set, 17 October 2026

[Amendment 22](22-desk-improvement.md) extends continuing operations/quality. It does not
replace R1–R8 or their activation sequence. Source state: public main 2f56cf289; current
open claims/PRs were not enumerated in this offline audit and must be reconciled at pickup.

| Work | Owner briefs | Source owners / treatment |
|---|---|---|
| Definition and pure calculation | desk-improvement/01–02 | Extend qualgen assessment/projection, semantic-owner index; no new miner |
| Diagnosis/experiment/follow-up validation and role procedure | desk-improvement/03–04 | Continuing-operations lineage; shared skill reference; concrete procedure activation remains reviewed |
| Native source bindings | desk-improvement/05 | Consume graph-execution/06 + brief-quality/05, compatible associations only |
| Runtime evidence | desk-improvement/06–08 | After graph-execution/26 + /22, /21, /27 respectively; /08 also consumes brief-quality/07; foundations receive consumer notes, no reverse dependency |
| Read-only proposal context | desk-improvement/09 | Existing Improve human/cadence and internal-module boundary |
| End-to-end native qualification | desk-improvement/10 | Actual producer→mapping→cards→assessment→view, recovery and unknown/private cases |

The new stream is parked and proposal-scoped. Every new brief is M/strong and carries
executable discriminating checks. Existing completed Evidence/approval is preserved.
Offline assistance can precede native emission, while measured native-mode benefit requires
its real producer closure. Public implementation has no private adopter dependency.

Semantic prerequisites: desk-improvement/01 consumes continuing-operations/01 (after
statusgen/22); /03 consumes continuing-operations/02. These are explicit build gates,
so generic source contracts are implemented once, before their assessment consumers.

## Perf-v3 attribution and forge cost — requested amendment, 19 October 2026

[Contract](../../performance-attribution-spec.md): new desktools-v2/63–65 own versioned local
schema/readers, desk/agent/CLI propagation and own-request timing/points. Unfinished /62,
statusgen/26 and performance-collection/01–09 are amended; completed /61 stays intact.
The target, future-state bindings, storage/recovery and runtime handoff carry the same
context/consent/coverage contract. This supersedes earlier v2-only future measurement intent,
not historical v1/v2 facts, Evidence or the collection stream's admission state.

## Write-policy and pickup amendment — 18 October 2026

| Obligation | Owning plan / candidate change | Proof and limits |
|---|---|---|
| One reusable local write evaluator and command adapter | New desktools-v2/66 (writeguard keeps its current parser; the /31 port is withdrawn) | Library/built-hook parity, effect boundary, source-bound consumers and external-module builds; other guards keep independent owners |
| Future/guideline recheck at implementation pickup | statusgen/20–21 amended; current spec and worker-kit instructions use the manual procedure | /20 scoped guideline/target discovery; /21 actual dispatch hold/continue behavior and shipped-version citation; no resolver/activation claimed |
| Managed workspace consumers | Consumer plans must bind allocation observations, hook profile and sandbox roots; qualification consumes exact producer versions | Public contracts stand alone; adopter tracking/rollout and active-worker acknowledgments remain in their own venue |

Inspected base: `c3d073572f41e09bedcac05944f9a202c3267353`. Historical accounts and
completed Evidence remain unchanged. Live claims/PRs were not read in this local
authoring pass: these candidate amendments require the existing affected-work
checkpoint/reconciliation before dispatch and never silently rebind a running worker.

Active-plan inspection, 18 October 2026: 52 open public PRs were enumerated through
the worker identity. medici-finance/assay#3409 (since merged) withdrew /31; medici-finance/assay#3429 (since
merged) also changed the brief specification.
This amendment keeps /66 independent and adds graph-execution/22 as its target
consumer. Reconcile source revisions at merge/pickup; no running worker is rebound
and no competing proposal is treated as approved. Dispatch refs were not read.

## Interactive Kubernetes owner routing — 20 October 2026

[Controlling amendment](28-interactive-kubernetes-desks.md). No new stream or runtime owner.

| Obligation | Applied authoring change | Acceptance |
|---|---|---|
| Separately interactive role Pods | desk-containers/06 version 5 | Five workloads/PVCs; structured offline negative controls; compatibility custody disclosed |
| Shared Kubernetes transport and managed standing profile | New agent-execution/09; /08 follows /09 | Current identity/stop/readiness checks, no workflow-store prerequisite; verifier privileges remain distinct |
| In-cluster operator | New desk-containers/28 | Compiled cellctl through existing clients; scoped operator capability, reconnect never launches |
| Whole-cell and optimization proof | New desk-containers/29 | Five-role flow, custody/recovery boundaries, bounded startup and zero idle model calls; separate real-profile witness |
| Optional fleet presentation | desk-containers/07 version 4 after /28 | Linux attachment layout; no native Windows gate or parallel launcher |
| Storage and scheduled compatibility | /27 and /12 dated routing | CNPG profile consumes base directly; CronJobs cannot co-own interactive roles |
| Shared runtime, roles and public migration | graph-execution/26–27 consumption; amendment 28 migration table | Existing R1–R8 semantics, exact-subject observations and perf-v3 counts retained; source handoff preserves history |

New tests are future implementation obligations. Existing completed Evidence is unchanged.
Active PostgreSQL/image-scan/performance PRs are recorded in amendment 28 and reconciled at
pickup. The parked execution stream is not silently admitted by this routing note.

## Extensible JSON amendment — 21 October 2026

[The census](29-extensible-json-contracts.md) is the owner/transition manifest for this
requested change. Desktools-v2/69 delivers the architecture and performance contract;
/67 implements compatible readers and /68 selects producers. Existing owners retain
all other families; their next contract-changing briefs must carry the reader-first
compatibility matrix and consumer dispositions before implementation. This amendment
changes no existing Verify/Evidence section or active owner's implementation. The
earlier whole-target ratification does not certify these later codec changes.

## Candidate-plus-priority amendment routing — requested 23 October 2026

Source pickup: App-fetched public main `9c4307a19`; release v1.0.57 call-path audit.
The operator requested one candidate-plus-current-priority planner for prose and discrete
desks and ranked GH reductions. This row set routes the delta within existing owners;
prior approval, active assignments, completed Evidence and enrollment remain unchanged.

| Obligation | Existing owner / actual amendment | Remaining adoption |
|---|---|---|
| Complete observation + explicit policy/time/capacity -> explainable ordered plan | desk-workflow-efficiency-spec; desk-tools/39–40 | graph-execution/27 uses tools/desk-local role bindings; no cross-module internal imports |
| Candidate coverage and priority parity for every role | desk-tools/42,45,51 and graph-execution/27 | Existing prose/discrete role cutovers prove A11; held/unknown/unmapped and repair populations remain |
| Ranked network request reductions and matched growth curves | desk-tools/44 and current measurement owners | A12 counts producer, HTTP attempts/points, fallbacks and final checks without double-counting |
| Shared refresh/invalidation and immutable source reuse | forge-neutral/37–43; 14-storage-forge-graph and 22-forge-service-and-source-queries | Qualified R6 path later; direct-reader bridge first; no per-caller outage poller |
| Custody/query/CLI compatibility | Existing desktools-v2 /09, /20–21 | No rerun of completed query work, new ranker, credential pooling or premature command deletion |

Reuse [the existing change map](../../desk-workflow-efficiency-change-map.md#candidate-plus-priority-ownership-reconciliation--23-october-2026)
for exact scope and sequencing. Recheck active work before dispatch/publication; material
assignment changes require its existing owner checkpoint/acknowledgment, not silent rebinding.
