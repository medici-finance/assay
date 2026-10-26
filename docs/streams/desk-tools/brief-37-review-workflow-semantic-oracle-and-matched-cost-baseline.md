---
brief: assay:assay:desk-tools:37
title: Review workflow semantic oracle and matched cost baseline
why: "Uncontrolled historical samples and incomplete process counters cannot prove a workflow improvement. Freeze intended behavior and reproducible complete-cycle workloads before changing collection."
wave: 1
depends: []
unblocks: ["desk-tools/38", "desk-tools/39"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 4
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Review workflow semantic oracle and matched cost baseline'
id: 202e9121-a4f6-49e5-90cb-dd5c8acaed1a
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
consumers:
  - "tools/desk/cmd/deskboard/: fixed-here (test-only production sweep adapter)"
  - "tools/desk/cmd/reviewloop/: fixed-here (test-only production plan adapter)"
  - "tools/desk/cmd/deskdispatch/: fixed-here (test-only pre-claim round and hold adapter)"
  - "tools/desk/cmd/issueboard/: fixed-here (test-only command and HTTP enumeration adapter)"
---

# Brief 37 — Review workflow semantic oracle and matched cost baseline

## Context

files: `tools/desk/internal/reviewtest/`, `tools/desk/internal/reviewtest/testdata/`, `tools/desk/cmd/{deskboard,reviewloop,deskdispatch,issueboard}/qualification_test.go`, `tools/desk/internal/deskkit/citrigger_test.go`, `docs/review-efficiency-qualification.md`, `changelog/desk-tools-37.md`

Implementation scope clarification (v2): the commands are separate `package main`
packages, so the fixture reaches their production handlers through test-binary adapters.
The four `_test.go` bridges add no shipped flags or alternative policy implementation.
The v3 observation boundary includes captured planner stdout and dispatch round text;
literal output assertions and production-rendering overlays supplement internal values.
The CI reader classification records that compiling those adapters stays within the desk
module. Cost coverage is explicit: local issue enumeration reaches the production HTTP
adapter; other typed fakes and the fixed Git reviewer do not establish full provider,
model, launcher or descendant CPU cost. Filling those counters remains desk-tools/38.

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: existing desk read/planning semantics and performance-collection meanings. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: existing desk read/planning semantics and performance-collection meanings
  contract: docs/desk-workflow-efficiency-spec.md §§2, 5–8; existing action table is the oracle
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text +0; local fixture only
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

1. Capture independent expected observations/actions from existing deskboard classification and
   reviewloop planning, including body-only edits, check/review applicability, overflow, access
   failure, superseded heads, duplicate lanes, WAIT/SURFACE/NO-OP, freshness and final idle gates.
   Describe which historical defects are corrected intent, rather than freezing them as truth.
   Exercise production adapters; a self-comparison of two copies is insufficient.
2. Build a local fake-provider/disposable-Git workload runner over existing public command
   entrypoints. Record revision/tool/plugin versions, fixture hash, host/method, seeds, cache state,
   scope/size, concurrency and result. Keep current token memoization, audit tail, pools and
   conditional reads in both arms. No real credentials or live provider data are required.
3. Declare the cost vector and permitted tradeoffs before comparisons: own vs descendant CPU,
   inclusive coverage, overlapping wall intervals, actual endpoint attempts/retries/304s, and actual
   vs estimated/unknown provider points. Record tool invocations and complete dispatcher-plus-agent
   work, never sum parent summaries and child events twice. No money conversion or invented
   numerical SLO.
4. Freeze a tool-only arm with supervision held constant. Add separate caller-only and combined arms
   when #3325 is qualified; plugin/launcher adoption and dispatch-context policy are explicit
   factors. Do not claim reduced context tokens equal CPU/provider savings. Keep this a bounded
   technical fixture, not the reusable scorecard/experiment system proposed in #3305.
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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowOracle$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestWorkflowOracle ' "$o")` | exit 0; named PASS. Literal actions, exact heads, lanes and holds agree with production paths; captured command output preserves visible row order, counts, proposals, idle and round text. Wrong-head/WAIT/order/depth controls and production planner-row/dispatch-scope overlays each fail the oracle. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCostCohort$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestCostCohort ' "$o")` | exit 0; named PASS. Fixture manifest reproduces cold/steady, skew/growth, concurrency and failed/partial runs; complete parent/child CPU, wall and request coverage is explicit. Unknown costs never become zero. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/37 --delivery e28201023c74b5f407ae98f9833ee4f79999f1db --base f92d2e9f24251bb3ac57c6447f229916d7748dfc` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |
| 4 | check:ci | `(cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestGitPackagesInstallQuiet$' ./internal/gitquiet)` | exit 0; named PASS; the new Git-fixture package uses the shared quiet runner without weakening fixture controls or cleanup. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

### 2026-10-20 independent verification (fresh verifier, admitted run)

Observed source: f09ef6239aa1439473717f9c795aacf393e5462a (merged main, detached home). Delivery: squash commit e28201023c74b5f407ae98f9833ee4f79999f1db, single parent f92d2e9f24251bb3ac57c6447f229916d7748dfc. Expected results were derived from the Verify Expect cells and Task text before reading the implementation. The fresh dispatcher admission check passed before any row ran and again before this block was finalized.

Verification-Table: delivery=e28201023c74b5f407ae98f9833ee4f79999f1db base=f92d2e9f24251bb3ac57c6447f229916d7748dfc original=f92d2e9f24251bb3ac57c6447f229916d7748dfc

Protected-table re-derivation against the delivery parent: rows 1–3 are original; row 4 is author-added and earns no acceptance credit. Row 1's Expect was strengthened by the delivery with its command unchanged; both the original obligations (planted wrong head, missing WAIT row and changed ordering each fail; uncapped depths) and the current ones were checked. Rows 2 and 3 are byte-identical to the original. The current Verify section equals the delivered section.

Execution witness: the admitted host adapter (declared offline after inspecting every row: local transport, disposable Git, typed fakes, no network, forge, cluster or nested container engine) refused before any container started, with `admitted-runner: refused: execution bytes/mode differ from source: scripts/windows-bootstrap-hashcheck-smoke.ps1`. That file carries the `eol=crlf` attribute, so the attested checkout writes CRLF bytes that differ from the LF blob, and the adapter by design refuses checkout conversions that change raw object bytes. No bound witness was published. This is an environment and adapter-scope limit, not an implementation result. Every row was then run by hand on the darwin host from the home root (go1.27.2, statusgen v1.0.52, warm module cache, no network sandbox); the home stayed clean afterwards.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowOracle$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestWorkflowOracle ' "$o")` | pass exit=0 sandbox=none (host hand run; admitted witness refused before execution) — `--- PASS: TestWorkflowOracle (8.59s)`; four planted mutations each turned the row red | sha256:168e88e0952d | 2026-10-20 | assay-verifier-app[bot] @ f09ef6239aa1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCostCohort$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestCostCohort ' "$o")` | fail (mutation obligation) — command exit=0 sandbox=none, `--- PASS: TestCostCohort (5.99s)`, but a planted unknown-to-zero cost mutation also exits 0 with the named PASS, so the row does not discriminate "unknown costs never become zero" | sha256:57ab19961da2 | 2026-10-20 | assay-verifier-app[bot] @ f09ef6239aa1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `statusgen --root . --consumers --brief desk-tools/37` | could-not-check exit=2 sandbox=none — `COULD-NOT-CHECK: assay:assay:desk-tools:37 is not in the diff against f09ef6239aa1439473717f9c795aacf393e5462a, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved.` | sha256:22a5ec6706a0 | 2026-10-20 | assay-verifier-app[bot] @ f09ef6239aa1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestGitPackagesInstallQuiet$' ./internal/gitquiet)` | author-added; execution exit=0 sandbox=none; no acceptance credit — `--- PASS: TestGitPackagesInstallQuiet (0.49s)` | sha256:fe354e66cfb8 | 2026-10-20 | assay-verifier-app[bot] @ f09ef6239aa1 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3762 run=e27c4bba367f009c51ba3de1d03cdb1cb84d601e276274fe3b5c0f90e8e8e2b0 source=f09ef6239aa1439473717f9c795aacf393e5462a brief=docs/streams/desk-tools/brief-37-review-workflow-semantic-oracle-and-matched-cost-baseline.md model=claude-opus-5-5 tier=strong

Mutation obligations. Each mutation below was applied alone in a throwaway clone at the attested source, the exact row command was run, and the clone was reset. The unmutated clone passed rows 1 and 2 first.

- Row 1, idle gate made to ignore blindness (the stale/truncated-board check skipped): exit 1, planner-overflow and planner-stale fail with `idle got "IDLE"; want "COULD-NOT-CHECK"`.
- Row 1, production planner rows dropped (the reviewloop plan stops rendering rows): exit 1, planner-mixed and planner-missing-head fail with `command output rows got []`.
- Row 1, production dispatch scope text changed from DELTA to OMITTED: exit 1, dispatch-delta and dispatch-duplicate-lane fail with `command output missing "- Scope: DELTA (clause 19)"`.
- Row 1, oracle comparator neutralized to always accept: exit 1, the built-in wrong-head, missing-WAIT, changed-order and depth controls report `oracle accepted planted mutation`. The negative controls are live.
- Row 2, overlapping child wall intervals summed instead of unioned: exit 1, `overlapping child wall double-counted`.
- Row 2, the failed cohort replaced with a clean needs-review/open cohort: exit 1, `failed cohort read as success`.
- Row 2, descendant CPU recorded as 0 instead of unknown for every invocation: exit 1, `unknown cost became absent or zero`.
- Row 2, descendant CPU recorded as 0 instead of unknown for every production command invocation (deskboard, reviewloop, deskdispatch, issueboard probes) while the local git-diff fixture invocation stays unknown: exit 0, `--- PASS: TestCostCohort (6.05s)` (full output sha256:9db10f73e916). The final assertion checks only that the serialized report as a whole contains at least one null descendant CPU value, so one remaining unknown anywhere masks zeros on every production invocation. Cohorts that never dispatch a git-diff agent (skew-growth, failed) can therefore carry only zeros. The row does not discriminate a plausible wrong mutation of its stated property. Class: non-discriminating acceptance assertion (missed in pre-merge review).

Row 3 supplemental probes. These carry no acceptance credit. (a) In the throwaway clone, with the delivery coordinates passed explicitly (`--delivery e28201023c74b5f407ae98f9833ee4f79999f1db --base f92d2e9f24251bb3ac57c6447f229916d7748dfc`): exit 0, `summary: 4 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing` against the immutable delivered snapshot. All four fixed-here sites are touched only by their qualification adapters. Each adapter calls the production handler: cmdPlan, sweepActionsRepo, roundOf/writeReviewRound, and issueboard's run over a GitHub forge with the local transport. The reviewtest package has no non-test importer. (b) The original command can derive delivery only after the Verification-Table marker above is committed. With this block committed locally in the throwaway clone, the exact row 3 command gave exit 0, `summary: 4 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing`. A fresh re-run of row 3 on main after this Evidence lands is the remaining obligation for that row.

RISK-VALUE: N/A — trigger not fired: risk frontmatter is present and all-no (irreversible: no); the delivery pins no hard value. It touches one file under a security-path trigger, tools/desk/internal/deskkit/citrigger_test.go, and only to add a test-only CI-registry opt-out entry for the new qualification test. The read-only lint flags this cross-read of the all-no risk answers as a NOTICE. That question belongs to the desk; this verification does not settle it. For completeness, the enumeration over the delivery diff found only reversible test-harness knobs: the oracle population floor 15 (qualification_test.go:231), the probe timeout of 45 seconds (:108), the build timeouts of 3 minutes (:56, :364), the truncation cap 100 (:538, :554; cap 100 in the deskboard adapter at :75), and Seed 37 (:501). None needs derivation.

**VERIFY: FAIL** — 1 of 3 original rows passes (row 1); row 2's mutation obligation fails (an unknown-to-zero descendant-CPU mutation on every production invocation stays green); row 3 is could-not-check at this source (delivery marker not yet committed); author-added row 4 executed exit 0 without credit; the admitted witness refused before execution (checkout conversion on an eol=crlf file).

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
