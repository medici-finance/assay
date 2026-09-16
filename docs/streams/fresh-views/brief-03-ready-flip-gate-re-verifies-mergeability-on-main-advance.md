---
brief: assay:assay:fresh-views:03
title: ready-flip gate re-verifies mergeability on main-advance
why: >-
  The ready-flip gate checks mergeability once, at flip time. Main advances continuously, so an
  unrelated merge can regress a ready-for-human PR to CONFLICTING through no push of its own — and
  it then sits in the human's merge queue looking ready but unmergeable, because neither
  review-loop monitor is wired to notice (#339). A PR's "ready" state is a derived view of main at
  a sha; when main moves, the view must be re-derived or retracted, not left to a human to catch.
wave: 1
depends: ["fresh-views/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [339]
schema: brief-v2
authored: 2026-09-16 by fresh-views scoping session
sources:
  - "docs/streams/fresh-views/spec.md §2 (stamp + refuse), §5 Q2"
  - "medici-finance/assay#339 — ready-flip gate doesn't re-verify mergeability on main-advance; event monitor blind (no head-sha/state change), cadence sweep filter OMITS CONFLICTING"
  - "tools/desk/cmd/deskpost (the ready-flip verb), tools/desk/cmd/deskboard (the cadence board sweep + its delta filter), fresh-views/01 (helper)"
  - "MEMORY: deskpost ready no merge-state gate / flips CONFLICTING PRs; check mergeStateStatus (corroborating prior observations)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main): #339 confirmed OPEN; deskpost + deskboard present"
exec-tier: strong
exec-tier-why: (c) the flip is a control on the human merge queue; a subtly wrong re-check (e.g. trusting GitHub's async null `mergeable`) leaves the exact regression #339 describes and survives a happy-path test
domain: complicated
version: 1
id: 264dfd06-790e-492f-ae91-a88b14453162
consumers:
  # Authoring PR: code-path consumers routed to the deferred, self-targeting disposition (rule 6);
  # each flips to fixed-here in the implementation commit that edits the path.
  - "tools/desk/internal/freshview: follow-up fresh-views/03 (this brief; deskpost calls CheckFresh at the flip boundary — flips to fixed-here when implemented)"
  - "tools/desk/cmd/deskpost: follow-up fresh-views/03 (this brief; the ready-flip re-check + retract — flips to fixed-here when implemented)"
  - "tools/desk/cmd/deskboard: follow-up fresh-views/03 (this brief; cadence delta filter admits CONFLICTING — flips to fixed-here when implemented)"
---

# Brief 03 — ready-flip gate re-verifies mergeability on main-advance

## Context
files:
- `tools/desk/cmd/deskpost/*` — the ready-flip verb: on flip, stamp the mergeability verdict with the head it was computed against; provide a re-check path that re-reads mergeability against CURRENT `origin/main` and retracts (un-readies) or refuses to flip when it regressed to CONFLICTING.
- `tools/desk/cmd/deskboard/*` — the cadence board sweep: its delta filter currently omits `CONFLICTING`; add it so a ready PR that went dirty on main-advance produces an actionable delta.
- `tools/desk/internal/freshview/*` — READ ONLY: the fresh-views/01 helper.

facts:
- #339 root cause 1: the event monitor (REST) keys on `<repo>#<num> <sha> <state>`; a main-advance that makes a ready PR DIRTY changes neither head-sha nor state, so the event never fires — structurally blind.
- #339 root cause 2: the cadence board sweep filters deltas on `actionable|MERGE-NOW|UNREVIEWED|DECAY|NEEDS-REVIEW|RE-REVIEW|MAIN-RED|errors` — the filter OMITS `CONFLICTING`, so even a CONFLICTING delta on a ready PR is dropped.
- GitHub's async `mergeable` can read `null` right after a main-advance; a correct re-check must not treat `null` as "mergeable". Prefer a `git merge-tree` against current `origin/main` (deterministic) as the ground-truth signal, with the API `mergeStateStatus` as corroboration — do not trust a single async field.
- mergeability at head H is a derived view: `readyAt(sha=H)` is only valid while `origin/main == H`. When main advances to H', the view must be re-derived. The fresh-views/01 stamp records H at flip; the cadence sweep is the re-derivation trigger.
- single point of failure (rule 10): the ONE control is the re-check at the cadence sweep. Second, independent layer: the flip verb ITSELF refuses to flip (or retracts) when its stamped head is behind current main at the moment of any subsequent flip/status write — a different component (deskpost vs deskboard) tripping on the same class, so a gap in the sweep cadence does not leave the regression completely unguarded.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **deskpost:** stamp the mergeability verdict with the head at flip time. Add a `--recheck` (or equivalent) path that re-reads mergeability against current `origin/main` via `git merge-tree` (ground truth) + `mergeStateStatus` (corroboration), treating `null`/unknown as NOT-mergeable (fail-closed), and retracts a ready PR that regressed to CONFLICTING. The flip verb refuses to (re)flip when its stamped head is behind current main.
2. **deskboard:** add `CONFLICTING` to the cadence delta filter so a ready PR gone dirty on main-advance surfaces as an actionable delta the review loop acts on.
3. Both use `freshview.CheckFresh` to detect the stamped-head-behind-current-main condition; do not re-implement the comparison.
4. Add a table test: a PR flipped ready at head H, then an unrelated commit advances main to H' producing a `git merge-tree` conflict → the re-check retracts/flags it, and the deskboard filter admits the CONFLICTING delta.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskpost/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskpost/... -run TestRecheckRetractsOnMainAdvance -v` | exit 0; a PR mergeable at flip and CONFLICTING after an unrelated main-advance is retracted/flagged, and a `null` mergeable is treated as NOT-mergeable | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/deskboard/... -run TestCadenceFilterAdmitsConflicting -v` | exit 0; a CONFLICTING delta is NOT filtered out | check:ci +neighbour |
| 4 | `cd tools/desk && grep -n "CONFLICTING" cmd/deskboard/*.go` | exit 0; CONFLICTING appears in the delta-filter allowlist | check:ci +dereference |
| 5 | `cd tools/desk && go test ./cmd/deskpost/... ./cmd/deskboard/... -run TestReadyRegressionEndToEnd -v` | exit 0; a PR ready at head H then CONFLICTING after an unrelated main-advance is BOTH retracted by deskpost AND emitted as an actionable CONFLICTING delta by the deskboard sweep — the two-component regression path end to end | check +flow |
| 6 | `statusgen --consumers --root .` | exit 0 — the diff-aware consumers gate corroborates every routing token against the branch diff | check:ci +dereference |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
