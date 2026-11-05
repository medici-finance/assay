---
brief: assay:assay:graph-execution:41
title: Persist loop-admin journal and recovery in PostgreSQL
why: "Process identity, consumed allowance and unknown launches must survive crashes and restored data before a shared supervisor can safely reuse the runner."
wave: 2
depends: ["graph-execution/40"]
unblocks: ["graph-execution/42"]
split-from: graph-execution/26
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Introduces durable process intent, shared reservations and passive recovery under independently protected incarnation state. Human acceptance confirms database loss or restore cannot regain launch allowance or revive an old execution group."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: fa7b1bb7-ae5e-4204-8447-cdcec9e5e42a
authored: 2026-11-05 by statusgen newbrief
sources:
- docs/streams/graph-execution/brief-26-shared-loop-admin.md
- docs/streams/graph-execution/task-workflow-program.md
- docs/proposals/non-ci-architecture/20-amendment-manifest.md
- docs/go-engineering.md
- "freshness-checked 2026-11-05 @ 7f61cff94b188d46b4e38e6259798d85bb5d4510"
- "PR #4157: delivered current-owner startup preparation; reused, not reimplemented"
exec-tier: strong
exec-tier-why: "Cross-module custody, durable failure recovery and independent lower-boundary authority checks require design judgment."
domain: complicated
outcome: none
consumers:
- "loopadmin/journal/: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "loopadmin/journal/migrations/: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "loopadmin/go.mod: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "loopadmin/go.sum: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "loopadmin/README.md: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "docs/loop-admin.md: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "docs/contracts.md: follow-up graph-execution/41 (this child; only implementation may change to fixed-here)"
- "ci/staged-workflows/loopadmin.yml: follow-up graph-execution/41 (this child; implementation corroborates the actual path)"
- "tools/desk/internal/deskkit/citrigger_test.go: follow-up graph-execution/41 (this child; implementation corroborates the actual path)"
---

# Brief 41 — Persist loop-admin journal and recovery in PostgreSQL

## Context

files:
- `loopadmin/journal/`
- `loopadmin/journal/migrations/`
- `loopadmin/testdata/supervisor/`
- `loopadmin/go.mod`
- `loopadmin/go.sum`
- `loopadmin/README.md`
- `docs/loop-admin.md` (planned)
- `docs/contracts.md`
- `changelog/graph-execution-41-journal.md` (planned)
- `ci/staged-workflows/loopadmin.yml`
- `tools/desk/internal/deskkit/citrigger_test.go`

facts: No loopadmin journal or migrations exist at pickup. Loopadmin currently has only its perfrec module dependency. The selected PostgreSQL access/pool is pgx and migration engine is Goose; catalogue selection is not version admission. The workflow foundation delivered by /36 owns a different schema/ledger and explicitly excludes loopadmin persistence; loopadmin must not import workflow. `spec/operator-incarnation-v1.md` supplies a protected source contract and passive records, not installed custody or a production incarnation source.

layering: Keep process-fact SQL and explicit migrations in loopadmin/journal, with typed process records and an injected protected operator-incarnation source. Operator backup/restore and activation are separately authorized adapters; no source chooses a restored SQL record as current authority. Use the selected PostgreSQL/migration libraries; no embedded fallback, copied workflow ledger or custom database driver/migration engine.

design-fit:
- owner: loopadmin/journal for process facts; existing operator-incarnation contract for current protected identity
- contract: spec/operator-incarnation-v1.md; add the process-fact owner to the existing docs/contracts.md index rather than creating another register
- retires: no earlier delivered control; this is the remaining parent scope
- weight: no new command/flag/refusal counts are ratified by authoring; the implementation must measure its exact delta, keep additions in the canonical owner and explain each positive dimension. No new scheduler, work store, alternate engine or credential broker is permitted.
- why-add: Process identity, consumed allowance and unknown launches must survive crashes and restored data before a shared supervisor can safely reuse the runner.

single-point-of-failure: the journal-admitted invocation boundary; protected operator incarnation and the adapter/caller authority fence remain independent and are exercised with direct bypass fixtures.

## Read first

- [Parent contract](brief-26-shared-loop-admin.md), version 10, including all amendments and its unchanged acceptance table.
- [Task workflow program](task-workflow-program.md), [structured input](work-input-amendment.md) and [stream routing](README.md).
- [Library ownership](../../library-first.md), [Go engineering](../../go-engineering.md), [semantic owners and controls](../../contracts.md).
- [R2 target and amendment ownership](../../proposals/non-ci-architecture/20-amendment-manifest.md), [operator incarnation](../../../spec/operator-incarnation-v1.md).
- [Implementation pickup](../../../plugins/assay/references/implementation-pickup.md); re-establish current source and active claims before effects.

## Ground rules

- This is a proposed split of graph-execution/26. Its parent remains the complete acceptance contract; this authoring does not deliver it or change its prerequisites, gate, Task, Verify or Evidence.
- Work offline in an owned branch. No operational database, provider/model call, image launch, service activation, live desk migration, release, workflow dispatch, merge or ready flip.
- Real PostgreSQL checks use only an explicitly owned disposable fixture. Missing fixture configuration fails its row; no self-skip, ambient database discovery or local embedded substitute.
- Preserve one canonical work identity and claim authority. No effect credential or operator administration in launch code. No credentials in packets, errors, artifacts or logs.
- Retain every existing independent guard and mutation. A control relocation must carry its consumer flow and negative control; any security weakening stops for the human.
- Stop at implemented behind a draft PR; independent verification and the human gate own acceptance. Record before/after weight and fail-first evidence as required by the worker kit.

## Task

1. Complete exact-version pgx/Goose dependency-risk records before manifest additions. Reuse the selected libraries and repository disposable-PostgreSQL fixture discipline without importing workflow, its store or domain tables. Retain one engine in the cell database with a separate loopadmin schema, forward-only migrations and version table; explicit migration ownership, no automatic operational DDL.
2. Define typed process records keyed by caller namespace, invocation and owner generation: stable request identity, intent/receipt, lifecycle/cancel state, admitted immutable limits, usage completeness, checkpoints, referenced artifacts and shared profile reservations. Persist intent and the journal admission receipt in one transaction before the provider starts. Provider acknowledgment arrives later and is reconciled by stable identity; separate caller-intent commits remain separate. Do not claim an external provider joins the SQL transaction.
3. Implement atomic shared-capacity admission and fail-closed durable launch-count consumption, scoped to the approved profile and time window, across callers/processes. Unknown, unreadable or unverifiable consumption and missing budget refuse. Restart, backup rollback, database loss or passive restore never create fresh allowance or authorize another unknown request. Derive best-effort pacing from recorded launches rather than a cached count; an unavailable seeded pacing estimate alone cannot refuse an otherwise admitted launch. Do not relax hard limits or confuse pacing with the launch brake.
4. Check independently protected current incarnation against activation before admission and receipt delivery, including key/cell/owner/generation, pause, stop confirmation and restore provenance required by the canonical contract. Mismatch, missing source/record or uncertain old group starts paused. Rotation and confirmed group stop are trusted operator acts outside this runtime. Journal data cannot prove its own freshness after a joint database/control restore.
5. Implement passive database backup/restore with referenced-artifact inventory and integrity checks, preserving spend, limits, pauses, unknown launch/cancel states and checkpoints. Restore cannot resume or launch; absent/corrupt artifacts, incomplete ownership or incarnation evidence hold. No deletion of durable data or operational restore is authorized by this source work.
6. Deliver real disposable PostgreSQL restart/deletion/rollback/restore, transaction, concurrency and incarnation fixtures. An unset explicit DSN fails, never skips; injected fake stores are not the SQL evidence. Add the no-embedded/no-workflow closure guard with planted import controls. Prepare the dedicated CI/fixture job and all actual cross-module triggers for human promotion where needed; document exact fixture prerequisites and scrubbed failures.

## Interface contract

Keep process-fact SQL and explicit migrations in loopadmin/journal, with typed process records and an injected protected operator-incarnation source. Operator backup/restore and activation are separately authorized adapters; no source chooses a restored SQL record as current authority. Use the selected PostgreSQL/migration libraries; no embedded fallback, copied workflow ledger or custom database driver/migration engine. Caller identity, invocation and current generations remain attached to every typed state, receipt and evidence item. Unknown stays unknown; process success is never work acceptance.

## Verify

The parent-row number in Expect identifies the unchanged obligation this child implements. These tests are prospective deliverables, not observed results. Preserve the original named selectors even where their existing contract names exceed the worker naming preference. Use targeted bounded package runs in the worker; CI owns the full matrix.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestJournalSharedStoreRestart$' ./journal/... > "$o" && grep -F -- '--- PASS: TestJournalSharedStoreRestart ' "$o")` | Parent row 6: exit 0; named PASS. Real disposable PostgreSQL: intent and journal receipt commit atomically before a child; caller/journal commit crash reconciles once; database loss holds an unknown request. Removing intent-loss refusal fails. |
| 2 | check +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestJournalIncarnationPaused$' ./journal/... > "$o" && grep -F -- '--- PASS: TestJournalIncarnationPaused ' "$o")` | Parent row 7: exit 0; named PASS. Real disposable PostgreSQL backup restored with missing/mismatched protected incarnation stays paused until trusted rotation and confirmed group stop; mismatched-activation auto-resume fails. |
| 3 | check:ci +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestNoEmbeddedJournalEngine$' ./journal/... > "$o" && grep -F -- '--- PASS: TestNoEmbeddedJournalEngine ' "$o")` | Parent row 8: exit 0; named PASS. All loopadmin package dependency closures reject SQLite/other embedded engines and workflow; an independently planted embedded import fails the named guard. |
| 4 | check +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopAdminLaunchBudgetBrake$' ./journal/... > "$o" && grep -F -- '--- PASS: TestLoopAdminLaunchBudgetBrake ' "$o")` | Parent row 9: exit 0; named PASS. Real disposable PostgreSQL: unreadable launch use refuses without child; repeated restarts cannot exceed the configured window budget; unavailable pacing estimate alone permits an otherwise admitted launch. Under-budget-on-unavailable mutation fails. |
| 5 | check +flow +mutation | `(cd loopadmin && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestJournalPassiveRestore$' ./journal/... > "$o" && grep -F -- '--- PASS: TestJournalPassiveRestore ' "$o")` | Exit 0; real fixture backup/restore retains unknowns, pauses, limits, usage completeness and artifact identity. Missing/corrupt referenced artifact, rolled-back allowance and auto-launch-on-restore mutations each fail. Nothing restores an operational database. |

## Pre-mortem and dispatch checks

Acknowledgment is treated as a cross-system transaction: row 1 crash boundary. Independent caller processes double-reserve: real concurrent SQL admission in row 4. A joint restore authenticates itself: rows 2/5 use a protected current source and external restore provenance. Unavailable usage becomes zero: row 4 mutation. A fake or ambient database masquerades as PostgreSQL evidence: explicit fixture setup plus all DSN-gated rows.

Rows discriminate through the named production mutations; every claimed fact is source-pinned above. Final design adequacy, the mandatory custody API and source/fixture fidelity are independent review obligations, not presence checks. All four parent risk flags and its human gate are conserved. This split authors no runtime profile or operational permission.

## Evidence

<!-- A non-implementer records actual command, exit, named result, immutable subject/environment and date. No prospective row is claimed passed by authoring. -->

## Review

Gate: human. Parent decision issue #5492 remains the decision surface for the complete /26 promise. No child approval is inferred from an empty issue or from authoring this split. At design pickup, present the concrete child acceptance candidate under the existing decision process before activation or final acceptance. The proposed decomposition must be reviewed before dispatch.
