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

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
