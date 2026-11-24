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
unblocks: ["desk-containers/17", "desk-containers/18", "desk-containers/20", "desk-containers/21", "desk-containers/22", "desk-containers/24", "desk-containers/28", "cell-api/01", "desk-containers/31"]
effort: M
gate: human
design: DR-cell-op-contract
decision-issue: 2985
ruling: "https://github.com/medici-finance/assay/issues/2985#issuecomment-6480327632"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
decision-trigger: creation
issues: []
schema: brief-v2
version: 3
authored: 2026-10-14 by a worker-desk fan-out session (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Public Assay includes the control plane and cockpit — 'Use one versioned HTTP/JSON contract for UI and remote clients … CLI and MCP are adapters over those capabilities … Keep the existing human-action absences on agent-facing surfaces'"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md §Runtime durability — 'A human decision binds a human actor, subject, revision and permitted act; agent/MCP surfaces cannot fabricate that act. Exact custody deployment remains a design decision before activation'"
  - "docs/proposals/non-ci-architecture/21-cell-operator-surface.md (added by this PR) §Approvals: deferred — the future approval design and the five properties that gate ever enabling approval-based apply, owned by R7/R3"
  - "Security reviews 5521033873 and 5521336354, correctness reviews 5521049854 and 5521344748 — every approval design reviewed so far was reachable by a desk running under the operator's own OS account (presence is not intent; a desk-authored registry; an unpinned enrollment root; a desk-writable consumed set)"
  - "Human ratification of deferred approvals: https://github.com/medici-finance/assay/issues/2985#issuecomment-6272864859; exact desk-containers/15 and DR-cell-op-contract binding: https://github.com/medici-finance/assay/issues/2985#issuecomment-6480327632; no approval-based or non-interactive apply until R7/R3 provide real isolation from the desk account; keep the result/plan contract shipping now"
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
  - "the R7 Cell API (serves the same result and plan documents; read/plan-only, no apply): out-of-scope (stream `cell-api`, active for implementation; requirements in docs/proposals/non-ci-architecture/21-cell-operator-surface.md)"
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
  - `digest`, the sha256 of the plan's RFC 8785 (JCS) canonical JSON with the `digest` and `confirm_code` fields omitted (`confirm_code` is derived from the digest);
  - a `confirm_code`: one lowercase word, shown to the operator. The word list is the plan-id word lists compiled into the tool, adjectives then nouns, in their fixed order. The word is the list entry at the digest's first 8 bytes, read as a big-endian unsigned integer, modulo the list's length; if that entry is either word of the plan's own id, the next entry is taken, wrapping at the end, until it is neither. `Seal` sets it, and the plan check recomputes it from the digest and the id. **Format change:** the delivered first form, the digest's first 8 hex characters, is retired; a stored plan whose `confirm_code` is still 8 hex characters is refused `plan-invalid`, with a detail naming the retired form.

  Plans are stored 0600 under the cell's `run/plans/`. `<verb> --plan [--json]` writes and prints a plan and changes nothing else; it works anywhere, including off a TTY and through the API.
- **Batch plans** list ordered child plans (id + digest). **The batch plan itself is consumed atomically before its first child runs.** Each child is re-measured and consumed immediately before its own step, and a child found already consumed **stops the sequence**, never skip-and-continue. The batch expiry bounds the start of each step.
- **State-changing invocations.** These are refused off a TTY: `up --apply` (reconcile); `drain`, `pause`, `resume` and `restart` in every form except `--plan`; `doctor --apply`; `gc --run` / `gc --apply`; `upgrade --apply`, plus any future `--apply`. Each registers the cobra annotation `cellop:state-changing`. Apply is reachable **only** through these annotated invocations. `drain`, `pause`, `resume` and `restart` apply on a TTY after the typed confirm. **Bare `doctor` and bare `upgrade` never apply and never write:** they print what they would do and write no plan; their plans come only from `doctor --plan` / `upgrade --plan`, and apply only through `--apply`. Read-only: `status`, `logs`, bare `doctor`, bare `gc`, bare `upgrade`, and any `--plan`.
- **Lifecycle primitives `up` and `down` are allowed off a TTY, and are classed identically in brief 17 and amendment 21.** `up` only STARTS roles that are not running; reconciling stale state needs `up --apply`, a state-changing invocation. `down` stops the cell: for each cadence-mode role it writes the role's STOP file, waits up to 20 s for the lease, and refuses if a pass is unfinished (`stopCadences`, `tools/desk/cmd/cellctl/cadence.go`); interactive-mode role sessions are not waited on but hard-killed with the cockpit session (`tools/desk/cmd/cellctl/down.go`), so their in-flight agent work dies, which is harsher than `drain --now`. With role arguments, `down <cell> <role...>` is the per-role interim stop (desk-containers/18): it stops only the named roles' cadence and closes only their window after re-verifying the window's identity against the live cockpit (cell, session and cell-qualified `<cell>-<role>` name, plus the harness pid and start time), refuses on any mismatch or stale receipt, refuses unless each role is already paused or drained and re-measures idle per its mode (work-safety guards, not authorization), and so does not close a sibling's or another cell's window. Off a TTY, `down` followed by `up` approximates a restart, so the off-TTY refusals of `restart` and `drain --now` are mistake guards against an accidental composite or abrupt stop. They are **not** a security boundary, consistent with the confirm being a convenience while approvals are deferred.
- **Apply is an interactive-terminal confirmation only.** Every state-changing invocation requires stdin and stdout to be a TTY; off a TTY it is refused `not-interactive` (exit 5). It then:
  1. shows the plan, its cells and its `confirm_code`;
  2. requires the operator to type that word (case and surrounding space are ignored), refusing `confirm-mismatch` otherwise;
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

Option 1 was [ratified on 2026-10-19](https://github.com/medici-finance/assay/issues/2985#issuecomment-6272864859); the [explicit brief and decision-record binding](https://github.com/medici-finance/assay/issues/2985#issuecomment-6480327632) was recorded on 2026-10-28.

Options:
1. **Defer approvals** (recommended) — ship plans, results and status now. Applying a plan is a typed confirmation at the operator's own terminal, documented as a convenience rather than a security control. Signed approvals wait until the five properties in amendment 21 can be met by the isolation work (R7/R3).
2. **Build signed approvals now** — implement the future design immediately, accepting that until isolation exists it can be bypassed by a desk under the same account.
3. **No apply from cellctl at all** — plans are display-only, and the operator runs each underlying command by hand.

Recorded outcome for #2985: option 1 — defer approvals, with the existing terminal-confirmation limit and the five approval prerequisites retained. The ruling links above record the existing design decision.

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
| 11 | check | `{ go -C tools/desk run ./cmd/cellctl/ cellop-fixture 2>&1; test $? -eq 1; } \| grep -c 'unknown command "cellop-fixture"'` | 1 — a default build compiles and reports the fixture verb as unknown (a compile failure prints no such line and fails this row) |
| 12 | check | `statusgen --root . --lint` | exit 0 |
| 12a | check | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-containers/15 --delivery 6cb553e1a1778c9de1ca22370388fd3a05112718 --base 92d3d10f1f3a614755f7acdbc42db05cf79728f9 > "$o" && cat "$o" && grep -Fxq 'summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing' "$o")` | exit 0 — the `consumers:` routing holds against the branch diff |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Non-implementer verifier run — 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier)

Verification-Attestation: medici-finance/assay#3185 run=10f1a9bafdb723071b2ff64b836036424f8e994b0506c2366defac8e2bf1daa7 source=00d2bb7f165194c0e4c9ef41e5db0cc6d018567d brief=docs/streams/desk-containers/brief-15-cell-op-machine-seam.md model=claude-opus-5-5 tier=any

Ran against merged main 00d2bb7f165194c0e4c9ef41e5db0cc6d018567d on darwin/arm64, go1.27.2, offline (KUBECONFIG=/dev/null), throwaway HOME and TMPDIR, GIT_CONFIG_NOSYSTEM=1, the desk-session markers (ASSAY_CELL, DESK_LOOP, DESK_SESSION) cleared from the row environment. The work was delivered by squash commit 6cb553e1a1778c9de1ca22370388fd3a05112718 (medici-finance/assay#2984); tools/desk/internal/cellop is unchanged between it and the verified head. statusgen for rows 12 and 12a was built from the verified head's own statusgen source, as CI builds it.

Editorial repair only: separate Date and Runner and transcribe the original principal `(on-behalf-of human:ian)` from the [original verifier-authored publication](https://github.com/medici-finance/assay/pull/3188). All fourteen original commands, expectations, observations, dates, verifier identity, attestation, source revision and results are retained; this is not a new verification, execution or acceptance.

The worker applies this one-item editorial exception under the [explicit human permission for this PR](https://github.com/medici-finance/assay/pull/4790#issuecomment-6482184018).

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | go -C tools/desk test ./internal/cellop/ -count=1 -v | exit 0; --- PASS for every named test | exit 0; 13 top-level --- PASS, 0 FAIL, including TestContractExamplesValidate, TestConcurrentApplyRunsOnce, TestApplyReverifies, TestPlanExpiryAndForeignCell, TestMarkerResetBound, TestBatchConsumedFirstAndStops; ok github.com/medici-finance/assay/tools/desk/internal/cellop 0.218s | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | go -C tools/desk test ./internal/cellop/ -run 'TestContractExamplesValidate' -count=1 -v | --- PASS; every JSON example validates and each example plan digest recomputes from JCS | exit 0; --- PASS: TestContractExamplesValidate. Independent recompute (sha256 of sorted-key compact JSON minus digest and confirm_code, equal to JCS for these number-free ASCII documents): all 5 example plans (4 in spec/cell-op-v1.md, 1 in docs/cellctl-machine-seam.md) digest-match and confirm_code equals the first 8 hex of the digest; each expires exactly 10 minutes after created | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | go -C tools/desk test ./internal/cellop/ -run 'TestConcurrentApplyRunsOnce' -count=1 -v | --- PASS; a consume-after-effect mutant fails | exit 0; --- PASS: TestConcurrentApplyRunsOnce. Mutation half, in a scratch clone at the same head: the mutations.json "consume after the effect" mutant applied, the same test exits 1 with --- FAIL: TestConcurrentApplyRunsOnce | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | go -C tools/desk test ./internal/cellop/ -run 'TestApplyReverifies' -count=1 -v | --- PASS; changed precondition refused plan-stale, not consumed | exit 0; --- PASS: TestApplyReverifies | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | go -C tools/desk test ./internal/cellop/ -run 'TestPlanExpiryAndForeignCell' -count=1 -v | --- PASS; expired and foreign-cell applies refused | exit 0; --- PASS: TestPlanExpiryAndForeignCell | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | go -C tools/desk test ./internal/cellop/ -run 'TestMarkerResetBound' -count=1 -v | --- PASS; deleted marker allows re-apply within expiry only if preconditions match; refused after expiry | exit 0; --- PASS: TestMarkerResetBound | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | go -C tools/desk test ./internal/cellop/ -run 'TestBatchConsumedFirstAndStops' -count=1 -v | --- PASS; batch consumed before first child; consumed child stops the sequence; skip mutant fails | exit 0; --- PASS: TestBatchConsumedFirstAndStops with 5 subtests PASS (concurrent applies: exactly one proceeds; a consumed child stops the sequence; a stale child stops the sequence; batch expiry bounds each step; a mismatched child is refused before anything). Mutation half, scratch clone: "batch not consumed before its first child" and "skip a consumed child and continue" mutants each make this test exit 1 (--- FAIL) | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestEveryStateChangingInvocationRequiresTTY' -count=1 -v | --- PASS; every annotated invocation refused not-interactive off a TTY, --plan works off a TTY, exempting any one fails | exit 0; --- PASS: TestEveryStateChangingInvocationRequiresTTY (3.62s) with 7 subtests PASS: enumerated list registered and annotated; apply, run, now and foreign annotated; refused off a terminal; --plan works off a terminal; bare read-only forms stay read-only off a terminal; every arrival at apply is annotated; exempting any one invocation fails | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8b | go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestApplyRefusesDeskSession' -count=1 -v | --- PASS; desk-session refusal on a pseudo-terminal | exit 0; --- PASS: TestApplyRefusesDeskSession (1.39s) | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | go -C tools/desk test ./cmd/cellctl/ -run 'TestNoApprovalSurface' -count=1 -v | --- PASS; no approve / --approval / yes / y / assume-yes / skip-verify | exit 0; --- PASS: TestNoApprovalSurface | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1 -v | --- PASS; --plan --json, typed-code apply, second apply refused already-consumed | exit 0; --- PASS: TestPlanApplyRoundTrip (1.08s) | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 11 | go -C tools/desk run ./cmd/cellctl/ cellop-fixture 2>&1 \| grep -c 'unknown command "cellop-fixture"' | 1 | pipeline exit 0; printed 1. Default build compiled and printed cellctl: unknown command "cellop-fixture" for "cellctl" (go run exit 1); same count 1 with /usr/bin/grep | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12 | statusgen --root . --lint | exit 0 | exit 0; LINT: PASS (NOTICE lines only, none for desk-containers/15) | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 12a | statusgen --root . --consumers --brief desk-containers/15 | exit 0; consumers routing holds against the branch diff | Literal command on the merged head: exit 2, the tool reports the brief is not in the diff against 00d2bb7f (the merged tree has no branch diff) and corroborates or disproves nothing, then names the re-run against the delivering change's own diff. That re-run, in a scratch clone at the squash commit 6cb553e1 with --base 92d3d10f1f3a614755f7acdbc42db05cf79728f9 (its parent): exit 0; summary: 3 corroborated, 0 disproved, 3 unchecked — the three fixed-here entries CORROBORATED, the three out-of-scope entries unchanged since the merge-base | 2026-10-16 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Supporting (maps to the +mutation class of rows 3, 7 and 8): go -C tools/desk run ./cmd/muhar -spec internal/cellop/mutations.json, in a scratch clone at 00d2bb7f — exit 0; Harness healthy: baseline GREEN, positive control CAUGHT; Totals: 13 caught, 0 NOT CAUGHT, 0 could-not-mutate (includes "no pre-run terminal gate", "exempt the variant flags of non-command verbs", "apply off a terminal", "an approval flag"). Observation: internal/cellop/mutations.json is absent from the truth-suite workflow's mutation matrix, so no CI job executes this sweep.

Risk-bearing values (frontmatter risk: all no, irreversible: no; enumerated over the cellop package and the brief's Deliverables; every entry is reversible by an edit and a release):
- RISK-VALUE: DERIVED — Expiry = 10 * time.Minute @ tools/desk/internal/cellop/plan.go:20 — the brief's settled "fixed 10-minute expiry"; the example plans in the spec carry created/expires exactly 600 s apart, recomputed above.
- RISK-VALUE: DERIVED — ConfirmLen = 8 @ tools/desk/internal/cellop/plan.go:23 — the brief's "first 8 hex characters of the digest"; independently recomputed for all 5 example plans.
- RISK-VALUE: DERIVED — exit codes 0 / 5 / 6 / 1 @ tools/desk/internal/cellop/result.go:48, 50, 52, 54 — the brief's exit-code table (0 ok, 1 failed or partial, 5 refused, 6 for an unmeasurable outcome), with any outcome outside the contract mapping to 1, never 0.
- RISK-VALUE: DERIVED — plan and consumed-marker file mode 0o600 @ tools/desk/internal/cellop/consume.go:53, 116 (directories 0o700 @ consume.go:45, 113) — the brief's "Plans are stored 0600"; consume uses O_EXCL creation, the atomic-consume rule.

**VERIFY: PASS** — 14/14 rows; row 12a satisfied by the corroboration run against the delivering commit's own diff (the literal form has no branch diff on merged main). Gate stays human: this records Evidence only, with no status change.

### Non-implementer execution witness — 2026-11-01

Ran the whole Verify table through the installed statusgen v1.0.67 `verifyrun --brief` on merged main ab6803d9f65c63121d2272bc140d306220d7ccba, darwin/arm64, go1.27.2, offline. The desk-session markers stayed set in the row environment. The table below is the tool's output, unaltered: 12 pass, 2 fail.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `go -C tools/desk test ./internal/cellop/ -count=1 -v` | pass exit=0 | sha256:450272ef898e | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 2 | `go -C tools/desk test ./internal/cellop/ -run 'TestContractExamplesValidate' -count=1 -v` | pass exit=0 | sha256:6f6a77862146 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 3 | `go -C tools/desk test ./internal/cellop/ -run 'TestConcurrentApplyRunsOnce' -count=1 -v` | pass exit=0 | sha256:8408af54bd56 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 4 | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyReverifies' -count=1 -v` | pass exit=0 | sha256:acaea49c47c4 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 5 | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanExpiryAndForeignCell' -count=1 -v` | pass exit=0 | sha256:8810239fa34b | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 6 | `go -C tools/desk test ./internal/cellop/ -run 'TestMarkerResetBound' -count=1 -v` | pass exit=0 | sha256:72d91eceed17 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 7 | `go -C tools/desk test ./internal/cellop/ -run 'TestBatchConsumedFirstAndStops' -count=1 -v` | pass exit=0 | sha256:a55dea2cb116 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 8 | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestEveryStateChangingInvocationRequiresTTY' -count=1 -v` | pass exit=0 | sha256:4eea0a2e440d | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 8b | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestApplyRefusesDeskSession' -count=1 -v` | pass exit=0 | sha256:4d1f2176ae03 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 9 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoApprovalSurface' -count=1 -v` | pass exit=0 | sha256:497f3ec6ee33 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 10 | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1 -v` | pass exit=0 | sha256:3d07dc38b39d | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 11 | `go -C tools/desk run ./cmd/cellctl/ cellop-fixture 2>&1 \| grep -c 'unknown command "cellop-fixture"'` | fail exit=1 | sha256:4355a46b19d3 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --lint` | fail exit=1 | sha256:3eb7c5a646c0 | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
| 12a | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-containers/15 --delivery 6cb553e1a1778c9de1ca22370388fd3a05112718 --base 92d3d10f1f3a614755f7acdbc42db05cf79728f9 > "$o" && cat "$o" && grep -Fxq 'summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing' "$o")` | pass exit=0 | sha256:e684a18755ff | 2026-11-01 | assay-verifier-app[bot] @ ab6803d9f65c (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5163 run=0df0c541b531dda904cba89445431c07d750af6354bee6c270d5fee8016abc86 source=ab6803d9f65c63121d2272bc140d306220d7ccba brief=docs/streams/desk-containers/brief-15-cell-op-machine-seam.md model=claude-opus-5-5 tier=strong

Row 11, primary output: the pipeline printed `1` (the witness hash sha256:4355a46b19d3 is the hash of that one line). `go run` printed `cellctl: unknown command "cellop-fixture" for "cellctl"` and exited 1, and verifyrun runs rows under `bash -o pipefail`, so the pipeline's status is 1 where the harness expects 0. The behaviour the Expect names was observed; the row as written cannot pass this runner.

Row 12, primary output: `PROBLEM: desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required` and `LINT: FAIL 1 problem(s)`. That is another brief's protected table, and the row's literal form lacks the flag the repository's own lint workflows pass. No PROBLEM names this brief. Supplemental probe, no acceptance credit: `statusgen --root . --lint --acceptance-forge` with a verifier token, on this tree including this witness, exited 0 with `LINT: PASS`.

Rows 1-10, 12a, read beyond exit status: every `-run` selector matched and ran with zero skips and zero failures in a separate inspection run of the same commands (row 1: 13 top-level tests, including all six named; row 8: seven subtests). Row 12a printed `summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing`; the three unchecked out-of-scope entries are the reviewer's call.

Supporting, no acceptance credit (the +mutation half of rows 3, 7 and 8): `go -C tools/desk run ./cmd/muhar -spec internal/cellop/mutations.json` in a scratch clone at the same commit, exit 0: `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 13 caught, 0 NOT CAUGHT, 0 could-not-mutate.` The truth-suite workflow's mutation matrix still does not list this spec.

Risk-bearing values (frontmatter risk: all no, irreversible: no), re-read at this commit over the cellop package; every entry is reversible by an edit and a release:
- RISK-VALUE: DERIVED — Expiry = 10 * time.Minute @ tools/desk/internal/cellop/plan.go:20 — the brief's fixed 10-minute expiry.
- RISK-VALUE: DERIVED — ConfirmLen = 8 @ tools/desk/internal/cellop/plan.go:23 — the brief's first 8 hex characters of the digest.
- RISK-VALUE: DERIVED — exit codes 0 / 5 / 6 / 1 @ tools/desk/internal/cellop/result.go:48, 50, 52, 54 — the brief's exit-code table.
- RISK-VALUE: DERIVED — file mode 0o600 @ tools/desk/internal/cellop/consume.go:53, 116 (directories 0o700 @ consume.go:45, 113) — the brief's plans stored 0600, with O_EXCL creation as the atomic consume.

**VERIFY: FAIL** — rows 11 and 12 exit 1 against their Expect, as witnessed above; 12 of 14 rows pass. Both traces end in the row definition under the current runner and lint, with no cellop or cellctl behaviour observed wrong. Closure needs a reviewed definition repair of those two rows and a fresh run. Gate stays human: Evidence only, with no status change.
### Independent bounded verification — 2026-11-20

Source ca3f1662db1dcd0ba0ee0c2176be9c9897295d73; installed statusgen v1.0.90; darwin/arm64. Exact admitted ordered v2 descriptor corroborated independently through verifier App reads. Both transition approvals and the repair permission/ruling are unedited trusted human comments (created_at equals updated_at). No source, Verify, index or config changes. Grounding preceded delivery diff/test inspection.

The following is the native installed runner witness, preserved verbatim. A machine pass below is exit-status evidence; required assertions and unavailable instruments are accounted separately afterwards.

Acceptance-Changes: 11=Command,12a=Command
Verification-Table: delivery=6cb553e1a1778c9de1ca22370388fd3a05112718 base=92d3d10f1f3a614755f7acdbc42db05cf79728f9 original=92d3d10f1f3a614755f7acdbc42db05cf79728f9
Acceptance-Transition: link=1 amendment=3b87b83e5b4a0a4819ce0f8ca3ea9035a1f7b56f approval=https://github.com/medici-finance/assay/issues/7257#issuecomment-7048857807
Acceptance-Transition: link=2 amendment=33b0f545ab3ab7d9a7ccd9a771ddfda7ffcfd7a2 approval=https://github.com/medici-finance/assay/issues/7257#issuecomment-7048859362
Evidence-Repair: commit=941e3096c939c15c03092e3752154164795eb13d approval=https://github.com/medici-finance/assay/pull/4790#issuecomment-6482184018

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `go -C tools/desk test ./internal/cellop/ -count=1 -v` | pass exit=0 | sha256:7b4822cd76b4 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `go -C tools/desk test ./internal/cellop/ -run 'TestContractExamplesValidate' -count=1 -v` | pass exit=0 | sha256:6ef01e0adb70 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `go -C tools/desk test ./internal/cellop/ -run 'TestConcurrentApplyRunsOnce' -count=1 -v` | pass exit=0 | sha256:318244a7da41 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyReverifies' -count=1 -v` | pass exit=0 | sha256:3aabaf2750d8 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanExpiryAndForeignCell' -count=1 -v` | pass exit=0 | sha256:36ea52486700 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `go -C tools/desk test ./internal/cellop/ -run 'TestMarkerResetBound' -count=1 -v` | pass exit=0 | sha256:b6f0b068c304 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `go -C tools/desk test ./internal/cellop/ -run 'TestBatchConsumedFirstAndStops' -count=1 -v` | pass exit=0 | sha256:99e892d988a2 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestEveryStateChangingInvocationRequiresTTY' -count=1 -v` | pass exit=0 | sha256:53f079aef634 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8b | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestApplyRefusesDeskSession' -count=1 -v` | pass exit=0 | sha256:c75fb03d114d | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoApprovalSurface' -count=1 -v` | pass exit=0 | sha256:b0522078ee87 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 10 | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1 -v` | pass exit=0 | sha256:666f616e791f | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 11 | `{ go -C tools/desk run ./cmd/cellctl/ cellop-fixture 2>&1; test $? -eq 1; } \| grep -c 'unknown command "cellop-fixture"'` | pass exit=0 | sha256:4355a46b19d3 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --lint` | fail exit=5 rule51=fail forge-rc=5 forge-out=sha256:3151053843c9 pin=v1.0.90 cred=verifier-run-read cred-digest=sha256:25ea86d21ed3 stamp=sha256:51069b0e1904455e48199d5f286c2700d34bd94a29695ba755323de63e212116 | sha256:684885b16029 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 12a | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-containers/15 --delivery 6cb553e1a1778c9de1ca22370388fd3a05112718 --base 92d3d10f1f3a614755f7acdbc42db05cf79728f9 > "$o" && cat "$o" && grep -Fxq 'summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing' "$o")` | pass exit=0 | sha256:69aff81c47a4 | 2026-11-20 | assay-verifier-app[bot] @ ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7302 run=7bd5d9ad62d8bcc1bb2fd292683056ef4e48351df4f1aff32f5fecfdc4174c29 source=ca3f1662db1dcd0ba0ee0c2176be9c9897295d73 brief=docs/streams/desk-containers/brief-15-cell-op-machine-seam.md model=gpt-6.1-sol tier=strong


Primary inspection: row 1 ran the package tests with no skips; row 2 ran TestContractExamplesValidate; row 9 ran TestNoApprovalSurface. Row 11 printed exactly 1. Row 12a printed summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing. Independent isolated cluster inspected rows 3,4,5,6,7,8,8b,10: every named test executed with zero skips; row 7 has five passing subtests and row 8 seven. Five example plan documents independently recompute their digest and current spec's word confirmation, with expiry exactly 600 seconds.

Row 12 remains uncovered: the native offline command returned 5 and the Rule51 host forge form also returned 5. The witness retains their hashes and the exact failure stamp. The runner did not retain primary row output, so this is not a claim that the implementation failed lint. Source inspection finds configuration refusal exits 5 before evaluation; that is an inference, not recovered primary output. Separate literal bare-lint inspection (no acceptance credit) exited 1 with protected-table approval-required PROBLEMs only on desk-tools/37, desktools-v2/65 and forge-neutral/54. A separate host forge lint is supplemental only and cannot replace the native stamp.

Mutation-control attempts are both retained. First attempt: supported isolated `go -C tools/desk run ./cmd/muhar -spec internal/cellop/mutations.json -j 2` returned wrapper exit 1 (muhar exit 2), HARNESS BROKEN baseline RED; planid_test.go:205 could not open ../../../../schemas/cell-op-v1.json because the worker workspace copied only tools/desk. This control attempt is discarded for env-fault, not hidden. The default in-place invocation was never executed against the admitted home.

Second control attempt, same admitted source and run: a full immutable tracked snapshot from `git archive ca3f1662db1dcd0ba0ee0c2176be9c9897295d73` was extracted outside the admitted home, preserving repository layout. Supported serial `go -C tools/desk run ./cmd/muhar -spec internal/cellop/mutations.json -j 1` executed only there (muhar CLI main.go:269 declares serial in-place). Exit 0; primary summary: `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 18 caught, 1 NOT CAUGHT, 0 could-not-mutate.` Rows 3 and 7's required mutants were CAUGHT: consume after the effect; batch not consumed before its first child; skip a consumed child and continue. Their expected named tests visibly failed under those mutants, establishing discrimination after a clean full-layout baseline.

Supplemental instrument finding, no claim that all nineteen mutants were caught: `exempt the variant flags of non-command verbs` was NOT_CAUGHT. It changes only fixture stand-in `markStateChangingFlag(cmd, v)` to `_ = v`. Reachability inspection explains the survivor: cellop_fixture.go:48–55 gives only gc a non-command variant (`run`); cobra.go:500–505 registers the real gcCmd before calling cellopFixtureCommands; fixture.go:59–60 skips that stand-in. Other non-command stand-ins doctor/upgrade have no variants, and command stand-ins take the other branch. This mutant therefore exempts no actual invocation on this merged source. Row 8's actual acceptance control remains its passing `exempting any one invocation fails` subtest: cellop_consent_tty_test.go:251–267 removes each actual command/flag annotation and requires detection by the behavioural arm. The obsolete external fixture mutant needs mutation-instrument maintenance; it supplies no positive acceptance credit and establishes no observed product failure.


Risk inventory: mechanically enumerated 2,424 literal-bearing source/document/test lines across the delivered change, retained externally with source coordinates; current production literals scanned separately. Highest rank is the ordering/authority contract and private storage; wrong values can cause duplicate actions or unwanted acceptance whose past effects are not undone by a later edit. Next is wire contract/expiry/confirmation consistency; reversible fixture knobs and test durations rank last. No approval-based authority exists; marker/TTY guards are documented mistake guards.

RISK-VALUE: DERIVED — plan/marker file mode = 0o600 @ tools/desk/internal/cellop/consume.go:53,116 — brief requires owner-only plans; consumed markers have the same privacy, and O_EXCL enforces one winner before effects. Directory mode = 0o700 @ consume.go:45,113 follows owner-only traversal.
RISK-VALUE: DERIVED — Expiry = 10 * time.Minute @ tools/desk/internal/cellop/plan.go:19 — brief fixes ten minutes; all five independent examples differ by 600 seconds.
RISK-VALUE: DERIVED — exit codes = 0,5,6,1 @ tools/desk/internal/cellop/result.go:48,50,52,54 — spec maps ok/refused/could-not-check/failure respectively and unknown outcomes must not imply success.
RISK-VALUE: DERIVED — DeskSessionMarkers = {ASSAY_CELL,DESK_LOOP,DESK_SESSION} @ tools/desk/internal/cellop/apply.go:16 — the brief names exactly those three markers; this is a convenience refusal and grants no authorization.
RISK-VALUE: DERIVED — schema/kind = cell-op-v1,cell-op-result,cell-op-plan @ tools/desk/internal/cellop/result.go:21–23 — exact shared contract markers from the brief/spec; schema constraints use the same literals.
RISK-VALUE: NAMED, NOT DERIVED — original ConfirmLen = 8 @ delivery6cb553e1 tools/desk/internal/cellop/plan.go:23 — original brief still says first eight digest hex; later main commit f7019983b19f9a180b59953fddfcd74a88e086c4 replaces it with a one-word code, and current spec/examples match that later design. No claim that the original eight-hex statement remains implemented; this stale wording needs reconciliation by the owning work, not a verifier edit.
RISK-VALUE: DERIVED — digest pattern = ^[0-9a-f]{64}$ @ tools/desk/internal/cellop/plan.go:27 — SHA256 has 256 bits, each hex digit encodes four, therefore 64 lowercase hex digits. Retired confirm regex {8} @ plan.go:30 recognizes/refuses the former shape.
RISK-VALUE: DERIVED — input bound = 256 @ tools/desk/internal/cellop/apply.go:273 — a defensive finite input cap; reversible operational bound ranked last, no irreversibility derivation required. Current plan-id retries maxDraws = 64 @ planid.go:90 also rank last.

**VERIFY: BLOCKED** — all 14 authored rows attempted. 13 native machine passes; row 12 native failure retained without acceptance credit; primary assertions support rows 1–11 and 12a, and required mutation halves of rows 3 and 7 are now supported by discriminating controls. Native host-lint evidence remains unavailable; the external mutation inventory also has one explained dead-fixture survivor, separately reported above. No lifecycle advance or human-gate closure.

Supplemental inspection correction: the first row-9 primary-output extraction accidentally included its Expect text and produced a shell/test-flag error. This diagnostic invocation is discarded as superseded, with no acceptance credit. Exact authored row 9 was rerun successfully (TestNoApprovalSurface RUN/PASS, exit 0); both logs retained. It never changed the native installed witness.

Supplemental host forge lint completed: `statusgen --root . --lint --acceptance-forge`, exit 0, primary terminal line `LINT: PASS`. This used the same installed v1.0.90 binary, source and verifier App token under the invoking configured roster, with no Git configuration edits. It supplies diagnostic evidence only; it does not replace the native Rule51 failure stamp or recover that row’s primary bytes.


Fresh-HOME roster explanation is explicitly SOURCE-BASED INFERENCE, not native primary output: admittedRowEnv in statusgen/verifieradmission.go:166–209 removes HOME/XDG_CONFIG_HOME/DESK_LOOP/ASSAY inputs; admittedRowDirs:277–296 creates fresh HOME and installs a roster only when roster bytes are nonnil. verifyrun.go:2993 takes those bytes from container.roster; a host run has no container roster. runVerifyCommandWith:956 uses that fresh HOME for each row, and Rule51 copies the same rowPlan (:1870) and calls runVerifyCommandWith again (rule51.go:215). Local non-CI scanClassForMode is write class (rosterconfig.go:698); absent home roster yields Problems (:796–805); main.go:1963–1966 then prints configuration REFUSED and exits 5 before lint evaluation. The observed native exit5 and configured supplemental host exit0 are consistent with that chain. Exact excerpts retained in containers15-roster-inference-source.txt. No raw native configuration diagnostic is asserted recovered.

Native lint blocker: https://github.com/medici-finance/assay/issues/7304

### Non-implementer verifier run — 2026-11-22 assay-verifier-app[bot] (claude-opus-5-5 verifier)

Fresh run requested by the driver on merged main ec489e62431cf9625138e38539d24b8abe2633e1 with installed statusgen v1.0.91 (desk tools v1.0.91), darwin/arm64, go1.27.4, offline (KUBECONFIG=/dev/null), the declared verifier Git environment, the normal HOME and GH_TOKEN unset. On a clean home (git status --porcelain empty), deskdispatch --check-verifier --root on this home with this brief exited 0 first, binding `"Run":"f2d888e7e3850b27a5aaa02b0a6678acc98aa541e67a5621f8ad1907bc6f1e18"` with `"capability":"verifier-attestation=checked-clean"`. All 14 rows then ran in one admitted host run, statusgen verifyrun --rule51 --acceptance-forge --acceptance-record on this brief, with the protected record of the 2026-11-20 entry (started 2026-11-22T15:42:57Z, finished 2026-11-22T15:46:49Z, exit 1 because row 12's offline result is recorded fail). Before the rows it printed `acceptance transition: reading approval https://github.com/medici-finance/assay/issues/7257#issuecomment-7048857807`, the same line for https://github.com/medici-finance/assay/issues/7257#issuecomment-7048859362, and `acceptance transition: reading evidence-repair approval https://github.com/medici-finance/assay/pull/4790#issuecomment-6482184018`. The marker block and table below are the tool's witness, unaltered.

Acceptance-Changes: 11=Command,12a=Command
Verification-Table: delivery=6cb553e1a1778c9de1ca22370388fd3a05112718 base=92d3d10f1f3a614755f7acdbc42db05cf79728f9 original=92d3d10f1f3a614755f7acdbc42db05cf79728f9
Acceptance-Transition: link=1 amendment=3b87b83e5b4a0a4819ce0f8ca3ea9035a1f7b56f approval=https://github.com/medici-finance/assay/issues/7257#issuecomment-7048857807
Acceptance-Transition: link=2 amendment=33b0f545ab3ab7d9a7ccd9a771ddfda7ffcfd7a2 approval=https://github.com/medici-finance/assay/issues/7257#issuecomment-7048859362
Evidence-Repair: commit=941e3096c939c15c03092e3752154164795eb13d approval=https://github.com/medici-finance/assay/pull/4790#issuecomment-6482184018

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `go -C tools/desk test ./internal/cellop/ -count=1 -v` | pass exit=0 | sha256:36aaebe122f1 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `go -C tools/desk test ./internal/cellop/ -run 'TestContractExamplesValidate' -count=1 -v` | pass exit=0 | sha256:47e410ca9a56 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `go -C tools/desk test ./internal/cellop/ -run 'TestConcurrentApplyRunsOnce' -count=1 -v` | pass exit=0 | sha256:dbbbf94ce77e | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `go -C tools/desk test ./internal/cellop/ -run 'TestApplyReverifies' -count=1 -v` | pass exit=0 | sha256:6c6251edea4a | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `go -C tools/desk test ./internal/cellop/ -run 'TestPlanExpiryAndForeignCell' -count=1 -v` | pass exit=0 | sha256:098b58d6b0e1 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `go -C tools/desk test ./internal/cellop/ -run 'TestMarkerResetBound' -count=1 -v` | pass exit=0 | sha256:54772f3b87d1 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `go -C tools/desk test ./internal/cellop/ -run 'TestBatchConsumedFirstAndStops' -count=1 -v` | pass exit=0 | sha256:de54a5c84039 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestEveryStateChangingInvocationRequiresTTY' -count=1 -v` | pass exit=0 | sha256:800cf69e39dc | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8b | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestApplyRefusesDeskSession' -count=1 -v` | pass exit=0 | sha256:74fb511e11fa | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `go -C tools/desk test ./cmd/cellctl/ -run 'TestNoApprovalSurface' -count=1 -v` | pass exit=0 | sha256:4d5b68704375 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 10 | `go -C tools/desk test -tags cellopfixture ./cmd/cellctl/ -run 'TestPlanApplyRoundTrip' -count=1 -v` | pass exit=0 | sha256:d8713bfa3fd3 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 11 | `{ go -C tools/desk run ./cmd/cellctl/ cellop-fixture 2>&1; test $? -eq 1; } \| grep -c 'unknown command "cellop-fixture"'` | pass exit=0 | sha256:4355a46b19d3 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --lint` | fail exit=1 rule51=pass forge-rc=0 forge-out=sha256:639e4e41ee31 pin=v1.0.91 cred=verifier-run-read cred-digest=sha256:13821e4fdfc8 stamp=sha256:cb1fe2804e6da7f7fb4f8a4be549d71e9bc0613c28028c54aa839ea353537ca7 | sha256:4bc6d2396683 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 12a | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-containers/15 --delivery 6cb553e1a1778c9de1ca22370388fd3a05112718 --base 92d3d10f1f3a614755f7acdbc42db05cf79728f9 > "$o" && cat "$o" && grep -Fxq 'summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing' "$o")` | pass exit=0 | sha256:c35ca9945fe5 | 2026-11-22 | assay-verifier-app[bot] @ ec489e62431cf9625138e38539d24b8abe2633e1 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7427 run=f2d888e7e3850b27a5aaa02b0a6678acc98aa541e67a5621f8ad1907bc6f1e18 source=ec489e62431cf9625138e38539d24b8abe2633e1 brief=docs/streams/desk-containers/brief-15-cell-op-machine-seam.md model=claude-opus-5-5 tier=strong

Earlier tries under this attestation earn no credit and are superseded by this run. Attempt 1 (2026-11-22T15:30:19Z to 15:33:17Z) ran all 14 rows with statusgen verifyrun --rule51 but without the protected record, so it carried no Verification-Table marker and verifyrun --check refused it; it was discarded from the home and does not land. Attempt 2 (2026-11-22T15:38:14Z to 15:39:15Z) refused before any row ran, exit 2, with `transition execution requires a clean immutable source`, because the home still carried attempt 1's uncommitted edit.

Row 12: the offline bare lint exited 1, and the witness records it as fail. Its output, sha256:4bc6d2396683, carries exactly three PROBLEM lines, none naming desk-containers/15. A separate inspection run of the same command on the same source produced output with the same hash and earns no acceptance credit. The lines are `PROBLEM: desk-tools/37: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required`, the same line for desktools-v2/65 and for forge-neutral/54, then `LINT: FAIL 3 problem(s)`. In the same admitted run, the tool's rule-51 lane ran the pinned binary as --root . --lint --acceptance-forge under the run's verifier read credential and stamped the row: rule51=pass, forge-rc=0, pin=v1.0.91, cred=verifier-run-read, on a clean tree. Rule-51 credit for this row comes only from that tool stamp (docs/brief-rules.md rule 51; ruling A: https://github.com/medici-finance/assay/issues/4823#issuecomment-7015568405), never from this prose. The fresh-HOME roster refusal recorded on 2026-11-20 under https://github.com/medici-finance/assay/issues/7304 did not recur: the forge lane exited 0 at v1.0.91.

I read rows 1-11 and 12a beyond exit status, using a separate inspection run of the same commands on the same source that earns no acceptance credit:
- Row 1 ran 23 top-level tests, all PASS with zero SKIP, including the six tests named in rows 2-7.
- Row 8 ran TestEveryStateChangingInvocationRequiresTTY with seven subtests, all PASS.
- Rows 8b, 9 and 10 each ran their named test to PASS with zero SKIP.
- Row 11's hash, sha256:4355a46b19d3, is the hash of the single line 1.
- Row 12a printed `summary: 3 corroborated, 0 disproved, 3 unchecked, 0 brief(s) claiming nothing`. The three unchecked entries are the out-of-scope consumers, and they are the reviewer's call.

The witness records the +mutation halves of rows 3, 7 and 8 as exit status only. I ran no mutant for this entry.

Risk-bearing values (frontmatter risk: all no, irreversible: no), re-read at ec489e62431c over the cellop package. Each value can be reversed by an edit and a release:
- RISK-VALUE: DERIVED — plan and consumed-marker file mode = 0o600 @ tools/desk/internal/cellop/consume.go:53, 116 (directories 0o700 @ consume.go:45, 113) — the brief says plans are stored 0600. Creation with O_EXCL is the atomic consume.
- RISK-VALUE: DERIVED — Expiry = 10 * time.Minute @ tools/desk/internal/cellop/plan.go:19 — the brief's fixed 10-minute expiry.
- RISK-VALUE: DERIVED — exit codes = 0, 5, 6, 1 @ tools/desk/internal/cellop/result.go:48, 50, 52, 54 — the brief's table: 0 ok, 5 refused, 6 could-not-check, 1 failed or partial. Any other outcome maps to 1, never 0.
- RISK-VALUE: DERIVED — DeskSessionMarkers = {ASSAY_CELL, DESK_LOOP, DESK_SESSION} @ tools/desk/internal/cellop/apply.go:16 — the three markers the brief names, and no others. They guard against mistakes and grant no authority.
- RISK-VALUE: DERIVED — Schema, KindResult, KindPlan = cell-op-v1, cell-op-result, cell-op-plan @ tools/desk/internal/cellop/result.go:21-23 — the contract names in the brief and spec.
- RISK-VALUE: DERIVED — digest pattern = ^[0-9a-f]{64}$ @ tools/desk/internal/cellop/plan.go:27 — sha256 is 256 bits at four bits per hex digit, so 64 lowercase hex digits.
- RISK-VALUE: NAMED, NOT DERIVED — retiredConfirmRE = ^[0-9a-f]{8}$ @ tools/desk/internal/cellop/plan.go:30 — the brief's facts still describe the confirm code as the digest's first 8 hex characters. At this source that form is recognised only so it can be refused: a one-word confirm replaced it after delivery. The brief's wording and the current design disagree, and the owning work should reconcile them; a verifier does not edit them.
- Ranked last, no derivation needed: confirm input bound 256 @ tools/desk/internal/cellop/apply.go:273 and maxDraws = 64 @ tools/desk/internal/cellop/planid.go:90.

**VERIFY: PASS** — 14/14 rows are credited by the tool witness above. Row 12's offline result stays recorded fail, and the row is credited by its rule-51 stamp. The gate stays human: this entry records Evidence only and changes no status.


## Review
Gate: human (see gate-why). The reviewer answers both core-system questions honestly: the single control between a desk and a state change it starts is NONE beyond the operator's account today (stated in the SPOF line and the spec); rows 3, 4 and 7 prove the lower layers (re-measure and atomic consume) against races, and are not presented as authorization.
