---
brief: assay:assay:graph-execution:26
title: Shared loop-admin launch supervision for desks and workflows
why: A reusable launch supervisor lets standing desks and graph stages share provider adapters, cancellation, recovery
  and cost controls without sharing scheduling authority.
wave: 6
depends:
- graph-execution/20
- graph-execution/28
- graph-execution/29
- graph-execution/16
unblocks:
- graph-execution/22
- graph-execution/27
- graph-execution/21
- performance-collection/09
effort: M
gate: human
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'yes'
  sensitive-data: 'no'
gate-why: Adds an authenticated launch endpoint and moves the process runner the process-launch audit scans; the owner confirms callers cannot change role, profile or limits and that the audit keeps sight of every exec site.
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — R2 owner row, R2 amendment scope (journal records process facts against the shared store) and the storage-routing conflict
- docs/proposals/non-ci-architecture/11-target-architecture.md — lines 92, 98, 100 (runtime coordination in PostgreSQL; per-cell database/role)
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md — lines 7, 52, 56 (one runtime implementation; execution-group stop; paused start)
- docs/proposals/non-ci-architecture/19-progressive-adoption-and-oversight.md — line 49 (no SQLite runtime)
- "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
- freshness-checked 2026-10-13 @ 4696d598e58820806d0bcf8c9b2dd87f29115400
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'workflow/controller: follow-up graph-execution/21'
- 'loopadmin/adapters: follow-up graph-execution/22 (consumes the supervisor and the moved process runner)'
- 'tools/desk/internal/loopengine: follow-up graph-execution/27'
- 'tools/desk/internal/cellprocess: follow-up graph-execution/26 (this brief; moved into loopadmin/process; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/26 (this brief; cadence path calls the moved runner; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/internal/forgeban: follow-up graph-execution/26 (this brief; process-launch audit also scans loopadmin; flips to fixed-here when the implementation edits the path)'
- 'operator lifecycle and cockpit: out-of-scope (adopter qualification)'
- 'loopadmin/journal PostgreSQL schema and migrations: follow-up graph-execution/26 (this brief; flips to fixed-here when the implementation edits the path)'
- 'spec/operator-incarnation-v1.md (read before each launch): follow-up graph-execution/16'
version: 7
id: bcbee0b0-8f12-4e7a-b028-0637b2bab0c3
---

# Brief 26 — Shared loop-admin launch supervision for desks and workflows

## Context

files: `loopadmin/supervisor/` (planned), `loopadmin/process/` (planned), `tools/desk/internal/cellprocess/` (moved into loopadmin/process by this brief), `tools/desk/cmd/cellctl/cadence.go`, `tools/desk/go.mod`, `tools/desk/go.sum`, `tools/desk/internal/forgeban/allowlist.go`, `tools/desk/internal/deskkit/forge_surface_test.go`, `docs/cellctl-cadence.md`, `loopadmin/journal/` (planned), `loopadmin/control/` (planned), `loopadmin/cmd/loop-admin/` (planned), `loopadmin/testdata/supervisor/` (planned), `loopadmin/README.md` (planned), `docs/loop-admin.md` (planned), `.github/workflows/loopadmin.yml` (planned), `changelog/graph-execution-26-loop-admin.md` (planned). Added by the R2 amendment (2026-10-13): `loopadmin/journal/migrations/` (planned).

facts: The new loopadmin module is bootstrapped by /20. At main 307fe1699 `tools/desk/internal/cellprocess` already runs one harness invocation and reaps its process group or job, and cellctl's cadence path (`tools/desk/cmd/cellctl/cadence.go`) calls it; the process-launch audit (`TestNoForgeCLIShellout` over the `tools/desk` tree, with the runner's exec site in `tools/desk/internal/forgeban/allowlist.go`) covers it there. `tools/desk/internal/loopengine` owns in-session queue behavior for standing desks; it launches no process. Workflow instance ownership remains outside the supervisor; the shared module has no dependency on the instance store (19). Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the caller-scoped launch endpoint (caller namespace, approved profile, role, generation, shared capacity) — behind it, the per-role cadence lease (27) and the role executors' own credentials, which a direct lower-boundary call still cannot obtain.

risk-answers: `irreversible: yes`, so `gate: human`, as for 14 and 16 in this stream. The supervisor runs offline fixtures only, enrolls no live desk, and holds no effect credential or operator administration, but it adds an authenticated launch endpoint and moves the process runner the process-launch audit scans: once it is implemented and verified, a human check skipped at verification cannot be recovered by a revert. Lowering the gate is a maintainer ruling, not an authoring choice.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Library-first amendment — 2026-10-08

Read `docs/library-first.md`. Reuse the declared component library where it shares authority,
and a typed SDK client where a process boundary enforces authority or isolation. Workflow
progression, process supervision and effect execution remain distinct; a shared binary or
library grants no additional role. Preserve the existing independent fence, no-store standing
mode, process-launch audit and cross-journal recovery tests. No cell taxonomy dependency is added.

## Non-CI R2 amendment — 2026-10-13 (journal on the shared coordination store)

The maintainer ratified the non-CI target (medici-finance/assay#2500, revision 23f655f5d);
approval activates nothing. Owner row: package R2 in the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md). This
section supersedes the **engine choice** in Task 2 and the 2026-10-02 routing that permitted a
per-supervisor embedded journal; every other obligation stands.

- **Engine.** The execution journal records process facts in the cell's PostgreSQL
  coordination database (target 11:92, 11:100), not in an embedded engine on the supervisor's
  filesystem. There is no SQLite runtime and one runtime database implementation (14:7,
  19:49). An unreachable database refuses the launch; nothing falls back to a local file.
- **Ownership unchanged.** The journal still owns process facts only (invocations, receipts,
  lifecycle, limits, usage, checkpoints). Work, decisions and acceptance stay with desk queues
  and graph-execution/19. loopadmin keeps **no Go dependency on `workflow/`**: its tables live
  in its own schema with its own forward-only versioned migrations
  (`loopadmin/journal/migrations/`) and its own version table in the same cell database; no
  edge to graph-execution/19 is needed (desk-decided default, reversible: adding the edge would
  move these migrations under 19's ledger). Standing mode needs
  the journal schema, not the instance tables, so row 1's "instance store absent" still holds.
- **Intent before launch, in one store.** Launch intent and its receipt remain together in
  the journal; R2 moves that journal to the cell's PostgreSQL database. They commit in one
  transaction keyed by caller namespace, invocation ID and owner generation. This journal
  transaction does not make separate caller-intent and journal-receipt commits atomic, even
  when both use the cell database. Task 2's stable-request-ID reconciliation still applies to
  the desk queue or instance store on the caller side.
- **Incarnation.** Before each child launch and receipt delivery, the supervisor checks the
  current operator incarnation from `spec/operator-incarnation-v1.md` (planned) (graph-execution/16)
  against the journal's activation record. A mismatch or missing record starts the supervisor
  paused; it launches nothing until rotation completes and the operator confirms the execution
  group stopped (14:52, 14:56). Backup and restore of the journal are database backup and
  restore under that rule; restore stays passive.
- **Tests** for rows 6 and 7 use a real disposable PostgreSQL through
  `ASSAY_TEST_POSTGRES_DSN` (an unset DSN fails the row; a test never self-skips). The
  supervisor's other rows keep their offline fakes.
- Risk answers and gate unchanged (`irreversible: yes`, human).

## R3 follow-up — not in this amendment

Package R3 later amends this brief for runner **guard events, budgets, resource capacity and
custody** (delivery sequence, operating obligations). That is not authored here; do not
implement it under these rows.

## Operation-count accounting amendment — 2026-10-15

Apply the target's [brake and pacing contract](../../proposals/non-ci-architecture/11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed).
The **child-launch budget** that Task 3 rechecks at every child launch and the bound on
recursive agent creation in Task 4 are **brakes**: they stay fail-closed. An unavailable,
unreadable or unverifiable child-launch count refuses the launch exactly as Task 4's absent
or unreadable budget does, and a supervisor restart cannot regain consumed launch allowance
(Task 3's preserved consumption). Required budget/profile configuration stays mandatory and
its absence is never unlimited. Hard resource/time/cost limits, shared capacity, launch
identity, generation fencing, stop state and unknown launch/cancel reconciliation retain
their contracts.

Only desk/tool **pacing** counts (cadence or throughput pacing of already-authorized,
budget-admitted launches) are best effort: no global network-spanning lock or exact budget
reservation, and one shared estimate, never per-process. Launch pacing is effect-class,
because each launch leaves a durable record in the journal: the estimate is re-derived
from the journal's recorded launches in the window before each launch, never zero, and a
cached pacing row is never the source. An unavailable seeded pacing estimate does not alone refuse an
otherwise-authorized launch or receipt delivery.

Qualify against target doc 15's §Operation-count qualification cases; row 9 below carries
the brake obligation and is red until the named test exists. Relaxing a brake is a separate
human decision; this amendment relaxes none. Existing Task/Verify rows and Evidence are
preserved.

## Task

1. Move `tools/desk/internal/cellprocess` into `loopadmin/process` as the module's one process runner, without copying it: keep its process-tree containment and uncertain-cleanup result unchanged, delete the old package, and re-point cellctl's cadence path at the moved package through a `require` plus relative `replace` of the loopadmin module in `tools/desk/go.mod` (the pattern `tools/loopresolve/go.mod` uses for `cellconfig`); cellctl's existing cadence tests stay green unchanged, and the tools/desk release build must resolve the relative `replace`. Extend the process-launch audit so it also scans `loopadmin/` and re-key the runner's exec-site entry to the moved path; the audit must not lose sight of that exec site. Implement one shared process/session supervisor over /20 and that runner. Accept only authenticated caller-scoped, operator-approved launch profiles and validated assignment/reservation references. Support standing-desk and workflow-stage modes with identical adapter and lifecycle semantics. An operator may run separate per-role processes of the same binary; reuse does not imply a privileged all-role daemon.
2. Persist launch intent before starting a child in an execution journal keyed by caller namespace, invocation ID and owner generation. Keep adapter receipts, lifecycle state, cancellation, limits, usage and checkpoints. (Engine superseded 2026-10-13: the cell's PostgreSQL coordination database per the R2 amendment above.) The journal is the second local embedded store the program's scope exception permits (a per-supervisor record on the supervisor's own filesystem; this brief selects and qualifies its embedded engine, with SQLite the expected choice, and loopadmin takes no dependency on workflow/); it is not a shared or distributed store. This journal owns process facts only; desk queues and the instance store (19) retain authoritative work/acceptance state. Caller intent and journal receipt reconcile by stable request ID, including a crash between their separate commits; do not claim a transaction across stores.
3. Implement start/observe/cancel/reconcile and idle session retention only when the adapter declares support. Duplicate input cannot launch twice; unknown launch/stop holds until reconciled; bounded restarts never create another unknown request. Recheck role/profile/generation and budget at every child launch and receipt delivery. Shared concurrency reservations bound all callers using this profile; preserve consumption after crash/restore.
4. Keep runtime launch authority separate from operator profile/install/credential administration. A desk client may request an allowed child invocation through a bounded, capability-scoped endpoint, but cannot choose arbitrary argv, escalate role, raise limits or recursively create unconstrained agents. Effect credentials and operator admin are absent; any inference credential stays confined to its approved adapter boundary and is excluded from packets, artifacts and logs. Fail closed: an unrecognised caller, an unreadable or missing profile or role-binding configuration, and an absent or unreadable budget each refuse the launch; none falls back to a default profile, role or limit.
5. Export sanitized process/session status with mode, caller binding and optional canonical workflow references. Viewer attach/detach never starts a process. The default command uses offline fixtures; live profile activation is separate. Provide backup/restore of the execution journal with referenced artifacts, preserving pause/spend/unknowns; restore is passive. Document state ownership and refusal behavior.
6. (R2, 2026-10-13) Implement the journal on the cell's PostgreSQL coordination database with loopadmin's own schema and versioned migrations; commit launch intent and journal receipt in one transaction; check the operator incarnation before every launch and receipt delivery and start paused on mismatch. Add a static guard that loopadmin imports no SQLite or other embedded database engine.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminBothModesNoGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminBothModesNoGraph " "$routing_out")` | exit 0; named PASS; both caller modes launch via one fake adapter; standing mode starts with the instance store absent; mutation: import the workflow module from the supervisor — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminUnknownRestartAndAccounting$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminUnknownRestartAndAccounting " "$routing_out")` | exit 0; named PASS; crash between caller intent and launch receipt reconciles once; unknown cancel blocks relaunch; usage and limits survive restart; mutation: relaunch on restart when the launch receipt is missing — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminCallerIsolationAndDirectBypass$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminCallerIsolationAndDirectBypass " "$routing_out")` | exit 0; named PASS; a direct lower-boundary call cannot change role/profile, reuse another caller receipt or exceed shared capacity; operator actions are unreachable; mutation: skip the caller-namespace check on receipt lookup — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProcessLaunchAuditCoversLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestProcessLaunchAuditCoversLoopAdmin " "$routing_out")` | exit 0; named PASS; the process-launch audit scans `loopadmin/` as well as `tools/desk`, finds the moved runner's exec site in its register, and fails on an unregistered exec site planted in a `loopadmin/` fixture; mutation: drop the `loopadmin/` root from the audit's scan — the named test must fail |
| 5 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminFailClosed$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminFailClosed " "$routing_out")` | exit 0; named PASS; an unrecognised caller, an unreadable profile or role-binding configuration and an absent budget each refuse the launch and start no child; mutation: treat an absent budget as unlimited — the named test must fail |
| 6 | check:ci +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestJournalSharedStoreRestart$" ./... > "$routing_out" && grep -q -- "--- PASS: TestJournalSharedStoreRestart " "$routing_out")` | exit 0; named PASS against a real disposable PostgreSQL; launch intent and receipt commit in one transaction before the child starts, a restart with the database intact reconciles once, and after the journal database is dropped an unknown launch holds instead of relaunching; mutation: relaunch a child whose launch intent row is missing after database loss — the named test must fail |
| 7 | check:ci +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestJournalIncarnationPaused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestJournalIncarnationPaused " "$routing_out")` | exit 0; named PASS; a journal restored from backup under a mismatched or missing operator incarnation starts paused and launches no child until rotation and confirmed group stop; mutation: resume launches when an activation record is present but its incarnation mismatches — the named test must fail |
| 8 | check:ci +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestNoEmbeddedJournalEngine$" ./... > "$routing_out" && grep -q -- "--- PASS: TestNoEmbeddedJournalEngine " "$routing_out")` | exit 0; named PASS; no loopadmin package imports a SQLite or other embedded database driver or the `workflow` module; mutation: add an embedded-driver import under `loopadmin/journal` — the named test must fail |
| 9 | check:ci +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminLaunchBudgetBrake$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminLaunchBudgetBrake " "$routing_out")` | exit 0; named PASS against a real disposable PostgreSQL journal; with the child-launch usage unreadable the launch is refused and no child starts; repeated supervisor restarts within one window cannot launch beyond the configured budget or regain consumed allowance; a seeded pacing estimate alone does not refuse an authorized launch; mutation: treat an unavailable child-launch count as under budget — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only. R2 amendment facts, declared files, edges, consumers and sizing were re-checked against main at 4696d598e on 2026-10-13; rows 6–8 were run at that head and are red because the named tests do not exist yet.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.

## Proposed measurement consumer — 2026-10-17

desk-improvement/06 consumes this brief for native lifecycle/usage export after this supervisor exists; existing PostgreSQL journal/custody/recovery remains authoritative.
This is a forward consumer-routing amendment only: the new consumer waits on this owner,
not the reverse. Original Task/Verify and completed Evidence are unchanged. Reconcile any
active implementation claim before adding capture hooks. Native emission qualification is
delivered by the named follow-up; this source contract alone does not claim it exists.
