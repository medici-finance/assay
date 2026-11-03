---
brief: "assay:assay:agent-execution:02"
title: "Prepared-environment receipts and reusable workspace port"
why: "Starting a healthy process in the wrong repository or with incomplete tools wastes work and produces misleading evidence. An explicit preparation result lets callers establish readiness before launch."
wave: 2
depends: ["agent-execution/01"]
unblocks: ["agent-execution/03", "agent-execution/06", "agent-execution/07"]
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
consumers: ["loopadmin/workspace: follow-up agent-execution/02 (this brief; flips to fixed-here when the implementation edits the path)", "loopadmin/supervisor: follow-up agent-execution/03", "workflow/executionqualification: follow-up agent-execution/04", "loopadmin/adapters/axcompat: follow-up agent-execution/06", "workflow/workspace: out-of-scope (candidate freeze and acceptance remain graph-execution/23 and /28)"]
version: 1
id: "d3782f80-bb6f-4b7d-9ed5-36994224e9d9"
---

# Brief 02 — Prepared-environment receipts and reusable workspace port

## Context

files: `spec/prepared-environment-v1.md` (planned), `schemas/prepared-environment-v1.json` (planned), `loopadmin/workspace/` (planned), `loopadmin/testdata/workspace/` (planned), `loopadmin/testdata/mutations.json`, `loopadmin/README.md` — All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: readiness receipt validation — backed by the workspace adapter checking actual revision/configuration and supervisor current-authority checks in /03.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: LaunchRequest.Workspace is currently a string reference. loopadmin/workspace does not exist. The runtime-resource-v1 schema covers five non-Git resource classes; it does not authorize Git workspace deletion. Existing candidate references belong to graph-execution/28 and are consumed by /23.

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

## Task

1. Define the prepared-environment receipt schema and pure Go types: caller/invocation, owner incarnation/generation reference, allocation/repository/exact revisions, recipe/tool/skill versions and harness-component bundle digest, optional image digest, approved writable paths, effective tool/network policy references, outcome, validation provenance and artifacts. Keep secrets out. Candidate/acceptance identities are opaque references validated by their existing caller owner, not new domain types.
2. Implement a small preparation/reconcile/validate port and deterministic orchestration over injected resource/Git adapters. No direct subprocess, network, new store, Git engine or maintenance scheduler. Ready requires effective configuration to match every mandatory input; fake adapters exercise the same production validator. Keep schemas and code covered by positive and drift-negative tests.
3. Persist/recover receipt references through caller-owned storage interfaces. Rebinding a cached environment to a new invocation requires current authorization, policy, identity and actual revision checks. Partial/unknown preparation is reconciled before retry. File restoration must not re-clone over unpublished changes.
4. Preserve existing retention/release ownership. A missing receipt, expired TTL or unknown allocation never authorizes deletion; this port emits a typed hold or delegates to the existing retirement owner. Add named flow and negative tests with executable mutations and document compatibility and changelog.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row runs on the delivery branch against its pre-merge base; post-merge verification uses that retained branch/base, not an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPreparedEnvironmentReadyFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestPreparedEnvironmentReadyFlow " "$execution_out")` | exit 0; named PASS; request through preparation port returns ready only for exact revisions, recipe and effective policy; mutation: skip effective repository revision validation — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPreparedEnvironmentReuseAndPartialFailure$" ./... > "$execution_out" && grep -q -- "--- PASS: TestPreparedEnvironmentReuseAndPartialFailure " "$execution_out")` | exit 0; named PASS; changed invocation/policy requires rebinding; partial preparation and uncertain release preserve unpublished files; mutation: reuse a ready receipt without checking current policy — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestPreparedEnvironmentSchemaAndSecrets$" ./... > "$execution_out" && grep -q -- "--- PASS: TestPreparedEnvironmentSchemaAndSecrets " "$execution_out")` | exit 0; named PASS; schema drift, wrong invocation/allocation and credential-bearing receipt data fail with sanitized errors; mutation: omit credential exclusion on a nested receipt field — the named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/02 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
