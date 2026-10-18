---
brief: assay:assay:desk-tools:40
title: Deliver one review queue observation and offline planning bridge
why: "Review currently joins actions and prs from separate sweeps because the first output omits required information; the planner must consume one complete qualified observation."
wave: 3
depends: ["desk-tools/38", "desk-tools/39"]
unblocks: ["desk-tools/43"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Deliver one review queue observation and offline planning bridge'
id: a635247d-ba2c-4242-8518-860b3d4f30b6
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-tools/40"
  - "plugins/assay/scripts/pr-monitor.sh: follow-up desk-tools/40"
  - "tools/desk/cmd/deskboard/throughput.go: follow-up desk-tools/40"
---

# Brief 40 — Deliver one review queue observation and offline planning bridge

## Context

files: `tools/desk/cmd/deskboard/` (existing), `tools/desk/cmd/reviewloop/` (existing), `tools/desk/internal/reviewflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `docs/deskboard.md` (planned), `docs/desk-workflow-efficiency-spec.md` (existing), `changelog/desk-tools-40.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: deskboard collection adapter and reviewloop pure plan adapter over reviewflow. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: deskboard collection adapter and reviewloop pure plan adapter over reviewflow
  contract: review-observation-v1 and approved spec A1–A3
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +1 review-queue, flags +2 at most, refusals +0 authority, rule-text +0 duplicate policy; legacy adapters retained temporarily
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

1. Deliver ReviewQueue as deskboard review-queue --out FILE using existing typed ReviewQueueSnapshot
   and bounded pools. Extend typed queries only for required missing facts; no raw GraphQL in
   commands. Collect shared facts once per cycle, retaining conditional requests and per-change
   reuse. Depth and all planning rows come from the same population.
2. Deliver PlanReview as reviewloop plan --snapshot FILE with zero network/effects.
   Missing/unsupported/stale/incomplete observations cannot silently become empty queues or
   authorization. Keep legacy --actions/--prs forms during the declared compatibility window; add
   independently captured compatibility fixtures.
3. Expose observation generation/cutoff in reporting so same-cutoff consumers can reuse it without
   pretending a later request is fresh. Provide a compact summary plus complete artifact; no output
   cap may trim the population used for idle/depth.
4. Follow existing CLI contracts and coordinate #3248 before touching its adapter. This is the
   direct-reader bridge, independent of PostgreSQL, forge service and the future tiny desk-review-
   queue command. Document that future spelling as a migration target, not a delivered binary.
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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestQueuePlanBridge$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestQueuePlanBridge ' "$o")` | exit 0; named PASS. Fake provider -> real review-queue command -> saved observation -> offline plan produces exact oracle once; deleting any required head/scope/action input fails rather than yielding idle. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestQueuePartialAndDepth$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestQueuePartialAndDepth ' "$o")` | exit 0; named PASS. Partial/denied/overflow/stale/schema faults refuse complete or idle claims; report depth equals same uncapped population. Legacy actions/prs/plan stdout and exit contracts remain qualified. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/40` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

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
