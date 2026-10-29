---
brief: "assay:assay:agent-execution:06"
title: "Investigate AX compatibility through offline contract fixtures"
why: "AX may eventually supply isolated remote execution, but its advertised lifecycle and current trust boundary need comparison with our actual requirements before adopting another platform."
wave: 3
depends: ["agent-execution/01", "agent-execution/02"]
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
consumers: ["loopadmin/adapters/axcompat: follow-up agent-execution/06 (this brief; flips to fixed-here when the implementation edits the path)", "docs/reports/agent-execution/ax-compatibility.md: follow-up agent-execution/06 (this brief; flips to fixed-here when the implementation edits the path)", "production AX adapter: out-of-scope (requires separately authored implementation and authorized qualification after this report)", "celld S5 and S6 experiments: out-of-scope (existing remote/cluster questions and authorization remain separate)"]
version: 1
id: "acdc950e-3762-4ab4-ac30-fab52bfc86d7"
---

# Brief 06 — Investigate AX compatibility through offline contract fixtures

## Context

files: `loopadmin/adapters/axcompat/` (planned), `loopadmin/testdata/axcompat/` (planned), `docs/reports/agent-execution/ax-compatibility.md` (planned) — All new paths are implementation deliverables. Include a changelog fragment and module/path-trigger changes where the implementation needs them.

single-point-of-failure: compatibility classification against required boundaries — backed by fixture mapping tests and independent source/report review; no executable production backend is installed.

facts: As inspected on 2026-10-16 at main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`: AX source is pinned at 8d2ccf36f194f6f58c57b90753bd3986639b562f, inspected 2026-10-16. Its runner docs describe restored workspace files with a fresh process tree; its security policy reports no control-API authentication/authorization and task-visible Gemini keys. These are source observations, not a reproduced deployment or a Substrate-wide preservation claim.

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

1. Retain a minimal scrubbed, license-attributed fixture set with exact source revision/paths and content hashes for the AX Task/Workspace, readiness, suspend/resume and trust-boundary observations. Inspect pinned implementation where docs conflict; preserve unresolved claims as could-not-check. No full vendored source tree or floating download.
2. Implement an offline-only compatibility mapper from our qualified profile and prepared-environment request to AX fixture shapes. It has no network client, Start adapter implementation or automatic deployment. Characterize stable invocation mapping, partial creation/lost acknowledgment, preparation, actual process completion and file/conversation/process preservation separately.
3. Evaluate control-API access, debug execution, credential custody, network enforcement, cancellation and duplicate-launch behavior against /01–02 requirements. Record checked-clean, checked-failed or could-not-check per claim with source/fixture evidence. The documented trust gaps must prevent a ready-for-production conclusion; a nominal Task Running result cannot establish completed work.
4. Run mapping and negative controls, dereference report claims against retained source fixtures, and publish a go/no-go/unknown decision report with required mitigations, unresolved questions and a proposed comparator/measurement plan. No benchmark, savings or deployment claim. Core delivery is independent of this report; a production adapter/live trial is future separately gated work.

## Interface contract

A pinned caller/invocation, profile and externally validated ownership enter. Typed observations, preparation/qualification references and explicit unknowns leave with that identity. A receipt, restored session or healthy environment grants no scheduling, provider-write or acceptance authority. Unsupported mandatory semantics refuse; no silent model, backend or fresh-session fallback.

## Verify

These named tests, fixtures and mutations are deliverables of this brief; they have not been run at authoring time. Each row must exercise the production path and fail under its named mutation, rather than merely checking a file exists. Reports use the resulting evidence; independent review owns design adequacy. The consumer row runs on the delivery branch against its pre-merge base; post-merge verification uses that retained branch/base, not an empty main-to-main diff.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAXCompatibilityMapping$" ./... > "$execution_out" && grep -q -- "--- PASS: TestAXCompatibilityMapping " "$execution_out")` | exit 0; named PASS; offline mapping preserves invocation/profile/receipt identity and refuses missing mandatory inputs; mutation: drop the invocation identity from the mapped task — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAXCompatibilityResumeAndTrustGaps$" ./... > "$execution_out" && grep -q -- "--- PASS: TestAXCompatibilityResumeAndTrustGaps " "$execution_out")` | exit 0; named PASS; file restoration does not imply process continuation and documented API/custody gaps cannot qualify production; mutation: classify task-visible credentials as a qualified custody boundary — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && execution_out=$(mktemp) && trap 'rm -f "$execution_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestAXCompatibilityEvidenceBindings$" ./... > "$execution_out" && grep -q -- "--- PASS: TestAXCompatibilityEvidenceBindings " "$execution_out")` | exit 0; named PASS; report classifications dereference exact pinned fixture content and hashes; mutated sources invalidate affected claims; mutation: ignore a source hash mismatch when retaining a positive classification — the named test must fail |
| 4 | check:ci | `statusgen --consumers --brief agent-execution/06 --base $(git merge-base origin/main HEAD)` | exit 0; declared implementation sites and follow-up references corroborate; out-of-scope judgments remain UNCHECKED and reviewer-owned; completeness is not proved |

## Pre-mortem and dispatch checks

The plausible wrong deliveries are precisely the mutations listed in Verify. Each must turn its named row red and the unmodified production flow green. The implementation records its executable mutation command/map entry beside the test so a non-implementer can repeat it. In particular, bypass the upper validation layer where the row names an independent lower control. Source baseline, declared consumers, module visibility, risk answers and M sizing were checked at authoring; design adequacy and report interpretation remain review-only. Recheck them at pickup rather than treating this dated inventory as current runtime proof.

## Evidence

<!-- Independent verifier appends exact command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Review exact-subject evidence, compatibility, independent failure controls and cross-component flow. Implementation verification grants no live activation or merge authority.
