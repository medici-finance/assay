---
brief: statusgen/05
title: 'Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable)'
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-20 (authored clean for the statusgen board)
exec-tier: strong
exec-tier-why: scoring-integrity + adversarial anti-gaming — the critical tier reorders above every score and its inputs must resist self-selection (stamped-label authority + a reciprocity lint on the depends graph); getting the un-gameable derivation right is the load-bearing risk.
sources:
  - "The ratified drives DESIGN — this brief transcribes its Phase 3 (floors + critical tier) and the anti-starvation-floor + hard-critical-tier paragraphs of its Scoring section; it does not redesign"
  - "Phase 1 (landed): the additive drive term, the HeldByDriveCap bucket wired-but-0, the coverage self-tax"
  - "Phase 2 (landed): the drive frontier + states this phase's worker floor pairs with; the classify-and-take-other-work rule"
  - "statusgen/nextup.go — perStreamCap=4, spanOfControl=20, per-stream MaxConcurrent, HeldByStreamCap/HeldByDriveCap, buildRevDeps + blockedCount, unblocksWeight — the machinery the floor + tier extend"
  - "statusgen/drives.go — driveFocusWeight/drivePushWeight/driveSurgeWeight, maxConcurrentDrives=2, driveCoverageThreshold=0.40 + scaleForCoverage — the tunable-heuristic const block this phase adds the slot/worker caps beside"
  - "The worker-desk skill — the ≤6/8-worker floor that pairs with phase-2's interim WIP cap"
gate-why: >-
  Inherits the ratified design's human gate for the same reason — this phase is scoring-integrity
  code. It introduces the HARD critical tier: a lexicographic `(criticalTier, score)` ordering that
  reorders the critical tier ABOVE every score, so no intensity (surge included) can bury a live
  fire. The membership of that tier is the governance surface: it is fed by machine-derived /
  stamped signals only — a stamped security/critical label and a dependency-edge reciprocity lint
  that makes `blockedCount` un-gameable — precisely so a stream cannot self-declare itself critical.
  The human ratifies (1) that this tier may sit above the whole measured score, and (2) that the two
  new signals (the stamped label's authority chain, the reciprocity rule) are the right anti-gaming
  bar. A model verifier can confirm the Verify table passes but cannot ratify either governance
  choice; the human sign-off is required.
why: >-
  Phase 1 landed the additive drive term and WIRED the `HeldByDriveCap` bucket but left it at 0;
  phase 2 added the frontier + states + tracking issue. The steer can now re-rank the board, but two
  load-bearing safety properties from the Scoring section are still unbuilt. (1) ANTI-STARVATION:
  because staleness caps at +300, a buried same-tier brief can never out-age a drive — starvation is
  permanent-by-arithmetic WITHOUT reserved capacity. So drive work must be floored at ≤15 of 20
  board slots (a 2-pass fill populating the wired-but-empty `HeldByDriveCap`) and ≤6 of 8 workers,
  with a stream's declared max-concurrent always winning (`effectiveCap = min(driveStreamCap,
  maxConcurrent)`). (2) NEVER-BURIED: a routine surge must be able to jump a routine P0 but NEVER a
  live fire — main-red, a stamped security finding, a genuine high-unblocks brief, or a
  reviewer-finding remediation. That requires a hard lexicographic tier ABOVE all scores, whose
  membership cannot be self-declared — which is why it needs a stamped security/critical label and a
  dependency-edge reciprocity lint (so `blockedCount` cannot be gamed into the tier). Until both
  land, a drive can starve the rest of the board or bury a fire.
---

# Brief 05 — Drives phase 3: anti-starvation floors + the hard critical tier

An **implementation brief**. It builds Phase 3 of the drives feature that the ratified drives design
specified and a human ratified, stacking on the landed Phase 1 (the additive scoring term + the
manifest) and Phase 2 (the lifecycle frontier). The ratified design is the settled model; this brief
transcribes its **Phase 3 — floors + critical tier** and the anti-starvation-floor + hard-critical-tier
paragraphs of its **Scoring** section into code. Do not redesign anything here — the ratified design
is the authority.

## Context

files (created/extended by this phase):
- `statusgen/nextup.go` — the **2-pass fill**: drive-boosted picks take at most `driveSlotCap` (15)
  of the `spanOfControl` (20) slots on the first pass; the held-back drive picks fall to the second
  pass and increment the wired-but-empty `HeldByDriveCap` bucket (mirrors `HeldByStreamCap`). The
  per-stream `effectiveCap = min(driveStreamCap, maxConcurrent)` where `maxConcurrent` is the
  stream's declared `MaxConcurrent` (always wins). `spanOfControl` = 20 and `perStreamCap` = 4 are
  UNCHANGED. The Next-up ordering becomes lexicographic **`(criticalTier, score)`** — the critical
  tier ranks above every score, the drive term re-ranks only WITHIN a tier.
- `statusgen/drivecritical.go` (new) — the hard critical-tier derivation, machine-only, never
  self-declared: `main-red` (statusgen already knows CI-red), `security/leak` (a **stamped** label —
  new signal 1), `high-unblocks` (`blockedCount ≥ 3`, via the existing `buildRevDeps`/`blockedCount`),
  and `reviewer-finding` remediation. Pure, deterministic. No intensity — `surge` included — can pass
  it.
- `statusgen/drives.go` — the new tunable-heuristic const block members beside
  `driveFocusWeight`/`maxConcurrentDrives`/`driveCoverageThreshold`: `driveSlotCap` (15 of 20) and
  the worker floor constant the skill mirrors (`driveWorkerCap` 6 of 8). The optional per-stream
  `driveStreamCap` manifest field (fail-neutral parse) that `effectiveCap` mins against
  `MaxConcurrent`.
- `statusgen/main.go` — the **dependency-edge reciprocity lint** (new signal 2): a `--lint` check
  that every typed `depends:` edge feeding `blockedCount` is a genuine, non-self-referential,
  both-endpoints-exist edge, so a brief cannot inflate its `blockedCount` into the `high-unblocks`
  arm of the critical tier by manufacturing spurious inbound edges. A reciprocity violation is a
  `--lint` PROBLEM (the anti-gaming gate), NOT a fail-neutral drive WARN — this is board-graph
  hygiene, not a drive-manifest input.
- `statusgen/nextup_test.go`, `statusgen/drivecritical_test.go` (new) — the phase-3 Verify tests.
- the `worker-desk` skill — the **≤6/8-worker** anti-starvation floor, pairing with phase-2's
  interim ~5 WIP cap; the existing loop is otherwise UNCHANGED.

facts (the settled Phase-3 model — transcribe, do not redesign):

### The anti-starvation floor (load-bearing)
Because staleness caps at +300, a buried same-tier brief can never out-age a drive: starvation is
permanent-by-arithmetic without reserved capacity. So:
- Drive-boosted picks take **≤ 15 of 20** board slots via a **2-pass fill**: pass 1 offers non-drive
  picks and drive picks up to `driveSlotCap`; pass 2 fills any remaining span with the held-back
  drive picks, each held-back item attributing to the **`HeldByDriveCap`** bucket (mirrors
  `HeldByStreamCap`). The reserved ≥5 slots are non-drive capacity that a drive cannot consume.
- Workers: drive work takes **≤ 6 of 8** — the worker-desk skill floor (this brief's skill delta),
  superseding phase-2's interim ~5 WIP cap as the durable guard.
- A stream's declared max-concurrent (the serialization guard) **ALWAYS wins**:
  `effectiveCap = min(driveStreamCap, maxConcurrent)`. `spanOfControl` = 20 and `perStreamCap` = 4
  are unchanged.

### The hard "never-buried" critical tier
A lexicographic **`(criticalTier, score)`** ordering that ranks the critical tier ABOVE all scores.
Members (**machine-derived / stamped, NEVER self-declared**): main-red fixes (statusgen already knows
CI-red), security/leak findings (a **stamped** label — new signal 1), high-unblocks
(`blockedCount ≥ 3`), reviewer-finding remediations. **No intensity can pass it** — so `surge` may
jump a routine P0 but NEVER a live fire. The two new signals this phase adds are exactly the
membership's anti-self-selection guarantees:
1. **A stamped security/critical label.** The security/leak arm reads a machine-readable stamped
   label, not a brief author's word — membership is granted by the stamping authority, not claimed
   in-brief.
2. **A dependency-edge reciprocity lint.** The high-unblocks arm reads `blockedCount` (the reverse
   transitive typed-`depends:` walk, `buildRevDeps`). Without a check, a brief could inflate its
   `blockedCount` — and climb into the critical tier — by manufacturing spurious inbound `depends:`
   edges. The reciprocity lint rejects dangling / self-referential / one-sided edges so
   `blockedCount` reflects only genuine dependencies and cannot be gamed into the tier.

### Coverage self-tax — ALREADY landed in phase 1 (kept green, not re-implemented here)
The ratified design's Phase 3 also lists "the coverage>40% NOTICE + concave scale-down." That was
pulled forward into **phase 1** (`driveCoverageThreshold = 0.40` + `scaleForCoverage`, proven by
`TestDriveOverlapAndCoverage`). This phase does **not** re-implement it; it keeps
`TestDriveOverlapAndCoverage` green (no regression) as the floor + critical tier land beside it.

### Preserved from phases 1-2 (the safety bar — do not regress)
- **Absent ⇒ inert.** With no manifest, the board is BYTE-IDENTICAL to the no-drive baseline
  (`TestDriveAbsentIsInert`, still green — the floor + tier only reshape ordering when a drive is
  active; the critical tier's inputs are board-graph facts, so a no-drive board's critical rows sort
  identically to today because there is no drive term to reorder against).
- **Fail-neutral.** A malformed / expired / over-concurrent manifest still applies ZERO boost, the
  board still generates, WARN + banner, never an rc≠0 PROBLEM. The new `driveStreamCap` field
  validates fail-neutral too.
- **A steer, not a value claim.** The drive term stays excluded from the gate-score and every metric.
  (The critical tier is an ORDERING key over board-graph facts — main-red / stamped-label /
  blockedCount / reviewer-finding — not the drive term; it is not exported as a metric either.)
- **The one sanctioned wall-clock input** (the UTC-day drive window) is unchanged; this phase adds no
  new wall-clock read.

## Out of scope — deferred to later phases (named, not silently dropped)
- **Phase 4 — watchdog + dashboard**: the last-regen heartbeat + the independent 2×-cadence
  meta-alarm (mtime, not content); the `## Drive:` STATUS.md section (operator slice first); the
  tracking-issue mirror. This phase does NOT touch STATUS.md output.
- The coverage self-tax is **not re-scoped here** — it landed in phase 1 and is only kept green.

## Ground rules
- NEVER git push to main / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented`. `gate: human`: a model verifier records Evidence but cannot flip to
  `verified`; the human ratifies the critical-tier + anti-gaming governance choices (gate-why above).
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Verify (executable — no prose-only DoD items)

The phase-3 subset of the ratified design's Verify table, plus the lint-clean row.
`TestDriveOverlapAndCoverage` landed in phase 1 and is listed here as a **no-regression** guard, not
new work.

| # | Command | Expect |
|---|---------|--------|
| 5 | `cd statusgen && go test ./ -run TestDriveCriticalTierNeverBuried -v` | exit 0 — the lexicographic (criticalTier, score) order ranks main-red / stamped-security / high-unblocks(blockedCount≥3) / reviewer-finding rows above ALL scores; no intensity, surge included, can pass the critical tier; membership is machine-derived / stamped, never self-declared |
| 6 | `cd statusgen && go test ./ -run TestDriveAntiStarvationFloor -v` | exit 0 — drive work is capped at 15 of 20 board slots via a 2-pass fill (held-back to HeldByDriveCap) and 6 of 8 workers; effectiveCap = min(driveStreamCap, maxConcurrent); spanOfControl 20 and perStreamCap 4 unchanged |
| 7 | `cd statusgen && go test ./ -run TestDriveOverlapAndCoverage -v` | exit 0 — NO REGRESSION: the phase-1 overlap (max not sum) + coverage>40% NOTICE + concave scale-down still hold as the floor + critical tier land beside them |
| R | `cd statusgen && go test ./ -run TestDriveDepEdgeReciprocity -v` | exit 0 — the dependency-edge reciprocity lint rejects a dangling / self-referential / one-sided depends edge (so blockedCount cannot be gamed into the high-unblocks arm) as a --lint PROBLEM, and passes a genuine reciprocated edge |
| A | `cd statusgen && go test ./ -run TestDriveAbsentIsInert -v` | exit 0 — SAFETY BAR preserved: with no manifest the board is byte-identical to the no-drive baseline; the floor + tier reshape ordering only when a drive is active |
| L | `cd statusgen && go run . --root .. --lint` | exit 0 — this brief and its README row lint clean |

## Evidence
<!-- appended at implementation/verification time -->

## Review
Gate: human. This phase adds the hard critical tier (a lexicographic order ABOVE every score) and the
two anti-self-selection signals that feed it — a stamped security/critical label and a dependency-edge
reciprocity lint that makes `blockedCount` un-gameable (gate-why above). A model verifier records
Evidence but cannot flip to `verified`; the human ratifies the critical-tier + anti-gaming governance
choices and records verdict + date in the stream README table.
