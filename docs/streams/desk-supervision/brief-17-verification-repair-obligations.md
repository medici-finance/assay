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

### Non-implementer verifier run — VERIFY: BLOCKED — 0/4 pass, 4 could-not-check, 0 fail — 2026-09-27 claude-opus-5-5-verifier

Runner is not the implementer (implementing commit ebd3e8637, PR #1399, by the worker App). Isolated detached worktree cut off `origin/main` at the merged head (HEAD == origin/main == `9585b4b6cc2ea8d35d367fb912e7c8216a765ba3`), offline envelope (`KUBECONFIG=/dev/null`), read-only. `gate: model`, all four risk answers `no`. All four rows are `check:ci` rows whose hermetic `statusgen verifyrun` witness could-not-run on darwin (it needs a Linux `unshare --net` network-off sandbox); the non-hermetic direct run passed for each, recorded below as supporting-only.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

Per-row key output, non-hermetic direct run on the host (supporting-only; same commands, same merged head, 2026-09-27, go1.27.1):

- Row 1: exit 0 — `--- PASS: TestRepairObligationFailToWorker (0.00s)`, `ok .../tools/desk/cmd/fanoutloop 0.258s`. The test feeds a duplicated delivery of one failure and asserts exactly one rework item carrying the immutable obligation id, the target repo, the reproduction text and the sorted failing rows, and that the dispatch prompt carries the REPAIR OBLIGATION framing.
- Row 2: exit 0 — `--- PASS: TestRepairObligationRestartAndDuplicate (0.00s)`. Asserts a post + claim + lost-ack re-post log folds to one obligation in state repairing; a live lease (20 min) is not assignable; a dead lease (about 2 h) re-queues the SAME id exactly once.
- Row 3: exit 0 — `--- PASS: TestRepairObligationMergeIsNotResolved (0.00s)`. Asserts repair-PR open and approval do not resolve; a merge moves to awaiting-reverification only; same-actor and wrong-revision passes are refused; an independent pass at the merged revision resolves and a resolved obligation is not dispatchable.
- Row 4: exit 0 — `--- PASS: TestRepairObligationCrossRepoFollowUp (0.00s)`. Asserts the deliverable sibling repo (not the tracking repo) is carried, a fresh repair follow-up branch is named when the original PR merged, the prompt forbids resuming the merged branch, and a duplicate log line folds to one obligation.

An in-container hermetic attempt (`statusgen verifyrun --in-container --dry-run`) was also made and refused with `could-not-attribute — no executing identity is available`: the container cannot resolve git identity from a linked worktree. Not pursued further.

Risk-bearing value enumeration (diff ebd3e8637, non-test Go files, plus the brief's Deliverables):

1. RepairObligationID truncation = `hex(sha256(...))[:16]` (64-bit key) @ tools/desk/internal/deskkit/repairobligation.go:137 — persisted in the append-only projection; changing it re-keys every existing obligation, so a new reader would no longer fold an old line with a new one (duplicate work). Sticky, fixable only with a migration.
2. SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — persisted schema tag; a rename makes older rows read as legacy/unclassified. Sticky, fixable with a migration.
3. FollowUpBranch prefix = "repair/" + slug + "-followup-" + attempt @ tools/desk/internal/deskkit/repairobligation.go:329 — naming only; reversible.
4. repairSidecarName = "repair-obligations.jsonl" @ tools/desk/cmd/fanoutloop/repair.go:40 — projection path; reversible (a rename strands the old file, re-readable by edit).
5. repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 (twin: repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66, added by brief 18) — lease horizon; reversible operational knob, ranks last.

RISK-VALUE: DERIVED — RepairObligationID truncation = 16 hex chars (64 bits) @ tools/desk/internal/deskkit/repairobligation.go:137 — the key only needs to be unique among obligations keyed by (repo, brief, receipt, rows); by the birthday bound the collision probability for n obligations is about n²/2^65, which stays below 1e-9 up to roughly 190,000 obligations, far above any plausible per-projection count. The inputs are joined with a NUL separator and the rows are sorted first, so the same failure always yields the same key (the idempotency the brief requires).
RISK-VALUE: DERIVED — SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — matches the contract document's name (repair-obligation-v1.md in this stream) and the versioned-marker rule in the Interface contract; a row without it is left visibly unclassified, never turned into an obligation.
RISK-VALUE: NAMED, NOT DERIVED — repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 — its comment says it "mirrors the dispatch-claim lease horizon", but the dispatch-claim horizons in the tree are 20 min (claimedClaimTTL, deskdispatch/dispatch.go:1273) and 120 min (DefaultStaleClaim, deskkit/claim.go:49); 45 min matches neither, and brief 18 copies it as a separate constant rather than sharing one. Reversible knob, ranked last; open question for the owning stream: which contract does 45 min derive from, and should the two copies share one constant?

Observations (not failures):

- The landing path records the obligation through a dry-run sink by default and treats a record failure as a NOTE while the FileBug escalation still lands (tools/desk/cmd/verifyloop/repair.go:50-59). The code comment says the real-filing cutover will harden "inability to file is an unlanded obligation"; until that cutover, that clause of the Interface contract is deferred rather than enforced.
- The resolve guard's same-actor check applies only when RepairedBy is set; an obligation merged with an empty RepairedBy relies on the wrong-revision check and the reviewer/verifier identity gates alone.
- The planned changelog fragment name differs (it landed as a differently named fragment and is now folded into CHANGELOG.md); deskdispatch/dispatch.go and prompt.go were not touched: the prompt is rendered by fanoutloop's own dispatch path. Neither is a Verify row.

VERIFY: BLOCKED — 0/4 pass hermetically, 4 could-not-check (check:ci hermetic witness owed on a Linux runner with `unshare --net`), 0 fail. The non-hermetic direct runs of all four rows pass with their named PASS lines. No implementation defect was observed. Advancing needs the four check:ci rows witnessed on a Linux runner. HELD at implemented.

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
