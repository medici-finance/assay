---
brief: "assay:assay:desk-supervision:17"
title: "Verification failures create durable worker repair obligations"
why: "A filed verification failure can remain unassigned while new briefs consume workers. Give each failed outcome one durable repair obligation that survives the reporting agent and returns to verification after its repair merges."
wave: 1
depends: ["desk-supervision/16"]
unblocks: ["desk-supervision/18"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["docs/streams/desk-supervision/recovery-increments.md", "freshness-checked 2026-09-20 @ 3db05fb44; source paths and open verification repair PR 1374 inspected"]
consumers: ["tools/desk/cmd/verifyloop: fixed-here", "tools/desk/cmd/fanoutloop: fixed-here", "plugins/assay/skills/worker-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Cross-component scheduling state must survive partial failure without manufacturing completion or bypassing an existing role gate."
domain: "complicated"
version: 1
id: "c9f365b7-d488-4442-9f90-a4b27fdcbe7c"
---

# Brief 17 — Verification failures create durable worker repair obligations

## Context

Home: medici-finance/assay.

files:
- tools/desk/internal/deskkit/repairobligation.go(planned)
- tools/desk/cmd/verifyloop/land.go
- tools/desk/cmd/fanoutloop/adapter.go
- tools/desk/cmd/fanoutloop/board.go
- tools/desk/cmd/deskdispatch/dispatch.go
- tools/desk/cmd/deskdispatch/prompt.go
- tools/desk/cmd/fanoutloop/repair_test.go (planned)
- plugins/assay/skills/worker-desk/SKILL.md
- plugins/assay/skills/verify-desk/SKILL.md
- docs/streams/desk-supervision/repair-obligation-v1.md (planned)
- tools/desk/README.md
- changelog/verification-repair-handoff.md (planned)

facts (2026-09-20; re-establish from the named files at pickup):
- desk-supervision/16 supplies stable failure receipts and wake predicates. This brief consumes that contract rather than reading natural-language Evidence as task state.
- fanoutloop already has an Awaiting-implementer-rework source and orphan priority. Extend those readers rather than creating a competing board.
- The existing dispatch claim prevents duplicate concurrent execution of one key, but draft-PR handoff releases the dispatch claim. A repair obligation must survive that release.
- A merged PR is immutable work history: repair requires a new branch/PR. Cross-repo alias/worktree resolution remains the existing dispatcher responsibility.

single-point-of-failure: the new scheduling classification; independent barriers remain the existing per-item claim and reviewer/verifier identity gates. A scheduling receipt can never authorize completion or a write.

## Read first

- [Recovery increments](recovery-increments.md) — scope, ordering, rollout and existing work.
- tools/desk/internal/loopengine/doc.go — existing executor and claim boundaries.
- The source files listed above; planned files are deliverables, not prerequisites.

## Interface contract

Use the existing target-repository issue/PR records plus dispatch claims as the durable authority. Add a versioned structured repair marker keyed by repo, brief, failing receipt and finding/row IDs. One stable obligation points to one issue, responsible role, current attempt/claim and linked repair PR; posting the marker is idempotent, and partial-write recovery reconciles existing records before adding another.

Obligation states are needs-assignment, repairing, awaiting-review, awaiting-merge, awaiting-reverification, waiting-external and resolved. These are obligation states, not new brief lifecycle cells. Only a valid independent verification result at the repaired revision resolves an implementation obligation. Worker completion, issue closure and merge alone cannot resolve it.

Check-definition defects route to a worker to amend the check under the existing review policy. Human/environment blockers stay waiting-external with their exact required action. Unknown classification is explicit triage, never an invented implementation bug. The writer retains its current role authority; inability to file is an unlanded obligation, not success.

## Ground rules

- Implement through a draft PR in this repository; stop at implemented. Independent verification owns acceptance.
- Work in an isolated checkout. Preserve existing role authority, stop flags, review lanes and human merge gates.
- No production queries, runtime activation, global configuration changes or autonomous-loop cutover in this code brief.
- Re-read open PR 1374 before editing verifier paths. If its overlapping work is still in flight, coordinate or stack explicitly; do not duplicate it.

## Task

1. Extend the existing failure landing to create or reconcile a structured repair obligation through the existing filing gate, with the failing rows, reproduction, expected behavior, deliverable repository and source receipt. An ambiguous remote response must be reconciled, not blindly reposted.
2. Extend the existing worker rework source to read outstanding obligations across configured roots. The immutable obligation key outlives agent sessions; lease expiry makes it assignable again without duplicating the obligation.
3. Have worker dispatch attach the obligation to its existing claim and workpad. Resolve the correct repository through the existing resolver; on a merged original PR, generate a follow-up branch. Do not resume or push the merged branch.
4. Reconcile forge transitions to awaiting-review/merge/reverification. The verifier independently decides its result; worker assertions cannot close the obligation. Preserve the blocked receipt until a legitimate wake event exists.
5. Update the two canonical skills and docs so a session exit cannot be mistaken for obligation completion. Tests cover loss of acknowledgment and restart as well as the happy path.

## Verify

All named tests below are planned deliverables. The verifier must observe each named PASS line; exit zero with no tests run is not a pass. Tests use injected forge/clock state, no production services.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1` | exit 0; named PASS for TestRepairObligationFailToWorker; one verifier failure creates exactly one rework item with its reproduction and target repository |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1` | exit 0; named PASS for TestRepairObligationRestartAndDuplicate; duplicate delivery and lost response reconcile to one obligation; replacement worker can resume after dead claim |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1` | exit 0; named PASS for TestRepairObligationMergeIsNotResolved; repair merge wakes independent verification; same-actor or wrong-revision pass cannot resolve; valid independent pass resolves |
| 4 | check:ci +flow | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1` | exit 0; named PASS for TestRepairObligationCrossRepoFollowUp; failed work delivered in a sibling with a merged original PR produces the correct repo, new branch and linked repair; no duplicate original work |

Pre-mortem → detection: Issue filing succeeds but worker sees nothing: row 1. Restart duplicates work: row 2. Merge masquerades as verified completion: row 3. Repair targets the tracking repo: row 4.

## Evidence

Pending implementation and independent verification. No acceptance result claimed by authoring.

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

Runs performed from the verifier's own worktree cut off origin/main at the merged SHA above.
Row commands run exactly as the Verify table specifies (GOWORK=off, -count=1), directly with
the host Go toolchain (go1.26.5 darwin/arm64).

| # | Command | Expected | Observed (exit + key output) | Date Runner |
|---|---------|----------|------------------------------|-------------|
| 1 | GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1 (from tools/desk) | exit 0; named PASS; one failure creates exactly one rework item with its reproduction and target repo | exit 0; "--- PASS: TestRepairObligationFailToWorker"; asserts exactly 1 rework item, immutable obligation id, repo, reproduction and sorted rows carried, repair-framed prompt | 2026-09-23 opus-5.5-verifier |
| 2 | GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1 (from tools/desk) | exit 0; named PASS; duplicate + lost-ack reconcile to one obligation; replacement worker resumes after dead claim | exit 0; "--- PASS: TestRepairObligationRestartAndDuplicate"; asserts fold-to-1 at repairing, live-lease not assignable, dead-lease re-queues the SAME id exactly once | 2026-09-23 opus-5.5-verifier |
| 3 | GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1 (from tools/desk) | exit 0; named PASS; merge wakes reverification; same-actor/wrong-revision cannot resolve; valid independent pass resolves | exit 0; "--- PASS: TestRepairObligationMergeIsNotResolved"; asserts PR-open/approval/merge all non-resolving, merge lands awaiting-reverification, same-actor + wrong-revision rejected, independent pass at repaired sha resolves and is non-dispatchable | 2026-09-23 opus-5.5-verifier |
| 4 | GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1 (from tools/desk) | exit 0; named PASS; sibling delivery with merged original produces correct repo, fresh branch, linked repair; no duplicate original work | exit 0; "--- PASS: TestRepairObligationCrossRepoFollowUp"; asserts cross-root collect folds duplicate to 1, deliverable sibling repo (not tracking repo), fresh follow-up branch, original_merged=true, prompt forbids resuming merged branch | 2026-09-23 opus-5.5-verifier |

Hermetic execution witness (statusgen verifyrun): all four check:ci rows returned could-not-run
on this host — the network-off sandbox uses Linux `unshare --net` and this host is darwin
(recorded verbatim as could-not-check; treated as neither pass nor fail per the verify-desk
addendum). The canonical hermetic witness requires a Linux runner; the direct-execution
observations above stand as the checked-clean result for each row. The witness rows are left
uncommitted in the verifier worktree's brief file for the desk.

RISK-VALUE: DERIVED — repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 — the only numeric literal the diff introduces; a reversible lease horizon (ranks last per kit §4: a timeout/knob correctable by an edit + redeploy). Right by the brief's own contract: lease expiry must re-queue the SAME obligation without duplicating (Task 2) — long enough that a live worker is never stolen from, short enough that a silently-dead session does not park the obligation forever; it mirrors the dispatch-claim lease horizon. Test row 2 exercises both bounds (a 20-min-old lease is not assignable; a ~2h-old lease re-queues once).

Enumeration note (kit §4 step 1, mechanical): the remaining constants the diff introduces are non-numeric identity/schema tokens, not risk-bearing thresholds — SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37; the closed obligation-state string set (needs-assignment / repairing / awaiting-review / awaiting-merge / awaiting-reverification / waiting-external / resolved) @ repairobligation.go:43-57; repairSidecarName = "repair-obligations.jsonl" @ repair.go:40; and typed payload key strings. None is a bound, tolerance, ratio or authority literal. No irreversible literal exists (brief risk metadata: all "no", gate model); the single-point-of-failure is the scheduling classification, and its independent barriers (per-item claim + reviewer/verifier identity gates) are exercised by row 3 — a receipt cannot authorize completion or a write.


## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
