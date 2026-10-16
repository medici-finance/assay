---
brief: assay:assay:graph-execution:16
title: Cell ownership, cumulative budgets and restoration fencing
why: Recovery can repeat effects or restart a paused Cell if ownership, budgets and stop state exist only in process memory. Durable state must constrain the old worker as well as its replacement.
wave: 5
depends:
- graph-execution/04
- graph-execution/09
- graph-execution/28
- graph-execution/29
unblocks:
- graph-execution/14
- graph-execution/25
- graph-execution/19
- graph-execution/26
- "forge-neutral/40"
effort: L
gate: human
risk:
  regulatory: no
  customer: no
  irreversible: yes
  sensitive-data: no
gate-why: Ownership and restore behavior constrain external effects; the owner confirms stale writers cannot retain independent authority.
decision-trigger: spec
issues: []
schema: brief-v2
authored: 2026-09-19 by Codex (author-brief)
sources:
- docs/streams/graph-execution/work-input-amendment.md
- freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3 (pending scope, not implementation)
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
- docs/streams/graph-execution/task-workflow-program.md — execution routing amendment 2026-10-02
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
- docs/proposals/non-ci-architecture/20-amendment-manifest.md — R2 owner row and R2 amendment scope (operator incarnation binding)
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md — lines 50-56 (fresh authority incarnation and key; held outside PostgreSQL; mismatch starts paused)
- docs/proposals/non-ci-architecture/15-regression-from-day-one.md — lines 51, 58 (rollback with a surviving old capability; failover with stale generation)
- docs/proposals/non-ci-architecture/16-delivery-sequence.md — line 56 (R2 operator incarnation binding)
- "medici-finance/assay#2500 — maintainer ratification of the whole target @ 23f655f5d (approval activates nothing)"
- freshness-checked 2026-10-13 @ 4696d598e58820806d0bcf8c9b2dd87f29115400
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'drainloop: fixed-here'
- 'tools/desk: follow-up graph-execution/14'
- 'workflow/store: follow-up graph-execution/19'
- 'workflow/controller: follow-up graph-execution/21'
- 'spec/operator-incarnation-v1.md: follow-up graph-execution/16 (this brief; flips to fixed-here when the implementation edits the path)'
- 'loopadmin supervisor launch admission (checks the current incarnation): follow-up graph-execution/26'
version: 5
id: db922112-28f0-44c0-b120-9d6c4f56e103
---

# Brief 16 — Cell ownership, cumulative budgets and restoration fencing

## Context

files: `drainloop/ownership.go` (planned), `drainloop/ownership_test.go` (planned), `drainloop/budget.go` (planned), `drainloop/budget_test.go` (planned), `drainloop/restore_test.go` (planned), `drainloop/README.md`, `docs/enforcement-model.md`, `changelog/graph-execution-16-cell-ownership-budgets.md` (planned). Added by the R2 amendment (2026-10-13): `spec/operator-incarnation-v1.md` (planned), `drainloop/incarnation.go` (planned), `drainloop/incarnation_test.go` (planned).

facts: 04 owns basic effect receipts and reconcile-before-retry. This extension adds ownership/budgets without replacing that protocol or widening the six-method Loop interface.

single-point-of-failure: the new contract or policy alone cannot establish safe execution — independent boundary enforcement and independently read fixture/evidence results must still reject a bypass.

## Human decision

Decision-trigger: spec. At implementation pickup, prepare concrete policy choices and negative-path evidence, then file a self-contained decision issue. No response permits no activation; schema/test work may proceed within the declared scope.

## Read first

- [Admission and assurance amendment](admission-assurance-spec.md).
- [Stream specification](spec.md) and the typed prerequisites above.

## Ground rules

- Work in an isolated branch and draft PR under repository rules; no merge, deployment or live infrastructure query.
- Stop at implemented; independent verification owns verified/done.
- Existing authority and human gates remain binding. Missing prerequisite evidence is could-not-check.
- Public fixtures use example-org and synthetic data; do not copy adopter evidence.

## Work-input amendment — 2026-09-30

Implement WI-3/WI-4 by extending this brief's durable reservation seam, not by adding a meter
service. Persist account/provider unit, model/pricing identity, attempt/request lineage,
raw usage, uncertainty and outstanding reservations. Charge preparation, summaries, failed
attempts, fallback and verification to the original work. Preserve missing telemetry as
unknown and budgets/stops across restart; test concurrent admission against one balance.

Keep quota units separate and make rolling/longer-period constraints and operator headroom
expressible. Fake-provider fixtures prove accounting/concurrency semantics only. Actual
model adapters and plan calibration remain adopting-pilot work. State whether a runner can
limit requests or only launches; a late alarm is not hard enforcement. No live provider
calls, fixed commercial rates or silent cross-provider fallback are added here.

## Task workflow amendment — 2026-10-02

Persist backend ownership/transfer generation, outstanding launch reservations, spend and stop state. Handback requires drain/reconciliation and independently fenced old writers. Missing confirmation of an old effect blocks conflicting admission. Use the existing ownership authority across hosts; a local workflow database is not a second lease authority.

Implement the named failure/flow case below in the declared test surface. This amendment does not record implementation evidence or authorize live activation.

## Non-CI R2 amendment — 2026-10-13 (operator incarnation binding)

The maintainer ratified the non-CI target (medici-finance/assay#2500, revision 23f655f5d);
approval activates nothing. Owner row: package R2 in the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md). The
2026-10-02 amendment above stands; this section adds the incarnation that makes it hold
across database deletion and rollback.

- **What is added.** Every ownership generation is valid only inside one **operator
  incarnation**: a fresh random identity and capability-signing key minted on every complete
  reconstruction, point-in-time rollback and ownership transfer (target 14:50). The trusted
  operator holds the current incarnation outside PostgreSQL, in protected host control state
  beside the repository assignment; a database backup or a Git configuration revision cannot
  restore an old incarnation as current.
- **Where it is checked.** At the effect boundary this brief already fences: every new effect
  admission checks the current incarnation together with owner and generation, so a stale
  queued request or capability fails even when its old SQL row and generation reappear
  (14:52, 15:51). Evidence keeps the old incarnation as provenance without reactivating it.
- **Start paused.** A mismatched or missing incarnation, or a missing activation record,
  starts admission paused (14:56). Restore rotates the incarnation and key and requires the
  operator's confirmation that the whole execution group was stopped before admission
  reopens; a list of attempts recovered from SQL is not that confirmation (14:52).
- **Contract.** `spec/operator-incarnation-v1.md` (planned) states the incarnation fields,
  the rotation triggers, the paused-start rule and how a consumer reads the current value.
  graph-execution/19 stores the activation record; graph-execution/26 checks it before a
  launch. Neither imports `drainloop`; they conform to the contract.
- **Unchanged.** The drainloop seam stays store-neutral (fake stores in this brief's tests);
  the real PostgreSQL rollback proof is graph-execution/19 row 7. Read Task 4's "not assumed from
  SQLite transactions" as "not assumed from database transactions": the runtime store is
  PostgreSQL (graph-execution/19's R2 amendment), and its transactions prove no fencing.
- Risk answers and gate unchanged (`irreversible: yes`, human). The decision issue filed at
  `spec` pickup now also carries the incarnation storage location and rotation procedure.

## R3 follow-up — not in this amendment

Package R3 later amends this brief for **exclusive repository assignment**: operator admission
rejects overlapping mutating assignment of one external repository before capabilities issue
(15:49). That is not authored here; do not implement it under these rows.

## Operation-count accounting amendment — 2026-10-15

Apply the target's [brake and pacing contract](../../proposals/non-ci-architecture/11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed).
This brief's cumulative attempt budgets, reservations and any per-target or per-repository
write ceiling it admits against are **brakes**: they stay fail-closed. When such a count is
unavailable or unverifiable, admission holds or applies a declared conservative fallback;
it never assumes the actor is under budget. Rows 4 and 5 keep their exact contracts.

Only **pacing counts** (desk/tool throughput pacing of already-authorized work) are best
effort. They need no exact budget-slot reservation, global lock across network work or
complete audit reconstruction. Their fallback estimate is seeded from the last retained
count and the window's recorded effect intents and receipts, never from zero. When neither
can be read, one recorded reset per window is allowed; a second treats the window as
exhausted. A missing estimate is reported as unavailable, not measured zero.

Ownership generations, cancellation/stop state, hard resource/time/cost limits and
unresolved-effect reservations keep their contracts. Qualify against target doc 15's
§Operation-count qualification cases; rows 10 and 11 below carry that obligation. They are
red until the named tests exist. Relaxing a brake is a separate human decision; this
amendment relaxes none. No completed Evidence or current runtime behavior changes.

## Task

1. Define durable reservation and ownership-generation seams, cancellation intent, cumulative attempt budgets and stop/override retention. Use existing storage where possible; adapter conformance must reject non-durable required stores.
2. Fence stale workers at the effect boundary. Require cohort cutover to remove legacy direct credentials or prove equivalent provider restrictions; a database lease alone is insufficient. Keep provider outcome unknown until authoritative reconciliation.
3. Add crash/restore/concurrent-owner fixtures with a separately enforcing fake provider. Bypass the scheduler to prove stale-generation rejection at the provider boundary; preserve pauses and unknown effects across restore. No production credentials or cluster calls.
4. Document limitations for providers without fencing or idempotency: hold and require authorized resolution, never claim exactly-once. Restore/failover is a tested adapter capability, not assumed from SQLite transactions.
5. (R2, 2026-10-13) Write `spec/operator-incarnation-v1.md` (planned) and extend the ownership-generation seam so effect admission checks the current operator incarnation with owner and generation. Mint a fresh incarnation and key on reconstruction, rollback and transfer; start paused on a mismatched or missing incarnation; keep the old incarnation as evidence provenance only. Prove it at the effect boundary with the scheduler bypassed. Add a test that resolves the contract's named fields against the incarnation type.

## Interface contract

Persisted Cell state → restart → admission/effect boundary preserves budget and stop state while rejecting a stale writer.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnership" ./...` | exit 0; output includes PASS for TestCellOwnership, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipStaleWriterDenied" ./...` | exit 0; output includes PASS for TestCellOwnershipStaleWriterDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipRestoreBudgetPause" ./...` | exit 0; output includes PASS for TestCellOwnershipRestoreBudgetPause, with no [no tests to run] for its owning package |
| 4 | check:ci +mutation | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetConcurrentReservations.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetConcurrentReservations$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetConcurrentReservations " "$wi_out")` | exit 0; named PASS; concurrent admissions cannot spend the same available allowance; mutation: exclude outstanding reservations from available allowance — the named test must fail |
| 5 | check:ci +flow | `(cd drainloop && wi_out=$(mktemp "${TMPDIR:-/tmp}/assay-TestCellBudgetRestoreAllAttempts.XXXXXX") && trap 'rm -f "$wi_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBudgetRestoreAllAttempts$" ./... > "$wi_out" && grep -q -- "--- PASS: TestCellBudgetRestoreAllAttempts " "$wi_out")` | exit 0; named PASS; failed-attempt spend, reservations, unknown usage and stop state survive restore |
| 6 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestOwnershipBackendHandbackUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestOwnershipBackendHandbackUnknown " "$routing_out")` | exit 0; named PASS; transfer with unknown effect holds, restore retains usage/stop, stale backend cannot write; mutation: release ownership on handback while an effect is unknown — the named test must fail |
| 7 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationStaleRefused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationStaleRefused " "$routing_out")` | exit 0; named PASS; with the scheduler bypassed, a capability minted under an earlier incarnation is refused at the effect boundary even when its owner and generation reappear from a restored store, and no effect is applied; mutation: compare only owner and generation, ignoring the incarnation — the named test must fail |
| 8 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationMismatchPaused$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationMismatchPaused " "$routing_out")` | exit 0; named PASS; a missing or mismatched operator incarnation starts admission paused, a store snapshot cannot set the current incarnation, and reopening requires rotation plus confirmed execution-group stop; mutation: take the current incarnation from the restored store's activation record — the named test must fail |
| 9 | check:ci +dereference +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestIncarnationSpecMatchesFields$" ./... > "$routing_out" && grep -q -- "--- PASS: TestIncarnationSpecMatchesFields " "$routing_out")` | exit 0; named PASS; every field `spec/operator-incarnation-v1.md` (planned) names exists on the incarnation type in `drainloop/incarnation.go` (planned), and every exported field is named in the contract; mutation: rename one field in the contract text — the named test must fail |
| 10 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellBrakeCountUnavailableHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellBrakeCountUnavailableHolds " "$routing_out")` | exit 0; named PASS; with the pacing path bypassed, an unavailable or unverifiable cumulative-budget or per-target write-ceiling count holds admission and no effect is applied at the fake provider; mutation: treat an unavailable brake count as under budget — the named test must fail |
| 11 | check:ci +flow +mutation | `(cd drainloop && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestCellPacingRestartNoMultiply$" ./... > "$routing_out" && grep -q -- "--- PASS: TestCellPacingRestartNoMultiply " "$routing_out")` | exit 0; named PASS; after the shared pacing counter is lost mid-window, K restarts and N processes seed from retained counts and recorded receipts and admit at most cap + C, or 2 × cap + C after one recorded reset, and a second unseeded reset treats the window as exhausted; mutation: start the fallback estimate at zero — the named test must fail |


The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Review a usage-record fixture through persistence and restore: a synthetic credential
must be rejected or replaced with an authorized reference, while numeric usage-token
counts survive unchanged. Check the WI-1 source-trust binding referenced by WI-4.

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
