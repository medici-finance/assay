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
- agent-execution/03
effort: L
gate: human
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'yes'
  sensitive-data: 'no'
gate-why: Adds an authenticated launch endpoint and moves the process runner the process-launch audit scans; the owner confirms callers cannot change role, profile or limits and that the audit keeps sight of every exec site.
issues: [3353]
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- "medici-finance/assay#3353 — cell startup scratch scan; measurements on v1.0.48"
- "medici-finance/assay#3099 — remaining parent launch telemetry coverage"
- "freshness-checked 2026-10-20 @ eb82d5b62 (merged main, installed v1.0.51); the 2026-10-18 v1.0.49 install recorded source 3cb57291e"
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
- "tools/desk/internal/cellscratch/: follow-up graph-execution/26"
- "tools/desk/internal/cellcache/: follow-up graph-execution/26"
- "perfrec/: follow-up graph-execution/26"
- "perfdata/adapters/: follow-up performance-collection/02 (optional planned phase consumer)"
- 'workflow/controller: follow-up graph-execution/21'
- 'loopadmin/adapters: follow-up graph-execution/22 (consumes the supervisor and the moved process runner)'
- 'tools/desk/internal/loopengine: follow-up graph-execution/27'
- 'tools/desk/internal/cellprocess: follow-up graph-execution/26 (this brief; moved into loopadmin/process; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/26 (this brief; cadence path calls the moved runner; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/internal/forgeban: follow-up graph-execution/26 (this brief; process-launch audit also scans loopadmin; flips to fixed-here when the implementation edits the path)'
- 'operator lifecycle and cockpit: out-of-scope (adopter qualification)'
- 'loopadmin/journal PostgreSQL schema and migrations: follow-up graph-execution/26 (this brief; flips to fixed-here when the implementation edits the path)'
- 'spec/operator-incarnation-v1.md (read before each launch): follow-up graph-execution/16'
version: 12
id: bcbee0b0-8f12-4e7a-b028-0637b2bab0c3
---

# Brief 26 — Shared loop-admin launch supervision for desks and workflows

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `loopadmin/supervisor/` (planned), `loopadmin/process/` (planned), `tools/desk/internal/cellprocess/` (moved into loopadmin/process by this brief), `tools/desk/cmd/cellctl/cadence.go`, `tools/desk/go.mod`, `tools/desk/go.sum`, `tools/desk/internal/forgeban/allowlist.go`, `tools/desk/internal/deskkit/forge_surface_test.go`, `docs/cellctl-cadence.md`, `loopadmin/journal/` (planned), `loopadmin/control/` (planned), `loopadmin/cmd/loop-admin/` (planned), `loopadmin/testdata/supervisor/` (planned), `loopadmin/README.md` (planned), `docs/loop-admin.md` (planned), `.github/workflows/loopadmin.yml` (planned), `changelog/graph-execution-26-loop-admin.md` (planned). Added by the R2 amendment (2026-10-13): `loopadmin/journal/migrations/` (planned). Additional startup scope: `tools/desk/cmd/cellctl/{up.go,down.go,launch.go,scratch_launch.go,perf.go,telemetry.go,prepare_test.go,launch_perf_test.go}`, `tools/desk/internal/cellscratch/`, `tools/desk/internal/cellcache/`, `tools/desk/internal/deskkit/perf.go`, `perfrec/`, `tools/desk/README.md` (tests named here are planned).

facts: The new loopadmin module is bootstrapped by /20. At main 307fe1699 `tools/desk/internal/cellprocess` already runs one harness invocation and reaps its process group or job, and cellctl's cadence path (`tools/desk/cmd/cellctl/cadence.go`) calls it; the process-launch audit (`TestNoForgeCLIShellout` over the `tools/desk` tree, with the runner's exec site in `tools/desk/internal/forgeban/allowlist.go`) covers it there. `tools/desk/internal/loopengine` owns in-session queue behavior for standing desks; it launches no process. Workflow instance ownership remains outside the supervisor; the shared module has no dependency on the instance store (19). Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the caller-scoped launch endpoint (caller namespace, approved profile, role, generation, shared capacity) — behind it, the per-role cadence lease (27) and the role executors' own credentials, which a direct lower-boundary call still cannot obtain.

risk-answers: `irreversible: yes`, so `gate: human`, as for 14 and 16 in this stream. The supervisor runs offline fixtures only, enrolls no live desk, and holds no effect credential or operator administration, but it adds an authenticated launch endpoint and moves the process runner the process-launch audit scans: once it is implemented and verified, a human check skipped at verification cannot be recovered by a revert. Lowering the gate is a maintainer ruling, not an authoring choice.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- CLI built on the desktools-v2 Cobra/Viper foundation (desktools-v2 cli-contract.md §7): every new command, verb or launcher this brief adds uses that shared foundation (added 2026-10-18).
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

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner's delta; unchanged obligations remain.

## Task

1. Move `tools/desk/internal/cellprocess` into `loopadmin/process` as the module's one process runner, without copying it: keep its process-tree containment and uncertain-cleanup result unchanged, delete the old package, and re-point cellctl's cadence path at the moved package through a `require` plus relative `replace` of the loopadmin module in `tools/desk/go.mod` (the pattern `tools/loopresolve/go.mod` uses for `cellconfig`); cellctl's existing cadence tests stay green unchanged, and the tools/desk release build must resolve the relative `replace`. Extend the process-launch audit so it also scans `loopadmin/` and re-key the runner's exec-site entry to the moved path; the audit must not lose sight of that exec site. Implement one shared process/session supervisor over /20 and that runner. Accept only authenticated caller-scoped, operator-approved launch profiles and validated assignment/reservation references. Support standing-desk and workflow-stage modes with identical adapter and lifecycle semantics. An operator may run separate per-role processes of the same binary; reuse does not imply a privileged all-role daemon.
2. Persist launch intent before starting a child in the cell's PostgreSQL coordination database, with loopadmin's own schema and versioned migrations under the existing R2 amendment. Key process records by caller namespace, invocation and owner generation; keep receipts, lifecycle, cancellation, limits, usage and checkpoints. This journal owns process facts only; existing desk queues and /19 retain work/acceptance ownership. Caller intent in the desk queue or /19 and the journal receipt reconcile by stable request ID, including a crash between their separate commits; the R2 journal transaction does not make them atomic, even when both use the cell database. Use the R2 transaction/incarnation rules for coordination records; provider acknowledgments and external effects still require stable identity and reconciliation, not an assumed cross-system transaction. Do not add a local embedded journal or a second work store.
3. Implement start/observe/cancel/reconcile and idle session retention only when the adapter declares support. Duplicate input cannot launch twice; unknown launch/stop holds until reconciled; bounded restarts never create another unknown request. Recheck role/profile/generation and budget at every child launch and receipt delivery. Shared concurrency reservations bound all callers using this profile; preserve consumption after crash/restore.
4. Keep runtime launch authority separate from operator profile/install/credential administration. A desk client may request an allowed child invocation through a bounded, capability-scoped endpoint, but cannot choose arbitrary argv, escalate role, raise limits or recursively create unconstrained agents. Effect credentials and operator admin are absent; any inference credential stays confined to its approved adapter boundary and is excluded from packets, artifacts and logs. Fail closed: an unrecognised caller, an unreadable or missing profile or role-binding configuration, and an absent or unreadable budget each refuse the launch; none falls back to a default profile, role or limit.
5. Export sanitized process/session status with mode, caller binding and optional canonical workflow references. Viewer attach/detach never starts a process. The default command uses offline fixtures; live profile activation is separate. Provide backup/restore of the execution journal with referenced artifacts, preserving pause/spend/unknowns; restore is passive. Document state ownership and refusal behavior.
6. (R2, 2026-10-13) Implement the journal on the cell's PostgreSQL coordination database with loopadmin's own schema and versioned migrations; commit launch intent and journal receipt in one transaction; check the operator incarnation before every launch and receipt delivery and start paused on mismatch. Add a static guard that loopadmin imports no SQLite or other embedded database engine.

- **Instance amendment:** Key standing invocation ownership/journal/profile/preparation and process control by instance plus execution generation, preserving the independent role binding, exact work-ownership owner and best-effort role pacing. Existing Task 3 shared concurrency reservations apply to explicitly classified hard resource/recursive-launch safety budgets, not the approximate role work target; do not implement that target as a central mutex or slot semaphore. Concurrent operations on one instance are idempotent; distinct worker/review instances may overlap. Reject same-instance unknown-stop relaunch; a separately admitted successor may do unrelated work while predecessor reservations remain. Preserve bounded startup and zero-model idle/attach.

## Deterministic waiting amendment — 2026-10-18

Under the [single delivery contract](task-workflow-program.md#standing-desk-efficiency-delivery--2026-10-18),
process observation, timers, cancellation reconciliation and delivery of completion
receipts run in code. A poll timeout or unchanged process status must never start a
model heartbeat, overseer or child whose sole job is to ask for progress. Queue and
external-CI semantics stay with /27's planned role adapters; the supervisor only
observes processes and enforces launch envelopes. Events are cues, not acceptance.
Persist the existing caller/invocation usage records for all bounded jobs and preserve
unknown usage, launch brakes and the startup telemetry/preparation contract. This adds
no separate watcher service or instrumentation store.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

### Cell preparation and startup acceptance — 2026-10-18

This brief owns shared preparation and bounded per-invocation storage admission, carried from
current cellctl into the single moved process owner. Repair #3353 in the current owner before
or with relocation; an independently landed issue repair is consumed and regression-tested,
never copied or reimplemented here. The current-path repair needs no workflow controller,
new journal, performance service or live enrollment. This brief's existing build dependencies
and human acceptance still govern the supervisor migration. Effort is L for the expanded
launch, storage-concurrency and recovery contract; no new stream or command is introduced.

Baseline: #3353 records a v1.0.48 dry-run scratch sweep of 173.16 s, 79 retained tasks and
101737458866 logical bytes, with partial=false and no reclamation. On 2026-10-18 the installed
binary reports v1.0.49 and build source 3cb57291e; its recorded scratch, cache, up and telemetry
source paths are unchanged from v1.0.48 and inspected main 56615f9ab. This is source comparison,
not a v1.0.49 timing result. That comparison holds for the 2026-10-18 inspection only. Main has
since gained desk-containers/17's lease-serialized `up` converge (medici-finance/assay#3377) and
perf-v2 boundary versioning (medici-finance/assay#3431). Preparation extends that existing `up`
owner and its lease serialization and adds no second launch serializer. Refresh the installed
build and source, and re-baseline timing, at implementation pickup.

1. Add one idempotent cell-preparation operation to the existing launch owner. `up` completes
   bounded shared preparation before starting selected roles; direct `desk`, cadence/restart
   and concurrent `up` use the same contract. Coalesce compatible preparation by cell identity,
   configuration/profile digest and owner generation; invalidation or failed/partial preparation
   cannot publish a reusable success. Existing-session attach performs no new launch work.
   Reuse shared Git/plugin preparation only where repository, resolved revision, harness config
   and read authority agree. Keep role worktree creation/merge, current stop/profile/budget
   checks, custody and execution leases per role. Never pass an environment skip flag as proof
   of authorization or safe storage state.
2. Separate scratch registration and cache consumer enrollment from full historical accounting.
   Neither first launch nor direct/repeated role starts may recursively scan protected historical
   scratch or the full cache to register a consumer. Use the existing storage owner's bounded
   admission contract, retain current hard pressure/free-space and lease checks, and make stale,
   missing or corrupt accounting explicit. Any bounded admission refusal must state the
   corrective maintenance action; unknown accounting is neither a zero nor permission to delete.
   Do not replace a launch safety check with an unvalidated cached success.
3. Keep full recursive sweep/accounting in the existing explicit maintenance lifecycle, outside
   startup/finish critical paths. A long maintenance scan must not hold the admission lock;
   revalidate ownership, evidence handoff, Git retention, child inactivity and leases under the
   relevant short lock immediately before any reclamation. Bound retained scan state and report
   incomplete accounting honestly. `down`/finish records ownership and outcomes promptly;
   interrupted shutdown recovers conservatively without forcing the next desk to scan history.
   Preserve pending evidence, resumable work, Git-bearing data and live/unverifiable children.
4. Correct the existing opt-in parent recorder using resolved effective configuration, including
   config-only opt-in, without extending consent or collecting secrets. Record shared preparation,
   lock wait, scratch admission, final cache enrollment, actual child start and cell time to first
   and all requested starts. Requested, refused, unknown and confirmed starts differ; process
   lifetime is not startup time. Parent/child correlation must not count shared preparation once
   per role or sum overlapping intervals. No-opt-in and failed/slow sinks preserve launch results.
5. Qualify offline through real up/desk/runner/storage seams with fake harnesses and synthetic
   stores: one and five roles; cold/warm preparation; fixed roles with 1x/10x retained history;
   simultaneous up/direct starts; cadence restart; maintenance held mid-scan; clean/interrupted
   down; changed profile/generation; corrupt accounting; and stop/authority refusals. Gate on
   zero recursive historical scratch/cache walks in admission and on finish/`down`, one
   successful shared preparation per compatible cell generation, one live owner per role and
   unchanged protected-data hashes.
   Keep wall/CPU/lock-wait distributions with sample counts and build/workload identifiers.
   No absolute end-to-end latency SLO is ratified by this amendment: report measured values and
   the remaining budget decision; deterministic work/authority bounds are mandatory acceptance.
   Update the existing cellctl/cadence documentation and carry these tests through relocation.

Failure detection: repeated walks, a finish-side sweep or a long scan lock fail row 10;
receipt bypasses and unsafe recovery fail row 11; absent parent phases or changed
consent/failure semantics fail row 12.
All new rows are implementation deliverables and must demonstrate the named negative control.

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
| 10 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellPrepBounded$' ./... > "$o" && grep -F -- '--- PASS: TestCellPrepBounded ' "$o")` | exit 0; named PASS. Production up/direct/cadence fixtures satisfy item 5 of §Cell preparation and startup acceptance at 1x/10x history and with maintenance held mid-scan; zero recursive walks in admission and on finish/`down`, and one compatible shared preparation. Mutation: restore Sweep in beginScratch, restore Sweep in `scratchLaunch.finish`, or a full cache walk at enrollment; each fails the test. |
| 11 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellPrepFences$' ./... > "$o" && grep -F -- '--- PASS: TestCellPrepFences ' "$o")` | exit 0; named PASS. Concurrent starts, stale preparation, changed profiles, missing accounting and interrupted down preserve per-role leases, stop/authority gates and protected hashes. Mutation: accept a stale preparation receipt as a current role lease or reclaim unacknowledged work; test fails. |
| 12 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestCellLaunchTelemetry$' ./... > "$o" && grep -F -- '--- PASS: TestCellLaunchTelemetry ' "$o")` | exit 0; named PASS. Config-only opt-in records preparation through confirmed child start; missing/unknown child starts remain incomplete, shared work counted once, no-opt-in writes nothing and slow/failed sink changes no launch result. Mutation: gate parent recording only on ambient environment or omit final enrollment; test fails. |
| 13 | check:ci +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestLoopAdminWaitDoesNotInvokeModel$' ./... > "$o" && grep -F -- '--- PASS: TestLoopAdminWaitDoesNotInvokeModel ' "$o")` | exit 0; named PASS. Production supervisor timers, unchanged observations and repeated receipts do not call adapter Start; one admitted invocation starts once and completion is delivered without a model waiter. Mutation: start a model on each observe timeout; the named test fails. |
| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && cd loopadmin && GOWORK=off go test -count=1 -v ./supervisor/... -run '^TestDeskInstanceOwners$' > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskInstanceOwners ' "$o")` | Named test executes (zero matching tests is not a pass); Distinct mixed-profile peers run, duplicate same-instance starts produce one owner, wrong generation/profile refuses below the caller, and predecessor child usage remains counted during successor start. Mutation: key ownership only by role or treat unknown stop as released makes the test fail. |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only. R2 amendment facts, declared files, edges, consumers and sizing were re-checked against main at 4696d598e on 2026-10-13; rows 6–8 were run at that head and are red because the named tests do not exist yet.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.

## Execution-environment follow-on routing — 2026-10-16

Reciprocal unblocks only: agent-execution/03. The [execution amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md) adds a follow-on obligation over this brief's existing output. Its Task, Verify, gate, version and historical Evidence are unchanged; this note grants no delivery, reassignment or activation.

## Proposed measurement consumer — 2026-10-17

desk-improvement/06 consumes this brief for native lifecycle/usage export after this supervisor exists; existing PostgreSQL journal/custody/recovery remains authoritative.
This is a forward consumer-routing amendment only: the new consumer waits on this owner,
not the reverse. Original Task/Verify and completed Evidence are unchanged. Reconcile any
active implementation claim before adding capture hooks. Native emission qualification is
delivered by the named follow-up; this source contract alone does not claim it exists.

## Interactive Kubernetes consumer — 20 October 2026

[The in-cluster operator/interactive target](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md)
consumes this owner through agent-execution/09. Preserve current preparation, journal,
role-policy, credential and zero-idle-launch acceptance. The standing Kubernetes profile
requires no workflow instance/controller and creates no separate launch owner. A role Pod
or terminal may stay available while bounded model invocations start only for admitted
work/judgment. Pod replacement cannot treat an unconfirmed old execution group as stopped.
Carry current perf-v3 context across the transport and retain unknown usage. Existing
negative-path, independent-control and startup work/count checks apply to this consumer;
/09 and desk-containers/29 own its adapter and effective-profile qualification.
