# Desk workflow efficiency — reconciliation with existing work

**Status:** routed — 17 October 2026.
**Routes-to:** docs/streams/desk-tools/ (with amendments in performance-collection and forge-neutral). The maintainer
confirmed merge of #3322 as approval. This authoring change adds the briefs and owner-local
Task/Verify amendments below; merge completes routing, not their implementation or adoption.
Refreshed main: `fb85924c039c27e7d2aa80e055bfce118e46125d`. Historical dispositions in the
original table remain source-qualified; current active-work checkpoints are recorded below.

## What can be folded, and what cannot

Fold an obligation into an existing owner when it extends that owner's contract. Preserve
completed implementation and Evidence; do not reopen it under a second name. Most new work
here is a bounded follow-on to shipped mechanisms. Several pending owners are already L;
route the extra requirement through that owner but split implementation rather than growing
one unreviewable brief. Spec approval must be followed by actual brief amendments and
consumer/Verify updates; the accompanying brief diffs now propose those Task/Verify and consumer changes.

| Existing owner, status at source pin | Disposition | Exact overlap and proposed treatment |
|---|---|---|
| desk-tools/21, cause-carrying errors/shared subprocess runner | Reuse | Instrument the existing runner, retaining redaction and error/exit semantics. No new tracing runner. |
| desk-tools/23, local perf-v1, implemented | Preserve; bounded follow-on | Correct command/kit mapping and missing counting paths. Keep the historical format/consent promise. This is additional implementation, not proof /23 never shipped. |
| desk-tools/24 and /25, audit/token optimizations, done | Preserve | Retain tail reads and token memoization in baseline and subject; no duplicate cache project or reimplementation. |
| desk-tools/27, pooled board reads/root reuse, in-progress | Finish existing contract separately | Reuse pools and root resolution. Retain its Evidence and verification debt; do not turn this M into the entire redesign or claim another concurrency saving for already-delivered code. Refresh the actual remaining verifier findings at pickup. |
| desktools-v2/09, access-pattern queries, verified | Reuse; bounded query follow-ons | `ReviewQueueSnapshot` is the starting point. Extend typed operations only for measured missing inputs; count requests and actual/computed points separately. Do not rewrite the verified snapshot or put raw GraphQL in a caller. |
| desktools-v2/20 and /21, board/dispatch CLI migration, todo; each L | Coordinate, do not merge wholesale | Own parser/configuration compatibility. New bridge forms use the shared CLI contract; preserve existing stdout/exit obligations. Keep semantic extraction in focused follow-ons rather than hiding it inside a parser port. Reconcile overlapping source changes before either implementation starts. |
| desktools-v2/45, board-reader CLI migration, implemented | Preserve | Issueboard's new projections must use its delivered CLI adapter. New semantics are follow-on work, not a second parser migration. |
| desktools-v2/56, deskperf CLI migration, todo/S | Keep separate | Parser migration does not own new performance meanings or richer collection. Extend reporting through the measurement owner without expanding this S into a profiler. |
| performance-collection/01, event/spool/local report, todo/L | Amend contract through its owner | Select bounded phase, coverage and request-cost distinctions; preserve profiles/consent, seven-day initial local window and optional-sink isolation. Split extra implementation if needed. No new spool/schema owner in desk-tools. |
| performance-collection/02, producer adapters, todo/L | Amend producer obligations; split implementation if needed | Carry selected phase and parent/child attribution, source versions, kit/caller mapping and honest unavailable/inclusive-CPU coverage. Operational dispatch/audit identities remain private; only the selected aliased profile can correlate them. |
| performance-collection/07, on-demand reports, todo/L | Fold diagnostic report requirements | Add phase/endpoint and complete-cycle views to the tool panel using source-qualified data. Preserve its existing three-report bundle, pooled distributions and offline/no-provider-read contract. No second reporting CLI/skill or analytics database. |
| performance-collection/03–06 and /08–09 | Retain independent scope | Database, ingestion, upload, daily reporting and automated repair are not prerequisites for local instrumentation or the read-path efficiency lane. Do not promote or activate the performance-collection stream by implication; its activation rides its own recorded ruling. |
| forge-neutral/36, narrow reader SDK, implemented | Reuse boundary | Current first shared read is open issues. A public review-observation API needs a separately scoped extraction; existing `internal` command packages are not external APIs. |
| forge-neutral/37–38, mirror/events, todo | Fold eventual bulk-reader consumer requirements | Scope, coverage, freshness, coalesced refresh and event/poll repair remain here. Migrate actual consumers through qualified versions; early direct-read optimization stays independent. No private or command-specific mirror. |
| forge-neutral/39–40, typed serving/custody, todo | Fold future queue/context serving contracts | Service requests and client adapters expose the same contracts; credentials and role effects remain separately contained. Not needed merely to share code in the desk module. |
| forge-neutral/41, receipts/write-through, todo | Preserve effect boundary | Observation reuse cannot replace current preconditions or turn an uncertain external write into permission to repeat it. No new effect ledger. |
| forge-neutral/42, exact-SHA source queries, todo | Reuse/extend context input | Future exact-source reads and immutable keys supply review packets. Early exact-object local reads can use existing gitcore without claiming the full serving/caching feature shipped. |
| forge-neutral/43, incremental indexes, todo | Defer optimization | Consider dependency-aware reuse only after duplicate collection is removed and measured. No cached approval/ready booleans. |
| forge-neutral/44, shared Git reuse trial, todo | Keep experiment separate | Feed measured packet/workspace cost into the trial. Its no-go result remains legitimate; no daemon or additional object-store project is justified by this spec. |
| forge-neutral/45, served cutover/fault qualification, todo | Fold service-path acceptance | Queue/context consumers join the eventual served fault/permission/coverage journey, with distinct read-only and publisher activation claims. Do not make this a prerequisite for the direct-reader bridge. |
| statusgen/15 and statusgen/evaluation | Preserve canonical owner | Eligibility/coverage meanings remain here. Do not port them into a queue CLI or perf report to avoid a dependency. Review/ready predicate promotion uses its existing policy owner and oracle. |
| build-less-brittle oracle, semantic owner and regression procedures | Apply existing method | Extract high-churn collection/classifier seams with intended-behavior fixtures; retain historical regressions. Static size/churn is nomination, not permission for a rewrite. |

## Adopter integration

The reusable public contracts cannot depend on a private adopter's stream or corpus. An
adopter using a desk-simplification program should make these bounded local amendments:

| Existing program responsibility | Amendment proposed after approval |
|---|---|
| Baseline/outcome meter and close-out | Preserve the original baseline; append a separately pinned matched workflow-cost cohort. Use shared diagnostic exports with their limitations, without replacing independent verified-outcome or maintenance-surface measures. |
| Library extraction, canonical fact/evaluation integration | Route actual importable review contracts through the shared owners. Do not implement review predicates twice to satisfy module placement. |
| Review role cutover | Add queue/planning/preparation consumer acceptance and watcher/report parity. Existing ready/verdict authority and fenced writer switch remain separate prerequisites; read-only fixes can ship earlier. |
| Worker and intake cutovers | Add role-specific issue projections and the actual consumer migration once qualified, preserving their distinct coverage and idle claims. |
| Coordinator cutover | Derive review depth from the shared projection instead of starting an independent full review sweep for each reporting call. |
| Consumer inventory and last-consumer retirement | Record replaced calls, subcommands, shared implementations, supported pins and wrappers. Retire the extra invocation before the command; remove a binary only after its last supported responsibility migrates. |
| Mirror adoption qualification | Add actual queue and issue-projection consumer journeys to scope/freshness/cost acceptance. No independent adopter mirror or early activation. |

## Remaining delivery gaps

Existing contracts do not by themselves deliver the complete single-observation queue and
planner input, the extraction of cmd-local classification/planning, cross-stage review
context reuse, or narrow issue projections. These need bounded implementation follow-ons
under the existing product owners. Do not claim they are already covered merely because a
brief title says query, migration or library.

The first dependency is a frozen observation/semantic preservation contract and local cost
attribution, not the full runtime. Then collection/extraction and pure planning can be
qualified against fake providers, followed by a compatible release and actual caller
migration. Persistent mirror/service integration follows its own existing dependencies;
publisher migration retains its fenced handover. Precise brief IDs, sizes and dependency waves are assigned below; active work was refreshed before authoring.

## Concrete caller and retirement coverage

| Surface | First consumers to migrate | What survives the review-only move |
|---|---|---|
| actions+prs+plan | `plugins/assay/skills/pr-review-desk/SKILL.md`, `tools/desk/cmd/reviewloop`, its fixtures and operator examples | `prs` remains for any independently inventoried consumer; legacy plan forms stay supported for the declared window. |
| classified review watcher and PR monitor | Review skill, `plugins/assay/scripts/pr-monitor.sh`, monitor-loop wrappers | Preserve independent wake/monitor-health responsibilities until their qualified replacement; do not delete a liveness check because queue data became richer. |
| review-depth reporting | `deskboard throughput`, `plugins/assay/scripts/assay-inbox.sh`, coordinator and desk capacity guidance | Dispatch/verify/intake populations and pool semantics remain their owners' meanings. A saved review observation cannot answer an unrelated newer reporting cutoff. |
| review preparation | `deskdispatch --kit review`, context/packet providers, review prompt/dispatch references | Worker/verifier dispatch, claims, attestation and required revalidation remain. |
| worker discovery | Worker skill, `deskboard dispatch/stalled/health`, issueboard consumers | Intake escalation and inbox views remain; an empty work-ready queue cannot stand for them. |
| intake observation | Intake skill and issueboard board/issues/intake callers | Local raw-intake parsing and its coverage remain independently useful. |
| release/install and external callers | `Makefile`, `.github/workflows/release.yml`, documented commands, supported version/pin inventory | Uninspected pins/cells/recipes are unresolved consumers, not evidence of zero use. |

Refresh this inventory at implementation and before deletion. A grep match nominates a
caller; inspect whether it executes, documents, tests or merely cites the surface. No
consumer, control or brief is retired by this authoring change.

## Authored delivery routing

All new units are M, strong tier. Existing L owners receive bounded contract/acceptance
amendments; these do not authorize a broad producer or profiler expansion. Additional
integrations require focused follow-ons if they exceed those seams. No completed brief or
Evidence is reopened. The numbering below is in the existing desk-tools stream.

| Brief | Scope | Depends on | Spec acceptance |
|---|---|---|---|
| desk-tools/37 | Independent behavior oracle and matched complete-cycle cost fixture | none | A1–A10 fixture foundation |
| desk-tools/38 | Legacy command/kit attribution and existing counter coverage | /37 | A6–A7 within perf-v1 coverage |
| desk-tools/39 | Actual pure classification/planning extraction and observation contract | /37 | A1–A3, A9 schema |
| desk-tools/40 | Complete ReviewQueue and offline PlanReview bridges | /38, /39 | A1–A3 |
| desk-tools/41 | Shared exact-source PrepareReview context | /38, /39 | A4, A8 |
| desk-tools/42 | Narrow work-ready/intake/inbox projections and actual discovery callers | /38 | A5, A9 |
| desk-tools/43 | Review skill, watcher, coordinator/inbox/depth migration and version rollback | /40, /41 | A3, A9 |
| desk-tools/44 | Complete-cycle qualification and bounded retirement record | /41, /42, /43 | A8–A10; all correctness criteria rerun |

| Existing owner amended | Concrete additional acceptance |
|---|---|
| performance-collection/01 | Phase/correlation schema and reducer, CPU coverage, attempts/points distinctions and no double count; A6–A7 |
| performance-collection/02 | Existing producer seams, versioned attribution and honest old-input gaps; A6–A7 |
| performance-collection/07 | Existing tool panel gains phase/endpoint and full-cycle comparison views; A6–A8 |
| forge-neutral/37–38 | Scope/coverage and event/poll repair for review and issue consumers; A2–A3, A5 |
| forge-neutral/39–40 | Legal public contract mapping and current admitted serving/custody; A1–A4, A9 |
| forge-neutral/41–43 | Context/effect boundary, exact-source keys and measured incremental invalidation; A3–A4 |
| forge-neutral/44 | Complete review-preparation cohort in existing Git trial; A4, A8 |
| forge-neutral/45 | Actual served review/issue consumer journeys plus installed-version/fault qualification; A1–A5, A9 |

Richer measurement remains owned by the performance-collection stream. Its
implementation/consent, DB/API and report availability are not prerequisites for local
cost qualification or early direct-reader shipping. No new collection policy is enabled.
The served path follows its existing runtime dependencies: /39 now requires the qualified
review contract (/39 in desk-tools); /45 requires compatible callers and cost qualification
(/43–44 in desk-tools). There is no inverse dependency from the early bridge to R6/service.

## Active-work checkpoints — 17 October 2026

- [#3248](https://github.com/medici-finance/assay/pull/3248) and
  [#3302](https://github.com/medici-finance/assay/pull/3302) are open board/dispatch CLI
  migrations. Their existing parser/config/stdout/exit acceptance is preserved. New semantic
  work coordinates with their selected main adapter at pickup, rather than expanding an
  active L or doing a second parser port.
- [#3325](https://github.com/medici-finance/assay/pull/3325), inspected at
  `12c36383ff5885bd54918939a64c55b3f50f041a`, is draft supervision/context guidance plus
  a Codex launcher reminder. It owns one routine watch owner, bounded handle waits and
  new-item context isolation. It supplies no runtime enforcement, queue/plan/context
  implementation or measured saving. /43 references its shared procedure if qualified;
  /37 and /44 separate tool-only, supervision-only and combined arms, recording installed
  plugin/launcher versions. It does not block /38–42 or justify deleting independent checks.
- [#3305](https://github.com/medici-finance/assay/pull/3305) is a pending, parked generic
  improvement/scorecard proposal. Its reusable reducer, experiment and quality-view work
  is not duplicated by the bounded /37 technical fixture. If merged, /44 consumes qualified
  output; an unmerged plan is not a producer dependency or activation decision.

Refresh these PR states and their source heads at implementation. No external task was
messaged and no active PR contract was silently changed by this authoring change.

## Perf-v3 consumer amendment — 19 October 2026

[Contract](performance-attribution-spec.md): perfrec /63–65 now owns the new local v3
instrument, typed CLI/env/launcher context and own-request timing/GraphQL points. Collection
retains aliased import, its event/spool/transport owner and its own admission state. Earlier statements
that richer measurement belongs only in collection apply to that transport contract, not to
this explicitly versioned local instrument. /62 and statusgen/26 adopt v3; collection/01–09
preserve identity/coverage/units into reports, without raw local locators in exports.
The source/consumer routing table in the contract classifies the remaining performance briefs;
completed implementations and their Evidence are not reopened.

## Candidate-plus-priority ownership reconciliation — 23 October 2026

Requested amendment at App-fetched main `9c4307a19`; source audit release v1.0.57.
The [planner contract and ranked call model](desk-workflow-efficiency-spec.md#21-candidate-plus-priority-planner--requested-amendment-23-october-2026)
extend existing delivery. No new stream/brief identity, second policy register or scheduler.

| Existing work | Reuse / bounded amendment | Consumer and retirement proof |
|---|---|---|
| desk-tools/39–40 | Complete review candidate observation + explicit policy/time/capacity pure plan; keep role evaluators canonical | Same plan value feeds prose, reactor and summary; do not make operation-key sort the slot priority. |
| desk-tools/42 | Issue projection input and enrichment/batching qualification | Work-ready, intake and inbox retain distinct complete populations; no global scan per root. Already shipped narrowing is baseline, not new savings. |
| desk-tools/43–44 | Actual caller migration and full-workflow parity/cost qualification, spec A11–A12 | Retire duplicate calls before code/commands; preserve producer/skill/schema compatibility, liveness and admission. |
| desk-tools/45,47,51 | Verification base classification, interrupted Evidence repair and skill lane integration | One complete multi-root candidate view, including owned Evidence repair; shared-blocker cache is per pass and final held-head checks remain. |
| graph-execution/27 | Five-role composition in the already-declared tools/desk-local deskclient adapter | Both prose and discrete clients consume the same role-qualified planner; Loop/reactor, claims, supervisor and bounded judgments keep their owners. |
| desktools-v2/09 and /20–21 | Reuse verified typed access-pattern queries; coordinate CLI adapters | Raw provider queries stay below Forge; parser migration does not create a ranker or take over policy. Preserve completed Evidence. |
| forge-neutral/37–43 | One ingestion owner, typed serving, exact-source reuse and invalidation | Share across windows only after scope/freshness qualification; include policy/time/capacity dependencies without fetching GH for a pure priority change. No fallback poller per caller. |
| Adopter desk-simplification and drain migration | Amend existing role cutover/parity and cleanup responsibilities locally | Do not recreate withdrawn core/policy plans, copy private plans into public Assay, or activate a parked program. |

The first independently useful head remains /37's oracle plus /39's shared pure contract,
then /40 and /43, qualified by /44. /27 composes the existing review/issue and verification
APIs. /45 supplies the canonical verification queue; /43 and /51 supply the actual review
and verifier prose adapters required by all-five-role parity. All three are reciprocal
prerequisites of /27: composition consumes their implementations instead of duplicating them.
This includes /51’s existing human gate; no earlier pure-contract acceptance waits on /27. R6 service dependencies remain on their own path;
read-only improvement does not wait on a credential/writer cutover. Active /42 work retains
its assigned revision until its owner acknowledges the bounded input/acceptance amendment.
