---
brief: assay:assay:desk-containers:15
title: "The cell-operation contract — one versioned result and plan schema shared by the future Cell API and the CLI; apply only as an interactive-terminal confirmation, with approval-based apply deferred"
why: >-
  The cell-level operator verbs (up, drain, restart, status, doctor, logs, gc, upgrade) will
  be driven both from a terminal and, later, from the Cell API that the cockpit and other
  clients use. If the CLI invents its own output and its own plan format, the API will either
  copy it or diverge, and every client will screen-scrape. This brief defines the operation
  result and the operation plan once, as a versioned contract both surfaces consume, so
  results, status and plans can be read and shown everywhere now. Applying a plan stays an
  interactive confirmation at the operator's own terminal, as today, and it is described
  honestly: a convenience confirm, not an authorization control. A real approval needs
  isolation from the desks' account that does not exist yet, so approval-based apply is
  deferred until it does.
wave: 7
depends: []
unblocks: ["desk-containers/17", "desk-containers/18", "desk-containers/20", "desk-containers/21", "desk-containers/22", "desk-containers/24"]
effort: M
gate: human
design: DR-cell-op-contract
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Public Assay includes the control plane and cockpit — 'Use one versioned HTTP/JSON contract for UI and remote clients … CLI and MCP are adapters over those capabilities … Keep the existing human-action absences on agent-facing surfaces'"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Runtime durability — 'A human decision binds a human actor, subject, revision and permitted act; agent/MCP surfaces cannot fabricate that act. Exact custody deployment remains a design decision before activation'"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) §Approvals: deferred — the future approval design and the five properties that gate ever enabling approval-based apply, owned by R7/R3"
  - "Security reviews 5521033873 and 5521336354, correctness reviews 5521049854 and 5521344748 — every approval design reviewed so far was reachable by a desk running under the operator's own OS account (presence is not intent; a desk-authored registry; an unpinned enrollment root; a desk-writable consumed set)"
  - "The driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge): defer approvals; no approval-based or non-interactive apply until R7/R3 provide real isolation from the desk account; keep the result/plan contract shipping now"
  - "docs/cellctl-go-cache.md — `cellctl cache <cell> status` already emits a versioned JSON document (could-not-check as null): the precedent for the result type"
  - "freshness-checked 2026-10-14 @ 9a8df91dc (origin/main) — no cross-verb result or plan schema exists in tools/desk/cmd/cellctl"
gate-why: >-
  The human confirms the honest single-point-of-failure statement. Today a state change that a
  desk starts has no second authorization layer: the interactive confirm is a convenience that
  a same-account process could drive, and isolating desks from the operator's account is R3's
  job. The human also confirms that approval-based and non-interactive apply are deferred, not
  built, until R7/R3 satisfy the five approval properties recorded in amendment 21.
exec-tier: strong
exec-tier-why: (a) the result and plan schema are design decisions the API and later briefs build on; (b) two consumers (CLI now, API later) must read identical meanings; (c) atomic consumption and re-measure are concurrency plumbing
domain: complicated
consumers:
  - "spec/cell-op-v1.md and schemas/cell-op-v1.json (the shared contract): fixed-here"
  - "tools/desk/internal/cellop (result and plan types, plan store, atomic consume, confirm-time re-measure harness): fixed-here"
  - "tools/desk/cmd/cellctl (CLI adapter mechanics: `--json`, `--plan`, interactive `--apply`; the build-tagged fixture verb; each verb brief then adopts the contract with its own rows, 17–24): fixed-here"
  - "the future approval design (operator key, registry, enrollment, signed payload) and its five gating properties: out-of-scope (owned by R7/R3; recorded in docs/proposals/non-ci-architecture/21-cell-operator-surface.md §Approvals: deferred)"
  - "the R7 Cell API (serves the same result and plan documents; read/plan-only, no apply): out-of-scope (stream `cell-api`, parked; requirements in docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
  - "the console cockpit (renders results and plans; does not apply): out-of-scope (the console cockpit lives in a separate repository; follow-up PR there)"
---

# Brief 15 — The cell-operation contract

## Context
files:
- `spec/cell-op-v1.md` (planned) — the contract: result, plan, consumption, exit codes, the apply-confirm rule and its stated limit
- `schemas/cell-op-v1.json` (planned) — JSON Schema for the result and plan documents
- `tools/desk/internal/cellop/` (planned) — `result.go`, `plan.go`, `consume.go`, `apply.go` (re-measure, then consume, then run), tests
- `tools/desk/cmd/cellctl/cellop_fixture.go` (planned) — a fixture verb compiled only under the `cellopfixture` build tag, so this brief's end-to-end row runs before any real verb adopts the contract
- `tools/desk/cmd/cellctl/consent_flag_test.go` (planned), `plan_roundtrip_test.go` (planned)
- `docs/cellctl-machine-seam.md` (planned)
- `changelog/desk-containers-15-cell-op-contract.md` (planned)

single-point-of-failure: today, NONE behind the operator's own account — a state change a desk starts has no second authorization layer. The interactive confirm, the re-measure and the atomic consume prevent mistakes and races, not a determined same-account process. Isolation from the desks' account is R3's job, and approval-based apply waits for it (amendment 21 §Approvals: deferred).

facts:
- **One contract, two adapters.** `cell-op-v1` defines the **result** and the **plan**. The CLI emits them now; the R7 Cell API serves the same documents later. Neither adapter may add a field the other cannot carry. The API path is **read/plan-only**; it serves no apply.
- **Result** (`cell-op-result`): `verb`, `cell`, `started`/`finished`, `outcome` (`ok` | `partial` | `refused` | `failed` | `could-not-check`), `roles[]`, `actions[]`, `plan_id`, `next[]`. Unmeasured values are `null`. Exit codes: 0 ok, 1 failed/partial, 5 refused, 6 could-not-check. Read verbs (status, logs) report per-field nulls inside an `ok` result and exit 6 only when the cell itself cannot be loaded.
- **Plan** (`cell-op-plan`):
  - verb, `cells[]` (every cell it affects, sorted), the action list, and each action's precondition evidence;
  - created time and a fixed **10-minute** expiry;
  - `digest`, the sha256 of the plan's RFC 8785 (JCS) canonical JSON with the `digest` field omitted;
  - a `confirm_code`: the first 8 hex characters of the digest, shown to the operator.

  Plans are stored 0600 under the cell's `run/plans/`. `<verb> --plan [--json]` writes and prints a plan and changes nothing else; it works anywhere, including off a TTY and through the API.
- **Batch plans** list ordered child plans (id + digest). **The batch plan itself is consumed atomically before its first child runs.** Each child is re-measured and consumed immediately before its own step, and a child found already consumed **stops the sequence**, never skip-and-continue. The batch expiry bounds the start of each step.
- **State-changing invocations.** These are refused off a TTY: `up --apply` (reconcile); `drain`, `pause`, `resume` and `restart` in every form except `--plan`; `doctor --apply`; `gc --run` / `gc --apply`; `upgrade --apply`, plus any future `--apply`. Each registers the cobra annotation `cellop:state-changing`. Apply is reachable **only** through these annotated invocations. `drain`, `pause`, `resume` and `restart` apply on a TTY after the typed confirm. **Bare `doctor` and bare `upgrade` never apply and never write:** they print what they would do and write no plan; their plans come only from `doctor --plan` / `upgrade --plan`, and apply only through `--apply`. Read-only: `status`, `logs`, bare `doctor`, bare `gc`, bare `upgrade`, and any `--plan`.
- **Lifecycle primitives `up` and `down` are allowed off a TTY, and are classed identically in brief 17 and amendment 21.** `up` only STARTS roles that are not running; reconciling stale state needs `up --apply`, a state-changing invocation. `down` stops the cell: for each cadence-mode role it writes the role's STOP file, waits up to 20 s for the lease, and refuses if a pass is unfinished (`stopCadences`, `tools/desk/cmd/cellctl/cadence.go`); interactive-mode role sessions are not waited on but hard-killed with the cockpit session (`tools/desk/cmd/cellctl/down.go`), so their in-flight agent work dies, which is harsher than `drain --now`. With role arguments, `down <cell> <role...>` is the per-role interim stop (desk-containers/18): it stops only the named roles' cadence and closes only their window after re-verifying the window's identity against the live cockpit (cell, session and cell-qualified `<cell>-<role>` name, plus the harness pid and start time), refuses on any mismatch or stale receipt, refuses unless each role is already paused or drained and re-measures idle per its mode (work-safety guards, not authorization), and so does not close a sibling's or another cell's window. Off a TTY, `down` followed by `up` approximates a restart, so the off-TTY refusals of `restart` and `drain --now` are mistake guards against an accidental composite or abrupt stop. They are **not** a security boundary, consistent with the confirm being a convenience while approvals are deferred.
- **Apply is an interactive-terminal confirmation only.** Every state-changing invocation requires stdin and stdout to be a TTY; off a TTY it is refused `not-interactive` (exit 5). It then:
  1. shows the plan, its cells and its `confirm_code`;
  2. requires the operator to type that code;
  3. re-measures every precondition, refusing `plan-stale` (exit 5) if any changed;
  4. **consumes atomically** by exclusive creation of `run/plans/consumed/<plan-id>`, refusing `already-consumed`;
  5. only then runs the actions.

  A failure after consumption leaves the plan consumed with a failed result.
- **What the confirm is, honestly.** It is a guard against operator mistakes and concurrent races, not an authorization control. A same-account process can drive a terminal and can delete the consumed marker; a deleted marker allows another interactive apply of the same plan within its remaining expiry, still subject to the re-measure. This limit is stated in `spec/cell-op-v1.md` (planned) and in each verb's `--help`.
- **Deferred, not built here:** an operator key, a key registry, enrollment, a signed approval payload, `cellctl approve`, `--apply --approval`, and any non-interactive or API apply. The future design and its five gating properties are in amendment 21; nothing in this brief may implement part of it.
- **Desk-driven apply is out of contract.** A desk driving a state-changing invocation through a pseudo-terminal (for example `script` or `expect`) is out of contract, and the spec says so. As a mistake guard only, never as a control, every state-changing invocation refuses `desk-session` when the desk-session markers are set (`ASSAY_CELL`, `DESK_LOOP`, `DESK_SESSION`); a determined process can unset them. An operator shell that has inherited a cell's environment is refused too, and must clear those variables first.
- **No blanket consent.** There is no `--yes` and no flag that skips the confirm, the re-measure or the consume.

## Human decision
Should applying a cell-operation plan stay an interactive confirmation at the operator's terminal for now, with signed approvals deferred until desks are isolated from the operator's account?

Several designs for a signed operator approval were reviewed. Each one could be defeated by an AI desk running under the operator's own user account: by raising a real presence prompt the operator cannot tell apart, by pointing verification at a key list it wrote, or by deleting the record that a plan was used. Until desks run with real isolation from that account, an approval would look stronger than it is.

Option 1 is the driver's stated preference (relayed, pending ratification via the medici-finance/assay#2890 merge).

Options:
1. **Defer approvals** (recommended) — ship plans, results and status now. Applying a plan is a typed confirmation at the operator's own terminal, documented as a convenience rather than a security control. Signed approvals wait until the five properties in amendment 21 can be met by the isolation work (R7/R3).
2. **Build signed approvals now** — implement the future design immediately, accepting that until isolation exists it can be bypassed by a desk under the same account.
3. **No apply from cellctl at all** — plans are display-only, and the operator runs each underlying command by hand.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows. Stop at `implemented`.
- This brief ships the result and plan contract, the plan store, the interactive apply mechanics and the fixture verb. Each verb's adoption is that verb's brief, with its own rows. No HTTP server and no approval mechanism is built here.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `spec/cell-op-v1.md` (planned) and `schemas/cell-op-v1.json` (planned), including the batch rule, the interactive-apply rule and the honest limit.
2. Implement `cellop`: result and plan types, plan store (0600, 10-minute expiry, JCS digest, confirm code), atomic consume and the batch-first rule, and the confirm-time re-measure harness.
3. Implement the CLI mechanics (`--json`, `--plan`, interactive `--apply`), the build-tagged fixture verb and the flag-walk test.
4. Docs and changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `go -C tools/desk test ./internal/cellop/ -count=1 -v` | exit 0; output contains `--- PASS` for every named test below |
| 2 | check +dereference | `go -C tools/desk test ./internal/cellop/ -run 'TestContractExamplesValidate' -count=1 -v` | `--- PASS` — every JSON example in `spec/cell-op-v1.md` (planned) and `docs/cellctl-machine-seam.md` (planned) validates against `schemas/cell-op-v1.json` (planned), and each example plan's `digest` recomputes from its JCS form |
| 3 | check +mutation | `go -C tools/desk test ./internal/cellop/ -run 'TestConcurrentApplyRunsOnce' -count=1 -v` | `--- PASS` — 16 concurrent applies of one plan: exactly one runs its actions and the rest are refused `already-consumed`; a mutant that consumes after the effect fails |
| 4 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyReverifies' -count=1 -v` | `--- PASS` — a plan whose precondition changed is refused `plan-stale` and is not consumed |
| 5 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanExpiryAndForeignCell' -count=1 -v` | `--- PASS` — apply after 10 minutes, and apply of another cell's plan, are refused |
| 6 | check | `go -C tools/desk test ./internal/cellop/ -run 'TestMarkerResetBound' -count=1 -v` | `--- PASS` — after the consumed marker is deleted, a second interactive apply of the same plan within expiry runs only if every precondition still matches (the stated bound); after expiry it is refused |
| 7 | check +mutation | `go -C tools/desk test ./internal/cellop/ -run 'TestBatchConsumedFirstAndStops' -count=1 -v` | `--- PASS` — two concurrent applies of one batch plan: exactly one proceeds, because the batch is consumed before its first child; a run that finds a child already consumed stops the sequence; a mutant that skips consumed children fails |
| 8 | check +mutation | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestEveryStateChangingInvocationRequiresTTY' -count=1 -v` | `--- PASS` — walks EVERY command and flag annotated `cellop:state-changing` (not only the enumerated list), and, as arm (b), checks every registered flag by behaviour rather than by name: the fixture harness instruments the plan-apply path, invokes each flag on a pseudo-terminal against stubs, and any flag whose path reaches an apply must carry the `cellop:state-changing` annotation on the flag itself (an unannotated one fails, whatever it is called). Every registered flag named `--apply`, `--run`, `--now` or `--foreign` must carry it too, whether or not it reached an apply in the run (the union of the behavioural and the four-name checks); each flag is driven with a valid fixture plan id. In addition, for each invocation in the enumerated list (`up --apply` (reconcile); `drain`, `pause`, `resume` and `restart` in every form except `--plan`; `doctor --apply`; `gc --run` / `gc --apply`; `upgrade --apply`), the test FAILS if it is not registered and annotated `cellop:state-changing`. The fixture build registers a stand-in for each one whose verb brief has not landed, and the real verb replaces it, so a renamed or dropped flag cannot pass silently. Each is run with stdin or stdout not a TTY and must be refused `not-interactive`, while each verb's `--plan` form must still succeed off a TTY. A mutant that exempts any one invocation fails. Each verb brief 17–24 re-runs this row once its verb exists |
| 8b | check | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestApplyRefusesDeskSession' -count=1 -v` | `--- PASS` — with any of `ASSAY_CELL`, `DESK_LOOP` or `DESK_SESSION` set, every state-changing invocation on a pseudo-terminal is refused `desk-session` (a mistake guard, stated as such) |
| 9 | check | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoApprovalSurface' -count=1 -v` | `--- PASS` — no registered cellctl command defines `approve`, `--approval`, `yes`, `y`, `assume-yes` or `skip-verify` |
| 10 | check +flow | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1 -v` | `--- PASS` — through the binary built with the fixture verb, on a pseudo-terminal: `--plan --json`, then `--apply` typing the confirm code, then a second apply refused `already-consumed` |
| 11 | check | `go -C tools/desk run ./cmd/cellctl/ cellop-fixture 2>&1 \| grep -c 'unknown command "cellop-fixture"'` | 1 — a default build compiles and reports the fixture verb as unknown (a compile failure prints no such line and fails this row) |
| 12 | check | `statusgen --root . --lint` | exit 0 |
| 12a | check | `statusgen --root . --consumers --brief desk-containers/15` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (see gate-why). The reviewer answers both core-system questions honestly: the single control between a desk and a state change it starts is NONE beyond the operator's account today (stated in the SPOF line and the spec); rows 3, 4 and 7 prove the lower layers (re-measure and atomic consume) against races, and are not presented as authorization.
