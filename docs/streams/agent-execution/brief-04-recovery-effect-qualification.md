---
brief: "assay:assay:agent-execution:04"
title: "Qualify restored execution against ownership and external effects"
why: "Restoring an environment cannot establish that its worker still owns the work or that an earlier publication failed. Integrated fault tests must catch duplicate effects and stale restored owners."
wave: 12
depends: ["agent-execution/03", "graph-execution/21", "graph-execution/25"]
unblocks: ["agent-execution/05"]
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
consumers: ["workflow/executionqualification: fixed-here", "workflow/controller: out-of-scope (implementation remains graph-execution/21; exercised through its public interface)", "workflow/publication: out-of-scope (implementation remains graph-execution/25; exercised through its public interface)", "loopadmin/supervisor: out-of-scope (implementation remains graph-execution/26; exercised through its public interface)", "tools/desk/internal/cellop: follow-up agent-execution/05"]
version: 1
id: "6edf4528-9b57-4291-82ac-098d25c57c1a"
---

# Brief 04 — Qualify restored execution against ownership and external effects

## Context

files: `workflow/executionqualification/` (planned), `workflow/testdata/executionqualification/` (planned), `workflow/testdata/mutations.json` (planned), `docs/reports/agent-execution/recovery.md` (planned). All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: current-owner check at effect admission — backed by unresolved-operation reservations in coordination and provider revision/idempotency preconditions where supported.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: workflow is absent at the source baseline. graph-execution/21 and /25 must deliver the real controller and publication path first; /04 recovery, /16 incarnation and /19 PostgreSQL remain their inherited dependencies. The qualification package is a test consumer of those implementations, not another controller, effect ledger or claim store.

Contract-consumption: agent-execution/02 supplies prepared-environment receipt and workspace-port types exercised here.

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

1. Build a bounded integration fixture around the implemented controller, supervisor and publication code with disposable PostgreSQL and an injected fake forge. Test setup provisions only its own synthetic database; ignore ambient DSNs and provider credentials, refuse non-fixture endpoints, and require no cluster or model network access. Do not substitute hand-built scheduling logic.
2. Inject faults after effect intent, after fake-provider commit but before acknowledgment, during cancellation, and during workspace/process restoration. Assert one stable logical operation with reconciliation or explicit hold; delivery retries cannot mint a second effect. Count calls at the fake-provider boundary, not by reading only a local success row.
3. Restore an old generation/incarnation and preserve paused/waiting state and cumulative usage. Bypass the upper controller, then require the role executor to reject a stale external action. Test missing workspace/session state, unsupported process continuity and unconfirmed stop; no new admission, replacement or budget reset is inferred.
4. Add executable mutations for each named failure, run the inherited recovery/conformance suites and retain a scrubbed report with exact subject, DB/profile versions, command results and limitations. This proves the fixture backend only; provider fencing and live profile activation stay separately qualified.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row runs on the delivery branch against its pre-merge base; post-merge verification uses that retained branch/base, not an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd workflow && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionRestoreLostPublishReply$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionRestoreLostPublishReply " "$execution_out")` | exit 0; named PASS; real controller/publication and supervisor retain one logical effect after provider commit plus lost reply; mutation: retry publication with a fresh logical effect key — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd workflow && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionRestoreStaleOwnerBypass$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionRestoreStaleOwnerBypass " "$execution_out")` | exit 0; named PASS; a restored stale owner is rejected at the executor even with controller validation bypassed; mutation: skip the executor generation check — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd workflow && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestExecutionRestoreStopBudgetAndMissingState$" ./... > "$execution_out" && grep -q -- "--- PASS: TestExecutionRestoreStopBudgetAndMissingState " "$execution_out")` | exit 0; named PASS; unconfirmed stop, paused work and missing preserved state hold; known spend remains charged and unknown usage remains unknown; mutation: reset cumulative spend when the process is recreated — the named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/04 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
