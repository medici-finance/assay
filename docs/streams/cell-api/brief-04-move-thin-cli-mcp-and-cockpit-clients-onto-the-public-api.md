---
brief: assay:assay:cell-api:04
title: Move thin CLI MCP and cockpit clients onto the public API
why: "Public service delivery is incomplete while callers import legacy internals or keep their own result meanings. Thin clients prove the same authorized status and reconnect contract across CLI, MCP and a later cockpit adapter."
wave: 14
depends: ["cell-api/03"]
unblocks: ["cell-api/07", "delivery-profiles/05"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the supported client matrix and selected read/status workflow, including explicit retained legacy consumers. Agent interfaces must not gain human acts or new ambient credentials."
issues: []
schema: brief-v2
version: 5
id: 9ca7f67b-e213-452c-93b3-1e86eb234144
authored: 2026-10-20 by cell-api authoring
decision-trigger: spec
exec-tier: strong
exec-tier-why: Cross-component compatibility, privacy and authority boundaries require design judgment and independent negative controls.
domain: complicated
outcome: none
sources:
  - docs/streams/cell-api/README.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md
  - docs/proposals/non-ci-architecture/21-cell-operator-surface.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - docs/proposals/non-ci-architecture/28-interactive-kubernetes-desks.md
  - https://github.com/medici-finance/assay/pull/2890
  - https://github.com/medici-finance/assay/issues/3669
  - freshness-checked 2026-10-20 @ 15b60414f3ec134942a0ec3162b550cd95cbbc8e
consumers:
  - "tools/desk/cellapi/: follow-up cell-api/04"
  - "tools/desk/cmd/deskcli/: follow-up cell-api/04"
  - "tools/desk/internal/cellapi/mcp/: follow-up cell-api/04"
  - "tools/desk/internal/cellapi/integration/: follow-up cell-api/04"
  - "tools/desk/testdata/cellapi/: follow-up cell-api/04"
  - "examples/cell-api-client/: follow-up cell-api/04"
  - "docs/cell-api-clients.md: follow-up cell-api/04"
  - "changelog/cell-api-clients.md: follow-up cell-api/04"
  - "legacy source consumers: out-of-scope (private source owner retains their exact crosswalk; this brief records public dispositions and does not edit them)"
---

# Brief 04 — Move thin CLI MCP and cockpit clients onto the public API

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `tools/desk/cellapi/` (planned), `tools/desk/cmd/deskcli/` (planned), `tools/desk/internal/cellapi/mcp/` (planned), `tools/desk/internal/cellapi/integration/` (planned), `tools/desk/testdata/cellapi/` (planned), `examples/cell-api-client/` (planned), `docs/cell-api-clients.md` (planned), `changelog/cell-api-clients.md` (planned)

facts:
- pin: public main 15b60414f, inspected 2026-10-20; these public client paths are planned.
- order: service and mounted authorization precede client migration; cockpit is a later client, not the migration head.
- compatibility: 01 owns the selected version matrix and complete consumer inventory.
- scope: selected reads/status/SSE only; full cockpit feature and packaging backlog is outside this brief.

layering: Flat CLI/MCP/presentation adapters consume the public client; meaningful policy stays server-side. Task 1–3 and Verify 1–3 check real client/server and transitive capability boundaries.

single-point-of-failure: MCP discovery can conceal a forbidden action while dispatch or a second adapter still exposes it. Verify 2 checks discovery and direct dispatch independently, then bypasses the client to exercise server refusal. Server authority from 03 remains the lower control; parity and reconnect tests do not replace it.

design-fit:
  owner: R7 client adapters over the single public contract
  contract: none — R7's transport responsibility is registered by cell-api/01; existing semantic owners and contracts remain controlling
  retires: []
  weight: verbs +1 public command; flags +0 new vocabulary; refusals +0 new policy; rule-text +0 (rehome the existing service or client command; record the actual implementation delta)
  why-add: Reuse the existing owners and one client seam; remove duplicate service composition only after the last-consumer proof. Package/process additions are bounded by the declared files, not permission to add another controller.

All planned paths and named tests below are implementation deliverables. The source pin
proves the starting point, not that these deliverables exist or were tested. The
[stream refactor map](README.md#bounded-refactor-map) defines the public transfer boundary.

## Read first

Read the [R7 ownership and admission record](README.md#authority-and-admission),
[cell operator contract](../../proposals/non-ci-architecture/21-cell-operator-surface.md),
[forge-service boundary](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
[interactive Kubernetes direction](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md),
`spec/cell-op-v1.md`, `docs/contracts.md` and `docs/go-engineering.md`.
Resolve every declared dependency's current contract and independent Evidence before
pickup. The package mapping is a reviewable proposal; a conflicting contract is
NEEDS_CONTEXT, never permission to select new custody or duplicate its owner.

## Human decision

Confirm the supported client matrix and selected read/status workflow, including explicit retained legacy consumers. Agent interfaces must not gain human acts or new ambient credentials.

Options:
1. **Accept the demonstrated bounded design** — accept the evidenced compatibility and containment for its declared scope; provisioning, deployment and source retirement retain their separate operator acts.
2. **Hold for correction** — name the missing proof or incompatible behavior; retain the current supported path until corrected.

Default if no answer: none — blocks until answered

The decision becomes concrete from the implementation design and evidence. No answer
keeps acceptance pending; it grants no time-based default or activation authority.

## Ground rules

- Work in an owned branch; no merge, ready flip, release, installation or live infrastructure contact.
- Public source and fixtures must be self-contained and synthetic. Never copy private locators, configuration, credentials, incidents or history into Assay.
- Preserve the single R2 PostgreSQL runtime, R3 isolation, R4 role authority and R6 forge mirror. A service, terminal, restored file or cached result grants no authority.
- Keep this stream parked until admitted. Stop at implemented; independent verification, human acceptance and operational cutover remain distinct.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner's delta; unchanged obligations remain.

## Task

1. Move the reusable CLI and MCP query adapters onto the public client package. Reuse existing contract meanings, explicit trusted endpoint/transport and qualified caller identity; do not import private service internals or discover credentials/endpoints from arbitrary requests. Preserve unsupported version and could-not-check behavior.

2. Generate the TypeScript client from the same reviewed contract and supply one small cockpit connection example for cell status, holds, SSE disconnect/reconnect and explicit resync. Exercise it only after the service; no frontend framework replacement, Electron packaging, new feature backlog or direct database/forge dependency. Record every other cockpit/web consumer as retained or separately deferred in 01’s inventory.

3. Keep the agent tool set closed: enumerate actual MCP discovery and dispatch; neither the legacy human-answer tool nor the adopt tool exists, including aliases/raw forwarding. Human named actions cannot be exposed through CLI query or MCP convenience wrappers. A direct call outside discovery must still fail at server authority.

4. Prove CLI, MCP and the thin cockpit example display the same independent fixture results, distinguish execution/effects/acceptance and retain null coverage. Put the parity and end-to-end reconnect harness in the separate desk-module `internal/cellapi/integration/` test package: it starts the mounted server and invokes CLI/MCP/TypeScript consumers without adding those dependencies to the public `cellapi` package. Keep 01's external-import test on that public package. Reconnecting cannot launch a model request, repeat an effect or grant a new lease. Mark migrated consumer receipts without retiring the old service; 07 owns that last-consumer decision. Update client/compatibility docs and generated artifacts.

- **Instance amendment:** Thin CLI/MCP/cockpit consumers render shared role capacity and individual desired/paused/draining/replacement state from the same contract. Resolve short names unambiguously; no local queue, status collapse, inferred human approval or lifecycle apply. Keep profile/usage provenance and zero-work reconnect behavior.

## Interface and acceptance contract

Consume cell-api/01's versioned HTTP/JSON and SSE seam and the existing cell-op-v1
result/plan meanings. Authentication, current authorization, provider custody and
acceptance are separate. Required coverage/freshness, exact subject and unknown outcomes
survive every adapter. No agent-facing legacy human-answer or adopt tools; no generic role,
URL, SQL-write, token or apply escape hatch. Human named actions remain closed and
separately qualified. Initial service exposure is read/plan-only; approval-based apply
stays absent until amendment 21's five R7/R3 properties are independently qualified.

Update the declared documentation and generated schema/client/profile examples affected
by the Task. Preserve prior Evidence and source history; unresolved consumer or contract
coverage is a hold. Do not invent deployment performance/recovery thresholds: reuse the
inherited workload and require the operator to select missing targets before exposure.

## Verify

These exact named tests must be delivered and run, with a recorded failing control
before the restored pass. Each uses real production composition and an independently
specified expected result; zero selected tests or a skipped required fixture is failure.
Only explicit disposable fixtures may open local listeners/databases. No production or
ambient kubeconfig is permitted. Real-profile qualification remains a separate operator
act; offline passes do not establish it.

For row 4, the independent verifier obtains the actual implementation delivery from its
merged PR and records its full immutable SHA in `ASSAY_DELIVERY_SHA` and its actual
first-parent SHA in `ASSAY_DELIVERY_BASE_SHA`. Neither defaults to HEAD, origin/main or
this authoring PR. Both commits and the fetched mainline must exist locally; the command
requires the delivery to have landed on that mainline before checking its immutable
snapshot. Missing, unknown, not-yet-landed or wrong-parent inputs fail closed. Record
exact corroborated/disproved/unchecked counts; unchecked required consumers remain held
and omitted consumers remain a review obligation. These are future delivery inputs.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellClientParity$' ./internal/cellapi/integration/... > "$o" && grep -F -- '--- PASS: TestCellClientParity ' "$o")` | exit 0; named PASS. Separate integration package drives real mounted fixture, CLI, MCP and generated TypeScript example; independently expected fields/unknowns match, and a stale generated client fails parity without broadening the public client's import closure. |
| 2 | check:ci +flow +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellAgentAbsences$' ./internal/cellapi/mcp/... > "$o" && grep -F -- '--- PASS: TestCellAgentAbsences ' "$o")` | exit 0; named PASS. Actual discovery and dispatch exclude answer/adopt and alias/raw human-act paths; planted second registration is rejected and server independently refuses direct calls. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && GOWORK=off go test -timeout 90s -count=1 -v -run '^TestCellClientReconnect$' ./internal/cellapi/integration/... > "$o" && grep -F -- '--- PASS: TestCellClientReconnect ' "$o")` | exit 0; named PASS. Dropped stream, expired cursor and revoked caller produce resync/refusal with zero new work/effect/model launches; a reconnect-triggered launch fails. |
| 4 | check:ci +dereference | `test -n "$ASSAY_DELIVERY_SHA" && test -n "$ASSAY_DELIVERY_BASE_SHA" && git merge-base --is-ancestor "$ASSAY_DELIVERY_SHA" refs/remotes/origin/main && statusgen --root . --consumers --brief cell-api/04 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | exit 0 only for the named landed implementation delivery and its first parent; record exact corroborated/disproved/unchecked counts, 0 disproved; unchecked required consumers stay held and omitted consumers require review. |
| 5 | check | `statusgen --root . --lint` | exit 0; no PROBLEM; no generated lifecycle cell or historical Evidence hand-edited. |
| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && go -C tools/desk test -count=1 -v ./internal/cellapi/integration/... -run '^TestDeskInstanceClients$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceClients ' "$o")` | Named test executes (zero matching tests is not a pass); Actual served CLI/MCP/TypeScript consumers agree on mixed population and ambiguous selector/unknown state; reconnect produces zero model calls. Mutation: one client renders only the first role member makes the test fail. |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| One client silently changes a shared field or bundles stale generated code | row 1 |
| MCP gains a human act through an alias or direct invocation | row 2 |
| Reconnect dispatches work or replays an unknown effect | row 3 |
| A fixture is mistaken for deployment authority or a private inventory is called complete without its owner | No row; review-only: inspect decision provenance, source-owner receipt and explicit operational limits before acceptance. |

## Evidence

<!-- Independent verifier appends one row per Verify item: exact subject/source revision,
command, exit, named result, failing control and restored pass, environment, date and
runner. Empty at authoring; no implementation or deployment result is asserted. -->

## Review

Gate: human. Review the actual mounted/effect boundary and retained ownership, the
compatibility inventory and the independent negative controls. Test success grants
neither deployment nor credential provisioning, source closure or a human decision.
