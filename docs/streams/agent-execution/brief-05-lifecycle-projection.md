---
brief: "assay:assay:agent-execution:05"
title: "Expose preparation, execution, recovery and acceptance separately"
why: "A running sandbox may contain a failed command, and a published candidate may still await acceptance. Operators need those distinct facts to recover the right component without accidentally relaunching work."
wave: 13
depends: ["agent-execution/03", "agent-execution/04", "graph-execution/27"]
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-10-16 by execution architecture authoring session"
sources: ["docs/proposals/non-ci-architecture/27-agent-execution-environments.md", "freshness-checked 2026-10-16 @ ec8d77e56627a7c2269f3d143ef6a31f3e41bd02 (origin/main)", "spec/loop-admin-runner-v1.md", "docs/celld-session-host-spec.md"]
exec-tier: "strong"
exec-tier-why: "Cross-component protocol, recovery and authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["loopadmin/supervisor: follow-up agent-execution/05 (this brief; flips to fixed-here when the implementation edits the path)", "tools/desk/internal/cellop: follow-up agent-execution/05 (this brief; flips to fixed-here when the implementation edits the path)", "tools/desk/cmd/cellctl: follow-up agent-execution/05 (this brief; flips to fixed-here when the implementation edits the path)", "R7 Cell API and cockpit: out-of-scope (consume this snapshot under their existing ingress and mounted-route qualification; no alternate endpoint here)"]
version: 2
id: "a4787a6a-d6e4-452b-ae9f-b275a91a97d4"
---

# Brief 05 — Expose preparation, execution, recovery and acceptance separately

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `loopadmin/runner/lifecycle.go` (planned), `loopadmin/runner/lifecycle_test.go` (planned), `loopadmin/supervisor/` (planned), `tools/desk/internal/cellop/`, `tools/desk/cmd/cellctl/`, `schemas/loop-admin-lifecycle-v1.json` (planned), `docs/cellctl-machine-seam.md`. All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: projection source/subject binding — backed by existing authoritative acceptance checks and a read-only client which cannot launch or acknowledge work.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: tools/desk/internal/cellop and cmd/cellctl/cellop_cli.go implement the current machine seam. The supervisor and standing-desk clients are planned in graph-execution/26–27. Doc 21 owns lifecycle vocabulary, and R7 owns Cell API ingress; celld is execution-host packaging, not another API.

Contract-consumption: agent-execution/01 supplies the qualified execution-profile types consumed here.

## Read first

- [Controlling future-state amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md).
- [Stream ownership and dependency waves](README.md).
- [Existing runner protocol](../../../spec/loop-admin-runner-v1.md).
- [Runtime and recovery owners](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md).
- [Cell operator lifecycle](../../proposals/non-ci-architecture/21-cell-operator-surface.md).

## Ground rules

- Own isolated branch and draft PR; no merge, deployment, workflow dispatch or live infrastructure contact.
- Offline fixtures and synthetic credentials only; no model/provider calls, new credential custody or profile activation.
- Preserve existing work identity, coordination/effect owners, historical Evidence and independent acceptance. Stop at implemented; a non-implementer owns verification.
- Refresh source, claims and active PRs at pickup. Missing prerequisites or contradictory contracts require a recorded hold, not a replacement implementation.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner\'s delta; unchanged obligations remain.

## Task

1. Publish one schema-backed sanitized snapshot from the existing supervisor with separate preparation, process, preservation/recovery, effect reconciliation and acceptance dimensions. Each has its authoritative source, subject, timestamp/coverage and unknown/unsupported representation. Reuse existing state vocabularies; no new durable lifecycle writer.
2. Join acceptance only through the existing caller-owned observation port. A desk-only installation without workflow still reports execution and explicitly unavailable work acceptance. Missing or denied observations cannot become success, idle or zero. Preserve model/backend/receipt identity without secrets, raw transcripts or unrestricted local paths.
3. Consume that snapshot in the existing cellop/cellctl machine and human status surfaces, with useful next-required-action text. An alive container plus failed child shows both facts; publication with pending independent review shows pending acceptance. Opening status/attaching a client creates zero launches and changes no pause or ownership. R7 receives a documented contract, not a second ingress implementation.
4. Test the complete supervisor snapshot to machine status and human rendering flow, including old-reader compatibility, stale/missing coverage, cross-cell filtering and secret exclusion. Wire each module test to its actual path triggers; retain named mutation controls and changelog.

- **Instance amendment:** Extend sanitized lifecycle snapshots with cell/role/instance/generation, desired/observed state, predecessor/successor, aggregate role occupancy and optional instance ceiling. Preserve paused/draining/unknown members in aggregates; full names are display locators. Keep independent preparation/process/effect/acceptance dimensions and per-invocation attribution.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row runs on the delivery branch against its pre-merge base; post-merge verification uses that retained branch/base, not an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd tools/desk && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionLifecycleStatusFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionLifecycleStatusFlow " "$execution_out")` | exit 0; named PASS; supervisor facts reach machine and human status without conflating process exit, effect outcome and acceptance; mutation: map a live environment with a failed child to successful work — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd tools/desk && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionLifecycleReadOnlyAndScope$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionLifecycleReadOnlyAndScope " "$execution_out")` | exit 0; named PASS; status/reattach creates no launch, preserves pauses and refuses cross-cell or secret-bearing output; mutation: start a missing session while serving status — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd tools/desk && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionLifecycleUnknownAndCompatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionLifecycleUnknownAndCompatibility " "$execution_out")` | exit 0; named PASS; old or incomplete snapshots stay explicitly unknown/unsupported rather than idle or accepted; mutation: default unavailable acceptance to success — the named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/05 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && cd tools/desk && GOWORK=off go test -count=1 -v ./cmd/cellctl/... -run '^TestDeskInstanceProjection$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceProjection ' "$o")` | Named test executes (zero matching tests is not a pass); Supervisor-to-machine/human status shows two workers/three reviewers and a draining predecessor without hiding a paused peer or unknown occupancy. Mutation: group members into one role row or turn unknown into zero makes the test fail. |
## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
