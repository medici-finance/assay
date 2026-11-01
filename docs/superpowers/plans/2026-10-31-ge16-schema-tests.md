# GE/16 schema and test preparation Implementation Plan

> **For executors:** Follow the approved Assay brief and the repository execution and review procedure. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepare reviewable passive ownership/budget/incarnation contracts and negative fixtures without claiming GE/16 delivery.

**Architecture:** Extend the standalone drainloop module with records and one pure consistency predicate. Tests call a fake boundary directly and round-trip an independently authored checkpoint; the existing Loop, effect and transactional handoff paths remain the execution owners.

**Tech Stack:** Go 1.27 standard library; existing Python mutation runner. No added dependencies.

**Spec:** `docs/streams/graph-execution/brief-16-cell-ownership-budgets.md` version 7, including WI-3/4, R2 and operation-count amendments; operator and checkpoint contracts in `spec/`.

## Global Constraints

- No merge, deployment or live infrastructure query.
- Keep the six-method Loop interface unchanged.
- Preserve unknown usage, stop/cancellation, outstanding effects and reservations.
- Gate: human; decision-trigger: spec; schema/test work may proceed, activation may not.
- No accepted Verify Evidence, readiness verdict or lifecycle completion in this preparation.

## Review Focus

- A restored old tuple may match owner/generation: reject against independently selected incarnation.
- Missing/unreadable current operator source cannot become a current value from SQL.
- A retained pause or missing stop confirmation cannot disappear during restore.
- Nullable telemetry cannot become measured zero; distinct quota units cannot merge.
- Lost schema fields and incomplete attempt phases must fail an independent literal fixture.

## Architecture pickup

Sources inspected at source revision `6152bbf`: resident AGENTS.md and CONTRIBUTING;
Go engineering/library guidance; GE/16 v7 and declared admission, work-input and
workflow-program contracts; relevant R2 owner/rotation and brake/pacing sections of
the ratified target/amendment manifest and regression floor; existing drainloop
`handoff.go`, `handoff_test.go`, records, README and mutation harness. The dispatch
packet matched this starting head. The broad architecture documents were read for
the affected recovery/count boundaries, not audited as whole-system implementation.

Disposition: **continue**, within the user's narrower schema/test authorization.
No source-backed incompatible owner was found at this seam. The existing
TransactionalJournal retains intent/claim-generation ownership; no new store,
scheduler, credential authority or adapter is introduced. The preparation's pure
predicate establishes record consistency only and installs no IncarnationSource.

Consumers: GE/19 activation/persistence and real PostgreSQL rollback; GE/26 launch
admission; GE/14/25 effect consumers; GE/21 controller; forge-neutral/40 and
performance-collection/09 remain existing follow-ups. No consumer is silently
activated or rebound. Dependencies GE/04, /09, /28, /29 were verified in the
source-pinned packet; their acceptance/evidence is preserved.

Unknowns: real provider fencing, signing-key custody, Kubernetes teardown, actual
database durability/rollback, concurrent reservation and pacing implementation.
These are could-not-check here and stay required delivery work. This manual
pickup neither invents an automated context receipt nor grants activation.

### Task 1: Passive record contract and fake boundary

**Files:** `drainloop/incarnation.go`, `ownership.go`, `budget.go`,
`incarnation_test.go`; `spec/operator-incarnation-v1.md`,
`spec/cell-checkpoint-v1.md`.

**Interfaces:** `IncarnationSource.CurrentIncarnation(context.Context, string)
(OperatorIncarnation, error)`; `MatchCellFence(OperatorIncarnation,
CellActivation, CellOwnership, CellOwnership) error`. Produces records only;
nil is consistency, not permission.

- [x] Define ownership/activation/current-incarnation fields separately.
- [x] Add direct fake-boundary negative cases and one current-tuple positive control.
- [x] Dereference contract/exported-field inventory in both directions.
- [x] Add committed mutations for stale incarnation, activation, owner/generation and contract drift.
- [x] Run bounded named tests and observe named runtime failures for mutations.

### Task 2: Retained budget and restore fixture

**Files:** `drainloop/restore_test.go`, `testdata/cell-checkpoint.json`,
existing `testdata/effect_mutations.py`.

**Interfaces:** `CellCheckpoint` contains ownership, activation, `CellBudget`,
cancel/stop flags, override references and pending effects. Budget records preserve
separate unit/window balances, reservations and original-work attempt usage.

- [x] Author a synthetic literal fixture with five charge phases and failed/unknown usage.
- [x] Assert round-trip preservation and refusal of its historical paused activation.
- [x] Add a schema-field-loss mutation that fails the named runtime assertion.
- [x] Keep transactional reservation, concurrency and real crash/restore proofs pending.

### Task 3: Decision and publication

**Files:** existing GE/16 Human decision block; this plan; declared README/enforcement
documentation and changelog fragment.

- [x] Preserve original Task/Verify/Evidence/dependency obligations.
- [x] Prepare bounded custody/rotation, reopen, budget, provider and pacing choices.
- [x] Commit decision-ready preparation; dispatcher filed decision issue #5111 through centralized ensure at spec. The issue remains pending, not approved.
- [x] Publish draft with supported issue linkage, implementation tests and mutation results.
- [x] Hand off exact PR/head and unresolved runtime obligations without a lifecycle change.

### Task 4: Review round 1

**Files:** Task 1 and 2 files; the GE/16 decision block; this plan.

- [x] Reply form names the brief (`graph-execution/16: <option>`), matching the decision gate.
- [x] One consistent single-field negative case and one committed mutant per `MatchCellFence` guard.
- [x] Contract test compares wire names and the stated schema; fixture schema and distinct windows asserted.
- [x] `DecodeCellCheckpoint` refuses absent, unknown and unsupported-schema records; absent hold flags never read as not held.
- [x] Dimension 1 open points (executor key read access, volume snapshot restore) recorded as open, not decided.
- [ ] Workflow path trigger for `spec/**` and the out-of-checkout spec read: linked follow-up, needs workflow scope.

## Local permission checkpoint

The first shell write created no source files: `zsh:1: can't create temp file
for here document: operation not permitted`, an OS sandbox denial. The same
command was then re-run with a permission escalation and succeeded. Which session
or person approved that escalation is not recorded: the earlier text said only
"the dispatcher", and this plan does not identify it further. No desk guard
refusal was bypassed.
