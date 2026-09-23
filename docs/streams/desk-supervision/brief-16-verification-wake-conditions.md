---
brief: "assay:assay:desk-supervision:16"
title: "Verification wake conditions — stop repeating unchanged blocked checks"
why: "Verification repeatedly consumes a slot on work whose required repair or external prerequisite has not changed. Keep the failure visible while making the next expensive run depend on a checkable change."
wave: 0
depends: []
unblocks: ["desk-supervision/17"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["docs/streams/desk-supervision/recovery-increments.md", "freshness-checked 2026-09-20 @ 3db05fb44; source paths and open verification repair PR 1374 inspected"]
consumers: ["tools/desk/cmd/verifyloop: fixed-here", "tools/desk/cmd/deskevidence: fixed-here", "plugins/assay/skills/verify-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Cross-component scheduling state must survive partial failure without manufacturing completion or bypassing an existing role gate."
domain: "complicated"
version: 1
id: "e20812d4-f859-41a4-b597-072d5a89d413"
---

# Brief 16 — Verification wake conditions — stop repeating unchanged blocked checks

## Context

Home: medici-finance/assay.

files:
- tools/desk/cmd/verifyloop/adapter.go
- tools/desk/cmd/verifyloop/queueclass.go
- tools/desk/cmd/verifyloop/land.go
- tools/desk/cmd/deskevidence/deskevidence.go
- tools/desk/internal/deskkit/verifywake.go (planned)
- tools/desk/cmd/verifyloop/wake_test.go (planned)
- plugins/assay/skills/verify-desk/SKILL.md
- docs/streams/desk-supervision/verify-wake-v1.md (planned)
- tools/desk/README.md
- changelog/verify-wake-conditions.md (planned)

facts (2026-09-20; re-establish from the named files at pickup):
- The current verifier plan classifies rows from board/frontmatter; its failure landing files a bug and leaves the brief implemented. Neither operation records a complete machine-readable wake condition.
- Issue 1309 / PR 1374 already owns multi-root planning, Evidence-only dispatch, stuck-flip detection, per-row lane derivation and verifier worktree setup. Merge current main containing that change when it lands; preserve its behavior. This brief owns failed/blocked retries only, not those seven repairs.
- desk-tools/16 deduplicates equivalent Evidence blocks after work has run; it does not prevent the repeated dispatch.
- Existing outcome sidecars are append-only and read by statusgen; additions must preserve legacy rows and the old outcome vocabulary.

single-point-of-failure: the new scheduling classification; independent barriers remain the existing per-item claim and reviewer/verifier identity gates. A scheduling receipt can never authorize completion or a write.

## Read first

- [Recovery increments](recovery-increments.md) — scope, ordering, rollout and existing work.
- tools/desk/internal/loopengine/doc.go — existing executor and claim boundaries.
- The source files listed above; planned files are deliverables, not prerequisites.

## Interface contract

Add an optional versioned scheduling receipt to the existing verification outcome path, not a second lifecycle database. Fields: stable receipt ID, repo and brief ID, verifier identity, observed outcome, Verify-row IDs, input revisions, blocker kind, blocker reference and wake predicate. Input revisions include the Verify definition, relevant deliverable/dependency revisions and applicable tool version. The receipt is scheduling evidence only: it cannot grant verified/done or satisfy acceptance.

Initial blocker kinds are implementation, check-definition, human-action, environment and unknown. Wake predicates are relevant-input-changed, referenced-action-completed, declared-deadline-reached, and explicit-recheck-with-reason. External observations are supplied by already authorized readers; this work adds no production probes.

A valid unchanged receipt emits a visible WAIT row with its blocker and next actor; it is excluded from costly dispatch. Unreadable inputs produce visible COULD-NOT-CHECK, never an empty queue or a passing result. Legacy or incomplete receipts remain visibly unclassified and eligible for one classification pass. Duplicate receipt IDs do not create another event. An unrelated main commit does not wake work when declared inputs are known unchanged; an incomplete input scope cannot establish unchangedness.

## Ground rules

- Implement through a draft PR in this repository; stop at implemented. Independent verification owns acceptance.
- Work in an isolated checkout. Preserve existing role authority, stop flags, review lanes and human merge gates.
- No production queries, runtime activation, global configuration changes or autonomous-loop cutover in this code brief.
- Re-read open PR 1374 before editing verifier paths. If its overlapping work is still in flight, coordinate or stack explicitly; do not duplicate it.

## Task

1. Implement receipt parsing/validation and generation through the existing verifier result/Evidence path. Agent-driven landings and deterministic landings emit the same shape; derive identity from the existing trusted writer, never from an unchecked free-text assertion.
2. Extend plan classification and dispatch eligibility with the receipt comparison. Keep held rows in summary counts; expose why, who owns the next action, and what will wake them. This classification must work with today's plan-and-desk-agent execution.
3. Ensure partial runs can still dispatch newly runnable rows while retaining held row results. Recheck the identity and validity of inputs on each plan; do not copy a cached PASS onto a new revision.
4. Update the canonical verifier skill to consume these decisions. Document explicit recheck, stale/unknown receipts and legacy migration. No autonomous runner cutover or human-gate change.
5. Add the named tests below, using synthetic repositories, fake clocks and injected readers. No fixed production backlog counts.

## Verify

All named tests below are planned deliverables. The verifier must observe each named PASS line; exit zero with no tests run is not a pass. Tests use injected forge/clock state, no production services.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnchangedIsVisibleWait$ -v -count=1` | exit 0; named PASS for TestVerifyWakeUnchangedIsVisibleWait; same failed input and blocker: zero verifier dispatches, one visible WAIT, including after process restart |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeRelevantChange$ -v -count=1` | exit 0; named PASS for TestVerifyWakeRelevantChange; changed relevant file, Verify definition, tool or completed action wakes the affected work; an unrelated commit does not |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnknownAndLegacy$ -v -count=1` | exit 0; named PASS for TestVerifyWakeUnknownAndLegacy; unreadable dependencies and legacy records are distinct from empty/pass; no fabricated unchanged claim |
| 4 | check:ci +flow | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakePartialRows$ -v -count=1` | exit 0; named PASS for TestVerifyWakePartialRows; one newly runnable row executes without repeating held rows; no partial result closes the whole brief |

Pre-mortem → detection: False suppression hides executable work: rows 2 and 3. Restart loses the hold: row 1. A partial pass closes a brief: row 4. Judgment over the declared deliverable scope remains review-owned.

## Evidence

Pending implementation and independent verification. No acceptance result claimed by authoring.

### Non-implementer verifier run — VERIFY: BLOCKED — 2026-09-23 opus-5.5-verifier

| # | Command | Expected | Observed (exit + key output) | Date Runner |
|---|---------|----------|------------------------------|-------------|
| 1 | cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnchangedIsVisibleWait$ -v -count=1 | exit 0; named PASS; same failed input+blocker gives zero dispatches, one visible WAIT, incl. after restart | Direct run: exit 0, "--- PASS: TestVerifyWakeUnchangedIsVisibleWait". Hermetic witness (statusgen verifyrun): could-not-run — check:ci network-off sandbox needs Linux unshare --net; host is darwin. | 2026-09-23 opus-5.5-verifier |
| 2 | cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeRelevantChange$ -v -count=1 | exit 0; named PASS; changed relevant file/Verify def/tool/completed action wakes work; unrelated commit does not | Direct run: exit 0, "--- PASS: TestVerifyWakeRelevantChange". Hermetic witness: could-not-run — network-off sandbox needs Linux unshare --net; host is darwin. | 2026-09-23 opus-5.5-verifier |
| 3 | cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnknownAndLegacy$ -v -count=1 | exit 0; named PASS; unreadable deps and legacy records distinct from empty/pass; no fabricated unchanged claim | Direct run: exit 0, "--- PASS: TestVerifyWakeUnknownAndLegacy". Hermetic witness: could-not-run — network-off sandbox needs Linux unshare --net; host is darwin. | 2026-09-23 opus-5.5-verifier |
| 4 | cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakePartialRows$ -v -count=1 | exit 0; named PASS; one newly-runnable row executes without repeating held rows; no partial result closes the whole brief | Direct run: exit 0, "--- PASS: TestVerifyWakePartialRows" (dry-run repair-obligation records emitted, needs-assignment). Hermetic witness: could-not-run — network-off sandbox needs Linux unshare --net; host is darwin. | 2026-09-23 opus-5.5-verifier |

Witness note: statusgen verifyrun --brief docs/streams/desk-supervision/brief-16-verification-wake-conditions.md
returned could-not-run for all four check:ci rows on this darwin host (the network-off sandbox uses
unshare --net, a Linux facility). Recorded verbatim as could-not-check per the three-state rule: not
treated as a fail, not treated as a pass. verifyrun appended its witness table to the brief's ##
Evidence section in the verifier worktree (left modified and uncommitted for the desk to read).

RISK-VALUE: DERIVED — SchemaWakeV1 = "verify-wake-v1" @ tools/desk/internal/deskkit/verifywake.go:35 — the schema discriminator that separates a complete receipt from a legacy one; correct because it is a stable, unique tag and the validator treats its absence as legacy (r.Schema != SchemaWakeV1 -> unclassified), matching the brief contract "Legacy or incomplete receipts remain visibly unclassified". Reversible (edit + redeploy) and grants no completion authority.

RISK-VALUE ENUMERATION (mechanical, step 1 — non-empty; every entry a literal at file:line):
- SchemaWakeV1 = "verify-wake-v1" @ tools/desk/internal/deskkit/verifywake.go:35
- BlockerImplementation = "implementation" @ tools/desk/internal/deskkit/verifywake.go:41
- BlockerCheckDef = "check-definition" @ tools/desk/internal/deskkit/verifywake.go:42
- BlockerHumanAction = "human-action" @ tools/desk/internal/deskkit/verifywake.go:43
- BlockerEnvironment = "environment" @ tools/desk/internal/deskkit/verifywake.go:44
- BlockerUnknown = "unknown" @ tools/desk/internal/deskkit/verifywake.go:45
- WakeRelevantInputChanged = "relevant-input-changed" @ tools/desk/internal/deskkit/verifywake.go:51
- WakeReferencedActionDone = "referenced-action-completed" @ tools/desk/internal/deskkit/verifywake.go:52
- WakeDeadlineReached = "declared-deadline-reached" @ tools/desk/internal/deskkit/verifywake.go:53
- WakeExplicitRecheck = "explicit-recheck-with-reason" @ tools/desk/internal/deskkit/verifywake.go:54
- inputKeyTool = "tool" @ tools/desk/internal/deskkit/verifywake.go:348
- inputKeyFilePrefix = "file:" @ tools/desk/internal/deskkit/verifywake.go:349

RANK (by irreversibility): all entries are string vocabulary tags, not numeric thresholds, bounds,
tolerances, ratios, timeouts, limits, or authority bindings. Every one is reversible by an edit and
a redeploy, and by the brief's own SPOF note ("A scheduling receipt can never authorize completion or
a write") none carries irreversible authority. The blocker-kind and wake-predicate vocabularies are
copied verbatim from the brief's Interface contract (blocker kinds: implementation, check-definition,
human-action, environment, unknown; wake predicates: relevant-input-changed,
referenced-action-completed, declared-deadline-reached, explicit-recheck-with-reason), so each matches
its spec source. No irreversible risk-bearing value exists in the enumerated scope. Item risk
metadata is all "no", gate model, irreversible "no".


## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
