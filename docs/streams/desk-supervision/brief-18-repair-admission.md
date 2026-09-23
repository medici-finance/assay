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

### Non-implementer verifier run — VERIFY: BLOCKED — 2026-09-23 opus-5.5-verifier

Decisive-instrument note: the four Verify rows are `check:ci`, whose authoritative execution is the network-off hermetic witness (`statusgen verifyrun`). On this darwin host the witness could-not-run for every row (it needs Linux `unshare --net`), so the decisive instrument could-not-check. The rows were also run directly (non-hermetic `go test`) and every one passed with its named PASS line and exit 0; that is recorded below as strong corroboration, not as the required witness. No row was observed failing.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|-------------|---|
| 1 | cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionDirectDispatch$ -v -count=1 | exit 0; named PASS; fresh dispatch held when it would steal a reserved repair slot; repair admitted | Hermetic witness (verifyrun) could-not-check: check:ci needs Linux unshare --net, host is darwin. Direct non-hermetic go test: exit 0, "--- PASS: TestRepairAdmissionDirectDispatch"; log shows "rework admitted: reserved work fills its own reservation" | 2026-09-23 | opus-5.5-verifier |
| 2 | cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionConcurrentAndCrash$ -v -count=1 | exit 0; named PASS; two dispatchers race last slot -> one admission; crash at each boundary leaks no slot / duplicates no worker | Hermetic witness (verifyrun) could-not-check: check:ci needs Linux unshare --net, host is darwin. Direct non-hermetic go test: exit 0, "--- PASS: TestRepairAdmissionConcurrentAndCrash" | 2026-09-23 | opus-5.5-verifier |
| 3 | cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionUnknownAndExternalWait$ -v -count=1 | exit 0; named PASS; unreadable state explicit; external-wait repairs do not idle slots; forged repair class refused | Hermetic witness (verifyrun) could-not-check: check:ci needs Linux unshare --net, host is darwin. Direct non-hermetic go test: exit 0, "--- PASS: TestRepairAdmissionUnknownAndExternalWait" | 2026-09-23 | opus-5.5-verifier |
| 4 | cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestRepairAdmissionFullCycleRestart$ -v -count=1 | exit 0; named PASS; failed verify -> claimed repair -> review/merge -> one reverify -> independent pass resolves; restart at every transition preserves the obligation | Hermetic witness (verifyrun) could-not-check: check:ci needs Linux unshare --net, host is darwin. Direct non-hermetic go test: exit 0, "--- PASS: TestRepairAdmissionFullCycleRestart" | 2026-09-23 | opus-5.5-verifier |

RISK-VALUE: DERIVED — repairObligationLeaseTTL = 45 * time.Minute @ tools/desk/cmd/deskdispatch/repairadmission.go:66 — it must mirror the fanoutloop repair-obligation lease horizon (repairLeaseTTL = 45 * time.Minute @ tools/desk/cmd/fanoutloop/repair.go:46) so both readers judge the same obligation assignable-vs-stale on one clock; the two literals match exactly, and the source comment states the mirror intent. It is a reversible operational knob (a lease/timeout horizon): a wrong value only changes how long a crashed occupancy is honoured before reclaim, undoable by an edit and redeploy, so it ranks last for irreversibility.

Enumeration note: the only numeric literal this diff introduces is the lease TTL above. The other new literals are control/identity tokens, not risk-bearing thresholds: EnvRepairAdmission = "ASSAY_REPAIR_ADMISSION" @ repairadmission.go:39, RepairAdmissionPolicyVersion = "repair-admission-v1" @ repairadmission.go:46, repairAdmissionOn = "on" @ repairadmission.go:51. The reservation floor / width / expiry values are NOT introduced or changed by this diff — they are read from width.go / widthstore.go (desk-supervision/05) and are out of enumeration scope for this item.


## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
