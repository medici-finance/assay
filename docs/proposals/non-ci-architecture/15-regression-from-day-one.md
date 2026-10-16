# A regression floor from the first implementation

**Draft regression plan, 10 October 2026.** Reuse existing public tests and fixtures at the pinned revisions below, then run their semantic cases against the replacement. Preserve approved behavior rather than every old command, storage layout or known bug. These are qualification requirements, not a claim that the new system or its tests exist.

## What can move and what needs an adapter

| Existing source | Valuable behavior to preserve | Reuse approach |
|---|---|---|
| [statusgen coverage tests](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/statusgen/coverage_test.go) | Unavailable evidence is not a pass; advice cannot supply a witness; changed acceptance invalidates applicability | Extract contract cases/fixtures and run against the new evidence evaluator. Existing helpers call package-main internals, so not every test can simply be imported. |
| [coverage input tests](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/statusgen/coverage_inputs_test.go) | Relevant input changes hold; unrelated inputs and valid squash landings have their declared treatment | Preserve the semantic cases, replacing concrete Git/storage helpers only where needed. |
| [eligibility tests](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/statusgen/eligibility_test.go) and [offline reader tests](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/statusgen/forgeread_test.go) | Declarations affect admission; unknown holds; offline does not fabricate an empty answer | Shared evaluator/observation conformance cases used by both paths. |
| [runner conformance kit](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/loopadmin/runner/conformance/conformance_test.go) | Unknown launch, late result, cancellation, credential/identity/usage shape, strict decode, standing and workflow modes | Already structured around an adapter subject; exercise the same kit against the new real adapter, retaining fake/hostile controls. |
| [claim tests](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/tools/desk/cmd/deskclaim-ref/deskclaimref_test.go) | Live holder refusal, malformed claims, transport uncertainty and acknowledgment | Transfer the intent to PostgreSQL-backed coordination; replace Git-ref-storage-specific expectations. Add stale-generation release/publication cases where coverage is missing. |
| Forge mirror and event delivery | Failed reads preserve stale rows; partial results remain incomplete; event resume/dedup detects retention gaps | Build neutral backend-independent cases against PostgreSQL and fake provider transports. |
| Human/agent authority boundary | Human decisions and publisher credentials cannot be reached through agent surfaces | Exercise actual mounted handlers and process capabilities with authorized positive controls. |
| [graph export tests](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/statusgen/graph_test.go) | Determinism, edge meanings, stable references and read-only behavior | Retain fixtures through the new resolver; add cross-repo qualification, targets, transitions and missing-source coverage. |


## How to establish the suite without preserving accidents

Freeze a source revision and inventory each selected behavior with its current test, fixture, owning contract and disposition: reuse unchanged, adapt harness, replace because approved semantics changed, or add because missing. Preserve provenance when sanitizing private fixtures for public Assay. Do not copy private roster/cell data into the public test corpus.

Reuse the existing [testledger](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/tools/desk/internal/testledger/ledger.go) and `R-retires-test` contract: issue-linked `// regression:` tags, `Retires-test:` explanations and a reviewer-visible departure report. Public build-less-brittle/11 is already recorded implemented. Its report is advisory, not an automatic equivalence gate. For cross-repository moves, explicitly map old test/fixture/contract to its replacement; do not let deletion in the private source and addition in public appear unrelated. Include recently closed incident fixes as preservation obligations; a closed issue does not itself prove every installed consumer received the fix.

Require the test runner to identify the actual implementation/binary, selected test cases and asserted expected property. Zero selected tests, an oracle that self-skips new behavior, an unrelated static UI legend and a registry test that never exercises the served route are not sufficient evidence. Use a focused negative mutation plus the authorized/valid positive case at each critical seam. Pin both comparison revisions explicitly; a test against merged HEAD cannot silently compare HEAD with itself. These obligations do not authorize changing the accepted product behavior just to pass the old suite.

Use contract-level fixtures as the long-lived core, with adapters for old and new implementations. Run both on identical frozen facts; record expected intentional differences explicitly. Test results need to identify which implementation/version ran. A fake passes protocol conformance, not real process isolation or database recovery.

Retain raw, independently reviewed examples for critical wire/evidence formats. Some current coverage helpers generate evidence using the same implementation that reads it; that can hide a shared writer/reader mistake. Keep those tests, but add independent expected fixtures and plausible negative mutations at critical seams. Do not mass-regenerate golden files merely to make a new implementation pass.

The first implementation slice should establish four layers:

1. Fast pure contract tests using the reusable work/evidence/policy fixtures.
2. Real PostgreSQL and local bare-Git integration tests for transactional intent handling, canonical-record reconstruction and mirror queries, with fake forge responses and no production credentials.
3. A complete verifier journey against disposable processes/repositories, exercising actual custody boundaries, publication and recovery.
4. Migration/rebuild tests comparing old and new projections at named revisions and preserving historical evidence/IDs.

CLI parser or path-layout tests can retire with an explicitly retired command. Tests preserving semantic safety, uncertainty, custody or historical interpretation must have a replacement before the old path disappears. Use the existing test runners; do not create a new testing framework merely to unite the inventory.

## New obligations the old suite cannot prove

| Scenario | Required assertion |
|---|---|
| Worker release and reviewer model stamp share a PR | Release compares holder/role/lane/attempt/generation; a non-holder leaves the claim intact. Author and reviewer provenance remain separately queryable. |
| Forge reports an old approval against a new merge head | The independently bound reviewed candidate wins; unknown or mismatched review provenance holds every relevant lane. |
| Test selector matches nothing or invokes the old oracle | No acceptance PASS; record actual tested implementation and selected cases, and require the failing behavioral control. |
| A read route is mounted outside middleware | Actual served unauthenticated request is refused; the test must fail if the bypass is enabled, regardless of a correct route table. |
| Audit/SQL budget history is lost | Unknown usage is not zero and does not refill authority. A brake count holds or applies its declared conservative fallback; a pacing count seeds from retained counts and receipts within the [declared bound](11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed). Cases: §Operation-count qualification below. |
| Two cells are assigned the same external repository | Operator admission rejects overlapping mutating assignment before capabilities issue; disjoint assignments and authorized upstream reads still work. |
| Repository verifier cutover with an old direct publisher | Previously valid legacy capability/credential is refused at the real effect boundary; a shared credential is restricted or its affected scope stays paused. Queue suppression alone must fail the test. |
| Database deletion or backup rollback with a surviving old capability | Fresh authority incarnation/key plus complete execution-group teardown; old capability cannot admit effects even when its SQL generation reappears. Already-submitted effects are reconciled separately. |
| Cross-cell ownership transfer | Old credentials/capabilities lose authority before the new assignment activates; independent SQL stores never allow competing publication. |
| Notification restart versus full SQL loss | Restart preserves dedup/unknown-outcome records; unreconstructable pre-loss delivery holds rather than blind repost. Accepted repeated verification does not imply duplicate notification permission. |
| Database deleted completely | Git definitions/evidence and forge reads reconstruct canonical identities and obligations; missing transient state is explicit and safe work may repeat. |
| Git publication unavailable or its acknowledgment lost | Hold/reconcile the evidence publication; unrelated SQL coordination need not require a Git append. |
| Crash after SQL intent commit but before execution | With the DB intact, resume/reconcile the intent; after complete loss, rediscover the obligation and apply its safe-repeat policy. |
| Crash after provider effect but before receipt | Reconcile the recorded intent against the provider; ambiguous outcome holds the item. |
| Two controllers contend across DB failover | One valid ordered ownership transition; stale generation cannot authorize new work. Provider effects already in flight remain subject to the handover protocol. |
| Webhooks duplicate, arrive out of order or are missed | No duplicate work; newer state is not rolled back; reconciliation catches gaps. |
| Partial pagination, permission removal or failed polling | Coverage remains incomplete/unavailable; prior rows are visibly stale, never silently treated as an empty queue. |
| A graph query crosses target versions or inaccessible cells | No mixed-version answer or hidden-data leakage through paths, counts or explanations. |
| Local Docker state moves to CNPG | Compatible schema/extension set, validated recovery point, paused handover and no admission before reconciliation. |

These are additional tests to author with implementation. They are not claimed to run in this documentation task, and runtime regression plumbing remains in this lane. The separately scoped [CI extension](25-ci-tooling.md) owns CI library/action qualification and tool migration; it preserves this regression floor.

## Continuous-work and resource qualification

Add these cases to the existing runtime suite; they are future executable obligations, not results of this documentation change.

| Scenario | Required assertion |
|---|---|
| Feedback races attempt completion or enqueue | Durable reevaluation survives every crash boundary; duplicate delivery admits no second writer; later feedback is not cleared with the previous generation. |
| Superseded candidate receives distinct instructions | Preserve the attributed inputs and disposition; refresh/coalescing does not turn an old message into new authority. |
| Crash after worktree removal but before branch retirement | Resume the recorded remaining steps; preserve evidence and report partial completion without repeating unsafe effects. |
| Retirement races allocation or encounters a moved ref | Current owner/generation and expected tip win; the stale removal plan is refused. |
| Missing ownership rows, detached reviews, ignored evidence or ambiguous squash history | Unknown resources and required unique work are held; missing records never imply deletion permission. Include the merge-then-revert counterexample. |
| Scratch deletion partially succeeds or checkout/temp volume fills | Ownership remains recoverable; refusal precedes unsafe allocation/removal and records the affected resource without leaking private paths. |
| Repeated tool calls and a restart after a lost write acknowledgment | Reads can observe fresh state; guards record actions; durable effect reconciliation prevents unsafe replay across invocations. |
| Critic fails or agent confidently reports completion | Neither event grants authority, acceptance or ready status; required independent evidence still controls. |
| Concurrent knowledge updates and source retirement | Expected-version conflict is visible; provenance/required history survives; unapproved context cannot change a gate. |
| Maintenance overlaps foreground work or loses its process | Store ownership, actual Git compatibility and retained objects remain correct; interrupted work is diagnosable and safely resumable. |
| Review attempt completes after a crash-delayed teardown | The detached review head retires on its recorded class plus completed owning attempt; no pushed/merged proof is required or attempted, and a missing allocation record holds visibly. |
| Worktree created outside an allocation verb | Held with a visible classification reason; no age, prefix or cleanliness check retires it; classification precedes any removal. |
| Branch is idle past its threshold while its change record is open, or merged and not reverted | The open record holds the branch; the merged-and-not-reverted record with its owning attempt complete and required evidence retained retires it without an ancestry proof, provided the local tip is the merged head or its ancestor. |
| Local branch tip advanced past its merged change head, or the revert state cannot be determined | Held as unpublished or ambiguous work; the merged record never retires commits the merged head does not contain, and an undeterminable revert state holds. |
| Stamped lock whose session beacon stopped updating, and an unstamped lock past its declared window | The dead-session lock retires only on recorded evidence; the unstamped lock is checked and holds until recorded holder-gone evidence or an explicit recorded classification; an unreadable beacon or missing stamp holds rather than guesses, and age alone never retires a lock. |
| Boot sweep, interval supervisor and manual sweep overlap | The maintenance singleton serializes them; no resource is removed twice and a recent sweep is skipped rather than nested. |

Run a sustained allocate → execute → review → retire → recover workload against disposable repositories as well as the single verifier journey. Vary ref count, pack count, retained history, alternate stores and concurrency independently. Pin implementations and workload seeds; separate cold/warm caches and native-Git/go-git readers. Record p50/p95/p99 foreground latency, index/object opens, resource counts, held reasons, maintenance cost and peak temporary space before and after. Growth required by retention is expected; measure whether disposable resources retire and explain retained growth instead of requiring constant total disk use.

Declare workload-specific limits before evaluating the result. Include stopped owners, partial cleanup, failed refresh and low-space controls. A smaller ref count or successful maintenance exit is insufficient evidence of improved latency or correct retention. Future cold-storage support additionally requires successful reconstruction and negative missing/corrupt-segment cases; it is not a prerequisite to the first verifier.

## Authoring and plan-evolution qualification

Use neutral synthetic cases for the public shared schema/resolver and author skills. Adopters can retain additional source-pinned overlay checks separately. The public suite must be independently runnable. Use existing test runners and record the implementation/query/target versions actually exercised; do not present a manually reasoned diagnostic as a passing automated suite.

| Case | Executable acceptance discriminator |
|---|---|
| Existing owners, downstream consumers and delivered work | Discover and disposition relevant source-backed probes, inspect actual implementation/lifecycle, retain extra legitimate discoveries, and report unsupported or inaccessible coverage. Withhold expected identities from the author; do not hard-code a fixed expected-answer list into production discovery. |
| Same inputs, then an unrelated edit | Canonical inputs reproduce the receipt; an unrelated change triggers assessment but preserves unaffected conclusions and approvals. |
| New/deleted consumer outside the prior read set | Re-enumeration detects changed membership; a prose consumer without README/typed edges causes affected reconciliation despite unchanged old file hashes. |
| Required private input unavailable | Scoped could-not-check; independent authorized work continues and public output reveals no private content/counts. |
| Clean Git merge of competing owner proposals | Refuse incompatible combined promotion; an unapproved rival does not block existing target-aligned review or implementation. |
| Old-revision active worker | Observe a real fixture claim/checkpoint; refuse silent rebinding, then accept only acknowledged compatible adjustment or cancellation/reconciliation. |
| Delivered runner with populated Evidence | Preserve original lifecycle, source identities and Evidence; route the new obligation to an appropriate unfinished owner or linked follow-on. |
| Destination change with dated historical sources | A patch that applies and preserves Evidence still fails if it rewrites historical source/decision meaning. The positive amendment preserves the old record and separately adds new intent; an explicit historical correction retains reviewable correction lineage. |
| End-to-end spec-to-brief handoff | Both skills use the same target/contract/discovery binding; candidate patches implement their manifest, reconcile consumers/dependencies/Verify and protect history. A missing amendment fails even when the impact list says “amend.” |
| Bootstrap and activation phases | Authorized implementation can produce the required runtime proofs; dispatch/activation with missing prerequisites is refused. Manual bootstrap authority cannot silently become a permanent fleet exception. |

The source-preservation control needs nonempty historical regions in an unfinished brief as well as the delivered-brief routing case; an unchanged delivered file alone does not test edits inside history. Combine deterministic negative mutations with fresh end-to-end authoring samples. Record missed obligations, unjustified hits, review effort and unnecessary churn; a single known-corpus replay cannot establish broad reliability or net savings.

## Establishing the implementation baseline

Before the first code substitution, run and retain results for the selected source tests at an explicit baseline revision and for the replacement at its own revision. Record unavailable dependencies as could-not-check, not passing tests. The source inventory above uses public Assay `f9327a5954d57e786464e244a8e9c06f695d980d`; refresh affected source and plan state before dispatch.

At minimum, select the runner/conformance packages and the statusgen cases `TestCoverageCouldNotCheckIsNotPass`, `TestCoverageAdviceCannotSupplyWitness`, `TestCoverageAcceptanceDigestChanged`, `TestEligibilityCouldNotCheckHolds`, `TestOfflineReaderNeverAnswersEmpty` and `TestGraphJSONLDeterministic`. Preserve the behavioral cases when extracting package-main helpers. A fake runner passing conformance does not prove production isolation, PostgreSQL reconstruction or publication fencing.

The architecture PR validates document consistency and publication scope. Its checks are not the implementation regression floor. Each delivery PR records its actual selected tests, implementation identity, positive case and relevant failing control.

## Progressive adoption and oversight qualification

Use the [adoption acceptance corpus](19-progressive-adoption-and-oversight.md#adoption-acceptance-and-architecture-qualification)
through actual producer and consumer boundaries. Add these obligations to the owning
regression ledger; no new test or public interface is claimed by this document.

| Boundary | Required failing control / positive case |
|---|---|
| Offline consumer and installation | Cold fixture inspection without ambient home/credentials/database/source checkout; interrupted setup preserves authored files and a bad pin holds |
| Evidence and assurance | A passing self-check substituted for an independent witness fails; unchanged canonical facts agree through offline and managed consumers |
| Policy and effects | Worker-controlled preset/env cannot raise authority; protected scope remains held under a low-touch preference, even with a permissive fake provider |
| Human decision and recovery | Expired/changed batch subject and reconstructed unknown consumption cannot authorize; unrelated still-valid tuples retain only their actual grant |
| Sampling and attention | Reroll/split evasion fails; eligible/sample/unknown denominators remain visible; quiet hours or exhausted attention budget do not answer a gate |
| Measurement and tuning | Incomplete score or reduced detection cannot enact a grant; human minutes, required-check coverage and independently sampled defects remain counter-metrics |

Qualification reports actual selected policy/contract/implementation revisions and
capability evidence. A schema-valid preset or a higher score is not a passing runtime
case. Measure first useful outcome and independent repeat use, including support and
repair effort. Read-only release acceptance is independent of managed writer activation.

## Write-through and source-cache qualification — 14 October 2026

Carry the [forge-service amendment](22-forge-service-and-source-queries.md) into the same
regression ledger. Exercise provider success followed by SQL failure, death after send,
conflicting operation payload, stale pre-write refresh, caller loss of a reply, and receipt
lookup after authorization revocation. Unknown does not replay; read-after-write requests
show a confirmed/newer observation or explicit pending coverage. Test cached bodies, counts,
errors and pagination against denied admissions and access epochs. Bypass ingress to prove
the role executor refuses escalation, stale incarnation and wrong candidate independently.

Source-query tests use disposable bare Git and independent literal fixtures at exact SHAs.
Changing a branch tip cannot change a cached old-SHA answer; private object reuse still needs
repository authorization. Compare incremental derived results with a fresh oracle after
membership, time, policy and permission changes. Burst/outage tests assert the configured
upstream request budget, while shared-Git experiments report native-Git/go-git latency, CPU,
bytes, retention and alternate-store recovery. No fixture result certifies live provider
permissions, a deployed identity provider or CNPG failover.

## Communications regression extension (proposed; pending approval)

Proposal against the ratified revision `23f655f5d`; not baseline text until approved and reconciled in the amendment manifest.

Preserve the commsgw bypass/identity/mailbox suites, commsloop policy/native-dispatch
suites, deskcomms command/peer-custody suites and cellctl comms manifest/lifecycle
suites through the [communications extraction](24-communications.md). The initial
proof is behavior parity plus a real external SDK consumer, not distributed delivery.
The specification's acceptance cases govern later two-cell outages, filter crashes,
retries, quota/expiry, authority bypass, SQL-outbox handoff and state migration.

## Operation-count qualification — 15 October 2026

These cases qualify the
[brake and pacing contract](11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed).
Each brake case bypasses the pacing path, so a correct pacing limiter cannot mask a
fail-open brake. C is the number of concurrently admitting processes.

| Scenario | Required assertion |
|---|---|
| A brake count is unavailable, unreadable or unverifiable | Per-target and per-repository write ceilings, daily and new-issue caps, the circuit breaker, the child-launch budget, per-caller ingress rate limits and retry budgets each hold, or apply their declared conservative fallback, and admit no effect. Treating the unavailable count as under budget must fail the test. |
| Concurrent pacing-count checks use the same estimate | Overshoot is at most one admission per concurrently admitting process; configured pacing still operates with no global lock across network work and no exact budget-slot reservation. |
| A pacing count's shared counter is lost mid-window | The shared fallback estimate is seeded from the last retained count and, for an effect-class count, the window's recorded effect intents and receipts, never zero. Coverage is reported unavailable rather than measured zero; counter failure alone neither holds publication nor requires an operator reset. |
| Repeated restart or per-process fallback within one window | For an effect-class pacing count, K restarts or N processes do not multiply the configured cap: the fallback estimate is shared, so admissions by other processes after the seed are counted. Within one operator incarnation, total admissions stay within cap + C, or 2 × (cap + C) after the single recorded unseeded reset. A second unseeded reset, or an unreadable reset record, treats the window as exhausted. Starting any fallback estimate at zero, or a per-process estimate that ignores other processes' admissions after the seed, must fail the test. Read-class pacing counts are outside this bound by contract; they must still seed from the last retained count, never zero. |
| Idempotency, approval or unknown-effect history is lost | Reconstruct from retained evidence or hold the affected effect. The pacing tolerance cannot authorize duplicate publication, clear an unknown outcome or recreate approval. |
