---
brief: "assay:assay:agent-execution:01"
title: "Qualified harness, model and backend capability contract"
why: "A resume flag cannot tell a caller whether files, conversation or running processes survive. A qualified profile makes those promises explicit before work is launched."
wave: 1
depends: ["graph-execution/20"]
unblocks: ["agent-execution/02", "agent-execution/06", "agent-execution/07"]
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
consumers: ["loopadmin/runner: follow-up agent-execution/01 (this brief; flips to fixed-here when the implementation edits the path)", "loopadmin/workspace: follow-up agent-execution/02", "loopadmin/supervisor: follow-up agent-execution/03", "tools/desk/internal/cellop: follow-up agent-execution/05", "loopadmin/adapters/axcompat: follow-up agent-execution/06"]
version: 2
id: "b402ec83-8a07-4551-b382-5dc017a45315"
---

# Brief 01 — Qualified harness, model and backend capability contract

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `spec/loop-admin-execution-profile-v1.md` (planned), `schemas/loop-admin-execution-profile-v1.json` (planned), `loopadmin/runner/executionprofile.go` (planned), `loopadmin/runner/executionprofile_test.go` (planned), `loopadmin/runner/conformance/`, `loopadmin/testdata/executionprofile/` (planned), `loopadmin/testdata/mutations.json`, `loopadmin/README.md`. All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: required-extension validation at the client boundary — backed by the adapter capability check and caller-owned current-profile validation, each exercised with the upper check bypassed.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: runner/request.go implements strict v1 decoding and ext/require negotiation; runner/usage.go uses boolean Resume and Snapshot fields. The baseline conformance suite passes. The new typed profile does not exist. The loopadmin module has no workflow dependency.

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

1. Define a schema-backed execution-profile/v1 extension over existing runner v1 ext/require negotiation. Bind the harness implementation/version, immutable harness-component bundle digest/provenance, model profile, backend/platform profile, profile digest and qualification evidence. Record requested versus actual reported identities; a requested model is never an observation.
2. Declare files, conversation, process memory, descendants and pending-interaction preservation separately, alongside cancellation confirmation, budget scope, telemetry and qualification limits. Retain existing v1 resume/snapshot meanings and doc 21 lifecycle vocabulary. Unknown and unsupported remain distinct; do not infer one capability from another.
3. Implement validation and compatibility in the existing runner package/client and adapter conformance kit. Old v1 requests remain accepted; mandatory new semantics on an old adapter refuse before Start. No new launcher, credentials, SDK dependency or workflow-store import. Include schema-to-source coverage and a malformed-artifact negative case.
4. Add the named production-boundary tests and executable mutations to the existing mutation map. Prove both desk and workflow request modes, role/profile substitution and a caller check with the upper contract check bypassed. Update protocol docs and changelog; no installed profile is changed.

- **Instance amendment:** Extend the qualified-profile binding with desk-instance and execution-generation references while retaining optional workflow fields. Different instances select independently qualified supported tuples; unsupported harness/provider/backend combinations still refuse. A profile/name is never instance authority or permission to copy conversation/process state.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row runs on the delivery branch against its pre-merge base; post-merge verification uses that retained branch/base, not an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileV1Compatibility$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileV1Compatibility " "$execution_out")` | exit 0; named PASS; old v1 requests still work, while an old adapter refuses a required extension before launch; mutation: ignore the new mandatory extension — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfilePreservationFlow$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfilePreservationFlow " "$execution_out")` | exit 0; named PASS; file-only restoration cannot satisfy conversation or process preservation in either caller mode; mutation: treat every resume declaration as full process preservation — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionProfileBindingAndSchema$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionProfileBindingAndSchema " "$execution_out")` | exit 0; named PASS; a substituted backend/profile and drifted schema are rejected; bypassing the client check still leaves adapter/caller checks effective; mutation: accept a mismatched qualified profile digest — the named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/01 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && cd loopadmin && GOWORK=off go test -count=1 -v ./runner/... -run '^TestDeskProfileBinding$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskProfileBinding ' "$o")` | Named test executes (zero matching tests is not a pass); Mixed instance profiles round-trip with distinct identity; wrong instance/generation and unsupported required tuple refuse; legacy one-desk profile remains valid. Mutation: compare role only rather than instance binding makes the test fail. |
## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
