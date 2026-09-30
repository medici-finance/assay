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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnchangedIsVisibleWait$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeRelevantChange$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnknownAndLegacy$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakePartialRows$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier run — VERIFY: BLOCKED (4 check:ci rows could-not-check on darwin, hermetic witness owed; direct runs supporting-only PASS 4/4) — HELD at implemented — 2026-09-27 claude-opus-5-5-verifier (verify-desk dispatch), merged main 9585b4b6cc2e

Runner is not the implementer (implementing change: PR #1396, squash 6ea69f797). Offline envelope (KUBECONFIG=/dev/null), own detached worktree at merged main. gate: model; risk all four answers no. The witness above is the authoritative record: every row is check:ci, and the check:ci hermetic lane needs a Linux network-off sandbox this darwin host does not have (the known Linux-witness gap, #1491). The pinned harness container was not used: its image is not cached locally and pulling it is an external fetch outside the offline envelope.

Per-row direct runs (supporting only — not a substitute for the hermetic witness), go1.27.1 darwin/arm64:

- Row 1 — exit 0; "--- PASS: TestVerifyWakeUnchangedIsVisibleWait (0.01s)"; the test asserts WAIT, zero dispatches, and a reason naming "blocker: implementation" and "next: worker", on a first pass and again on a fresh loop re-reading the same sidecar (the restart case).
- Row 2 — exit 0; "--- PASS: TestVerifyWakeRelevantChange (0.00s)"; four briefs woken (changed file, tool version, Verify definition, completed action), the fifth (declared input unchanged) held; exactly 4 dispatches.
- Row 3 — exit 0; "--- PASS: TestVerifyWakeUnknownAndLegacy (0.00s)"; unreadable input is could-not-check (never WAIT); legacy row and empty-scope receipt both dispatch for one classification pass.
- Row 4 — exit 0; "--- PASS: TestVerifyWakePartialRows (0.00s)"; held rows "2,3" recorded, dispatch prompt carries WAKE-HELD, Land never flips on FAIL or BLOCKED.
- Supporting: whole verifyloop package exit 0; the five deskkit EvaluateWake unit tests PASS.

Mutation check (own worktree, each restored with a path-specific checkout; tree confirmed clean after): 5 of 5 mutants caught by the four named tests —
M1 queueclass hold branch returns dispatch: rows 1 and 2 FAIL; M2 revision comparison disabled in EvaluateWake: row 2 FAIL; M3 empty input scope treated as complete: row 3 FAIL; M4 unreadable inputs ignored: row 3 FAIL; M5 partial-hold branch disabled: row 4 FAIL.

Risk-bearing value (kit section 4). Enumeration over the implementing diff (tools/desk/internal/deskkit/verifywake.go, tools/desk/cmd/verifyloop briefscan.go / queueclass.go / dispatch.go / adapter.go / main.go):

- SchemaWakeV1 = "verify-wake-v1" @ tools/desk/internal/deskkit/verifywake.go:35
- blocker kinds = implementation / check-definition / human-action / environment / unknown @ verifywake.go:41-45
- wake predicates = relevant-input-changed / referenced-action-completed / declared-deadline-reached / explicit-recheck-with-reason @ verifywake.go:51-54
- IsFailedOrBlocked outcome set = "verify-fail", "fail", "blocked", "needs_context", "needs-context" @ verifywake.go:112
- Complete() input-scope bound: len(r.Inputs) > 0 @ verifywake.go:132
- NextActor binding = implementation→worker, check-definition→brief-author, human-action→human, environment→operator, else verifier @ verifywake.go:147-156
- deadline comparison: fires when !now.Before(dl) (inclusive) @ verifywake.go:267; accepted layouts RFC3339 and "2006-01-02" @ verifywake.go:297
- input keys: inputKeyTool = "tool", inputKeyFilePrefix = "file:" @ verifywake.go:348-349; root containment rejects ".." / absolute @ verifywake.go:367
- latest-receipt rule: last sidecar line per brief wins, out[rec.Brief] = rec @ tools/desk/cmd/verifyloop/briefscan.go:396
- partial-hold condition: len(rec.Rows) > 0 && len(remaining) > 0 @ briefscan.go:427
- bucket slug "wait" @ tools/desk/cmd/verifyloop/queueclass.go:100

Ranking: none is irreversible. The receipt is scheduling evidence only; it never flips status or authorizes a write (Land still flips only on a whole-brief PASS, row 4 plus M5). The worst wrong value is false suppression (a runnable brief held); it is undone by an edit and redeploy, or at once by an explicit-recheck receipt. The top-ranked entries are the ones that decide suppression:

RISK-VALUE: DERIVED — Complete() bound len(r.Inputs) > 0 @ tools/desk/internal/deskkit/verifywake.go:132 — an empty declared scope makes "every declared input unchanged" vacuously true, which would hold a brief forever on no evidence; the brief's contract ("an incomplete input scope cannot establish unchangedness") requires exactly this bound, and M3 shows row 3 catches its removal.
RISK-VALUE: DERIVED — unreadable-first ordering, if len(unreadable) > 0 before the changed/hold decision @ verifywake.go:238 — the three-state instrument rule: a partly-read scope cannot prove unchanged, so could-not-check must win over hold; M4 shows row 3 catches its removal.
RISK-VALUE: DERIVED — IsFailedOrBlocked outcome set @ verifywake.go:112 — the brief scopes this to failed/blocked retries only; "verified" is left to the stuck-flip lane, and a PASS never produces a hold. The set covers the loopengine verdict spellings (FAIL / BLOCKED / NEEDS_CONTEXT, lower-cased) and the sidecar spelling "verify-fail". An outcome outside the set is never held, which fails toward dispatch (safe).
RISK-VALUE: DERIVED — partial-hold condition len(rec.Rows) > 0 && len(remaining) > 0 @ tools/desk/cmd/verifyloop/briefscan.go:427 — a receipt naming no rows is a whole-brief hold, and one naming a strict subset leaves the rest runnable, which is what Task 3 requires; the dispatch prompt forbids a whole-brief PASS on that run (M5 caught).
RISK-VALUE: DERIVED (reversible, ranks last) — deadline fires when !now.Before(dl) @ verifywake.go:267 — inclusive at the deadline; being early or late by one tick costs one verifier slot at most. The NextActor binding (verifywake.go:147-156) is a label printed on the WAIT line, not an authority grant; reversible.

Observations (outside the Verify table; judgment over the declared deliverable scope is review-owned, so these are for the desk to route, not a FAIL):

1. Receipt generation is not wired into a deterministic landing path. NewWakeReceipt has no caller outside tests; verifyloop's land.go and deskevidence were not changed by #1396, although the frontmatter lists deskevidence as a consumer (fixed-here) and Task 1 asks that agent-driven and deterministic landings emit the same shape. In practice receipts reach the sidecar only when an agent writes the JSON row by hand (23 of 46 current sidecar rows carry wake_schema). No code checks that the verifier field in a hand-written row matches the landing identity, so the "never from an unchecked free-text assertion" rule holds only by procedure.
2. The offline reader (RootRevisionReader) answers only "tool" and "file:" keys. A receipt that declares a "verify-def:" input, as test 2 does with a fake reader, reads as could-not-check in production. This is fail-safe (never a false hold) but means a Verify-definition change is detected only when the brief file itself is a declared file: input.
3. tools/desk/README.md cites the spec as docs/streams/example-stream/verify-wake-v1.md, a path the slug-neutralising commit introduced. It does not resolve; the real spec is docs/streams/desk-supervision/verify-wake-v1.md.

VERIFY: BLOCKED — rows 1-4 could-not-check (check:ci hermetic witness owed on a Linux runner, #1491); direct runs PASS 4/4 and 5/5 mutants caught, supporting only. Status stays implemented. blocker: environment.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ 35496323b8fc (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer re-verify on merged main 35496323b8fc591651e44537baf51206cc22bbdd. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm, `--network none`, a full clone pinned to this SHA, statusgen built from main's own source) and passed 4/4. Second table: the hand run on the host (darwin/arm64, go1.27.1).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnchangedIsVisibleWait$ -v -count=1` | pass exit=0 | sha256:858ad7a2895b | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeRelevantChange$ -v -count=1` | pass exit=0 | sha256:b39423aa1307 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakeUnknownAndLegacy$ -v -count=1` | pass exit=0 | sha256:a42af252455e | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/verifyloop/ -run ^TestVerifyWakePartialRows$ -v -count=1` | pass exit=0 | sha256:5888a434c0b6 | 2026-09-30 | assay-verifier-app[bot] @ 35496323b8fc (on-behalf-of human:ian) (git-config) |

| # | Verify row | Expected | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | row 1 as written (exact command in the witness table above) | exit 0; named PASS; zero dispatches, one visible WAIT, incl. after restart | exit 0; "--- PASS: Test Verify Wake Unchanged Is Visible Wait (0.00s)"; "ok .../tools/desk/cmd/verifyloop 0.396s". Same result re-run under a network-denied macOS sandbox (exit 0, PASS) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 2 | row 2 as written (exact command in the witness table above) | exit 0; named PASS; changed file / Verify def / tool / completed action wake, unrelated commit does not | exit 0; "--- PASS: Test Verify Wake Relevant Change (0.01s)"; "ok .../cmd/verifyloop 0.214s". Network-denied re-run exit 0, PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 3 | row 3 as written (exact command in the witness table above) | exit 0; named PASS; unreadable and legacy distinct from empty/pass; no fabricated unchanged | exit 0; "--- PASS: Test Verify Wake Unknown And Legacy (0.00s)"; "ok .../cmd/verifyloop 0.233s". Network-denied re-run exit 0, PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |
| 4 | row 4 as written (exact command in the witness table above) | exit 0; named PASS; one newly runnable row executes without repeating held rows; no partial closes the brief | exit 0; "--- PASS: Test Verify Wake Partial Rows (0.00s)"; "ok .../cmd/verifyloop 0.211s". Network-denied re-run exit 0, PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ 35496323b8fc (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun` on linux, network-off, 4/4 pass at 35496323b8fc. This supersedes the hand run's could-not-check note on the hermetic witness.

RISK-VALUE: DERIVED — Complete() bound len(r.Inputs) > 0 @ tools/desk/internal/deskkit/verifywake.go:132 — with an empty declared scope "every declared input unchanged" is vacuously true, which would hold a brief forever on no evidence; the brief's contract ("an incomplete input scope cannot establish unchangedness") requires exactly a non-empty scope; M3 shows row 3 catches its removal.
RISK-VALUE: DERIVED — unreadable-first ordering, if len(unreadable) > 0 @ tools/desk/internal/deskkit/verifywake.go:238 — three-state instrument rule: a partly read scope cannot prove unchanged, so could-not-check must win over both hold and fire; M4 shows row 3 catches its removal.
RISK-VALUE: DERIVED — IsFailedOrBlocked set {"verify-fail","fail","blocked","needs_context","needs-context"} @ tools/desk/internal/deskkit/verifywake.go:112 — the brief scopes wake to failed/blocked retries only; the set covers the loopengine verdict spellings lower-cased plus the sidecar spelling "verify-fail"; any outcome outside the set is never held, which fails toward dispatch (safe); M6 caught by all four rows.
RISK-VALUE: DERIVED — partial-hold condition len(rec.Rows) > 0 && len(remaining) > 0 @ tools/desk/cmd/verifyloop/briefscan.go:419 — a receipt naming no rows is a whole-brief hold, one naming a strict subset leaves the remainder runnable, which is Task 3; M5 caught by row 4.
RISK-VALUE: DERIVED (reversible, ranks last) — deadline fires on !now.Before(dl) @ tools/desk/internal/deskkit/verifywake.go:267 — inclusive at the deadline instant; an off-by-one tick costs at most one verifier slot. NextActor / "wait" / schema tag are printed labels, not authority grants; reversible.

Notes:
- The four named tests caught all 7 deliberately broken versions of the code (mutation probes run in a scratch export).
- Finding (does not fail a row): in a scratch probe, the verifyloop landing flipped a PASS verdict for an item with held rows 2 and 3. Row 4's "no partial result closes the brief" is proven only for FAIL and BLOCKED. Today the only wired flip writer is the dry-run printer, and statusgen's held-row detector guards the real flip. Suggested follow-up: the landing refuses to flip while held rows are set.
- Carried: the wake-receipt constructor is still called only from tests. The offline reader understands only tool and file inputs, so verify-def inputs read as could-not-check, which fails safe. The tools/desk README and a code comment cite a nonexistent example-stream spec path; the real spec is the desk-supervision verify-wake-v1 contract.

VERIFY: PASS

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
