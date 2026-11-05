---
brief: assay:assay:desk-tools:39
title: Extract shared review observation and pure planning contracts
why: "Queue classification and plan assembly live inside commands, so callers repeat collection and cannot consume one complete observation through a pure shared boundary."
wave: 2
depends: ["desk-tools/37"]
unblocks: ["forge-neutral/39", "desk-tools/40", "desk-tools/41", "graph-execution/27"]
effort: M
gate: human
decision-issue: 4521
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-17 by statusgen newbrief
sources:
  - "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Extract shared review observation and pure planning contracts'
id: 5a306961-7f3a-4e54-8c3c-42d0a9ddf2a5
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "tools/desk/internal/reviewflow/selection.go: fixed-here (remaining candidate-plus-priority amendment)"
  - "tools/desk/internal/reviewtest/selection_test.go: fixed-here (priority/capacity/coverage and mutation proof)"
  - "spec/review-observation-v1.md: fixed-here (Go selection API and compatibility contract)"
  - "docs/contracts.md: fixed-here (existing semantic-owner index)"
  - "tools/desk/internal/deskclient/: follow-up graph-execution/27"
  - "tools/desk/cmd/deskboard/: fixed-here (shared classification facts; observation collection follows in desk-tools/40)"
  - "tools/desk/cmd/reviewloop/: fixed-here (shared plan assembly; observation CLI handoff follows in desk-tools/40)"
  - "tools/desk/cmd/deskdispatch/: follow-up desk-tools/41 (consume the qualified observation during candidate admission)"
---

# Brief 39 — Extract shared review observation and pure planning contracts

## Context

files: `tools/desk/internal/reviewflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `tools/desk/cmd/deskboard/board.go` (existing), `tools/desk/cmd/reviewloop/plan.go` (existing), `spec/review-observation-v1.md` (planned), `schemas/review-observation-v1.json` (planned), `docs/contracts.md` (existing), `changelog/desk-tools-39.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: tools/desk internal reviewflow, extracted from existing cmd classifiers/planner. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: tools/desk internal reviewflow, extracted from existing cmd classifiers/planner
  contract: review-observation-v1 per approved spec §§3–5; statusgen/evaluation retains eligibility and coverage semantics
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text +0; shared contract replaces cmd-local copies
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Candidate and priority amendment — 2026-10-23

Extend the existing review observation/planning extraction, not its canonical domain meanings.
Define candidate observation and explicit approved policy/evaluator revision, time and capacity
as separate pure inputs, following efficiency spec §2.1 and A11–A12. Return ordered proposals
plus held/unknown/watch and unmapped dispositions; retain exact subject/lane, scope and reasons.
Policy rank is not coalescing-key order. Never drop a candidate absent from a priority mapping
or reconstruct full coverage from shown/capped rows. graph-execution/27 consumes this existing
reviewflow API as one role binding; other domain evaluators stay with their current owners.
Keep collection out of the pure library and preserve the internal/public module boundary.

Add the parity case below inside the already-declared internal/reviewtest test surface.
These are future implementation obligations; existing approval and Evidence are unchanged.

## Task

1. Specify typed review-observation-v1: requested/admitted scope, collection interval and per-input
   provenance/version/freshness/completeness; provider-qualified change identity, full head SHA,
   base SHA when used, body/review/check applicability; all action rows, lanes, rank, holds and
   WAIT/SURFACE/NO-OP. Derive depths from that same uncapped population. It is an interval
   observation, not an atomic provider snapshot.
2. Extract the actual existing classification and pure plan assembly into
   tools/desk/internal/reviewflow. Commands become adapters. Use intended-behavior oracle and
   historical regression fixtures before moving code; no command importing another command, broad
   rewrite or second ready predicate. Share the existing evaluation semantic owner through its legal
   module API; if missing, route extraction through that owner rather than copying it.
3. Pure packages take typed facts, perform no network/filesystem/process/credential reads and return
   explicit partial/stale/schema errors. Operational collection/auth remains with deskkit. The
   tools/desk internal package is not importable from statusgen/forgeread/other modules; eventual
   public serving extraction belongs to forge-neutral/39.
4. Publish contract and consumer compatibility fixtures. Review observations are untrusted advisory
   input, not claim/effect authority. Preserve coalescing, exact-head lanes and existing action
   table; do not merge independent reviewer conclusions.
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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewContractFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewContractFlow ' "$o")` | exit 0; named PASS. Existing and extracted production classifiers/planners match independent oracle across heads/body/reviews/checks/lanes/order/holds/uncapped depths. Mutation of one applicability or idle input fails. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewBoundary$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewBoundary ' "$o")` | exit 0; named PASS. Root contract/planner runs with provider effects and credentials forbidden; schema rejects wrong versions, missing scope and incomplete-as-empty. Cross-module import probe cannot consume desk internal packages. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/39` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPriorityJoin$' ./... > "$o" && grep -F -- '--- PASS: TestReviewPriorityJoin ' "$o")` | exit 0; named PASS. the extracted pure contract matches an independent oracle for identical observation/policy/time/capacity, selected head/lane/order and held/unknown sets. Actual driver integration is accepted downstream in /43 and graph-execution/27; it is not a prerequisite here. Permute identity keys against rank, include unmapped and capped candidates, change policy only and deny provider/process/file reads; a second sorter or any provider call fails. |

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

## Standing-desk consumer routing — 2026-10-18

graph-execution/27 consumes this brief's planned pure policy/projection API from a
tools/desk-local binding. It does not import cmd/main across modules, duplicate the
classifier, change this brief's acceptance or add a process supervisor here. Preserve
the current oracle, authority boundary and planned library home. This is a forward
consumer, not a dependency on future workflow/controller delivery.

## Implementation pickup — remaining amendment, 23 October 2026

Continue at public main `4a4b8e4f3`. The original extraction and observation contract already
shipped in PR #3593; reuse reviewflow/PlanObservation and the existing command adapters.
The remaining merged amendment is the explicit candidate-plus-priority/capacity join and
TestReviewPriorityJoin, not another classifier or a second queue. The source is the merged
planner amendment in PR #4218, with docs/library-first.md, docs/contracts.md and
spec/review-observation-v1.md at this source revision. Claim inspection found no active
owner or open delivery PR for this brief. The driver explicitly requested this implementation
and draft PR; no approval stamp, new ruling or lifecycle advancement is inferred.

The additive internal Go API is PlanCandidates; collection and command/prose adoption remain
/40, /43 and graph-execution/27, with their existing gates. Legacy board/reviewloop ordering
is preserved and checked by the existing regression suite. The original fixed-here entries
for those adapters describe the inherited extraction, not new edits in this follow-up.
No status cell, historical Evidence or human gate is changed. Existing lifecycle reconciliation
holds for the earlier delivery are disclosed rather than bypassed or repaired by a new stamp.
