---
brief: assay:assay:desk-tools:38
title: Correct desk performance attribution and counter coverage
why: "Ordinary desk operations collapse to other and some subprocess/HTTP paths escape existing counters, hiding the cost of the behavior being optimized."
wave: 2
depends: ["desk-tools/37"]
unblocks: ["desk-tools/40", "desk-tools/41", "desk-tools/42"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 3
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Correct desk performance attribution and counter coverage'
id: fb4b42dd-6877-4b9e-947f-09f9f4b8a201
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
consumers:
  - "tools/desk/cmd/deskperf/: out-of-scope (unchanged reader accepts additive verb strings; exercised by existing command suite and LoadPerfHistory flow)"
  - "tools/desk/internal/deskkit/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/cmd/deskboard/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/cmd/deskdispatch/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/cmd/issueboard/: fixed-here (production attribution/counters and regression paths)"
  - "tools/desk/internal/reviewtest/perf_test.go: fixed-here (actual entrypoints, optional sink and planted production mutations)"
  - "tools/desk/internal/deskkit/perf-mutations.json: fixed-here (retain the argv-leak mutation at the changed production record construction)"
  - "docs/review-efficiency-qualification.md: fixed-here (production entrypoint acceptance and honest remaining coverage)"
  - "docs/telemetry.md: fixed-here (closed vocabulary, inventory and measurement limits)"
  - "tools/desk/cmd/deskread/: out-of-scope (existing reader/parser is unchanged; shared process/client helpers gain counters without caller changes)"
  - "tools/desk/cmd/deskwt/: out-of-scope (existing hook caller and process policy unchanged)"
  - "tools/desk/cmd/deskroster/: out-of-scope (existing liveness/visibility readers preserve results; shared transport now observes attempts)"
  - "tools/desk/cmd/verifyloop/: out-of-scope (existing verifier Git caller and policy unchanged)"
  - "tools/desk/cmd/{cellctl,deskautolane,deskclose,deskevidence,deskflip,deskgit,deskinbox,deskmerge,deskmonitor,deskpathguard,deskpost,deskrun,desksupervise,deskverdict,scanloop}/: out-of-scope (class sweep: shared helper counters apply; caller policy, argv and parsing contracts are unchanged)"
  - "perfrec/: out-of-scope (closed schema and offline writer unchanged)"
---

# Brief 38 — Correct desk performance attribution and counter coverage

## Context

files: `tools/desk/internal/deskkit/{perf,runtool,forgetransport,roletoken,verifierattestation,verifiertrackingcheckout,hooks,killswitch,dispatchrecord,callout,untrustscan,riskcallout,layaadvisor,trustliveness,trustliveness_gitlab,repovis}.go` (existing), `tools/desk/internal/deskkit/{perfprocess,perfhttp,citrigger}_test.go` (new/existing), `tools/desk/internal/deskkit/perf-mutations.json` (existing), `tools/desk/cmd/deskboard/{board,dispatch,nextup}.go` (existing), `tools/desk/cmd/deskboard/{perfchild,perfleaf,perfentry}_test.go` (new), `tools/desk/cmd/deskdispatch/{dispatch,stamponly,kits}.go` (existing), `tools/desk/cmd/{deskdispatch,issueboard}/perfentry_test.go` (new), `tools/desk/internal/deskkit/perf.go` (existing), `tools/desk/internal/deskkit/perf_test.go` (existing), `tools/desk/internal/deskkit/perfcoverage_test.go` (existing), `tools/desk/cmd/deskboard/main.go` (existing), `tools/desk/cmd/deskdispatch/main.go` (existing), `tools/desk/cmd/issueboard/main.go` (existing), `tools/desk/internal/reviewtest/` (planned), `docs/telemetry.md` (existing), `docs/review-efficiency-qualification.md` (existing), `changelog/desk-tools-38.md` (planned)

facts: 17 October 2026 at the source pin above. Version 2 expands the declared paths to the actual shared process/client observers and test-only entrypoint adapters; no execution policy or perf-v1 field changes. Version 3 retains the existing argv-leak mutation at its current source anchor and strengthens row 2 with successful ordinary-entrypoint work, nonempty audit history and actual budget/breaker reader observations plus production mutations. desk-tools/37 is merged through PR #3331 and integrated with a two-parent main merge. The deskboard CLI port #3248 merged at pickup and is integrated at its actual recording callback; its help/version/parse refusals keep their no-sample contract. The deskdispatch port #3302 remains open; preserve its parser acceptance separately. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 is merged and #3302 remains an active CLI port; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: deskkit PerfRun and existing subprocess/transport counters. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: deskkit PerfRun and existing subprocess/transport counters
  contract: perf-v1 closed-field schema and existing consent; performance-collection/01–02 own richer events
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text +0; existing counters only
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Task

1. Correct command vocabulary and actual dispatch-kit attribution at the production wrappers.
   Preserve perf-v1 fields and readers. If a kit needs a new enum value, demonstrate old-reader
   compatibility and document its mapping; do not add raw argv or infer kit from role. Unknown stays
   explicit.
2. Inventory process launches and forge transports used by the three tools; route bypasses through
   the existing shared runner/counter where semantics allow. Preserve child error/exit handling and
   redaction from desk-tools/21. Document uncovered paths and own-process CPU limitations. Never
   relabel request counts as GraphQL points.
3. Do not add a new spool, daemon, remote sink, listener or legacy schema fields. Phase/correlation
   and quota distinctions belong to performance-collection/01–02; annotate what cannot yet be
   reconstructed. Preserve desk-tools/23 Evidence and /24–27 delivered savings.
4. Reconcile actual main with active CLI ports #3248 and #3302 at pickup. Keep parser/stdout/exit
   compatibility and add attribution tests on the selected adapter; do not expand those ports or
   supersede their acceptance.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCommandKit$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCommandKit ' "$o")` | exit 0; named PASS. Real entrypoints record stable allowlisted subcommands/kit mappings, including aliases/help/error forms; unknown/private argv never leak. Existing perf-v1 fixtures decode unchanged. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCoverageFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCoverageFlow ' "$o")` | exit 0; named PASS. Production runner and transport record planted child/HTTP attempts, retry and 304 exactly once on instrumented paths; consent-off and broken optional sinks preserve exit/stdout/work/audit/budget. Successful file-list, intake and stamp entrypoints preserve seeded history, append exactly one successful audit record and leave the same real budget/breaker decisions; skipped-work and erased-history production mutations fail these assertions. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/38` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

Historical record — retained as quotation, without current certification.

> > Historical record (superseded for closure while provenance is reviewed):
> > 
> > Historical host run — superseded by the admitted Linux run below; retained as quotation.
> > 
> > > <!-- Independent verifier appends one row per Verify command after implementation. -->
> > > **VERIFY: FAIL** — environment-blocked / could-not-check; no implementation failure proven and no certification. The mandatory execution witness returned exit 2 before every Verify row: the required Linux network-off sandbox is unavailable on the Darwin host. This is a runner capability limitation, not an observed implementation failure. No named acceptance test executed in this witness.
> > >
> > > | # | Command | Result | Output | Date | Runner |
> > > |---|---------|--------|--------|------|--------|
> > > | 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCommandKit$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCommandKit ' "$o")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4 (on-behalf-of human:ian) (forge-identity) |
> > > | 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCoverageFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCoverageFlow ' "$o")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4 (on-behalf-of human:ian) (forge-identity) |
> > > | 3 | `statusgen --root . --consumers --brief desk-tools/38` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4 (on-behalf-of human:ian) (forge-identity) |
> > > Verification-Attestation: medici-finance/assay#3336 run=616125540e6d554df4b3278c1ff15cd3a77acc1db20bdd66c012e6b23660535e source=8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae brief=docs/streams/desk-tools/brief-38-correct-desk-performance-attribution-and-counter-coverage.md model=gpt-6.1-sol tier=strong
> > >
> > >
> > > Independent source review at 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae supports the stated attribution and shared-counter routing, subject to the unavailable mandatory execution witness and the consumer-check limitation below. Additive verb strings remain accepted by the unchanged perf-v1 reader. This review does not replace execution evidence.
> > >
> > > RISK-VALUE: DERIVED — perfKitVerb.unknown = "other" @ tools/desk/cmd/deskdispatch/kits.go:127 — the brief requires explicit unknown attribution without private argv; a fixed fallback carries no caller text.
> > > RISK-VALUE: DERIVED — perfActive.start = true @ tools/desk/internal/deskkit/perf.go:120 — selection must be enabled only while the already-consented invocation executes; the deferred false at line 121 ends its lifetime even when returning.
> > > RISK-VALUE: DERIVED — perfActive.finish = false @ tools/desk/internal/deskkit/perf.go:121 — a completed invocation must not retain an active selector for later work.
> > >
> > > Risk-path trigger: topology.yaml lines 203–206 classify the changed deskkit and deskboard paths. The frontmatter's all-no risk answers do not remove that path classification. Those triggers require the literal derivation/security-review obligations; they do not rewrite this brief's gate:model/all-no frontmatter into a human gate. This run provides no certification.
> > >
> > > The stream README still lists brief 38 as todo at the verified source despite delivery merge eded7ba6e4461a3afac436ff7196987d83268bb3. Its lifecycle cells were not edited during verification.
> > >
> > > Runner capability follow-up: [assay#1800](https://github.com/medici-finance/assay/issues/1800). A supported runner must retain this exact run's admission and immutable source closure before a new result can certify the rows.
> > >
> > > Supplemental local diagnostics, independently rerun at the attested source after resolving the guard's output-target ambiguity with literal absolute temporary paths:
> > >
> > > - Row 1's Go selection and named-PASS assertion: exit 0, `--- PASS: TestPerfCommandKit (8.07s)`; full inner output sha256:b9854bb764bdf901b657c145200aeac6bf27ee8c2d808fa333478cd1ac618e89. The actual-kit-assignment production mutation was rejected.
> > > - Row 2's Go selection and named-PASS assertion: exit 0, `--- PASS: TestPerfCoverageFlow (16.08s)`; full inner output sha256:d19d1da18d53aafc283d07fb4ca743aa0c7844f38565b36b199d0d14d54f4132. Ordinary deskboard/issueboard/deskdispatch work, skipped-work/audit-erasure/local-refusal mutations, production raw-process paths and account transports passed their assertions.
> > > - Row 3's consumer command, with --root spelled as its equivalent admitted absolute directory: exit 2, `COULD-NOT-CHECK: assay:assay:desk-tools:38 is not in the diff against 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` The installed command requires a delivering branch-head/merge-base diff to corroborate a merged brief. The admitted source was not changed to obtain that result.
> > >
> > > These local diagnostics do not supply the required Linux network-off execution witness and do not turn its three could-not-run rows into passes. No product behavior failure was observed; the consumer row also has a post-merge invocation limitation.
> > 
> > **2026-10-18 independent Docker follow-up**
> > 
> > 
> > The driver approved a temporary container seccomp exception. The immutable local runner image was sha256:91886f2fd3fbc5acc840d84d5b44a53107fefa6f12bb2a6c618e0eb1d76c23d2, with the original home/source and Git metadata retained. The runner changed only the approved seccomp profile; the mandatory witness independently used `unshare --net --map-root-user` for every check:ci row. Container admission passed before execution and again after all checks, with the original attestation below. Only the brief's Evidence changed; the source stayed at 8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae.
> > 
> > | # | Command | Result | Output | Date | Runner |
> > |---|---------|--------|--------|------|--------|
> > | 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCommandKit$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCommandKit ' "$o")` | pass exit=0 sandbox=unshare | sha256:3bcb67446e29 | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4+dirty (on-behalf-of human:ian) (forge-identity) |
> > | 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCoverageFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCoverageFlow ' "$o")` | pass exit=0 sandbox=unshare | sha256:73a3f1219f83 | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4+dirty (on-behalf-of human:ian) (forge-identity) |
> > | 3 | `statusgen --root . --consumers --brief desk-tools/38` | pass exit=0 sandbox=unshare | sha256:3d75f17e3e49 | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4+dirty (on-behalf-of human:ian) (forge-identity) |
> > Verification-Attestation: medici-finance/assay#3336 run=616125540e6d554df4b3278c1ff15cd3a77acc1db20bdd66c012e6b23660535e source=8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae brief=docs/streams/desk-tools/brief-38-correct-desk-performance-attribution-and-counter-coverage.md model=gpt-6.1-sol tier=strong
> > 
> > 
> > Rows 1 and 2 exited 0 after their exact named-PASS assertions. The production entrypoint/kit mappings, unchanged reader decoding, ordinary work/audit/budget preservation, process/HTTP attempts and planted production mutation assertions therefore executed under the required network-off witness. The existing own-process CPU, descendant/endpoint/retry/quota unknowns remain limits of the dataset.
> > 
> > Row 3 needs a separate interpretation: its exit was 0, but the actual captured output reported **0 corroborated, 0 disproved, 15 unchecked, 0 briefs claiming nothing**, against the already-merged source. Every declaration was byte-identical at that base. The command explicitly says unchecked entries are not passes. The Evidence-only working-tree change made the brief selectable; it did not supply an implementation diff. No base or source override was used to obtain this output.
> > 
> > I independently reviewed delivery merge eded7ba6e4461a3afac436ff7196987d83268bb3 against its first parent, alongside the current admitted source. The eight fixed-here declarations match the actual delivery diff:
> > 
> > | Declared consumer site | Independent delivery-diff corroboration |
> > |---|---|
> > | tools/desk/internal/deskkit/ | 19 changed paths: attribution, raw process observers, transport placement, shared callers, regression tests and mutation anchor |
> > | tools/desk/cmd/deskboard/ | 7 changed paths: parsed callback/aliases, four raw-output sites and production test adapters |
> > | tools/desk/cmd/deskdispatch/ | 5 changed paths: actual parsed kit selection, compiled mapping and main-entrypoint fixture |
> > | tools/desk/cmd/issueboard/ | 2 changed paths: selected handler attribution and main-entrypoint fixture |
> > | tools/desk/internal/reviewtest/perf_test.go | Changed actual-entrypoint, sink-preservation and production-overlay acceptance |
> > | tools/desk/internal/deskkit/perf-mutations.json | Changed argv-leak mutation anchor, retained its original purpose |
> > | docs/review-efficiency-qualification.md | Changed qualification scope and honest remaining coverage |
> > | docs/telemetry.md | Changed closed vocabulary, counted-attempt semantics and explicit measurement limits |
> > 
> > I also checked all seven out-of-scope declarations against that delivery diff: no path under deskperf, deskread, deskwt, deskroster, verifyloop, the enumerated command class, or perfrec was changed. Their exclusions are justified by their unchanged parser/policy/schema contracts and the shared helpers' observation-only substitutions. The unchanged reader decodes verb strings without an enum gate; new dispatch:/stamp-only: strings remain ordinary non-step records. Direct process helpers preserve the original command object and error, and copied counting clients preserve caller-owned transports. The mandatory production-path assertions now pass. This is the independent review satisfying the row's implementation-diff and out-of-scope obligations; it is not a claim that the automatic consumer command corroborated those 15 declarations.
> > 
> > Risk-value enumeration and derivations from the initial review remain applicable to the unchanged source. In particular:
> > 
> > RISK-VALUE: DERIVED — perfKitVerb.unknown = "other" @ tools/desk/cmd/deskdispatch/kits.go:127 — Task 1 requires explicit unknown attribution without private argv; this is a fixed fallback with no caller text.
> > RISK-VALUE: DERIVED — perfActive.start = true @ tools/desk/internal/deskkit/perf.go:120 — parsed attribution is selectable only while the already-consented PerfRun executes after resetting its selected value.
> > RISK-VALUE: DERIVED — perfActive.finish = false @ tools/desk/internal/deskkit/perf.go:121 — the deferred reset ends that interval on return.
> > 
> > The path-trigger derivation/security-review obligations do not rewrite the brief's gate:model/all-no frontmatter into a human gate. Existing collection consent and activation remain separate. The README's todo discrepancy remains recorded; this verifier made no lifecycle or README edit. The broader host-runner capability issue remains independently tracked by [assay#1800](https://github.com/medici-finance/assay/issues/1800); this explicitly approved local container run does not claim a fleet-wide runner fix.
> > 
> > **VERIFY: PASS** — mandatory rows now executed with `sandbox=unshare`; consumer routing is accepted through the explicit delivery-diff/source review below, not through the command's unchecked entries. This supersedes the earlier host-only environment hold for this source and run; the historical host result remains recorded.
> > 
> > Review follow-up — 2026-10-18, assay-verifier-app[bot].
> > 
> > The earlier record is preserved above as historical quotation. Its tool-emitted fingerprints do not make the original underlying row stdout/stderr available: the first Docker attempt did not retain those bytes. A new execution cannot authenticate an earlier fingerprint. The retained original reports, literal inventory, rederivation, supplemental outputs and admission logs have been preserved separately for inspection.
> > 
> > An independent verifier has completed a separately identified new attempt with captured row output, exact fingerprint comparison and genuine admission before execution and after normal credential recovery. This observed runtime result is not yet an accepted verification: access to primary artifacts and the authorization record remains under review. The earlier final admission failure from an expired cached credential is retained beside the later successful recovery check; no source or sandbox rule was changed to pass it.
> > 
> > The brief remains implemented, without a Verified stamp, until the provenance findings are independently cleared. No prior or new execution summary substitutes for inspectable primary proof. Publication of the prepared restricted artifact index awaits the driver's authorization; no restricted payload was posted after the publication refusal.
> > 
> > The separate frozen-admission-history outcome-writer defect remains tracked in #3343. It does not resolve the primary-artifact review finding.
> > 
> > Admission binding (pre-work dispatch provenance only; not an accepted verification result):
> > 
> > Verification-Attestation: medici-finance/assay#3336 run=616125540e6d554df4b3278c1ff15cd3a77acc1db20bdd66c012e6b23660535e source=8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae brief=docs/streams/desk-tools/brief-38-correct-desk-performance-attribution-and-counter-coverage.md model=gpt-6.1-sol tier=strong
> > 
> > **VERIFY: BLOCKED** — provenance review and artifact-access approval pending.
>
> Primary-artifact audit follow-up — 2026-10-18, independent non-implementer inspection.
>
> **2026-10-18 independent inspection of retained execution evidence**
>
> A non-implementer has now inspected the authorized primary approval and execution artifacts for the fresh captured attempt at source `8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae`. All 70 final-manifest entries match their sizes and full SHA-256 values. All 9 namespace recorder records agree with raw byte counts, hashes, exact argv and zero exits, without capture errors; six are isolation/helper probes and three are the authored rows. This is an audit of the recorded attempt, not a new execution or model stamp. The original first-Docker attempt's discarded row streams remain unavailable and are not reconstructed by this audit.
>
> The original human approval explicitly covers the temporary disposable seccomp exception while every check remains network-isolated. Pre-admission and the separately recovered final admission bind to the exact same original run/source/brief/plan/home. The first final-admission failure and launcher exit 5 remain preserved; normal custody recovery later succeeded without rerunning acceptance rows or altering the source. The recorder invokes real unshare and its qualification covers exact combined bytes, nonzero exit, refusal, child signal and isolated namespace, with fail-closed capture storage. Harness tool versions/build hashes are separately recorded from the tested source.
>
> Recorded row 1: exit 0, `--- PASS: TestPerfCommandKit (7.85s)`, raw SHA-256 f438799207e68f585599a51e14d235eef4725045617e1f6b74062a5c12322872. Recorded row 2: exit 0, `--- PASS: TestPerfCoverageFlow (11.68s)`, raw SHA-256 8af17bf2e8dc2252d0639c7c6fee9e185dcd6df199abee911d055e4434f7ff01. Recorded row 3: exit 0, raw SHA-256 3d75f17e3e49f52355b5184f039ba303c86550ffe9c75d50fc3ccfbff78180c3; its explicit result remains 0 corroborated / 0 disproved / 15 unchecked. That is not automated corroboration. Public excerpts are separately identified and do not inherit a redacted raw artifact's digest.
>
> Independent inspection of delivery merge eded7ba6e4461a3afac436ff7196987d83268bb3 against its first parent confirms all 8 fixed-here sites in its diff and all 7 out-of-scope categories unchanged there. Source review supports the unchanged reader/caller-policy exclusions and observation-only shared process/client substitutions, alongside the recorded production-flow assertions. This supplies the stated manual routing review; it does not relabel unchecked entries as automatic passes or claim every caller regression suite ran. Existing measurement limits remain.
>
> Relevant tool/source, reader/schema, brief, README and qualification/telemetry document bytes are unchanged between admitted 8f and observed main 06587d38fc8195bde31098616f7143f287a06baa. The execution claim remains bound to 8f.
>
> The primary approval/artifact-access hold is resolved for this inspection.
>
> The driver authorized the restricted evidence index and it was posted for the authorized reviewer audience. A separate non-implementer has now inspected the primary execution bytes, manifests, recorder qualification and original approval excerpt. Artifact access and provenance are established for this fresh attempt; the normal PR review remains independent. No additional Verify command was run for this audit.
>
> Captured native execution witnesses for the separate fresh attempt:
>
> | # | Command | Result | Output | Date | Runner |
> |---|---------|--------|--------|------|--------|
> | 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCommandKit$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCommandKit ' "$o")` | pass exit=0 sandbox=unshare | sha256:f438799207e6 | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4+dirty (on-behalf-of human:ian) (forge-identity) |
> | 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCoverageFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCoverageFlow ' "$o")` | pass exit=0 sandbox=unshare | sha256:8af17bf2e8dc | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4+dirty (on-behalf-of human:ian) (forge-identity) |
> | 3 | `statusgen --root . --consumers --brief desk-tools/38` | pass exit=0 sandbox=unshare | sha256:3d75f17e3e49 | 2026-10-18 | assay-verifier-app[bot] @ 8f1206fe5eb4+dirty (on-behalf-of human:ian) (forge-identity) |
> Verification-Attestation: medici-finance/assay#3336 run=616125540e6d554df4b3278c1ff15cd3a77acc1db20bdd66c012e6b23660535e source=8f1206fe5eb4b4ab19f5f2f7266c515aa71f8eae brief=docs/streams/desk-tools/brief-38-correct-desk-performance-attribution-and-counter-coverage.md model=gpt-6.1-sol tier=strong
>
> Exact command-output excerpts for the named-PASS assertions (complete row streams, not retained inner verbose test logs):
>
> Row 1: 37 bytes, SHA-256 `f438799207e68f585599a51e14d235eef4725045617e1f6b74062a5c12322872`.
>
> ```text
> --- PASS: TestPerfCommandKit (7.85s)
> ```
>
> Row 2: 40 bytes, SHA-256 `8af17bf2e8dc2252d0639c7c6fee9e185dcd6df199abee911d055e4434f7ff01`.
>
> ```text
> --- PASS: TestPerfCoverageFlow (11.68s)
> ```
>
> The consumer row contains a configuration-bearing prefix in its raw stream. Its complete raw bytes remain restricted and were independently hashed; public excerpts are separately redacted and do not claim the raw digest. The raw consumer output SHA-256 is `3d75f17e3e49f52355b5184f039ba303c86550ffe9c75d50fc3ccfbff78180c3`; the actual summary is 0 corroborated, 0 disproved, 15 unchecked. Exact delivery merge `eded7ba6e4461a3afac436ff7196987d83268bb3` against its first parent corroborates the eight fixed-here sites. All seven out-of-scope groups remain unchanged there and their reader, policy and schema exclusions were checked against source. These are independent semantic judgments; no unchecked tool result is counted as automatic corroboration.
>
> **VERIFY: BLOCKED** — all three rows and bounded expectations have observed passing results with inspected primary proof. Formal completion remains blocked by the frozen-history outcome writer defect [#3343](https://github.com/medici-finance/assay/issues/3343) and normal independent PR disposition. Lifecycle remains implemented with no Verified stamp. The audit supplies no new execution stamp, ready flip, merge or operational activation.

Fresh independent verification — 2026-10-18.

The earlier primary-artifact certification claims are withdrawn: the temporary raw execution artifacts referenced above are no longer available for review. Their excerpts and hashes do not establish inspectable execution provenance. This new run does not authenticate or reconstruct those discarded bytes. Historical outcome records remain unchanged.

### Independent verification — 2026-10-18

Observed source: a944c61762c50835f4fdd40ccc7785a51317f354. Actual delivery: merged PR #3332, eded7ba6e4461a3afac436ff7196987d83268bb3. Grounding used its original brief and approved spec before reading delivery diff/tests. A fresh genuine dispatcher admission preceded execution and was repeated through the host admission bridge; no cached receipt substituted for a check. The disposable runtime had network disabled; each row also used the actual network-namespace sandbox. All 6,144 tracked file bytes matched the admitted host source manifest (SHA-256 53582c00eec951f81caaf392a60297b73cb4a57f7ae558b458eee23f6bc669c4).

Verification-Table: delivery=eded7ba6e4461a3afac436ff7196987d83268bb3 base=da7a4be21e61acf2f8a2c09749f8cec7d60468f1 original=da7a4be21e61acf2f8a2c09749f8cec7d60468f1

Protected-table rederivation used the actual delivery parent: three original rows, zero author-added. The original Command, Expect and class govern; later added row-2 prose earns no separate acceptance credit. Original row 1 requires finite operation/actual-kit attribution and unchanged perf-v1 decoding; row 2 requires once-per-attempt production counter coverage and preserved optional-sink operational effects; row 3 requires actual consumer corroboration with exit 0.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCommandKit$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCommandKit ' "$o")` | pass exit=0 sandbox=unshare | sha256:7afa5ccb6b1e | 2026-10-18 | assay-verifier-app[bot] @ a944c61762c5 (on-behalf-of human:ian) (git-config) |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfCoverageFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestPerfCoverageFlow ' "$o")` | pass exit=0 sandbox=unshare | sha256:df74ddf40efe | 2026-10-18 | assay-verifier-app[bot] @ a944c61762c5 (on-behalf-of human:ian) (git-config) |
| 3 | `statusgen --root . --consumers --brief desk-tools/38` | fail exit=2 sandbox=unshare | sha256:8eb091b3f34f | 2026-10-18 | assay-verifier-app[bot] @ a944c61762c5 (on-behalf-of human:ian) (git-config) |
Verification-Attestation: medici-finance/assay#3423 run=0dc9b2dcabc4649e25e83ab4617271ff21d93a9401525f587ece1944d99635fb source=a944c61762c50835f4fdd40ccc7785a51317f354 brief=docs/streams/desk-tools/brief-38-correct-desk-performance-attribution-and-counter-coverage.md model=gpt-6.1-sol tier=strong

Observed primary output by original Verify row:

- Row 1: exit 0; `--- PASS: TestPerfCommandKit (17.67s)`. Full Go output contains the real main-entrypoint mapping cases and the production kit-assignment mutation. Combined row output SHA-256 7afa5ccb6b1e929798ed0cfc3219779079b92ee3cb3f2573956ed80006955833.
- Row 2: exit 0; `--- PASS: TestPerfCoverageFlow (29.49s)`. Full Go output contains successful ordinary deskboard, issueboard and deskdispatch work, plus production skipped-work, erased-audit-history and local-refusal counter mutations. The accepted test also requires the named process, account-transport, statusgen-child and leaf-transport cases. Combined row output SHA-256 df74ddf40efefb3387f99c3ff9f78e4666cba491cc98aa5d7ec775581434eb8a.
- Row 3: exit 2; `statusgen: --consumers: COULD-NOT-CHECK` because the merged brief is not in the clean diff against a944c61762c50835f4fdd40ccc7785a51317f354. The tool states no entry was corroborated or disproved. The isolated runtime also emitted a missing-roster warning; both messages are retained in primary stderr. Combined row output SHA-256 8eb091b3f34fbb956245b0815dffef61e622ea0215d035930f6b04587ed5d946. This is a could-not-check result, never acceptance success or proof of an implementation defect. Tracked by [#1915](https://github.com/medici-finance/assay/issues/1915); canonical rebaseline refused behaviour-changed, so this run does not rewrite the row.

Independent source/consumer review supports the bounded attribution/counter design: existing process errors/streams/cancellation policy are preserved; the shared leaf counter excludes locally refused requests; the client-copy helper preserves caller policy; reader schema remains unchanged; direct command process sites are instrumented. Source review is separate from the unfulfilled consumer command. The changelog fragment was subsequently aggregated into CHANGELOG.md by 56615f9ab834a2218709271e55ce3cc4ba80c991.

The topology's deskkit/deskboard risk paths trigger value review. Enumeration covers the complete delivery diff and named deliverables, including all changed literal-bearing Go lines, fixture programs and the retained mutation JSON. No operational timeout, budget, breaker threshold, credential or publishing authority value changes. Production labels and optional active booleans are reversible measurement values; fixture IDs/status codes/permissions/counts do not create production policy. Fixture counts derive from explicit attempted operations: six process attempts, three account/visibility reads, three leaf attempts including retry/304, zero leaf attempts after local refusal, and three statusgen launches.

RISK-VALUE: DERIVED — perfKitVerb fallback = "other" @ tools/desk/cmd/deskdispatch/kits.go:127 — original Task 1 requires explicit unknown attribution without raw kit/argv leakage; other is the established perf-v1 fallback.

RISK-VALUE: DERIVED — perfKitVerb review label = "dispatch:review" @ tools/desk/cmd/deskdispatch/kits.go:111 — the parsed review kit and dispatch operation select this compiled bounded label; equivalent compiled mappings cover the four accepted kits for each operation and confer no role authority.

Limits: synthetic providers establish semantics, not production speed, quota points or complete fleet coverage. Own-process CPU and instrumented attempts remain partial; descendants/inclusive CPU and GraphQL points remain unknown. The recorder retains full row and Go output, but nested command-test binary build hashes were not captured; no complete child-build provenance is claimed. No live infrastructure, source/index changes, lifecycle advance, verified stamp or verified outcome was performed.

**VERIFY: FAIL** — original table 2/3 passed; row 3 executed and returned could-not-check (exit 2), so its expected exit 0 was not met. The implementation remains at implemented pending the tracked check-definition/environment disposition.

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
