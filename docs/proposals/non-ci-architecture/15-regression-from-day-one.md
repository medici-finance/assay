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
| Audit/SQL budget history is lost | Unknown usage is not zero and does not refill authority; conservative hold/bound or an explicitly authorized reset, with retained evidence. |
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

These are additional tests to author with implementation. They are not claimed to run in this documentation task, and existing CI-only test plumbing is outside the architectural refactor scope except where needed to ensure the selected suite actually runs.

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
