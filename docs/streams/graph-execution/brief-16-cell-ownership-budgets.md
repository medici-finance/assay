---
brief: assay:assay:graph-execution:16
title: Cell ownership, cumulative budgets and restoration fencing
why: Recovery can repeat effects or restart a paused Cell if ownership, budgets and stop state exist only in process memory. Durable state must constrain the old worker as well as its replacement.
wave: 2
depends:
- graph-execution/04
- graph-execution/09
unblocks:
- graph-execution/14
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
- docs/streams/graph-execution/admission-assurance-spec.md
- freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f
exec-tier: strong
exec-tier-why: Cross-component contracts and independent failure controls must agree; the implementation requires design judgment.
domain: complicated
consumers:
- 'drainloop: fixed-here'
- 'tools/desk: follow-up graph-execution/14'
version: 1
id: db922112-28f0-44c0-b120-9d6c4f56e103
---

# Brief 16 — Cell ownership, cumulative budgets and restoration fencing

## Context

files: `drainloop/ownership.go` (planned), `drainloop/ownership_test.go` (planned), `drainloop/budget.go` (planned), `drainloop/budget_test.go` (planned), `drainloop/restore_test.go` (planned), `drainloop/README.md`, `docs/enforcement-model.md`, `changelog/graph-execution-16-cell-ownership-budgets.md` (planned)

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

## Task

1. Define durable reservation and ownership-generation seams, cancellation intent, cumulative attempt budgets and stop/override retention. Use existing storage where possible; adapter conformance must reject non-durable required stores.
2. Fence stale workers at the effect boundary. Require cohort cutover to remove legacy direct credentials or prove equivalent provider restrictions; a database lease alone is insufficient. Keep provider outcome unknown until authoritative reconciliation.
3. Add crash/restore/concurrent-owner fixtures with a separately enforcing fake provider. Bypass the scheduler to prove stale-generation rejection at the provider boundary; preserve pauses and unknown effects across restore. No production credentials or cluster calls.
4. Document limitations for providers without fencing or idempotency: hold and require authorized resolution, never claim exactly-once. Restore/failover is a tested adapter capability, not assumed from SQLite transactions.

## Interface contract

Persisted Cell state → restart → admission/effect boundary preserves budget and stop state while rejecting a stale writer.

Every shared consumer above must be reconciled against the implementing diff. Planned commands/tests below are deliverables, not claims that they already exist. Update the declared documentation and changelog with the implementation.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnership" ./...` | exit 0; output includes PASS for TestCellOwnership, with no [no tests to run] for its owning package |
| 2 | check:ci +mutation | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipStaleWriterDenied" ./...` | exit 0; output includes PASS for TestCellOwnershipStaleWriterDenied, with no [no tests to run] for its owning package |
| 3 | check:ci +flow | `cd drainloop && GOWORK=off go test -count=1 -v -run "^TestCellOwnershipRestoreBudgetPause" ./...` | exit 0; output includes PASS for TestCellOwnershipRestoreBudgetPause, with no [no tests to run] for its owning package |

The flow row must call production contract code across the seam; isolated serializers or a hand-built expected JSON are insufficient. Negative rows must prove a distinct lower boundary where applicable, not merely repeat the upper validator.

## Evidence

<!-- Independent verifier records command, exit, key output/digest, subject revision, environment and date. No implementation or execution evidence is asserted by this authoring change. -->

## Review

Gate: human. Confirm scope, consumer routing, negative-path independence and exact-subject evidence; a confidence score cannot enlarge permission.
