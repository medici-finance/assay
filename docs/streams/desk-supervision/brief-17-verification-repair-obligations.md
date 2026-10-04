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
### Verification — 2026-09-30 (assay-verifier-app[bot] @ 35496323b8fc (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer re-verify on merged main 35496323b8fc591651e44537baf51206cc22bbdd. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm, `--network none`, a full clone pinned to this SHA, statusgen built from main's own source) and passed 4/4. Second table: the hand run on the host (darwin/arm64, go1.27.1).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1` | pass exit=0 | sha256:675e97b84419 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1` | pass exit=0 | sha256:78e9bb627f0e | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1` | pass exit=0 | sha256:86dffbdb72fc | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1` | pass exit=0 | sha256:4861c2f1ffff | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |

| # | Verify row | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | row 1 as written (exact command in the witness table above) | exit 0; named PASS; one failure -> exactly one rework item with reproduction + target repo | exit 0 — "--- PASS: Test Repair Obligation Fail To Worker (0.00s)", "ok .../tools/desk/cmd/fanoutloop 0.411s" (host, go1.27.1 darwin/arm64). Network-off Linux re-run (docker --network none, GOPROXY=off, only lo + unconfigured tunl0 in the netns, go1.25.14 linux/arm64): exit 0, same named PASS line. Hermetic statusgen verifyrun witness: could-not-check, see note W | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 2 | row 2 as written (exact command in the witness table above) | exit 0; named PASS; duplicate + lost response reconcile to one; replacement resumes after dead claim | exit 0 — "--- PASS: Test Repair Obligation Restart And Duplicate (0.00s)", "ok ... 0.215s" (host). Network-off Linux re-run: exit 0, same named PASS line | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 3 | row 3 as written (exact command in the witness table above) | exit 0; named PASS; merge wakes verification only; same-actor / wrong-revision pass refused; independent pass resolves | exit 0 — "--- PASS: Test Repair Obligation Merge Is Not Resolved (0.00s)", "ok ... 0.220s" (host). Network-off Linux re-run: exit 0, same named PASS line | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 4 | row 4 as written (exact command in the witness table above) | exit 0; named PASS; sibling repo, new branch, linked repair, no duplicate | exit 0 — "--- PASS: Test Repair Obligation Cross Repo Follow Up (0.02s)", "ok ... 0.530s" (host). Network-off Linux re-run: exit 0, same named PASS line | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun` on linux, network-off, 4/4 pass at 35496323b8fc. This supersedes the hand run's could-not-check note on the hermetic witness.

RISK-VALUE: DERIVED — RepairObligationID truncation = 16 hex chars (64 bits) @ tools/desk/internal/deskkit/repairobligation.go:137 — the key only has to be unique among obligations in one projection. The birthday bound gives collision probability of about n^2/2^65, which stays below 1e-9 up to about 190,000 obligations, far more than any plausible count. The rows are sorted and the parts are NUL-joined before hashing, so one failure always yields one key. That determinism is the idempotency the Interface contract requires ("posting the marker is idempotent").
RISK-VALUE: DERIVED — SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — it matches the contract document name (docs/streams/desk-supervision/repair-obligation-v1.md, which exists at the verified SHA) and the Interface contract's "versioned structured repair marker" rule.
RISK-VALUE: DERIVED — repairReceiptID truncation = 16 hex chars @ tools/desk/cmd/verifyloop/repair.go:136 — same 64-bit birthday argument as (a). Keying it on (repo, brief, target sha, verdict) means a re-land of the same failed verification at the same revision gets the same receipt, and so the same obligation ("an ambiguous remote response must be reconciled, not blindly reposted"). A failure observed at a NEW revision is a new receipt and a new obligation, which is what the contract's receipt-keyed identity says should happen.
RISK-VALUE: NAMED, NOT DERIVED — repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 — the code comment says it "mirrors the dispatch-claim lease horizon", but the dispatch-claim horizons in the tree are claimedClaimTTL = 20 * time.Minute (tools/desk/cmd/deskdispatch/dispatch.go:1360) and DefaultStaleClaim = 120 * time.Minute (tools/desk/internal/deskkit/claim.go:49). 45 min matches neither, and brief 18 copies it into a second constant. It is a reversible knob and ranks last. Open question for the owning stream: which contract does 45 min come from, and should the two copies share one constant? (The 2026-09-27 run reported the same thing.)

Notes:
- The NAMED, NOT DERIVED lease horizon above holds this brief at implemented. The desk files a question issue for the derivation, links it here, and only then flips.
- The landing sink is dry-run by default and a record error is only a NOTE, so "inability to file is an unlanded obligation" is enforced only at cutover. Row 1 injects obligations directly.
- The source receipt is a stand-in hash, not desk-supervision/16's wake-receipt id. The same-actor refusal fires only when the repairer is set.
- A BLOCKED result that ran no row is recorded against row 1 nominally.
- The tools/desk README cites a nonexistent example-stream contract path.

VERIFY: PASS

### Non-implementer verifier run: 2026-10-02T22:38:52Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

Runner is not the implementer (implementing commit ebd3e8637, PR #1399, authored by the worker App). Detached worktree at merged main, read-only, offline envelope (KUBECONFIG=/dev/null). Host: darwin/arm64, go1.27.1. Each row was executed exactly as authored, with HOME and TMPDIR redirected to a throwaway scratch directory outside any checkout (the desk test tree otherwise writes to the live audit log). `gate: model`; risk answers regulatory / customer / irreversible / sensitive-data all `no`.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---|---|---|---|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1` | exit 0; named PASS for TestRepairObligationFailToWorker | exit 0 — "--- PASS: TestRepairObligationFailToWorker (0.00s)", "ok github.com/medici-finance/assay/tools/desk/cmd/fanoutloop 0.112s" (host run, darwin; supporting the landed Linux witness, see below) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1` | exit 0; named PASS for TestRepairObligationRestartAndDuplicate | exit 0 — "--- PASS: TestRepairObligationRestartAndDuplicate (0.00s)", "ok ... 0.108s" (host run, darwin) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1` | exit 0; named PASS for TestRepairObligationMergeIsNotResolved | exit 0 — "--- PASS: TestRepairObligationMergeIsNotResolved (0.00s)", "ok ... 0.108s" (host run, darwin) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1` | exit 0; named PASS for TestRepairObligationCrossRepoFollowUp | exit 0 — "--- PASS: TestRepairObligationCrossRepoFollowUp (0.00s)", "ok ... 0.113s" (host run, darwin) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (`statusgen verifyrun --brief ... --dry-run`, statusgen v1.0.31, host darwin, exit 2):

- Row 1: could not be executed on this host — check:ci hermetic execution requires a network-off sandbox (`unshare --net`, a Linux facility); output hash sha256:e3b0c44298fc (empty).
- Row 2: could not be executed on this host — same reason.
- Row 3: could not be executed on this host — same reason.
- Row 4: could not be executed on this host — same reason.

Fresh witness at this revision: 0/4 executed, 0 failed. The witness already landed in this section on 2026-09-30 (Linux, network-off, at 35496323b8fc) still audits clean: `statusgen verifyrun --check` on this brief reports 4 of 4 Verify rows pass, 0 fail, 0 missing, exit 0. Nothing under tools/desk/cmd/fanoutloop, the deskkit repair-obligation source, the verifyloop repair/landing sources, or the desk module's go.mod / go.sum differs between 35496323b8fc and this revision (empty diff), so that witness covers byte-identical code under test.

Findings:

- No row passes vacuously. Each `-run` pattern matched exactly one test and each printed its own named PASS line under `-v`. History search (`git log -S` on each `func Test...(` signature) shows all four tests were introduced by ebd3e8637 (#1399), the implementing change, and the test file has had no later commit.
- Changed since the 2026-09-27 blocked outcome record (sha 9585b4b6cc2e): of its 16 declared file inputs, 7 differ — this brief (Evidence appended), repair-obligation-v1.md (wording on where the wake receipt lives), the verify-desk and worker-desk skill bodies, the tools/desk README, and deskdispatch dispatch.go and prompt.go. The other 9 are byte-identical, including every file the four rows compile and execute (fanoutloop adapter.go, board.go, repair.go, repair_test.go; verifyloop land.go, repair.go; deskkit repairobligation.go). The record's tool input was v1.0.27; this run used v1.0.31.
- The 2026-09-27 record's blocker is stale on two counts. Its environment condition (no hermetic witness) was cleared on 2026-09-30 by the Linux network-off witness above, but no newer outcome record was written, so the scheduler still reads the brief as blocked. And its reference, #1491 (open), tracks the in-container defects (pipe-escape parsing, a missing binary in the harness image); the darwin sandbox gap itself is #1800 (open), which carries the container recipe that produced the 2026-09-30 witness.
- The only open item naming this brief is #1897 (open, label question, no replies): the derivation of the 45-minute repair lease. See the risk lines below; it is a reversible knob.
- Carried forward, still true at this revision, none a Verify row: the obligation sink is dry-run by default and a record error is a NOTE, so "inability to file is an unlanded obligation" is enforced only at cutover; the source receipt id is a stand-in content hash; the same-actor refusal applies only when the repairer is set.

Risk-bearing values (enumerated over the non-test Go files of ebd3e8637 plus the Deliverables; ranked by irreversibility):

1. RepairObligationID truncation = hex(sha256(...))[:16] @ tools/desk/internal/deskkit/repairobligation.go:137 — persisted key; a change re-keys existing obligations (duplicate work), fixable only with a migration.
2. repairReceiptID truncation = hex(sha256(...))[:16] @ tools/desk/cmd/verifyloop/repair.go:136 — feeds entry 1; same stickiness.
3. SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — persisted schema tag; a rename makes old rows read as unclassified.
4. FollowUpBranch = "repair/" + slug + "-followup-" + attempt @ tools/desk/internal/deskkit/repairobligation.go:329 — naming only; reversible.
5. repairSidecarName = "repair-obligations.jsonl" @ tools/desk/cmd/fanoutloop/repair.go:40 — projection file name; reversible.
6. repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 — lease horizon; reversible operational knob, ranks last.

RISK-VALUE: DERIVED — RepairObligationID truncation = 16 hex chars (64 bits) @ tools/desk/internal/deskkit/repairobligation.go:137 — the key must be unique only among obligations in one projection; the birthday bound n^2/2^65 stays below 1e-9 up to about 190,000 obligations. Inputs (repo, brief, receipt, sorted rows) are trimmed and NUL-joined before hashing, so one failure always yields one key, which is the idempotency the Interface contract requires.
RISK-VALUE: DERIVED — repairReceiptID truncation = 16 hex chars (64 bits) @ tools/desk/cmd/verifyloop/repair.go:136 — same bound; keyed on (repo, brief, sha, verdict), so a re-land of one failed verification at one revision reconciles to the same receipt and obligation, and a failure at a new revision is a new one.
RISK-VALUE: DERIVED — SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — equals the contract document's name in this stream and satisfies the "versioned structured repair marker" rule.
RISK-VALUE: NAMED, NOT DERIVED — repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 — its comment says it mirrors the dispatch-claim lease horizon, but the horizons in the tree are claimedClaimTTL = 20 * time.Minute (tools/desk/cmd/deskdispatch/dispatch.go:1364) and DefaultStaleClaim = 120 * time.Minute (tools/desk/internal/deskkit/claim.go:49), and a second copy lives at tools/desk/cmd/deskdispatch/repairadmission.go:66. No contract in the tree states 45 minutes, so no derivation is possible from the source. It is a reversible knob (edit and redeploy), ranked last, and is routed as #1897.

Summary: 4/4 rows pass by hand at this revision with their named PASS lines; the hermetic witness requirement is met by the Linux witness landed 2026-09-30 over unchanged code; the fresh witness could not be executed on this darwin host. The one open routed question (#1897) concerns a reversible knob.

VERIFY: PASS

Desk landing note (verify-desk, 2026-10-02): the rows pass and the outcome record for this run is `blocked`. The status does not move on it while #1897 is open: the 45-minute repair lease is named but not derived, and that question is the hold on this brief.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Runner is not the implementer (implementing commit ebd3e8637, PR #1399, authored by the worker App). Detached worktree at merged main ade741f44372ccdc0986c2b0973f621745fbced7, read-only, offline envelope (KUBECONFIG=/dev/null). Host: darwin/arm64, go1.27.1. `gate: model`; risk answers regulatory / customer / irreversible / sensitive-data all `no`.

Why this re-run: the 2026-10-02 outcome record (blocked at e1d99484f, blocker #1897) went stale because three declared inputs changed on main: the verify-desk and worker-desk skill bodies and the tools/desk README. None of those changes touches the repair-obligation text. Between e1d99484f and ade741f44, no added or removed line in the three files mentions repair, obligation or rework. The lines matching "repair obligation" / "repair-obligation" are identical at both revisions (verify-desk 3, worker-desk 4, README 8). The hunks are about comms routing through cellctl, roster beacons, the CLI foundation, deskpr and receipt recovery (commits 3a47c06e4, 704711ad3, b8ef5c794, 854ba7207, 29488116b and others). The code the rows compile and run (fanoutloop, verifyloop, deskkit repairobligation.go) is byte-identical between the two revisions. The desk module's go.mod / go.sum did change (dependency additions), so the rows were run again rather than carried forward.

Revisions: the by-hand rows in the table below ran on the darwin host at ade741f44372. The execution witness further down ran at 70deba75a577, four commits later on main. Every declared input of this brief's outcome record is byte-identical between the two revisions except tools/desk/README.md. Only e2b845bfe (#2135) changed it, and that change rewords the `deskpr update` / `deskpr edit --decided` usage text and does not touch the repair-obligation text. `git diff --stat ade741f44 70deba75a --` over the 16 declared inputs (the brief, repair-obligation-v1.md, the verify-desk and worker-desk SKILL.md, tools/desk/README.md, deskdispatch dispatch.go / prompt.go / repairadmission.go, fanoutloop adapter.go / board.go / repair.go / repair_test.go, verifyloop land.go / repair.go, deskkit/repairobligation.go, loopengine/doc.go) prints only:

```
 tools/desk/README.md | 11 ++++++-----
 1 file changed, 6 insertions(+), 5 deletions(-)
```

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---|---|---|---|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1` | exit 0; named PASS for TestRepairObligationFailToWorker; one verifier failure creates exactly one rework item with its reproduction and target repository | exit 0 — "--- PASS: TestRepairObligationFailToWorker (0.00s)", "ok github.com/medici-finance/assay/tools/desk/cmd/fanoutloop 0.341s" (host, darwin). Linux network-off witness at 70deba75a577: pass exit=0, sha256:c9c8309610cc | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1` | exit 0; named PASS for TestRepairObligationRestartAndDuplicate; duplicate delivery and lost response reconcile to one obligation; replacement worker resumes after dead claim | exit 0 — "--- PASS: TestRepairObligationRestartAndDuplicate (0.00s)", "ok ... 0.143s" (host, darwin). Linux network-off witness at 70deba75a577: pass exit=0, sha256:71f52877fc99 | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1` | exit 0; named PASS for TestRepairObligationMergeIsNotResolved; merge wakes verification only; same-actor / wrong-revision pass refused; independent pass resolves | exit 0 — "--- PASS: TestRepairObligationMergeIsNotResolved (0.00s)", "ok ... 0.105s" (host, darwin). Linux network-off witness at 70deba75a577: pass exit=0, sha256:e90cdc750731 | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1` | exit 0; named PASS for TestRepairObligationCrossRepoFollowUp; sibling deliverable repo, fresh follow-up branch, linked repair, no duplicate | exit 0 — "--- PASS: TestRepairObligationCrossRepoFollowUp (0.01s)", "ok ... 0.119s" (host, darwin). Linux network-off witness at 70deba75a577: pass exit=0, sha256:c7b0e9e9cd5d | 2026-10-04 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun --brief` (write mode, `--timeout 300s`, GOFLAGS=-count=1, GOPROXY=off), run inside golang:1.25-trixie (go1.25.14 linux/arm64) with `docker --network none`. The checkout was a throwaway clone detached at merged main 70deba75a5775d50574695d2fb24efeb757c552f, with origin/main pinned to the same sha. statusgen was built inside the container from the clone's own `statusgen/` source. The host module cache and the roster (`roster.env`) were mounted read-only; no credential, token or key was mounted. The table is copied byte-for-byte from the clone's brief, where verifyrun appended it. 4/4 pass, exit 0:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationFailToWorker$ -v -count=1` | pass exit=0 | sha256:c9c8309610cc | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationRestartAndDuplicate$ -v -count=1` | pass exit=0 | sha256:71f52877fc99 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationMergeIsNotResolved$ -v -count=1` | pass exit=0 | sha256:e90cdc750731 | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run ^TestRepairObligationCrossRepoFollowUp$ -v -count=1` | pass exit=0 | sha256:c7b0e9e9cd5d | 2026-10-04 | assay-verifier-app[bot] @ 70deba75a577 (on-behalf-of human:ian) (forge-identity) |

Correction: an earlier copy of this witness had the on-behalf-of annotation inserted by hand; this table is the unedited output of a re-run with the roster mounted read-only.

On the darwin host, `statusgen verifyrun --brief` (v1.0.31, write mode) produced 4 could-not-run rows (exit 2): check:ci hermetic execution needs `unshare --net`, which is Linux-only. That host table is NOT the witness to land. `verifyrun --check` reads the newest witness table. On the brief with the darwin table appended, it reports "0 pass, 0 fail, 4 could-not-run/missing (of 4 Verify rows)", exit 2. That darwin table would shadow both the 2026-09-30 Linux witness and this one.

verifyrun --check summary: brief as on main (2026-09-30 Linux witness newest): "4 pass, 0 fail, 0 could-not-run/missing (of 4 Verify rows)", exit 0. Brief at 70deba75a plus the 70deba75a577 Linux witness table above (checked in the same container, right after the witness run): "4 pass, 0 fail, 0 could-not-run/missing (of 4 Verify rows)", exit 0.

Findings:

- No row passes vacuously. Each `-run` pattern matched exactly one test, and each printed its own named PASS line under `-v`. The test file's only commit is ebd3e8637. The tests assert the stated behavior: exactly one item from a duplicated delivery, a live lease is not stolen while a dead lease re-queues the same id, a merge leads to awaiting-reverification and same-actor or wrong-revision passes are refused, and a sibling repo case gets a follow-up branch and a prompt that forbids resuming the merged branch.
- #1897 (question, raised-by:desk) is still OPEN with 0 comments. The 45-minute repair lease derivation has not been answered.
- Carried forward, still true at this revision, none a Verify row: the obligation sink is dry-run by default, and a record error is only a NOTE. So "inability to file is an unlanded obligation" is enforced only at cutover. The source receipt id is a stand-in content hash. The same-actor refusal applies only when the repairer is set.

Risk-bearing values: enumerated over the non-test Go files of ebd3e8637 plus the Deliverables. The literals and line numbers are the same at ade741f44:

1. RepairObligationID truncation = hex(sha256(...))[:16] @ tools/desk/internal/deskkit/repairobligation.go:137 — persisted key; changing it needs a migration.
2. repairReceiptID truncation = hex(sha256(...))[:16] @ tools/desk/cmd/verifyloop/repair.go:136 — feeds entry 1; same stickiness.
3. SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — persisted schema tag.
4. FollowUpBranch = "repair/" + slug + "-followup-" + attempt @ tools/desk/internal/deskkit/repairobligation.go:329 — naming; reversible.
5. repairSidecarName = "repair-obligations.jsonl" @ tools/desk/cmd/fanoutloop/repair.go:40 — projection file name; reversible.
6. repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 (twin repairObligationLeaseTTL @ tools/desk/cmd/deskdispatch/repairadmission.go:66) — reversible operational knob; ranks last.

RISK-VALUE: DERIVED — RepairObligationID truncation = 16 hex chars (64 bits) @ tools/desk/internal/deskkit/repairobligation.go:137 — the key only has to be unique within one projection. The birthday bound n^2/2^65 stays below 1e-9 up to about 190,000 obligations. The inputs are trimmed, the rows sorted and the parts NUL-joined, so one failure always yields one key. That is the idempotency the Interface contract requires.
RISK-VALUE: DERIVED — repairReceiptID truncation = 16 hex chars (64 bits) @ tools/desk/cmd/verifyloop/repair.go:136 — same bound. It is keyed on (repo, brief, sha, verdict), so a re-land at the same revision reconciles to the same receipt, and a failure at a new revision gets a new one.
RISK-VALUE: DERIVED — SchemaRepairV1 = "repair-obligation-v1" @ tools/desk/internal/deskkit/repairobligation.go:37 — it equals the name of the contract document in this stream (repair-obligation-v1.md) and meets the "versioned structured repair marker" rule.
RISK-VALUE: NAMED, NOT DERIVED — repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 — its comment says it "mirrors the dispatch-claim lease horizon". The claim horizons in the tree are claimedClaimTTL = 20 * time.Minute (tools/desk/cmd/deskdispatch/dispatch.go:1364) and DefaultStaleClaim = 120 * time.Minute (tools/desk/internal/deskkit/claim.go:49). The only other 45-minute value in the tree is roleTokenMemoMaxAge (tools/desk/internal/deskkit/roletoken.go:186), a token-cache bound that is unrelated to claim leases. No contract states 45 minutes for a repair lease. This is routed as #1897, which is still open.

Summary: 4/4 rows pass, on the host at ade741f44372 and in a fresh Linux network-off execution witness at 70deba75a577. Every declared input except tools/desk/README.md (#2135 only) is byte-identical between those two revisions. The changed inputs do not touch the repair-obligation text, and the code under test is unchanged. Under the desk's 2026-10-02 landing ruling, the only hold is the open derivation question for the reversible 45-minute lease.

VERIFY: BLOCKED — rows 4/4 pass; held on medici-finance/assay#1897 (open): repairLeaseTTL = 45m is named but not derived.

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
