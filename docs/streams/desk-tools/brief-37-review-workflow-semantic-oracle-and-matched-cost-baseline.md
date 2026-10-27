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
### Container re-verification — 2026-10-27 (source e25479a69d69, after the row 3 amendment)

Observed source: merged main e25479a69d69 (detached home, admitted by `deskdispatch --check-verifier`, attestation medici-finance/assay#4587). Actual delivery: merged PR #3331, squash e28201023c74, first parent f92d2e9f2425. The 2026-10-20 failure on row 2 was repaired after delivery by merged PRs #3868 (unknown cost corruption rejected per invocation) and #4084 (every cohort's result checked). Row 3's Command was later amended by merged PR #4415 (two-parent merge 0440eb71c7c8, version 3 to 4); that amendment changes only row 3's Command code span. `deskpathguard rederive` at the observed source reports 4 rows, 0 author-added against the current table; row 4 entered with the delivery itself and is absent from the authoring table, so it carries no acceptance credit.

The table directly below is the execution witness: `statusgen verifyrun` v1.0.63, non-dry, `--sandbox=container-netns`, run inside a closed admission bracket. The bracket was opened on the host at 2026-10-27T02:18:45Z and closed on the host at 02:20:54Z. `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match`. The witness bytes, including the `Admission-Bracket:` line, follow unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowOracle$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestWorkflowOracle ' "$o")` | pass exit=0 sandbox=container-netns | sha256:b65266582b27 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCostCohort$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestCostCohort ' "$o")` | pass exit=0 sandbox=container-netns | sha256:ae495113d71b | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 3 | `statusgen --root . --consumers --brief desk-tools/37 --delivery e28201023c74b5f407ae98f9833ee4f79999f1db --base f92d2e9f24251bb3ac57c6447f229916d7748dfc` | pass exit=0 sandbox=container-netns | sha256:f3051911c67b | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestGitPackagesInstallQuiet$' ./internal/gitquiet)` | pass exit=0 sandbox=container-netns | sha256:c16b1053dcda | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:d515aab7266fbd4c6aebadc70cda8c12add58886b41248b41fc9b63dc2a03d0d
Container-Image: sha256:955fd6c46c93ee746d8f560b4c90b172a759d2543337d8173cdaabcd2013bca4
Verification-Attestation: medici-finance/assay#4587 run=df5c4efe3b387b8c19f9e5607d47321cbf67508d80c4e85e47a2332bf276d015 source=e25479a69d69ad678c15253309fcac6b7b0c86b0 brief=docs/streams/desk-tools/brief-37-review-workflow-semantic-oracle-and-matched-cost-baseline.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=df5c4efe3b387b8c19f9e5607d47321cbf67508d80c4e85e47a2332bf276d015 source=e25479a69d69ad678c15253309fcac6b7b0c86b0 tree=sha256:d515aab7266fbd4c6aebadc70cda8c12add58886b41248b41fc9b63dc2a03d0d witness=sha256:c0db54c3a58c8c4962bb9d148af6a844437cd7713acfe552f193c0402b946f5b opened=2026-10-27T02:18:45Z closed=2026-10-27T02:20:54Z before=receipt after=check-verifier

Environment: a local docker container from image assay-verify:go1.27 (local image id sha256:955fd6c46c93, linux/arm64, Go 1.27.2), started with `--network none`, the engine's default seccomp profile and no added privileges. The home, its git directory, the Go module cache, the statusgen binary and the admission receipt were bind-mounted read-only; the only writable mount was a new directory for the witness file. GOPROXY=off. statusgen is the published v1.0.63 linux-amd64 release binary, sha256 6775de412839… checked against the release checksums, run under the engine's amd64 emulation. The host statusgen is the same release (darwin-arm64, sha256 matches the release checksums). The container's admission step recomputed the mounted home's tree digest and matched the sha256:d515aab7266f value the host recorded at the open step.

Primary output (supplemental, no acceptance credit): the four row commands were re-run in a separate throwaway network-off container at the same source, with output kept for inspection.

| # | Exit | Key output | Output sha256 | Date | Runner |
|---|---|---|---|---|---|
| 1 | 0 | `--- PASS: TestWorkflowOracle (19.61s)`; 29 named subtests pass, among them the board, planner, dispatch and issue cases, the built-in controls `wrong-head`, `missing-WAIT`, `changed-order`, `wrong-depth` and the five printed-output controls, and the production overlays `production-reviewloop` and `production-deskdispatch` | f4c7e12680d1 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (claude-opus-5-5 verifier; supplemental, no acceptance credit) (on-behalf-of human:ian) |
| 2 | 0 | `--- PASS: TestCostCohort (7.63s)`; subtests `cold-small`, `steady-small`, `skew-growth`, `concurrent`, `partial`, `failed` and `attempts-304-and-unknown` all pass | 9e34e0210a94 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (claude-opus-5-5 verifier; supplemental, no acceptance credit) (on-behalf-of human:ian) |
| 3 | 0 | `consumers corroboration — base f92d2e9f2425..e28201023c74 (immutable delivered snapshot)`; `summary: 4 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing` | 1ee8f4cd9b35 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (claude-opus-5-5 verifier; supplemental, no acceptance credit) (on-behalf-of human:ian) |
| 4 | 0 | `--- PASS: TestGitPackagesInstallQuiet (0.96s)` | d179ba7cae9d | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (claude-opus-5-5 verifier; supplemental, no acceptance credit) (on-behalf-of human:ian) |

The original pre-amendment row 3 command, `statusgen --root . --consumers --brief desk-tools/37`, was also run in that throwaway container. It resolved the same immutable delivered range from the committed `Verification-Table:` marker and exited 0 with the same summary line. That run is supplemental and gets no acceptance credit.

Mutation probes (supplemental, no acceptance credit). Each probe ran in a throwaway copy of the observed source, in a network-off container, with exactly one change applied; the unmutated copy passed first. Row 2: the control and all 9 corruptions listed in the package's cost-mutations fixture each turn the row command red (exit 1). That includes the corruption the 2026-10-20 block recorded as surviving: setting the production command's descendant CPU to zero now fails with `cold-small invocation 0 ("deskboard"): descendantCpuMs must be explicit null, got "0"`. The same holds for the second-site Git agent's descendant CPU, inclusive CPU, retry lineage made known-false, quota points made zero, and each of the four serialized fields removed. Row 1: a comparator that always accepts fails with `oracle accepted planted mutation`; a production dispatch scope changed from DELTA to OMITTED fails with `command output missing "- Scope: DELTA (clause 19)"`; dropping the production planner rows fails with `command output rows got []`. Every probe was killed.

Risk-bearing rows: none. The risk metadata is present and all answers are no, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag (rows 1 and 2 are tagged `+flow +mutation`), so no fail-first witness is required.

Closure gate: this brief's Evidence is a protected record (the 2026-10-20 block records `Verification-Table: delivery=e28201023c74b5f407ae98f9833ee4f79999f1db base=f92d2e9f24251bb3ac57c6447f229916d7748dfc original=f92d2e9f24251bb3ac57c6447f229916d7748dfc`). Row 3's delivered command differs from its current command, so closure needs the explicit human-approved acceptance transition in docs/protected-paths.md. A merge or review of #4415 is not that approval. On the forge at landing time, no issue in this repository carries the acceptance-transition marker for this brief, and no `Approve acceptance-transition v1` comment exists for it.

Even after an approval lands, two further limits apply. First, a transition run executes on the host only, and on a darwin host every check:ci row is could-not-run (#1800). Second, a container bracket combined with a host transition run does not close yet (#4299). The delivered table also differs from the original in row 1's Expect, which the delivery rewrote while keeping the required exit 0, and it adds row 4. Whether the transition route accepts the Expect change is the open question in #4505.

`statusgen verifyrun --check` on this block, applied to a scratch checkout at the observed source: it refuses as landed, exit 2: `refused — protected table: refusing: verdict mismatch: the latest recorded verdict is VERIFY: BLOCKED, not PASS`. A scratch-only probe put a live `Verification-Table:` marker and a strict PASS marker in place of this verdict and left out the supplemental table. It refused too, exit 2, with and without `--acceptance-forge`: `refused — protected table: current Verify section differs from immutable delivery`. Neither form produces an N-pass summary line.

Input drift: none. The home stayed detached at the observed source for the whole run, and the attestation's source is the observed source.

Risk-value: the item is not irreversible and its risk metadata is all no. The enumeration over the delivered test harness, re-read at the observed source, finds only reversible test knobs; none is a production bound.

RISK-VALUE: DERIVED — oracle population floor = 15 @ tools/desk/internal/reviewtest/qualification_test.go:237 — Task requires a frozen population wide enough to cover every board, planner, dispatch and issue lane; the oracle fixture holds 18 cases at the observed source, and the floor only refuses a narrowed fixture; it never lets a failing case pass.
RISK-VALUE: DERIVED — probe timeout = 45s @ tools/desk/internal/reviewtest/qualification_test.go:110, build timeout = 3m @ qualification_test.go:58 and :370 — harness ceilings on local fixture work; exceeding one fails the test, never passes it.
RISK-VALUE: DERIVED — truncation cap = 100 @ tools/desk/internal/reviewtest/qualification_test.go:587 and :603, Seed = 37 @ qualification_test.go:550 — the cap mirrors the deskboard adapter's population cap of 100 so the growth cohort exercises truncation; the seed is a fixed label for reproducible cohort ordering.

**VERIFY: BLOCKED** — all 4 rows pass in the network-off container witness at source e25479a69d69, inside a closed admission bracket (`verdict=match`), and every row 1 and row 2 mutation probe was killed, including the unknown-to-zero case that failed on 2026-10-20. Closure is blocked because row 3's amended command (#4415) has no recorded human acceptance-transition approval. The amendment is only creditable through that approval, and the host transition run and its combination with a container witness remain limited (#1800, #4299, #4505). The README row stays `implemented`.

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
