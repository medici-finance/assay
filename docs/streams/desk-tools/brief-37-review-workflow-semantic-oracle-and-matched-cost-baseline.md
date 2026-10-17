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
version: 3
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
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/37` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |
| 4 | check:ci | `(cd tools/desk && GOWORK=off go test -count=1 -v -run '^TestGitPackagesInstallQuiet$' ./internal/gitquiet)` | exit 0; named PASS; the new Git-fixture package uses the shared quiet runner without weakening fixture controls or cleanup. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
