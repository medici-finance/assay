---
brief: assay:assay:statusgen:05
title: 'Drives phase 3 — anti-starvation floors + the hard critical tier (≤15/20 slots via a 2-pass fill, ≤6/8 workers, effectiveCap; a lexicographic never-buried tier fed by a stamped security label + a dependency-edge reciprocity lint so blockedCount is not gameable)'
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
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
version: 1
id: 3e27b6ba-5b63-4e96-9bd8-7b12836d6b78
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

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian))

Non-implementer run against merged main `7aa3835d7f3323b04f7f4f69aafc82a8afefc37e`, darwin, offline
(`KUBECONFIG=/dev/null`), every test under a throwaway HOME that carries only a copy of the
roster, so the witness can stamp its on-behalf-of principal. A first witness write made before the
roster copy lacked that annotation, failed --lint, and was discarded uncommitted. The table directly below is the
execution witness written by `statusgen verifyrun` built from this tree, landed as the tool wrote
it. No Verify row is `check:ci` and no row needs a sibling checkout.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd statusgen && go test ./ -run TestDriveCriticalTierNeverBuried -v` | could-not-run exit=0 — Expect requires a count ≥ 3 but the output carries no number to read | sha256:f1f48520c340 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test ./ -run TestDriveAntiStarvationFloor -v` | pass exit=0 | sha256:979e1c8b43bd | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test ./ -run TestDriveOverlapAndCoverage -v` | pass exit=0 | sha256:10fa214324b0 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| R | `cd statusgen && go test ./ -run TestDriveDepEdgeReciprocity -v` | pass exit=0 | sha256:755692f89178 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| A | `cd statusgen && go test ./ -run TestDriveAbsentIsInert -v` | pass exit=0 | sha256:0477afd45094 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| L | `cd statusgen && go run . --root .. --lint` | pass exit=0 | sha256:de68c1b5faf1 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |

Expect-conformance, read row by row against merged main. A green exit proves the test ran; each
Expect cell also names behaviour, and three of those clauses do not hold on main.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---|---|---|---|---|
| 5 | `cd statusgen && go test ./ -run TestDriveCriticalTierNeverBuried -v` | main-red / stamped-security / high-unblocks / reviewer-finding rows rank above ALL scores | FAIL — exit 0; --- PASS with 5 subtests. The witness row carries the tool's third state (no verdict derived) because the Expect's blockedCount≥3 is parsed as a numeric floor over the test output; a --dry-run of the same tree earlier the same day derived pass (output sha256:a9870a3c3e9e), so the tool verdict on this row turns on incidental numbers in go test output. Read directly, only ONE of the four arms can lift a row on merged main: main-red is a stub (mainRedCritical returns false, statusgen/drivecritical.go line 100, and its subtest asserts the arm stays off); stamped-security is inert (criticalStampAuthorities is an empty map, drivecritical.go line 62, so no stamp is authorized); reviewer-finding cannot reach the board (a finding-named brief carries StaleRef, a hard Next-up exclusion at statusgen/nextup.go line 430). Only high-unblocks is live. Expected condition NOT met → row FAILS. | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go test ./ -run TestDriveAntiStarvationFloor -v` | 15 of 20 slots via a 2-pass fill (held-back to HeldByDriveCap) AND 6 of 8 workers; effectiveCap min; span 20 / perStreamCap 4 unchanged | FAIL — exit 0; --- PASS with 4 subtests. The slot floor holds: 15 drive + 5 non-drive picks, 5 held-back picks attributed to HeldByDriveCap, backfill when no non-drive work exists, effectiveCap = min(driveStreamCap, maxConcurrent), and the spanOfControl 20 / perStreamCap 4 asserts. The worker floor does not: driveWorkerCap = 6 (statusgen/drives.go line 88) has no reader (line 93 is `var _ = driveWorkerCap`), no test asserts it, and the worker-desk skill carries no drive worker floor (row S2). Expected condition NOT met → row FAILS. | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test ./ -run TestDriveOverlapAndCoverage -v` | no regression of overlap max-not-sum, coverage>40% NOTICE, concave scale-down | PASS — exit 0; --- PASS with 4 subtests (overlap-max-not-sum, coverage-over-40pct-taxes-and-notices, concave-monotonic, at-most-2-concurrent) | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| R | `cd statusgen && go test ./ -run TestDriveDepEdgeReciprocity -v` | dangling / self-referential / one-sided depends edge is a --lint PROBLEM; a reciprocated edge passes | FAIL — exit 0; --- PASS with 6 subtests. Self-referential and dangling edges are PROBLEMs and a reciprocated edge passes, as expected. A one-sided edge is NOT a PROBLEM: the subtest one-sided-depends-is-a-notice-not-a-problem pins it at NOTICE (statusgen/brieffile.go line 1731 routes reciprocityNotices into the notices channel), per the ruling recorded on #92; this Expect cell predates that ruling. blockedCount still counts one-sided edges (buildRevDeps, statusgen/nextup.go line 293, walks every declared depends edge with no reciprocity filter), so three manufactured one-sided inbound edges lift a brief into the high-unblocks arm while --lint exits 0. Expected condition NOT met → row FAILS. | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| A | `cd statusgen && go test ./ -run TestDriveAbsentIsInert -v` | no manifest → byte-identical to the no-drive baseline | PASS — exit 0; --- PASS: TestDriveAbsentIsInert. The critical-tier mark is applied only when a drive is active (statusgen/nextup.go line 736), which is what keeps the no-drive board identical | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| L | `cd statusgen && go run . --root .. --lint` | exit 0, no PROBLEM | PASS — exit 0, no PROBLEM line (witness above); re-run after this Evidence append, still exit 0 with no PROBLEM | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| S1 | `cd statusgen && go test ./ -run TestOneSidedDependsIsNoticeNotProblem -v` | supporting row R: how a one-sided edge surfaces end to end | exit 0; --- PASS: TestOneSidedDependsIsNoticeNotProblem — the fixture-level check that a one-sided edge is a NOTICE and never a PROBLEM | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| S2 | `grep -rn -E 'driveWorkerCap\|6 of 8' plugins/assay/skills/worker-desk/` | supporting row 6: the worker-desk skill carries the 6-of-8 drive worker floor | exit 1; no match. The only other mention of driveWorkerCap in the tree is its declaration and the blank reference in statusgen/drives.go | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| S3 | `grep -n 'for _, dep := range b.Depends' statusgen/nextup.go` | supporting row R: whether blockedCount filters unreciprocated edges | exit 0; line 293 (buildRevDeps) and line 479 — both add every declared depends edge, with no reciprocity check | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**VERIFY: FAIL** — 3 of 6 Verify rows meet their Expect (7, A, L); rows 5, 6 and R do not, on
clauses merged main does not implement. Gate: human, so this is Evidence only and the status stays
`implemented`. Suggested amendment for the brief author and the human: either land the missing
pieces (a ruled main-red marker, a ratified stamp authority, the worker-desk drive floor, and a
blockedCount that counts only reciprocated edges or a reciprocity lint back at PROBLEM), or amend
the rows 5, 6 and R Expect cells to what shipped and have the human ratify the narrower tier.

**Risk-bearing values.** The enumeration covers the implementing diff (feat(statusgen): drives
phase 3, `0d7144e95`), the later tier change (`6c64b6354`), and every value the Deliverables name.

- E1 `driveSlotCap = 15` @ statusgen/drives.go:87
- E2 `driveWorkerCap = 6` @ statusgen/drives.go:88
- E3 `highUnblocksThreshold = 3` @ statusgen/drivecritical.go:47
- E4 `criticalStampAuthorities = map[string]bool{}` @ statusgen/drivecritical.go:62
- E5 `securityCriticalStampRe = critical-security\(([0-9A-Za-z_.:-]+)\)` @ statusgen/drivecritical.go:69
- E6 `mainRedCritical` returns `false` @ statusgen/drivecritical.go:100
- E7 reciprocity tier = `notices` (NOTICE, not PROBLEM) @ statusgen/brieffile.go:1731
- E8 drive-stream-cap bounds: `DriveStreamCap < 0` rejected @ statusgen/drives.go:494, applied only when `StreamCap > 0` @ statusgen/drives.go:467
- E9 `defaultSpanOfControl = 20` @ statusgen/nextup.go:54 (unchanged, named)
- E10 `perStreamCap = 4` @ statusgen/nextup.go:22 (unchanged, named)
- The worker-pool size 8 in "6 of 8" has no literal anywhere in the tree, so it is dropped as an entry.

Ranking. Nothing here is irreversible: the board is recomputed on every regen, so a wrong value is
undone by an edit and a redeploy. What ranks highest is who can enter the tier that sits above every
score: E7 with E3 (the gameable high-unblocks arm), then E4 and E6 (the two arms that are off). E1
and E2 are reversible capacity knobs. E5, E8, E9 and E10 rank last and need no derivation (E9 and
E10 are asserted unchanged by row 6).

RISK-VALUE: NAMED, NOT DERIVED — reciprocity tier = notices @ statusgen/brieffile.go:1731 — the recorded ruling on #92 set the reciprocal-depends lint to NOTICE as a data-quality check, but this brief's gate-why makes the same lint the anti-gaming guarantee for the high-unblocks arm. At NOTICE, blockedCount still counts one-sided edges and --lint exits 0, so a brief can reach the tier above every score on manufactured edges. Whether the high-unblocks arm may stay live while its guard is non-fatal, or blockedCount should count only reciprocated edges, is a governance ruling this verifier cannot derive.
RISK-VALUE: DERIVED — highUnblocksThreshold = 3 @ statusgen/drivecritical.go:47 — the brief pins blockedCount ≥ 3, and it matches the score's own arithmetic: unblocksWeight 500 × 3 = 1500 exceeds one priority tier (weightP0 3000 − weightP1 2000 = 1000), so 3 is the smallest count at which the ordinary score already lets unblocking outrank a whole tier. Whether the count can be trusted is the E7 question above.
RISK-VALUE: NAMED, NOT DERIVED — criticalStampAuthorities = map[string]bool{} @ statusgen/drivecritical.go:62 — empty is the fail-safe placeholder (a stamp grants nothing), but which authority may stamp a brief into the tier is gate-why item 2's ratification, and no spec in the tree names one.
RISK-VALUE: NAMED, NOT DERIVED — mainRedCritical = false @ statusgen/drivecritical.go:100 — the main-red arm needs a ruled in-tree main-red marker. The brief's premise that statusgen already knows CI-red does not hold for the offline board path; the only red/green read in statusgen is the network-backed DORA timing path.
RISK-VALUE: DERIVED — driveSlotCap = 15 @ statusgen/drives.go:87 — staleness caps at stalenessCapDays 30 × stalenessPerDay 10 = 300, below the weakest drive intensity (driveFocusWeight 800), so within a priority tier a non-drive brief can never out-age a drive pick and any reserve of at least one slot is what prevents permanent starvation. 20 − 15 = 5 reserved slots is the ratified design's split, which the brief transcribes.
RISK-VALUE: NAMED, NOT DERIVED — driveWorkerCap = 6 @ statusgen/drives.go:88 — the value matches the brief's 6 of 8, but nothing reads it and the pool size 8 has no literal, so the ratio cannot be checked against anything that runs.

**Open question for the human (gate: human).** Before sign-off, rule on the four NAMED, NOT DERIVED
values above, verbatim: reciprocity tier = notices @ statusgen/brieffile.go:1731;
criticalStampAuthorities = map[string]bool{} @ statusgen/drivecritical.go:62; mainRedCritical =
false @ statusgen/drivecritical.go:100; driveWorkerCap = 6 @ statusgen/drives.go:88. The first
decides whether the only live arm of the critical tier can be gamed.
### Non-implementer verifier re-run: 2026-09-30, assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian). VERIFY: FAIL — row R: the dependency-edge reciprocity lint does not report a self-referential (or one-sided) depends edge on brief-v2 briefs (--lint exit 0, LINT: PASS on a v2 self-loop fixture; every brief in the tree is v2); rows 5, 6, 7, A, L pass

Re-run after #1825 and #1822 (the 2026-09-25 FAIL rows 5 and 6). gate: human, so this is Evidence only: status stays implemented, and the sign-off belongs to the human gate.

**Hand run**:
| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 5 | `cd statusgen && go test ./ -run TestDriveCriticalTierNeverBuried -v` | main-red / stamped-security / high-unblocks / reviewer-finding rows rank above ALL scores; no intensity passes the tier; membership machine-derived or stamped | PASS — exit 0; --- PASS with 9 subtests. An isolated sub-verification drove each of the four arms end to end through nextUp: each lifts a P2 row (total 1000) above a P0 surge row (total 7700), each stays off without its input, and a critical-looking flag in a brief's own frontmatter grants nothing. In deployment two arms have no input: nothing in the repo passes the main-health flag, and the critical-stamp authority key is absent from the deployed roster (named for the human below) | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `cd statusgen && go test ./ -run TestDriveAntiStarvationFloor -v` | 15 of 20 slots via a 2-pass fill held back to HeldByDriveCap; 6 of 8 workers; effectiveCap min; span 20 and perStreamCap 4 unchanged | PASS — exit 0; --- PASS with 10 subtests. Independent probes: 25 drive plus 10 non-drive picks give 15 drive and 5 non-drive with HeldByDriveCap 10; drive-only backfills to 20; worker floor offers 6, 2, 0, 0 drive rows at 0, 4, 6, 9 in flight and none when claims are unknown; the worker-desk skill states the 6-of-8 floor; span 20 and perStreamCap 4 match the pre-phase-3 source | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test ./ -run TestDriveOverlapAndCoverage -v` | no regression of overlap max-not-sum, coverage over 40 percent NOTICE, concave scale-down | PASS — exit 0; --- PASS with 4 subtests (overlap-max-not-sum, coverage-over-40pct-taxes-and-notices, concave-monotonic, at-most-2-concurrent) | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| R | `cd statusgen && go test ./ -run TestDriveDepEdgeReciprocity -v` | dangling / self-referential / one-sided depends edge is a --lint PROBLEM; a reciprocated edge passes | FAIL — exit 0; --- PASS with 9 subtests, but they call the lint helpers with brief-v1 style ids. End to end on a brief-v2 fixture, a brief whose depends and unblocks name itself lints exit 0 with LINT: PASS and no self-referential PROBLEM, and a one-sided edge emits no line at all; the same fixtures in brief-v1 give 2 self-referential PROBLEMs (exit 1) and a one-sided NOTICE. Every brief in the tree is brief-v2. Dangling edges are a PROBLEM in both schemas and a reciprocated edge passes. The anti-gaming purpose holds: the high-unblocks arm reads a reciprocated-only graph, so three one-sided inbound edges do not lift a brief and three reciprocated ones do. Expected condition NOT met → row FAILS. | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| A | `cd statusgen && go test ./ -run TestDriveAbsentIsInert -v` | no manifest gives a board byte-identical to the no-drive baseline; floor and tier reshape order only with a drive | PASS — exit 0; --- PASS: TestDriveAbsentIsInert. Independent end-to-end probe: with all four critical inputs present and no manifest, the Next-up board and the next-up JSON are byte-identical across each phase-3 merge and its parent, and toggling the critical inputs changes nothing until a drive is active | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |
| L | `cd statusgen && go run . --root .. --lint` | exit 0, this brief and its README row lint clean | PASS — exit 0; LINT: PASS, zero PROBLEM lines | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian) |

**Execution witness**:
Command: `HOME=<throwaway> verifyrun --brief docs/streams/statusgen/brief-05-drives-phase3-floors-critical-tier.md --root <wt>` (statusgen built from <wt>/statusgen at b0088804294b, --version=dev; HOME is a throwaway holding only a copy of the roster) — exit 0; `verifyrun --check` on the written table, exit 0: "6 pass, 0 fail, 0 could-not-run/missing (of 6 Verify rows)". No row is check:ci, so the Linux docker recipe was not needed. The worktree brief was restored with git checkout after the table was captured.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd statusgen && go test ./ -run TestDriveCriticalTierNeverBuried -v` | pass exit=0 | sha256:34dc00237460 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test ./ -run TestDriveAntiStarvationFloor -v` | pass exit=0 | sha256:64030c75d0fd | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test ./ -run TestDriveOverlapAndCoverage -v` | pass exit=0 | sha256:8831b926569f | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| R | `cd statusgen && go test ./ -run TestDriveDepEdgeReciprocity -v` | pass exit=0 | sha256:ad40e71604e9 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| A | `cd statusgen && go test ./ -run TestDriveAbsentIsInert -v` | pass exit=0 | sha256:3afd3bf25cba | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| L | `cd statusgen && go run . --root .. --lint` | pass exit=0 | sha256:a12a79f62f3d | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

**check-verified** (hypothetical README flip on a throwaway clone):
Throwaway clone: <wt>/.cv (git clone --no-hardlinks, detached at b0088804294b, refs/remotes/origin/main set to the same SHA).
- What I changed there:
  - Inserted a "### Verification — 2026-09-30 re-verify" block (the witness table, then the hand table, then `**VERIFY: FAIL** — row R.`) into the brief just before `## Review`.
  - Set README row 05 Status to `verified` and the Verified cell to `2026-09-30 assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5)`.
- `statusgen brief --root . --check-verified statusgen/05` → exit 0. Output: the brief JSON only, with row.status "verified" and the Verified cell echoed. It printed no verdict line and did not object to a gate:human row being set to verified, or to the VERIFY: FAIL line inside Evidence.
- `statusgen --root . --lint` → exit 0, `LINT: PASS`, 0 PROBLEM lines. Two NOTICEs name this brief:
  - "is gate:human at verified but has no decision-issue — file one via --decision-issues"
  - the risk-files-crossread could-not-check NOTICE (no `files:` line in Context).
- The clone was reset afterwards. This is hypothetical only: the item is gate:human and row R fails, so nothing should be set to verified.

RISK-VALUE: NAMED, NOT DERIVED — selfID = bf.Brief @ statusgen/brieffile.go:1685 — observed wrong for brief-v2. The declared ref is stream/NN form, while bf.Brief is the hierarchical v2 id, so the self-referential PROBLEM never fires on a v2 brief. Row R fails on this value (details under Notes). The correct key is the brief's stream/NN id; I cannot derive a value it would be "right" at.
RISK-VALUE: NAMED, NOT DERIVED — reciprocity tier = notices @ statusgen/brieffile.go:1745 — set by the driver's recorded ruling: a data-quality lint that returns to PROBLEM once the backlog is reconciled. The tier is no longer load-bearing for anti-gaming, because the high-unblocks arm reads a reciprocated-only graph (buildReciprocatedRevDeps, statusgen/nextup.go:368). On brief-v2 the NOTICE does not fire at all (E8/E7 share the same keying).
RISK-VALUE: DERIVED — highUnblocksThreshold = 3 @ statusgen/drivecritical.go:78 — 3 × unblocksWeight 500 = 1500, which exceeds one priority tier (weightP0 3000 − weightP1 2000 = 1000), while 2 × 500 = 1000 does not. So 3 is the smallest count at which the ordinary score already lets unblocking outrank a whole tier, and it matches the brief's blockedCount ≥ 3. The count now reads reciprocated edges only.
RISK-VALUE: NAMED, NOT DERIVED — criticalStampAuthorities = map[string]bool{} (unset) @ statusgen/drivecritical.go:91 — empty/unset is fail-safe: a stamp grants nothing, and each stamp is reported as a NOTICE. Which authority may stamp is gate-why item 2's human ratification, and the deployed roster carries no ASSAY_CRITICAL_STAMP_AUTHORITIES key today, so the arm is inert in deployment.
RISK-VALUE: NAMED, NOT DERIVED — main-health state "red" @ statusgen/mainhealth.go:61 — the main-red arm fires only on an injected --main-health red:<ref> input, and no caller in .github/ or tools/ supplies it: deskboard runs --next-up --root only, at tools/desk/cmd/deskboard/dispatch.go:166. So in deployment the arm always reads could-not-check, and the board names that only while a drive is active. Who supplies the input is a wiring or governance decision I cannot derive.
RISK-VALUE: DERIVED — driveSlotCap = 15 @ statusgen/drives.go:94 — staleness caps at 30 × 10 = 300, which is below the weakest drive intensity (driveFocusWeight 800). Within a tier, a non-drive brief can therefore never out-age a drive pick, and only reserved capacity prevents permanent starvation. 20 − 15 = 5 reserved slots is the ratified split. The sub-verification observed exactly 15 drive + 5 non-drive picks with HeldByDriveCap 10.
RISK-VALUE: NAMED, NOT DERIVED — driveWorkerCap = 6 @ statusgen/drives.go:95 — matches the brief's 6 and is now enforced on the dispatch queue and across roots. But it is an absolute count, not a share of the pool: the "8" is the worker-desk width default (tools/desk/internal/deskkit/width.go:211), so the ratio holds only at the default width. The 6-of-8 split itself is the ratified design's number, with no first-principles derivation in the tree.

Notes:
Prior FAIL rows (2026-09-25):
- **Row 5 now PASSES.**
  - #1825 wired all three previously dead arms: main-red via --main-health, stamped-security via the roster authority set, and reviewer-finding via a finding's control:.
  - The isolated sub-verification lifted a low-score row above a surge row through each arm end to end.
- **Row 6 now PASSES.**
  - #1822 made driveWorkerCap bind the --next-up dispatch queue and the deskboard cross-root merge.
  - The worker-desk skill now states the floor.
- **Row R still FAILS, for a different reason than before.**
  - The earlier gaming concern is fixed: high-unblocks now reads reciprocated edges only, and 3 one-sided inbound edges do not lift a brief, verified end to end.
  - But end to end on a brief-v2 fixture, the reciprocity lint does not report a self-referential depends/unblocks edge (lint exit 0, LINT: PASS) and emits nothing for a one-sided edge. Every brief in the tree is brief-v2. On main's own tree the live lint emits zero one-sided and zero self-referential lines, while the row R sub-verifier counted 49 one-sided edges in the frontmatter.
  - The row's own test passes only because it calls the lint helpers with brief-v1-style ids.
  - The one-sided-at-NOTICE part is the driver's recorded ruling and is not what fails this row. The self-referential clause is.

Findings routed for filing:
- F1 (bug, statusgen): the dependency-edge reciprocity lint is blind on brief-v2 briefs. A self-referential edge is not a PROBLEM, and a one-sided edge gets no NOTICE.
  - Reproduction: a two-brief brief-v2 fixture where sx/01 declares depends ["sx/01"] and unblocks ["sx/01"]. `statusgen --root <fixture> --lint` → exit 0, LINT: PASS. The same fixture as brief-v1 → exit 1, 2 self-referential PROBLEMs.
  - Code: statusgen/brieffile.go:1685, :1690-1692, :1799, :1826.
  - The row R test needs a checkBriefFiles-level brief-v2 fixture.
  - Not checked against the contents of #1925/#1926/#1927; the desk should dedupe.
- F2 (question / needs-decision): the main-red arm has no input in deployment. Nothing in the repo passes --main-health. The brief's premise that "statusgen already knows CI-red" does not hold offline (mainhealth.go says so), so wiring a caller is a decision.
- F3 (needs-decision, already inside gate-why item 2): ASSAY_CRITICAL_STAMP_AUTHORITIES is absent from the deployed roster, so the stamped-security arm grants nothing. The changelog asks that every roster reader be re-pinned before the key is added.
- F4 (test gap, low): TestDriveAbsentIsInert (row A) is phase-1 code with no critical inputs, so on its own it does not prove the phase-3 half of row A. The sub-verification did prove it end to end:
  - with no drive, STATUS.md and --next-up JSON are identical across #1825 and #1822 versus their parents;
  - toggling the stamp, authority, --main-health red and reciprocated edges changes nothing without a drive.
  - Separately, criticalStampNotices (statusgen/main.go:700) adds a stderr/--lint NOTICE even with no drive. The board is unaffected and the exit code stays 0.
- The lint also raises gotest-run-vacuous NOTICEs on every go test row (unanchored -run with no --- PASS assertion). The brief author may want to anchor them. This is not a failure.

Gate: human. This is Evidence only: the status stays `implemented`, and a model never sets it to verified.

VERIFY: FAIL — row R: the dependency-edge reciprocity lint does not report a self-referential (or one-sided) depends edge on brief-v2 briefs (--lint exit 0, LINT: PASS on a v2 self-loop fixture; every brief in the tree is v2); rows 5, 6, 7, A, L pass

## Review
Gate: human. This phase adds the hard critical tier (a lexicographic order ABOVE every score) and the
two anti-self-selection signals that feed it — a stamped security/critical label and a dependency-edge
reciprocity lint that makes `blockedCount` un-gameable (gate-why above). A model verifier records
Evidence but cannot flip to `verified`; the human ratifies the critical-tier + anti-gaming governance
choices and records verdict + date in the stream README table.
