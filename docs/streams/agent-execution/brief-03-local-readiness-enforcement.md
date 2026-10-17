---
brief: "assay:assay:agent-execution:03"
title: "Enforce qualified profiles and workspace readiness in local execution"
why: "New contracts only help when the real launch path consumes them. The first local adapter must refuse mismatched environments before starting a process, while preserving existing launcher and custody boundaries."
wave: 8
depends: ["agent-execution/02", "graph-execution/22", "graph-execution/26"]
unblocks: ["agent-execution/04", "agent-execution/05"]
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "yes", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-10-16 by execution architecture authoring session"
sources: ["docs/proposals/non-ci-architecture/27-agent-execution-environments.md", "freshness-checked 2026-10-16 @ ec8d77e56627a7c2269f3d143ef6a31f3e41bd02 (origin/main)", "spec/loop-admin-runner-v1.md", "docs/celld-session-host-spec.md"]
exec-tier: "strong"
exec-tier-why: "Cross-component protocol, recovery and authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["loopadmin/supervisor: fixed-here", "loopadmin/adapters/localharness: fixed-here", "workflow/executionqualification: follow-up agent-execution/04", "tools/desk/internal/cellop: follow-up agent-execution/05", "tools/desk/cmd/cellctl: out-of-scope (resolved argv and enrollment remain graph-execution/27; no new builder)"]
version: 1
id: "1740cfc2-3d30-4c26-b8b2-a4a369e3df7a"
gate-why: "Extends the actual process-launch gate beside inference credentials; the operator confirms the resolved profile, effective mount/environment restrictions and independent custody boundary before adoption."
decision-trigger: "spec"
---

# Brief 03 — Enforce qualified profiles and workspace readiness in local execution

## Context

files: `loopadmin/supervisor/` (planned), `loopadmin/adapters/localharness/` (planned), `loopadmin/testdata/localharness/` (planned), `loopadmin/testdata/mutations.json`, `docs/workflow-runner-qualification.md` (planned). All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: supervisor launch gate — backed by independent local-adapter effective-profile validation and an isolated child with no operator/publisher credentials.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: At the source baseline supervisor and localharness are absent. graph-execution/26 owns their process-runner move and PostgreSQL journal; /22 owns the first resolved Claude CLI-profile adapter. This brief follows those implementations and adds readiness/profile enforcement rather than rebuilding them. Celld SDK and remote experiments remain separately scoped.

## Human decision

Decision-trigger: spec. At pickup, prepare the concrete launcher/profile boundary and negative-path evidence, then author a self-contained options section and file the decision through the existing gate. The decision concerns adoption of the implemented launch boundary; no response permits no activation. Offline construction and fixture tests can proceed under the inherited ground rules.

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

## Task

1. Consume /01 and /02 in the implemented shared supervisor/local adapter for both standing-desk and workflow modes. Resolve the approved harness/model/backend tuple; validate effective preparation and current authority before invoking the one existing process runner. Refuse required unsupported capabilities or stale receipts before creating a child.
2. Bind observed harness/model/backend and receipt identity to the existing invocation/journal, using its cell PostgreSQL storage and existing stop/budget rules. Never introduce another journal engine, profile argv builder or credential injection path. Workflow fields remain optional in standing mode.
3. Supply the workspace port from the existing profile-bound Git/resource adapter. Reconcile partial setup, preserve restored unpublished files and route release holds to existing retirement owners. Keep profile configuration outside the writable candidate domain. Explicitly test denied host-home, credential/socket and uncontrolled writable-mount access in synthetic profile fixtures.
4. Qualify the real supervisor/adapter code with fixture executables and synthetic credentials, bypassing its upper gate to prove the adapter refusal independently. Run the original runner conformance suite and the named new mutation tests. Produce a scrubbed qualification report with exact versions and limitations; no model/provider calls or live enrollment.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row runs on the delivery branch against its pre-merge base; post-merge verification uses that retained branch/base, not an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalPreparedLaunchBothModes$" ./... > "$execution_out" && grep -q -- "--- PASS: TestLocalPreparedLaunchBothModes " "$execution_out")` | exit 0; named PASS; both caller modes use the same supervisor/adapter and one readiness-gated process runner; mutation: start the child before validating the preparation receipt — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalPreparedLaunchBoundaryBypass$" ./... > "$execution_out" && grep -q -- "--- PASS: TestLocalPreparedLaunchBoundaryBypass " "$execution_out")` | exit 0; named PASS; with supervisor validation bypassed, the adapter still rejects profile/mount/credential-domain widening; mutation: accept a profile with an unauthorized writable mount — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalPreparedLaunchRestoreAndUnknown$" ./... > "$execution_out" && grep -q -- "--- PASS: TestLocalPreparedLaunchRestoreAndUnknown " "$execution_out")` | exit 0; named PASS; restored unpublished files survive, missing readiness holds and an unknown launch is not replaced; mutation: reprepare by overwriting an already restored workspace — the named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/03 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
