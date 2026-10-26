---
brief: assay:assay:desk-tools:42
title: Narrow issue discovery projections before enrichment
why: "Worker discovery pays broad issue enrichment cost while worker-ready, intake and inbox populations require different coverage and decisions."
wave: 3
depends: ["desk-tools/38"]
unblocks: ["desk-tools/44", "graph-execution/27"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Narrow issue discovery projections before enrichment'
id: 1bd8acf4-cef5-4b69-b663-3a2f406a3f09
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
design: DR-issue-projections
consumers:
  - "tools/desk/internal/deskclient/: follow-up graph-execution/27"
  - "plugins/assay/skills/worker-desk/SKILL.md: fixed-here"
  - "plugins/assay/skills/worker-desk/references/dispatch-runbook.md: fixed-here"
  - "plugins/assay/skills/intake-desk/SKILL.md: fixed-here"
  - "plugins/assay/scripts/assay-inbox.sh: fixed-here"
  - "tools/desk/cmd/deskinbox/: fixed-here"
  - "tools/desk/cmd/deskboard/: out-of-scope (independent board populations; intake-depth note already names the local intake reader)"
---

# Brief 42 — Narrow issue discovery projections before enrichment

## Context

files: `tools/desk/cmd/issueboard/` (existing), `tools/desk/internal/issueflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `plugins/assay/skills/worker-desk/SKILL.md` (existing), `plugins/assay/skills/intake-desk/SKILL.md` (existing), `plugins/assay/scripts/assay-inbox.sh` (existing), `changelog/desk-tools-42.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: issueboard typed observation/projection adapters and their actual worker/intake callers. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: issueboard typed observation/projection adapters and their actual worker/intake callers
  contract: approved spec §6 and A5; existing issue trust/escalation/closure owners
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0 baseline binaries, flags +1 projection selector at most, refusals +0 authority, rule-text +0 duplicate trust policy
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

1. Introduce narrow work-ready, intake and inbox projections over shared issue observations; select
   candidate fields before expensive comment/history enrichment. Preserve required histories for the
   selected decision. Batch typed reads and then tune bounded concurrency only using measured
   residual cost.
2. Preserve trusted author/edit and identity rules, complete comment/page handling, escalation,
   addressed-item and positive closure semantics. Declare per-projection freshness and completeness;
   never answer whole-intake idle from an empty worker projection.
3. Extract only the shared pure issue selection seam into tools/desk/internal/issueflow where
   needed; no separate service, credentials, cache or duplicate trust predicates. Use delivered
   issueboard CLI adapter (desktools-v2/45), existing typed forge reads and legal module boundaries.
4. Migrate actual worker/intake/inbox discovery invocations using a supported producer/plugin
   version matrix and reversible pin change. Inventory full-board users that remain. Preserve
   watcher health and existing desk-simplify writer/custody gates; a narrower reader does not switch
   executors. Record calls removed and complete-cycle cost with old broad reads disabled in the
   subject.
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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIssueProjectionFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestIssueProjectionFlow ' "$o")` | exit 0; named PASS. Actual worker/intake/inbox paths select distinct typed projections before history enrichment; oracle outputs match old semantics and unnecessary histories are not requested. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIssueProjectionFaults$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestIssueProjectionFaults ' "$o")` | exit 0; named PASS. Author/edit trust, identity, comments, overflow, permission failures, escalation and positive closure mutations fail; empty work-ready cannot prove intake/inbox idle. Adjacent full board output stays compatible. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/42 --delivery a92fea2720a5d2fab84bc5a47c827413adaf762b --base 2f44050f8a706d0865e1a95ca3cd8709abe1bd2f` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

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

## Implementation acceptance hold

The actual deskinbox consumer preserves the inbox script's positive ruling screen,
cards and flow. The merged [perf-v3 integration](https://github.com/medici-finance/assay/pull/3584)
now connects both readers to the canonical schema/context/provider seam from
desktools-v2/63–65. The [qualification report](../../issue-projection-qualification.md)
records the actual selected-profile writer/reader/reducer qualification; this is
source qualification, not an independent verification claim.

Shared delivery/acceptance and the independent desk-tools/38 verification prerequisite
remain completion holds. The later [reader-reuse follow-up](https://github.com/medici-finance/assay/pull/3931)
also leaves its requested perf-v4 before/after measurement pending the canonical codec
and producer, as documented in [issue projections](../../issue-projections.md). Its resolver
counts do not satisfy that measurement. None of these source qualifications activates
any deployment, writer, credential, pin, release or collection policy.

## Standing-desk consumer routing — 2026-10-18

graph-execution/27 consumes this brief's planned pure policy/projection API from a
tools/desk-local binding. It does not import cmd/main across modules, duplicate the
classifier, change this brief's acceptance or add a process supervisor here. Preserve
the current oracle, authority boundary and planned library home. This is a forward
consumer, not a dependency on future workflow/controller delivery.

## Prospective planner consumer checkpoint — 23 October 2026

The efficiency spec §2.1/A11–A12 nominates the existing issueflow projection as a
candidate input to graph-execution/27's shared role planner. Preserve the currently
assigned Task/Verify/revision until its owner acknowledges any material amendment.
At pickup, retain all work-ready/intake/inbox populations and projection-before-enrichment;
qualify per-operation batching only through this owner and existing forge-neutral/37–38.
No hidden global scan per root, parallel issue classifier or status/acceptance claim is added.
This checkpoint records requested integration, not new execution Evidence or activation.
