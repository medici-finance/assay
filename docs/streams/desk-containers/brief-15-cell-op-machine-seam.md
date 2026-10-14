---
brief: assay:assay:desk-containers:15
title: "The cell-operation contract — one versioned result, plan and plan-approval schema shared by the future Cell API and the CLI, with the CLI as the first adapter"
why: >-
  The cell-level operator verbs (up, drain, restart, status, doctor, logs, gc, upgrade) will
  be driven both from a terminal and, later, from the Cell API that the cockpit and other
  clients use. If the CLI invents its own output and its own confirmation, the API will
  either copy it or diverge, and the cockpit will screen-scrape or fake a TTY, which removes
  the human from the confirmation. This brief defines the operation result, the plan, and
  the plan approval once, as a versioned contract that both surfaces consume. The approval
  is a human decision record in the architecture's existing decision model, not a new
  approval mechanism. The CLI is the first adapter over it.
wave: 7
depends: []
unblocks: ["desk-containers/17", "desk-containers/18", "desk-containers/20", "desk-containers/21", "desk-containers/22", "desk-containers/24"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Public Assay includes the control plane and cockpit — 'Use one versioned HTTP/JSON contract for UI and remote clients … CLI and MCP are adapters over those capabilities, with different allowed operations. Keep the existing human-action absences on agent-facing surfaces'"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Runtime durability — 'A human decision binds a human actor, subject, revision and permitted act; agent/MCP surfaces cannot fabricate that act'"
  - "docs/proposals/non-ci-architecture/19-progressive-adoption-and-oversight.md §Sampling and batching — a decision binds explicit subject/revision/action/policy tuples, the authorized actor, expiry, revocation and consumption semantics, revalidated before each applicable effect"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (proposed) (added by this PR) — classifies this contract as target-owned: the result/plan types serve the R7 Cell API and the CLI; the approval record belongs to the evidence-and-decisions owner"
  - "Driver answers 2026-10-14 (in session, relayed by the coordinator): plan TTL is 10 minutes; apply must verify an approval signed over the plan id and digest by an operator key — recording an `--approved-by` name alone is not enough; driver ruling the same day: define these types once for both the future API and the CLI, aligned with the existing human-action model"
  - "docs/cellctl-go-cache.md — `cellctl cache <cell> status` already emits a versioned JSON document (`schema: cell-go-cache-v1`, could-not-check as null): the precedent for the result type"
  - "#2880 — the reconcile preflight's 'no --yes, ever' boundary, which this contract keeps"
  - "freshness-checked 2026-10-14 @ caca38a58 (origin/main) — no cross-verb result schema, plan record or plan-approval record exists; the decision domain's actor/subject/revision/act binding is architecture, not yet code"
exec-tier: strong
exec-tier-why: (a) the schema, plan lifecycle and decision-record mapping are design decisions the API and later briefs build on; (b) two consumers (CLI now, API later) must read identical meanings; (c) approval verification and apply-time re-verification are safety plumbing
domain: complicated
consumers:
  - "schemas/cell-op-v1.json and spec/cell-op-v1.md (planned; the shared contract): follow-up desk-containers/15 (this brief)"
  - "tools/desk/internal/cellop (planned; Go types, plan store, approval verification, apply harness): follow-up desk-containers/15 (this brief)"
  - "tools/desk/cmd/cellctl (the CLI adapter: `--json`, `--plan`, `--apply`, `approve`): follow-up desk-containers/15 (this brief), then each verb brief adopts it"
  - "the R7 Cell API (authenticated queries and scoped requests; HTTP/JSON over the same contract): out-of-scope (no owner yet per the amendment manifest; docs/proposals/non-ci-architecture/21-cell-operator-surface.md (proposed) records the requirement)"
  - "the evidence-and-decisions human-decision record (actor, subject, revision, permitted act): out-of-scope (architecture owner; this brief maps the plan approval onto it and does not define a rival record — reconciled when that owner's contract ships)"
  - "the console cockpit (renders results, produces plan approvals from a human click): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 15 — The cell-operation contract

## Context
files:
- `spec/cell-op-v1.md` (planned) — the contract: result, plan, plan-approval, exit codes, lifecycle
- `schemas/cell-op-v1.json` (planned) — JSON Schema for the three documents
- `tools/desk/internal/cellop/` (planned) — `result.go`, `plan.go` (store + lifecycle), `approval.go` (verify), `apply.go` (verify-then-re-measure-then-run harness), tests
- `tools/desk/cmd/cellctl/approve.go` (planned) — `cellctl approve`
- `tools/desk/cmd/cellctl/consent_flag_test.go` (planned) — the no-blanket-consent flag walk
- `tools/desk/cmd/cellctl/plan_roundtrip_test.go` (planned) — end-to-end plan → approve → apply through the built binary
- `docs/cellctl-machine-seam.md` (planned) — CLI-adapter usage of the contract
- `changelog/desk-containers-15-cell-op-contract.md` (planned)

single-point-of-failure: the plan approval check at apply — behind it, apply-time re-measurement of every precondition (an independent layer that refuses a correctly approved but stale plan), single consumption, and the TTL.

facts:
- **One contract, two adapters.** `cell-op-v1` defines three documents. The CLI emits and consumes them now; the future R7 Cell API serves the same documents over HTTP/JSON. Neither adapter may add a field the other cannot carry. The Go types in `cellop` are the in-process API.
- **Result** (`cell-op-result`): `verb`, `cell`, `started`/`finished` (UTC RFC 3339), `outcome` (`ok` | `partial` | `refused` | `failed` | `could-not-check`), `roles[]` (`role`, `state`, `detail`, `evidence{}`), `actions[]`, `plan_id`, `next[]`. An unmeasured value is `null` with could-not-check, never zero. In the CLI, `--json` puts exactly this document on stdout; human text goes to stderr. Exit codes are unchanged: 0 ok, 1 failed/partial, 5 refused, 6 could-not-check.
- **Plan** (`cell-op-plan`): verb, cell, the exact action list, each action's **precondition evidence** (pid + process start time, checkpoint sequence, lock holder, byte counts), created time, a fixed **10-minute TTL**, and a **digest** (sha256 of the canonical JSON). It is stored owner-only (0600) under the cell's `run/plans/`. Creating a plan changes nothing else.
- **Plan approval is a human decision record**, mapped onto the architecture's decision model rather than invented beside it. A `cell-op-approval` binds:
  - **actor**: the approving operator's key id;
  - **subject**: cell + plan id;
  - **revision**: the plan digest;
  - **permitted act**: apply exactly this plan's actions;
  - **expiry**: the plan TTL;
  - **consumption**: once.

  It carries an Ed25519 signature by an **operator-tagged key** provisioned in the cell's trust store under the existing comms custody rules. Agent-facing surfaces never hold an operator key, so they cannot fabricate the act; the cockpit produces the record from a human click. When the evidence-and-decisions owner ships its decision contract, `cell-op-approval` becomes a profile of it. That reconciliation is a recorded follow-up, not a rival schema.
- **Apply** (`<verb> --apply <plan-id> --approval <file>` in the CLI; the same request in the API) runs two independent layers:
  - **Layer 1, approval:** a missing approval, an unknown or non-operator key, a bad signature, or a signature over another plan id or digest is refused `approval-invalid` (exit 5) before any probe runs.
  - **Layer 2, re-measure:** any precondition that differs from the plan is refused `plan-stale` (exit 5), and nothing runs.

  If both pass, it performs exactly the planned actions, records the approval and the result in the plan, and marks the plan consumed.
- **No blanket consent.** There is no `--yes`, no flag that skips either layer, and no apply without an approval record. On a TTY the CLI may keep a typed `[y/N]` prompt for the operator at that terminal; that path still goes through a plan internally, with the approval signed locally by the operator's key.
- Expired, consumed, malformed or foreign-cell plans and approvals are refused (exit 5), with the reason.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- This brief ships the contract, the package and the CLI adapter mechanics. Adopting the contract in each verb belongs to that verb's brief. No HTTP server is built here.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `spec/cell-op-v1.md` (planned) and `schemas/cell-op-v1.json` (planned) for result, plan and approval, including the decision-model mapping above.
2. Implement `cellop`: types, plan store (atomic create, 0600, 10-minute TTL, digest, single consume), approval verification against operator-tagged trust-store keys, and the apply harness.
3. Implement the CLI adapter mechanics (`--json` writer, `--plan`, `--apply --approval`, `cellctl approve`) and the flag-walk test.
4. Write `docs/cellctl-machine-seam.md` (planned) with a worked plan → approve → apply example; changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./internal/cellop/ -count=1` | exit 0 |
| 2 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyRefusesUnsignedApproval' -count=1` | exit 0 — no approval or an empty one is refused `approval-invalid`, and no precondition probe is called |
| 3 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyRefusesWrongKey' -count=1` | exit 0 — a signature by a key absent from the trust store, and one by the cell's desk (non-operator) key, are both refused |
| 4 | check +mutation | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyRefusesApprovalForOtherPlan' -count=1` | exit 0 — an approval for plan A presented with plan B, and an approval whose plan file was edited after signing (digest changed), are refused; a mutant that checks the id but not the digest fails |
| 5 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestValidApprovalStillReverifies' -count=1` | exit 0 — a correctly signed approval for a plan whose precondition changed is refused `plan-stale` (layer 2 holds with layer 1 passed) |
| 6 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanSingleUseAndTTL' -count=1` | exit 0 — a second apply, an apply after 10 minutes, and an apply of another cell's plan are each refused |
| 7 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestStdoutOnlyDocument' -count=1` | exit 0 — a harness that logs progress still leaves stdout parseable as exactly one schema-valid result |
| 8 | check +dereference | `go -C tools/desk test ./internal/cellop/ -run 'TestContractExamplesValidate' -count=1` | exit 0 — every JSON example in `spec/cell-op-v1.md` (planned) and `docs/cellctl-machine-seam.md` (planned) is extracted and validated against `schemas/cell-op-v1.json` (planned) |
| 9 | check +flow | `go -C tools/desk test ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1` | exit 0 — through the built binary: `--plan --json`, `cellctl approve` with a fixture operator key, `--apply --approval` in a second process; a third apply is refused |
| 10 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoBlanketConsentFlag' -count=1` | exit 0 — no registered cellctl command defines `yes`, `y`, `assume-yes` or `skip-verify` |
| 11 | check | `statusgen --root . --lint` | exit 0 |
| 11a | check | `statusgen --root . --consumers --brief desk-containers/15` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no: the package acts on nothing by itself. The reviewer checks that rows 2–5 fail against a mutant that skips either layer, and that the approval record maps onto actor/subject/revision/act without adding a second decision schema.
