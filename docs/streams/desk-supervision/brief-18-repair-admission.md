---
brief: "assay:assay:desk-supervision:18"
title: "Enforce repair reservations at worker dispatch"
why: "The planner reports reserved repair slots but still emits every fresh task. Enforce the existing reservation at the dispatch boundary so a busy agent cannot fill those slots with fresh work while repairs wait."
wave: 2
depends: ["desk-supervision/17"]
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["docs/streams/desk-supervision/recovery-increments.md", "freshness-checked 2026-09-20 @ 3db05fb44; source paths and open verification repair PR 1374 inspected"]
consumers: ["tools/desk/cmd/fanoutloop: fixed-here", "tools/desk/cmd/deskdispatch: fixed-here", "plugins/assay/skills/worker-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Cross-component scheduling state must survive partial failure without manufacturing completion or bypassing an existing role gate."
domain: "complicated"
version: 1
id: "975de7d9-a5ce-4426-a660-4eb2c8e4bdc0"
---

# Brief 18 — Enforce repair reservations at worker dispatch

## Context

Home: medici-finance/assay.

files:
- tools/desk/internal/deskkit/width.go
- tools/desk/internal/deskkit/widthstore.go
- tools/desk/internal/deskkit/repairadmission.go (planned)
- tools/desk/cmd/fanoutloop/main.go
- tools/desk/cmd/deskdispatch/dispatch.go
- tools/desk/cmd/deskdispatch/repairadmission_test.go (planned)
- plugins/assay/skills/worker-desk/SKILL.md
- tools/desk/README.md
- changelog/repair-admission.md (planned)

facts (2026-09-20; re-establish from the named files at pickup):
- desk-supervision/05 is done: width/reservation storage and reporting exist. Its plan test deliberately preserves all fresh rows. This brief adds the missing admission effect, not another width setting.
- deskdispatch is the existing agent-facing claim/worktree boundary. Integrating here benefits the current desk-agent loop without exposing a new autonomous run command.
- desk-supervision/17 supplies outstanding repair obligations and current claims. The reservation reads those sources, not the number of historical failure events.
- Width overrides and their expiry already belong to widthstore; do not renew a human-presence lease or silently extend that expiry.

single-point-of-failure: the new scheduling classification; independent barriers remain the existing per-item claim and reviewer/verifier identity gates. A scheduling receipt can never authorize completion or a write.

## Read first

- [Recovery increments](recovery-increments.md) — scope, ordering, rollout and existing work.
- tools/desk/internal/loopengine/doc.go — existing executor and claim boundaries.
- The source files listed above; planned files are deliverables, not prerequisites.

## Interface contract

Before any worker launch consumes a slot, evaluate role/cell width, live admitted claims and outstanding runnable resume/rework demand using the existing reservation settings. Fresh admission must not consume the reserved floor while that class has runnable demand. Waiting-external repairs do not idle the pool. No repair waits behind a full queue of newly admitted fresh work merely because the caller ignored the planner's printed advice.

Admission and slot reservation must be serialized across dispatchers sharing the same scheduling scope; use an authoritative compare-and-swap/lease under the existing claim backend. A process-local mutex is insufficient for a multi-host claim namespace. The per-item mutual-exclusion claim still applies independently. Define recovery order for a crash between capacity reservation and item claim; never report a successful admission before both are established. Release occupancy when an attempt ends while retaining its unresolved repair obligation.

Unreadable occupancy/demand is a visible could-not-check; it does not fabricate a free slot or lose the queue. A caller-provided class cannot relabel fresh work as repair: resolve the obligation/PR from the authoritative source. Keep all current identity, permission, review, budget and human merge gates.

## Ground rules

- Implement through a draft PR in this repository; stop at implemented. Independent verification owns acceptance.
- Work in an isolated checkout. Preserve existing role authority, stop flags, review lanes and human merge gates.
- No production queries, runtime activation, global configuration changes or autonomous-loop cutover in this code brief.
- Re-read open PR 1374 before editing verifier paths. If its overlapping work is still in flight, coordinate or stack explicitly; do not duplicate it.

## Task

1. Add one admission evaluator and one serialized claim-boundary enforcement path used by deskdispatch. The planner previews the same decision and names the waiting repair; it does not become a second scheduler.
2. Preserve explicit reservation values, expiry and current upper bounds. Ship enforcement opt-in initially with a recorded policy/version so an adopter can validate its baseline before activation; no new global default width or hidden pause.
3. Make retry/restart reconcile occupancy and item claims before repeating a launch. A returned refusal names the owning repair or unreadable source. Leases bound crashed occupancy; use existing stop and liveness mechanisms.
4. Exercise the full existing plan -> dispatch -> review/merge observation -> independent verification handoff in a fake-forge integration fixture. Resume after killing the dispatcher at each state boundary.
5. Document opt-in, rollback to the previous admission mode, source completeness requirements and bypass limits. A raw harness launch outside deskdispatch is outside this enforcement claim and must be named as such.

## Verify

All named tests below are planned deliverables. The verifier must observe each named PASS line; exit zero with no tests run is not a pass. Tests use injected forge/clock state, no production services.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionDirectDispatch; direct fresh dispatch is held when it would steal a reserved repair slot; the repair is admitted |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionConcurrentAndCrash; two dispatchers race for the last slot: one admission; crash at each reservation/claim boundary cannot leak an unbounded slot or duplicate a worker |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionUnknownAndExternalWait; unreadable state is explicit; externally blocked repairs do not idle usable slots; forged repair class is refused |
| 4 | check:ci +flow | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionFullCycleRestart; a failed verify becomes a claimed repair, review and merge lead to one reverify, independent pass resolves it; restart at every transition preserves the obligation |

Pre-mortem → detection: Planner is ignored: row 1 invokes dispatch directly. Multi-host race or crash exceeds width: row 2. External holds starve all work or labels bypass the gate: row 3. Components pass individually but handoff fails: row 4.

## Evidence

Pending implementation and independent verification. No acceptance result claimed by authoring.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier notes — 2026-09-27 opus-5.5-verifier (verify-desk dispatch) @ merged main 9585b4b6cc2e

Runner is not the implementer (implementing commit 89e6d2a8f, PR #1400). Own detached worktree off origin/main; offline envelope (KUBECONFIG=/dev/null); no production services touched.

**Witness instrument.** The statusgen witness above recorded all four rows could-not-run: check:ci rows are replayed inside a network-off sandbox built on `unshare --net`, which this darwin host does not provide. That is the instrument reporting itself honestly; it is not a row failure. A Linux-runner witness replay is still owed if the flip gate requires a hermetic witness.

**Host execution of every row (real output).** Each row's exact command was run at the merged head on the host, then all four were re-run together with GOPROXY=off and GOFLAGS=-mod=readonly to show no module fetch or network was needed:

- Row 1: exit 0. `--- PASS: TestRepairAdmissionDirectDispatch`. Log line `admission OK (policy repair-admission-v1): rework admitted — reserved work fills its own reservation (2/3 slots occupied)`. The test asserts that the fresh dispatch is refused with exit 5, that the hold names the waiting repair, that the lease is released, and that the repair is admitted with exactly one admission.
- Row 2: exit 0. `--- PASS: TestRepairAdmissionConcurrentAndCrash`. The test asserts that B gets exit 6 while A holds the lease, that B gets exit 5 once A's claim fills the last slot, and that there is exactly one admission. It also checks that a crash after the lease but before the item claim leaves occupancy unchanged and records no admission, and that recovery places exactly one claim.
- Row 3: exit 0. `--- PASS: TestRepairAdmissionUnknownAndExternalWait`. The test asserts that unreadable occupancy gives exit 6, no admission and a released lease. A waiting-external obligation adds runnable demand 0. An item that does not match an assignable obligation resolves to fresh, which refuses the forged class. An assignable implementation obligation resolves to rework with demand 1.
- Row 4: exit 0. `--- PASS: TestRepairAdmissionFullCycleRestart`. After each transition the test re-reads the sidecar to simulate a restart. It checks that needs-assignment is assignable and fresh work is held, and that a live repairing lease is not assignable. A merged repair becomes awaiting-reverification and stays unresolved. An independent pass at the repaired SHA resolves it, a same-actor pass does not, and the resolution survives a restart.
- Supplementary: the deskkit admission unit tests (`go test ./internal/deskkit/ -run Admission`) exit 0.

**Observations. None of these fails a Verify row.**
1. Task 1 says the planner previews the same decision and names the waiting repair, and the consumers list says the fanoutloop command is fixed here. The implementing commit does not touch fanoutloop. The planner still prints only its existing advisory `classes: … (fresh capped at k by reservation)` line. It does not call EvaluateAdmission and does not name the waiting repair.
2. Row 2 exercises only the crash before the item claim. A crash after the item claim, with the lease still held and the claim placed, is described in comments but not exercised. The TTL reclaim of a crashed lease is modelled by clearing the fake lease by hand.
3. Row 4 drives obligation-record transitions plus the gate against an injected backend. It is not a fake-forge fixture that runs dispatch, review and merge observation through deskdispatch or fanoutloop, and it does not kill a dispatcher process at each boundary (Task 4 wording).
4. The worker-desk reservation ships with rework=0. With ASSAY_REPAIR_ADMISSION=on alone the rework floor is 0, so no fresh dispatch is ever held. Enforcement needs both the env opt-in and a rework reservation above 0. The docs imply this but do not say it.
5. The occupancy count excludes any claim id ending in `--admission`, not only the lease key. An item key with that suffix would be under-counted, which is the unsafe direction. This is minor and unlikely.
6. The planned changelog fragment was folded into CHANGELOG.md at v1.0.20 (commit b9f1f42be). Its absence from changelog/ is expected.

**Risk-bearing values: enumeration over the implementing diff (the deskkit and deskdispatch repairadmission sources, the dispatch.go wiring, docs).** Risk metadata is present and every field is "no"; the gate is model and nothing is irreversible. The enumeration is done anyway.
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51 (exact match; anything else is off)
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key suffix "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:223 and :277

Ranking: every value is reversible. The gate ships off and rollback is unsetting one env var or pinning the previous binary, with no state to unwind. The ones ranked highest, because they decide admission, are the hold boundary and the lease TTL.

RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — admitting fresh work when free == floor leaves floor-1 free slots, fewer than the reserved floor, so the reservation is stolen. Holding at `free <= floor` and admitting only when free > floor leaves at least floor free slots after admission, which is exactly the no-steal condition.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — with no free slot, any admission would push occupancy past the width ceiling.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must equal the rework source's repairLeaseTTL = 45 * time.Minute (tools/desk/cmd/fanoutloop/repair.go:46), so that the gate and the planner agree on which obligations are assignable. The value is right today. It is a duplicated literal rather than a shared constant, so a future edit to one side could let the two drift apart.

VERIFY: PASS — every Verify row's named PASS line was observed with exit 0 by host execution at merged main 9585b4b6cc2e. The statusgen hermetic witness is could-not-run on this darwin host (no `unshare --net`), and that replay is owed on a Linux runner if the gate requires it. The observations above are routed to the desk and do not fail any row.

### Non-implementer verifier re-run: 2026-10-02T22:04:20Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

Runner is not the implementer (implementing PR #1400). Own detached worktree off origin/main; offline envelope (KUBECONFIG=/dev/null); no production services touched; nothing written to the tree (working tree clean after the pass). The pass was dispatched against cf31c32418ba49f93c679913813768542db1c072; the worktree head is two commits later (e1d99484ffd9), and the tools, plugins and desk-supervision stream trees are byte-identical between the two, so every result below holds for both SHAs.

Each row was run by hand exactly as authored, on a darwin/arm64 host with go1.27.1. Disclosures: the commands ran under a throwaway HOME (the desk tests otherwise write to the live audit log) with the Go build and module caches pointed at the real caches, and with the system temp directory redirected into the verifier's own scratch directory.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---|---|---|---|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionDirectDispatch | exit 0; `--- PASS: TestRepairAdmissionDirectDispatch (0.00s)`; log line `admission OK (policy repair-admission-v1): rework admitted — rework admitted: reserved work fills its own reservation (2/3 slots occupied)` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionConcurrentAndCrash | exit 0; `--- PASS: TestRepairAdmissionConcurrentAndCrash (0.00s)`; log lines `fresh admitted: 1 free slot(s) above the reserved floor (0)` and twice `fresh admitted: 2 free slot(s) above the reserved floor (0)` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionUnknownAndExternalWait | exit 0; `--- PASS: TestRepairAdmissionUnknownAndExternalWait (0.00s)`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 0.285s` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | exit 0; named PASS for TestRepairAdmissionFullCycleRestart | exit 0; `--- PASS: TestRepairAdmissionFullCycleRestart (0.00s)`; `ok github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 0.157s` | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

By hand: 4 of 4 rows observed exit 0 with their named PASS line.

**Execution witness.** `statusgen verifyrun --brief <this brief> --dry-run` (statusgen v1.0.31), run from the worktree root: exit 2, 0 of 4 rows witnessed. All four rows report the same line: `could-not-run (exit=-, sha256:e3b0c44298fc) — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses unshare --net, a Linux facility, and this host is darwin`. The dry run wrote nothing to the brief. The container witness path was not attempted, per the dispatch instruction. This is the instrument reporting could-not-check; it is neither a row failure nor a pass.

**What changed since the 2026-09-27 record (outcome blocked at 9585b4b6cc2e).**
- Witness rows 1-4: unchanged, still could-not-run for the same reason (host is darwin, no network-off sandbox). The tool moved from v1.0.27 to v1.0.31 and the result is identical. The cited blocker #1491 is still OPEN (labels bug, needs-decision; last updated 2026-10-01). Note that #1491 describes the container witness path's defects; the direct cause observed here is the absence of a Linux network-off sandbox on this host, which only a Linux runner or a working container witness removes.
- Hand rows 1-4: passed on 2026-09-27 and pass again now.
- Declared inputs that changed on main since the record: the deskdispatch dispatch.go wiring, the deskkit widthstore, the worker-desk skill body and the desk tools README (commits include #1631/#1650, #1803, #1822, #1918, #1919, #2000, #2009, #2010, #2023, #2036). None of those commits adds or removes an admission or reservation line in dispatch.go or widthstore. The admission sources themselves (deskkit repairadmission.go, deskdispatch repairadmission.go and its test file, fanoutloop main.go and repair.go) are byte-identical to the hashes in the 2026-09-27 record, and the four rows still pass against the changed wiring.
- Scope gap (Task 1, planner preview): NOT closed. fanoutloop main.go and repair.go are unchanged since the record, and a search of the fanoutloop command for any reference to the admission evaluator finds none (the one match for the word is unrelated prose in its dispatch prompt text). The planner still does not preview the admission decision or name the waiting repair, although the brief's consumers list marks fanoutloop fixed-here. No issue tracking this gap was found by search. No Verify row tests it, so it fails no row; it remains a routed finding.

**Findings carried forward, unchanged from the 2026-09-27 notes (sources unchanged):** row 2 exercises only the crash before the item claim; row 4 drives obligation-record transitions against an injected backend rather than a fake-forge fixture that kills a dispatcher process at each boundary (Task 4 wording); enforcement needs both the env opt-in and a rework reservation above 0; the occupancy count excludes any claim id ending in the admission suffix, not only the lease key.

**Risk-bearing values: enumeration over the implementing diff (deskkit and deskdispatch repairadmission sources, the dispatch.go wiring, docs).** Risk metadata is present and every field is "no"; gate is model; nothing is irreversible. Enumeration re-done at this head; every literal is at the same line as on 2026-09-27.
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key suffix "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:223 and :277

Ranking: all reversible (the gate ships off; rollback is unsetting one env var or pinning the previous binary, no state to unwind). Highest ranked, because they decide admission: the two hold boundaries and the lease TTL.

RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — admitting fresh work at free == floor would leave floor-1 free slots, fewer than the reserved floor; holding at free <= floor and admitting only when free > floor leaves at least floor slots free after admission, which is exactly the no-steal condition.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — with no free slot any admission pushes occupancy past the width ceiling.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — must equal the rework source's repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 so the gate and the planner agree on which obligations are assignable; equal today, but a duplicated literal rather than a shared constant, so the two can drift.

VERIFY: BLOCKED — all four Verify rows pass by hand (4/4, exit 0, named PASS lines observed), but the execution witness could not run any of the four check:ci rows on this darwin host (0/4, no network-off sandbox); the verdict is unchanged from 2026-09-27 and #1491 is still open. Status stays implemented. Separately, the Task 1 planner-preview scope gap is still open and untracked.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Runner: assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), not the implementer (implementing PR #1400). Merged main ade741f44372ccdc0986c2b0973f621745fbced7. Own detached worktree off origin/main; offline envelope (KUBECONFIG=/dev/null); no production services touched. Host darwin/arm64, go1.27.1, statusgen v1.0.31. The rows ran under the real HOME. The desk audit log was checked afterwards and has no entry in the 09:10:13Z–09:10:16Z run window, so the tests wrote nothing there. Expectations were derived from the brief's Verify table, the test source and the admission sources before the prior Evidence was read.

**Why this re-run.** The receipt landed at e1d99484f (outcome blocked, #1491) went stale because two declared inputs changed on main: the worker-desk skill body and the desk tools README. None of the changed hunks touches the repair-admission text. The skill body's hunks start at line 222, and its admission paragraph is at line 113. The README's hunks sit near lines 71, 448, 603, 682 and 2002, and its admission and rollback paragraphs are at lines 1402–1416. The only nearby word match is unrelated roster-pruning prose. The admission sources (the deskkit and deskdispatch repairadmission sources and the test file), the dispatch.go wiring, widthstore, width and the fanoutloop command are all byte-identical between e1d99484f and ade741f44 (empty diffstat). So the behaviour under verification has not changed.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---|---|---|---|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` (run with -timeout 300s) | exit 0; named PASS for TestRepairAdmissionDirectDispatch | exit 0; `--- PASS: TestRepairAdmissionDirectDispatch (0.00s)`; log `admission OK (policy repair-admission-v1): rework admitted — rework admitted: reserved work fills its own reservation (2/3 slots occupied)`; `ok …/tools/desk/cmd/deskdispatch 0.345s` | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` (run with -timeout 300s) | exit 0; named PASS for TestRepairAdmissionConcurrentAndCrash | exit 0; `--- PASS: TestRepairAdmissionConcurrentAndCrash (0.00s)`; log `fresh admitted: 1 free slot(s) above the reserved floor (0)` then twice `fresh admitted: 2 free slot(s) above the reserved floor (0)`; `ok … 0.139s` | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` (run with -timeout 300s) | exit 0; named PASS for TestRepairAdmissionUnknownAndExternalWait | exit 0; `--- PASS: TestRepairAdmissionUnknownAndExternalWait (0.01s)`; `ok … 0.187s` | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` (run with -timeout 300s) | exit 0; named PASS for TestRepairAdmissionFullCycleRestart | exit 0; `--- PASS: TestRepairAdmissionFullCycleRestart (0.12s)`; `ok … 0.266s` | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

Run by hand, 4 of 4 rows exited 0 and printed their named PASS line. Supplementary check: `GOWORK=off go test ./internal/deskkit/ -run Admission -count=1` exited 0 (`ok …/internal/deskkit 0.470s`).

**What the tests assert, read from the test source.** Row 1 checks that a fresh dispatch is refused with exit 5, that the hold names the waiting repair, and that the lease is released, and that the rework candidate is admitted with exactly one admission. Row 2 runs two dispatchers sequentially against one shared fake lease. B gets exit 6 while A holds the lease and exit 5 after A fills the last slot, with one admission in total. A crash after the lease but before the item claim leaks no occupancy, and recovery places exactly one claim. Row 3 checks that unreadable occupancy gives exit 6 with the lease released. A waiting-external obligation adds 0 runnable demand. An item with no assignable obligation resolves to fresh, so a forged class is refused. An assignable implementation obligation resolves to rework. Row 4 re-reads the obligation sidecar after every transition (needs-assignment → claimed → merged → awaiting-reverification → resolved). Only an independent pass at the repaired SHA resolves the obligation, a same-actor pass is refused, and the resolution survives a restart.

**Execution witness, written by `statusgen verifyrun --brief docs/streams/desk-supervision/brief-18-repair-admission.md --timeout 300s`** (exit 2; appended to the brief's Evidence in the verifier worktree only, not committed):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |

**`statusgen verifyrun --check docs/streams/desk-supervision/brief-18-repair-admission.md`**: exit 2. Summary line: `docs/streams/desk-supervision/brief-18-repair-admission.md: 0 pass, 0 fail, 4 could-not-run/missing (of 4 Verify rows)`. Each row reports `could-not-run — the witness records could-not-run — the row produced no verdict`. This is could-not-check, not a pass and not a failure.

**Blocker.** medici-finance/assay#1800 is still OPEN (read 2026-10-04): the darwin verify host has no network-off sandbox (`unshare --net`) for the check:ci witness, and a Linux runner clears it. The earlier receipt cited #1491, which tracks in-container witness defects rather than this host gap.

**Findings, independently re-observed at this head. None fails a Verify row, all are routed to the desk.** The sources are unchanged since 2026-09-27.
1. Task 1 planner preview is not done, and no issue tracks it. A search of the fanoutloop command for the admission evaluator or the dispatch gate returns nothing. The planner does not preview the admission decision or name the waiting repair, even though the consumers list marks fanoutloop as fixed-here.
2. Row 2 runs the race sequentially against a fake in-memory lease rather than concurrent goroutines or the real claim backend's compare-and-swap. It exercises a crash at only one boundary (after the lease, before the item claim). The crash after the item claim is described in a comment but not exercised, and the TTL reclaim is simulated by clearing the fake lease by hand.
3. Row 4 drives obligation-record transitions and an injected-backend gate. It is not a fake-forge fixture that runs dispatch, review and merge observation, and it does not kill a dispatcher process at each boundary (Task 4 wording).
4. Enforcement needs `ASSAY_REPAIR_ADMISSION=on` and a rework reservation above 0. With the env opt-in alone the floor is 0 and no fresh dispatch is ever held.
5. The occupancy count skips any claim id ending in `--admission`, not only the lease key. An item key with that suffix would be under-counted, which is the unsafe direction. Minor.

**Risk-bearing values: enumeration over the implementing diff scope** (the deskkit and deskdispatch repairadmission sources and the dispatch.go wiring). The risk metadata is present and every field is "no". Every literal is at the same line as in the earlier records.
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key suffix "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:223 and :277

Ranking: all of these are reversible. The gate ships off, and rollback means unsetting one env var or pinning the previous binary, with no state to unwind. The highest-ranked values are the two hold boundaries and the lease TTL, because they decide admission.

RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — the floor counts only reserved classes with runnable demand. Admitting fresh work at free == floor would leave floor-1 free slots, below the reservation. Admitting only when free > floor leaves at least floor slots free after admission, which is exactly the no-steal condition.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — with free = width − occupancy at or below 0, any admission would push occupancy past the width ceiling, whatever the class.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must equal the planner's rework-source repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46, so the gate and the planner agree on which obligations are assignable. They are equal today. The value is a duplicated literal rather than a shared constant, so the two can drift apart.

VERIFY: BLOCKED — all four Verify rows passed by hand at ade741f44 (4/4, exit 0, named PASS lines observed). The execution witness recorded 0/4: every check:ci row is could-not-run because this darwin host has no network-off sandbox. Blocker medici-finance/assay#1800 is still OPEN. The outcome is unchanged from e1d99484f, and the status stays implemented. The Task 1 planner-preview gap is still open and untracked.

### Non-implementer verifier re-run — 2026-10-11 claude-opus-5-5-verifier @ merged main 251e9473928e

Runner: assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), not the implementer (implementing PR #1400). Merged main 251e9473928e6e8c8c5ffbd331833900331043f1. Own detached worktree off origin/main; offline envelope (KUBECONFIG=/dev/null); no production service or cluster contacted. Host darwin/arm64, go1.27.1, statusgen v1.0.37. Pre-work admission: `deskdispatch --check-verifier` exit 0, attestation run 463478f9496e (medici-finance/assay#2567), model claude-opus-5-5, tier any. The expectations were taken from the brief's Verify table and the test source, and the earlier Evidence was read only after that.

**Execution witness, written by `statusgen verifyrun --brief docs/streams/desk-supervision/brief-18-repair-admission.md`** (exit 2). All four check:ci rows are could-not-run on this darwin host because it has no `unshare --net` network-off sandbox (medici-finance/assay#1800, still OPEN). No container witness was attempted. This is the instrument reporting could-not-check: it is not a pass and not a failure.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2567 run=463478f9496e282ddd8e99aadbcdea8e438d05e7813d74394b913b6f0ac4a2cd source=251e9473928e6e8c8c5ffbd331833900331043f1 brief=docs/streams/desk-supervision/brief-18-repair-admission.md model=claude-opus-5-5 tier=any

**Hand run.** Because the witness cannot run here, each row was run by hand exactly as authored, from the worktree root, at 2026-10-11T10:07Z. Disclosures: HOME and the system temp directory pointed at the verifier's own scratch directory; the Go build and module caches pointed at the cell's existing caches; GOPROXY=off, so no module fetch was possible. The worktree was clean afterwards.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | exit 0; named PASS; fresh held when it would steal a reserved repair slot; repair admitted | exit 0; `--- PASS: TestRepairAdmissionDirectDispatch (0.00s)`; log `admission OK (policy repair-admission-v1): rework admitted — rework admitted: reserved work fills its own reservation (2/3 slots occupied)`; `ok …/tools/desk/cmd/deskdispatch 0.286s`. The test asserts that fresh gets exit 5, the hold names the waiting repair, and the lease is released, and that the repair is admitted with exactly one admission. Expect established. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | exit 0; named PASS; race for last slot gives one admission; crash at each reservation/claim boundary leaks no unbounded slot and duplicates no worker | exit 0; `--- PASS: TestRepairAdmissionConcurrentAndCrash (0.00s)`; log `fresh admitted: 1 free slot(s) above the reserved floor (0)` then twice `fresh admitted: 2 free slot(s) above the reserved floor (0)`; `ok … 0.208s`. Expect only partly established. The race is sequential against an in-memory fake lease and gives one admission. Only one crash boundary is exercised: after the lease, before the item claim. The test's own doc comment names a second boundary (after the item claim), but the body never exercises it. The TTL reclaim is simulated by clearing the fake lease by hand. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | exit 0; named PASS; unreadable state explicit; external-wait repairs do not idle slots; forged repair class refused | exit 0; `--- PASS: TestRepairAdmissionUnknownAndExternalWait (0.00s)`; `ok … 0.201s`. The test asserts that unreadable occupancy gives exit 6, no admission and a released lease. A waiting-external obligation adds runnable demand 0. An unmatched item resolves to fresh, and an assignable implementation obligation resolves to rework. Expect established. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | exit 0; named PASS; failed verify becomes claimed repair; review and merge lead to one reverify; independent pass resolves; restart at every transition preserves the obligation | exit 0; `--- PASS: TestRepairAdmissionFullCycleRestart (0.00s)`; `ok … 0.210s`. Expect only partly established. The sidecar is re-read at four points: needs-assignment, claimed, merged, resolved. Three transitions (repair PR opened, awaiting merge, merged) are applied between two re-reads, so there is no restart after the first two. Review and merge are obligation-record method calls, not a fake-forge observation, and no dispatcher process is killed. An independent pass resolves the obligation and a same-actor pass does not. | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

By hand, 4 of 4 rows exited 0 and printed their named PASS line. Rows 1 and 3 fully establish their Expect. Rows 2 and 4 establish only part of theirs: for the clauses listed above the instrument did not look, which is could-not-check, not an observed misbehaviour. Supplementary check: `GOWORK=off go test ./internal/deskkit/ -run Admission -count=1 -v` exited 0, with 12 PASS lines including TestEvaluateAdmissionFloorHoldsFreshAdmitsRepair, TestEvaluateAdmissionExternalWaitDoesNotIdle and TestEvaluateAdmissionPoolFullHolds.

**What changed since the 2026-10-04 record at ade741f44.** The admission sources are byte-identical at this head: both repairadmission.go files (deskkit and deskdispatch), the deskdispatch repairadmission test file, and fanoutloop main.go and repair.go. dispatch.go changed (storage admission, packets, review rounds), but the repair-admission wiring is the same: enforceAdmission runs before stepClaim, the lease is released when the claim fails, and it is released after the claim succeeds. The worker-desk skill body and the desk tools README changed too. The brief itself changed only by the 2026-10-04 Evidence landing.

**Findings. These are already tracked; none is new code misbehaviour.**
1. The coverage gaps in rows 2 and 4 are those in medici-finance/assay#1922 (desk: "Rows 2 and 4 do not establish their full Expect … Re-author both rows, or add the missing cases"), which is still OPEN.
2. The deliverable gaps are tracked in medici-finance/assay#1923 (OPEN). (a) The Task 1 planner preview is missing: fanoutloop has no reference to EvaluateAdmission or enforceAdmission at this head, although the consumers list marks fanoutloop fixed-here. (b) The Task 4 fake-forge fixture with dispatcher kills is missing. (c) Rework reserve 0 means the opt-in alone holds nothing. (d) The occupancy count skips any claim id with the admission suffix. The 2026-10-04 record called (a) "untracked", but #1923 tracks it.
3. The shared-constant question about the lease TTL is medici-finance/assay#1897 (OPEN).

**Risk-bearing values: enumeration over the implementing diff scope** (the deskkit and deskdispatch repairadmission sources and the dispatch.go wiring). Risk metadata is present and every field is "no"; the gate is model and nothing is irreversible. Each literal was re-read at 251e9473928e and sits at the same line as before.
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key suffix "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:223 and :277

Ranking: every value is reversible. The gate ships off, and rollback means unsetting one env var or pinning the previous binary, with no state to unwind. The two hold boundaries and the lease TTL rank highest because they decide admission.

RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — admitting fresh work at free == floor would leave floor-1 free slots, below the reservation. Admitting only when free > floor leaves at least floor slots free after admission, which is the no-steal condition.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — free = width − occupancy. At or below 0, any admission would push occupancy past the width ceiling, whatever the class.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must equal the planner's repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46, so that gate and planner agree on which obligations are assignable. The two are equal at this head. They are a duplicated literal, so they can drift apart (#1897).

**VERIFY: FAIL** — rows 2 and 4: each command exits 0 with its named PASS line, but the test does not establish the full Expect. Row 2 exercises no crash after the item claim. Row 4 has no restart between the review-opened and awaiting-merge transitions and no fake-forge observation. Rows 1 and 3 pass. No admission misbehaviour was observed. The gap is in Verify coverage and the deliverable, tracked in medici-finance/assay#1922 and #1923. The check:ci witness is 0/4 could-not-run on darwin (#1800). Status stays implemented.

### Non-implementer verifier re-run — 2026-10-15 @ merged main 8a1c4aa15569

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk), not the implementer (implementing PR #1400, merge commit 89e6d2a8f7fb; follow-up cell opt-in PR #1430, commit db509b167cc2). Source 8a1c4aa15569d489e0b19444875a8f461dbb7e10, own detached worktree off origin/main. Pre-work admission: `deskdispatch --check-verifier` exit 0, run 60b98622669477ad, model claude-opus-5-5, tier any. Offline envelope: KUBECONFIG=/dev/null, GOPROXY=off, GOFLAGS=-mod=readonly; no cluster, production service or forge contacted. HOME, the Go build cache and the system temp directory pointed at a throwaway scratch directory outside the worktree; the module cache was read in place. Host darwin/arm64, go1.27.2. Window: started 2026-10-15T03:47:28Z; rows ran 03:48:09Z to 03:48:26Z. The worktree was clean afterwards (no tracked or ignored files added).

**Grounding, written from the brief before any source was read.** Each row must exit 0 and print the named PASS line, and the test behind it must establish the row's Expect: row 1, a direct fresh dispatch is held while a reserved repair waits, and the repair is admitted; row 2, two dispatchers race for the last slot with exactly one admission, and a crash at each reservation/claim boundary leaks no unbounded slot and duplicates no worker; row 3, unreadable state is explicit, externally blocked repairs do not idle usable slots, and a forged repair class is refused; row 4, a failed verify becomes a claimed repair, review and merge lead to one reverify, an independent pass resolves it, and a restart at every transition preserves the obligation.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | exit 0; named PASS; fresh held when it would steal a reserved repair slot; repair admitted | exit 0; `--- PASS: TestRepairAdmissionDirectDispatch (0.00s)`; log `admission OK (policy repair-admission-v1): rework admitted — rework admitted: reserved work fills its own reservation (2/3 slots occupied)`; `ok …/tools/desk/cmd/deskdispatch 0.279s`. Test source asserts fresh gets exit 5, the hold names the waiting repair, the lease is released, and the repair is admitted with exactly one admission. Expect established. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | exit 0; named PASS; race for last slot gives one admission; crash at each reservation/claim boundary leaks no unbounded slot and duplicates no worker | exit 0; named PASS line observed for the row's test (0.00s); log `fresh admitted: 1 free slot(s) above the reserved floor (0)` then twice `fresh admitted: 2 free slot(s) above the reserved floor (0)`; `ok … 0.217s`. Expect only partly established: the race is modelled sequentially against an in-memory fake lease (one admission, established). Only the crash after the lease and before the item claim is exercised; the crash after the item claim is named in the test's doc comment but its body has no such case, and the lease reclaim is simulated by clearing the fake lease by hand. Duplicate-worker prevention through the item-claim CAS is not exercised. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | exit 0; named PASS; unreadable state explicit; external-wait repairs do not idle slots; forged repair class refused | exit 0; named PASS line observed for the row's test (0.00s); `ok … 0.230s`. Test source asserts unreadable occupancy gives exit 6, no admission and a released lease; a waiting-external obligation adds runnable demand 0; an item with no assignable obligation resolves to fresh through the real backend reading an injected sidecar, so a forged repair label cannot jump the floor; an assignable implementation obligation resolves to rework with demand 1. Expect established. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | exit 0; named PASS; failed verify becomes claimed repair; review and merge lead to one reverify; independent pass resolves; restart at every transition preserves the obligation | exit 0; named PASS line observed for the row's test (0.00s); `ok … 0.242s`. Expect only partly established: the sidecar is re-read at four points (needs-assignment, claimed, merged, resolved), but the repair-PR-opened and awaiting-merge transitions are applied with no re-read between them, so a restart is not exercised at every transition. Review and merge are obligation-record method calls, not a fake-forge observation, and no dispatcher process is killed. An independent pass resolves the obligation, a same-actor pass does not, and the resolution survives a re-read. | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

By hand, 4 of 4 commands exited 0 with their named PASS line. Rows 1 and 3 establish their Expect; rows 2 and 4 do not establish the clauses listed in their observed cells. Supplementary: `GOWORK=off go test ./internal/deskkit/ -run Admission -v -count=1` exited 0 with 12 PASS lines, including the floor-holds-fresh, external-wait and pool-full evaluator units.

**What changed since the 2026-10-11 record at 251e9473928e.** Both repairadmission sources (deskkit and deskdispatch), the deskdispatch test file, the deskkit repair-obligation source and the fanoutloop command are byte-identical (empty diffstat). The test file has not changed since PR #1400. dispatch.go changed (claim-store fallback, dispatch record, launch reporting), and the admission wiring is unchanged: the gate runs before the item claim, its lease is released when the claim fails, and released again after the claim succeeds.

**Findings, independently re-observed; none is observed admission misbehaviour.**
1. Rows 2 and 4 coverage gaps as above, tracked in #1922 (its status was not re-read; offline envelope).
2. Task 1 planner preview: a search of the fanoutloop command for the admission evaluator or the dispatch gate returns nothing, although the consumers list marks fanoutloop fixed-here. Task 4's fake-forge fixture with dispatcher kills is also absent. Tracked in #1923 (status not re-read).
3. The occupancy count skips any claim id ending in the admission suffix, not only the lease key; an item key with that suffix would be under-counted (the unsafe direction). Minor.

**Risk-bearing values: enumeration over the implementing diff scope** (deskkit and deskdispatch repairadmission sources, the dispatch.go wiring). Risk metadata is present and every field is "no"; gate model; nothing irreversible. Each literal re-read at 8a1c4aa15569:
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key suffix "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:223 and :277

Ranking: all reversible (the gate ships off; rollback is unsetting one env var or pinning the previous binary, with no state to unwind). The two hold boundaries and the lease TTL rank highest because they decide admission.

**VERIFY: FAIL** — rows 2 and 4: each command exits 0 with its named PASS line, but the test does not establish the full Expect (row 2: no crash after the item claim and no item-claim duplicate check; row 4: no restart between the repair-PR-opened and awaiting-merge transitions, no fake-forge observation). Rows 1 and 3 pass. Test sources unchanged since 2026-10-11; tracked in #1922 and #1923. Status stays implemented.
RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — the floor sums reservations only over reserved classes with runnable demand; admitting fresh work at free == floor leaves floor-1 free slots, below the reservation, while admitting only when free > floor leaves at least floor free slots afterwards, which is exactly the no-steal condition.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — free = width − occupancy; at or below 0 any admission, of any class, pushes occupancy past the width ceiling.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must equal the planner's rework-source repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 so gate and planner agree on which obligations are assignable; equal at this head, but a duplicated literal rather than a shared constant (#1897).
Verification-Attestation: medici-finance/assay#3002 run=60b98622669477ad81bbe3a0c08d60faae02e3fbd1971a3f9c705ebc1a719c70 source=8a1c4aa15569d489e0b19444875a8f461dbb7e10 brief=docs/streams/desk-supervision/brief-18-repair-admission.md model=claude-opus-5-5 tier=any

### Non-implementer verifier re-run — 2026-10-20 @ merged main b73d90d774fc

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk), not the implementer (implementing PR #1400; coverage follow-up commit 76ef477cc484, 2026-10-16). Source b73d90d774fca8aec8974ef410da63ebcb10eece, own detached worktree off origin/main. Pre-work admission: `deskdispatch --check-verifier` exit 0, attestation medici-finance/assay#3614, model claude-opus-5-5, tier strong. This is a fresh, full substantive pass, because the admission sources and the row 2 and row 4 tests changed after the latest prior Evidence (the 2026-10-15 re-run at 8a1c4aa15569, verdict FAIL on rows 2 and 4). The execution witness below was written by `statusgen verifyrun --sandbox container-netns` (exit 0; `verifyrun --check` exit 0, 4 pass, 0 fail, 0 could-not-run of 4 rows). It ran in a local docker container: image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), `--network none`, sandbox=container-netns, the engine's default seccomp profile, GOPROXY=off, on a throwaway clone of this worktree at the source. That runner class follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:22e31cc2a18b | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:f7290c0f3a74 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:752ad7421715 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:4bbf77a1ef70 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |

**Substance.** The latest prior by-hand Evidence is the 2026-10-15 re-run at 8a1c4aa15569. There, rows 1 and 3 established their Expect, and rows 2 and 4 exited 0 but did not establish theirs. This pass re-ran every row by hand at b73d90d774fc. Before reading any source, it wrote down each row's Expect from the brief text. Then it read the test code each row runs. Host darwin/arm64, go1.27.2. KUBECONFIG=/dev/null, GOPROXY=off, GOFLAGS=-mod=readonly. HOME and the system temp directory pointed at the verifier's own scratch directory. Rows ran 2026-10-20T04:06:31Z to 04:06:43Z, and the worktree was clean afterwards.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | exit 0; named PASS; fresh held when it would steal a reserved repair slot; repair admitted | exit 0; `--- PASS: TestRepairAdmissionDirectDispatch (0.00s)`; log `admission OK (policy repair-admission-v1): rework admitted — rework admitted: reserved work fills its own reservation (2/3 slots occupied)`; `ok …/tools/desk/cmd/deskdispatch 0.246s`. The test calls the gate directly with no planner. Fresh work gets exit 5, the hold names the waiting repair, and the lease is released. The repair is admitted with exactly one admission. Expect established. | 2026-10-20 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | exit 0; named PASS; race for last slot gives one admission; crash at each reservation/claim boundary leaks no unbounded slot and duplicates no worker | exit 0; `--- PASS: TestRepairAdmissionConcurrentAndCrash (2.10s)`; `ok … 2.364s`. Each dispatcher is now a separate OS process: the test binary re-executes itself and runs the same admit, claim, release sequence that dispatch() runs. The processes share a compare-and-set file claim store. Race: three rounds, two contenders released on one signal, and the test fails unless at least one round shows contention on the lease. Each round gives exactly one admission, the loser is held on a full pool, the store holds width claims, and the lease is released. Crash after the lease and before the item claim: the process is killed. Another dispatcher waits (exit 6) while the dead lease is inside its TTL and is admitted after the TTL. The crashed item is still dispatchable, and no slot leaks. Crash after the item claim and before the lease release: the process is killed. A resume of the same item is refused while the item claim is live, and the dead claim counts as occupancy within its TTL. Once the claim expires, exactly one resume reclaims it and a second resume is refused, so no duplicate worker appears. Expect established. | 2026-10-20 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | exit 0; named PASS; unreadable state explicit; external-wait repairs do not idle slots; forged repair class refused | exit 0; `--- PASS: TestRepairAdmissionUnknownAndExternalWait (0.00s)`; `ok … 0.236s`. Unreadable occupancy gives exit 6, no admission and a released lease. A waiting-external obligation adds runnable demand 0. The real backend reads an injected sidecar: an unmatched item resolves to fresh, so a forged repair class is refused, and an assignable implementation obligation resolves to rework with demand 1. Expect established. | 2026-10-20 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | exit 0; named PASS; failed verify becomes claimed repair; review and merge lead to one reverify; independent pass resolves; restart at every transition preserves the obligation | exit 0; `--- PASS: TestRepairAdmissionFullCycleRestart (1.20s)`; `ok … 1.422s`. Fresh work is held while the repair is runnable. At needs-assignment a dispatcher process is killed at both boundaries, and the expired repair claim is reclaimed by exactly one resume. The obligation then moves through repairing, awaiting-review, awaiting-merge, awaiting-reverification and resolved. Each forge observation goes through the production observation step, the one fanoutloop also uses. At every one of those states a dispatcher process is killed while it holds the lease. The obligation is then re-read from disk with one id, and replaying the last forge observation changes nothing. A resume is refused while the repair worker holds the item. The merge produces one owed reverification at the merged SHA and does not resolve the obligation. A same-actor pass is refused, and an independent pass resolves the obligation. The sidecar records 6 lines for one obligation id. Expect established. | 2026-10-20 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

By hand, all 4 rows exited 0 with their named PASS line, and every row now establishes its Expect, so 0 rows fail. The production dispatch.go calls the same admitAndClaim sequence with its stepClaim call, so the fixture drives the shipped ordering.

**Findings. None fails a Verify row.**
1. The Task 1 planner preview is still missing at this head. The fanoutloop command has no reference to the admission evaluator, and it still prints only its advisory classes line. It does not preview the decision or name the waiting repair, although the consumers list marks fanoutloop fixed-here. This is part (a) of medici-finance/assay#1923, which stays open.
2. Row 4 builds the failed-verify obligation directly through the obligation constructor; producing it from a verify run is brief 17's scope. The forge is an in-test struct whose observations pass through the production observation step. The fixture does not include the planner stage named in Task 4. The claim store is deskkit's file compare-and-set store, not the forge-ref store; the Verify preamble permits injected forge state.
3. The occupancy count now excludes only the scope's own lease key, pinned by the regression test TestCountLiveSuffixItem. The earlier under-count finding is closed.

**Risk-bearing values: enumeration over the implementing diff scope** (the deskkit and deskdispatch repairadmission sources, the dispatch.go wiring, and the 76ef477cc follow-up). Risk metadata is present and every field is "no"; the gate is model and nothing is irreversible. Each literal was re-read at b73d90d774fc:
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key leaf "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:253. The suffix match in the occupancy count was removed by 76ef477cc.
- claimedClaimTTL = 20 * time.Minute @ tools/desk/cmd/deskdispatch/dispatch.go:1590. It existed before this item and bounds a crashed lease or claim in the row 2 and row 4 fixture.

Ranking: every value is reversible. The gate ships off, and rollback means unsetting one env var or pinning the previous binary, with no state to unwind. The two hold boundaries and the lease TTL rank highest because they decide admission.

RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — the floor sums reservations only over reserved classes with runnable demand. Admitting fresh work at free == floor would leave floor-1 free slots, below the reservation. Admitting only when free > floor leaves at least floor slots free afterwards, which is exactly the no-steal condition.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — free = width − occupancy. At or below 0, any admission of any class would push occupancy past the width ceiling.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must equal the planner's repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46, so that gate and planner agree on which obligations are assignable. They are equal at this head, but the value is still a duplicated literal rather than a shared constant (#1897).
Verification-Attestation: medici-finance/assay#3614 run=0e4bb25af0dbca905238a9a2f1a427099991a5bec917e19ec69f521c7ab87962 source=b73d90d774fca8aec8974ef410da63ebcb10eece brief=docs/streams/desk-supervision/brief-18-repair-admission.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — at b73d90d774fc all 4 rows pass on substance by hand, each with its Expect established, and in the container-netns execution witness (4 pass, 0 fail). The row 2 and row 4 coverage gaps behind the earlier FAIL are closed by 76ef477cc. The Task 1 planner preview is still absent and stays tracked in #1923; it is outside every Verify row.

### Non-implementer verifier re-witness — 2026-11-02 @ merged main 2c6c21336a60

Runner: assay-verifier-app[bot] (claude-opus-5-5 verifier, tier strong, dispatched by verify-desk), not the implementer. This is the one-time batch re-witness at a single main commit that the driver ruled on medici-finance/assay#2026. Source 2c6c21336a606e64df057294f21177f16db7b9f8, in the dispatch's own detached worktree. Pre-work admission: `deskdispatch --check-verifier` exit 0, attestation medici-finance/assay#5245, model claude-opus-5-5, tier strong. Earlier blocks above are left as they were; nothing from them is copied here. The rows ran in the closed admission bracket. The host opened it at 2026-11-02T00:31:28Z. The rows ran in a local docker container: image ghcr.io/medici-finance/verify-runner pinned by digest (statusgen v1.0.67, go1.27.3 linux/arm64), `--network none`, the engine's default seccomp profile, sandbox=container-netns, with the home and its git directory mounted read-only. The engine is OrbStack behind the `default` docker context, which the 2026-10-22 interim-runner ruling allows. The host closed the bracket at 2026-11-02T00:32:44Z after a fresh `--check-verifier`. `statusgen verifyrun --bracket-digest` reports verdict=match.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:fcf40341ceb0 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:b604c33dc1a1 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:bd4fd5998313 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:b861c4453180 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:ddf14768e4b78f02e94f6fae4c310c08dc06e0a8db50a2fb68244111759cd68c
Container-Image: sha256:5fa242b9d02a5f769fe9d541f5a8ad33ae291e418a735617faf4812ef21e8629
Verification-Attestation: medici-finance/assay#5245 run=d5cae70f55d3b8611e0062880dd04c9dd95579bdc2a3ff35d46b21887e95a115 source=2c6c21336a606e64df057294f21177f16db7b9f8 brief=docs/streams/desk-supervision/brief-18-repair-admission.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d5cae70f55d3b8611e0062880dd04c9dd95579bdc2a3ff35d46b21887e95a115 source=2c6c21336a606e64df057294f21177f16db7b9f8 tree=sha256:ddf14768e4b78f02e94f6fae4c310c08dc06e0a8db50a2fb68244111759cd68c witness=sha256:f91d97e48efa1d133a1b5ef8b13e6f0ff13e912080e76e1fa39a8d1962a1ce22 opened=2026-11-02T00:31:28Z closed=2026-11-02T00:32:44Z before=receipt after=check-verifier

**Named PASS lines (supplemental, no acceptance credit).** The witness table records each row's output digest, not its text. To see each named PASS line, the same four row commands were run again, unchanged, in the same image with `--network none` and the home mounted read-only. The output went to the verifier's scratch directory. All four exited 0:
- row 1: `--- PASS: TestRepairAdmissionDirectDispatch (0.00s)`; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 0.068s`
- row 2: `--- PASS: TestRepairAdmissionConcurrentAndCrash (3.67s)`; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 3.731s`
- row 3: `--- PASS: TestRepairAdmissionUnknownAndExternalWait (0.00s)`; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 0.036s`
- row 4: `--- PASS: TestRepairAdmissionFullCycleRestart (2.40s)`; `ok  github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 2.429s`

**Risk-bearing values: re-enumerated at 2c6c21336a60** over the deskkit and deskdispatch repairadmission sources and the dispatch.go wiring. Since b73d90d774fc only one commit touched these files, the deskclaim-ref alias-check fix 19e1c01d4. Risk metadata is present and every field is "no"; the gate is model and nothing is irreversible.
- EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ tools/desk/internal/deskkit/repairadmission.go:39
- RepairAdmissionPolicyVersion = "repair-admission-v1" @ tools/desk/internal/deskkit/repairadmission.go:46
- repairAdmissionOn = "on" @ tools/desk/internal/deskkit/repairadmission.go:51
- pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134
- fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154
- admissionLoop = "worker-desk" @ tools/desk/cmd/deskdispatch/repairadmission.go:60
- repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66
- repairSidecarRel = docs/streams/repair-obligations.jsonl @ tools/desk/cmd/deskdispatch/repairadmission.go:71
- lease-key leaf "--admission" @ tools/desk/cmd/deskdispatch/repairadmission.go:253
- claimedClaimTTL = 20 * time.Minute @ tools/desk/cmd/deskdispatch/dispatch.go:1697. This value predates the item, and the line has moved since the last pass.

Ranking: every value is reversible. The gate ships off, and rollback means unsetting one env var or pinning the previous binary. The two hold boundaries and the lease TTL rank highest.

RISK-VALUE: DERIVED — fresh-hold boundary `free <= floor` @ tools/desk/internal/deskkit/repairadmission.go:154 — admitting fresh work only when free > floor leaves at least floor slots free afterwards, which is exactly the no-steal condition. At free == floor, an admission would leave floor-1.
RISK-VALUE: DERIVED — pool-full hold `free <= 0` @ tools/desk/internal/deskkit/repairadmission.go:134 — free = width − occupancy, so at or below 0 any admission would push occupancy past the width ceiling.
RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must equal the planner's repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46 so that the gate and the planner agree on which obligations are assignable. The two are equal at this head, but it is still a duplicated literal.

**VERIFY: PASS** — at 2c6c21336a60 all 4 rows pass in the bracketed container-netns execution witness (4 pass, 0 fail, 0 could-not-run of 4), and the supplemental run showed each named PASS line.

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
