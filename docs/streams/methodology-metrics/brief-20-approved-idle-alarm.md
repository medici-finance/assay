---
brief: methodology-metrics/20
title: Approved-idle alarm + merge-when-green — approval is a perishable state when main outruns the merge
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: '2026-07-11 by Fable session (human:<name>: write the merge-velocity vs review-lap economics into a methodology brief)'
sources: ["lived data 2026-07-10/11 (the PR #285 lap log — measured below)", "methodology-metrics/10 (drain-the-constraint precedent)", "methodology-metrics/17 (the Awaiting-Age sibling: same disease at the verify stage)", "desk-tools/02 (deskboard v2, implemented — the surface this extends)", "freshness-checked 2026-07-11 @ 2a8cd673"]
why: >-
  Merge-velocity vs review-lap economics: when main merges faster than the
  approve→flip→merge latency, an APPROVED verdict decays — the PR re-conflicts, the
  worker resolves, the reviewer re-reviews, and the approval must be re-earned. A prompt
  merge after APPROVED-at-head costs ~zero marginal; every delay lap costs one
  conflict-resolution (worker), one delta-review (reviewer tokens), and lead-time.
  Measured 2026-07-10 evening: PR #285 was App-APPROVED at head twice and re-conflicted
  four times (~5 keep-current merges) while main merged 15+ PRs in ~3h — three full laps
  that a 5-minute merge window would have made free. Nothing on the board today ranks
  "approved and decaying" above "new review to dispatch", so the cheapest action loses.
---

# Brief 20 — Approved-idle alarm + merge-when-green

## Context
files: tools/desk/cmd/deskboard/ (v2, implemented — action classifier + report) + tests;
docs/streams/methodology-metrics/README.md (row)
out-of-repo files: ~/.claude/skills/pr-review-desk/SKILL.md (the drain-before-pull policy
line — staged as a diff in the PR body, applied live LAST before `implemented`, committed
in the ~/.claude stopgap repo; rule 7: max ONE out-of-repo brief in flight — check the
board before dispatch)
facts:
- **The economics, stated once (the rule the alarm enforces):** approval is perishable.
  Cost of merging an APPROVED-at-head PR promptly ≈ 0. Cost of NOT merging it before the
  next main merge that touches its files = 1 keep-current conflict resolution + 1
  RE-REVIEW delta + the latency; the lap repeats until merged. When main's merge rate is
  high, merge-when-green strictly dominates FIFO desk work: drain APPROVED PRs before
  dispatching new reviews (the mm/10 drain-the-constraint logic applied one gate earlier).
- deskboard v2 (`../assay-toolkit/tools/desk/cmd/deskboard`, desk-tools/02, implemented) already computes
  per-PR ACTION classes {NEEDS-REVIEW, RE-REVIEW, BLOCKED, CHECK, WAIT-CI, CI-RED,
  MERGE-CURR, FLIP, READY, MERGED-tombstone} from head-vs-review state and CI rollup.
- **Deliverable 1 — MERGE-NOW class + approved-age:** a PR whose latest App review is
  APPROVED at the CURRENT head with CI green (READY, or FLIP where the desk flip is the
  only step left) ranks at the TOP of the board as **MERGE-NOW**, carrying
  `approved-age` (time since that approval). When approved-age exceeds a threshold
  (default 20m ≈ the measured lap period; flag-tunable like --span), the report emits a
  decay banner: the approval is perishable — merge it or it re-laps. Read-only like all
  of deskboard; GET-only gh calls; C-10 fail-closed unchanged.
- **Deliverable 2 — drain-before-pull (skill policy):** pr-review-desk leads every board
  report with MERGE-NOW items and surfaces them to human:<name> BEFORE dispatching any new review;
  a desk turn that dispatches reviews while MERGE-NOW items sit unsurfaced is the
  anti-pattern (name it in the skill).
- **Stacked-PR corollary (record as a rule line in the skill):** a stacked PR multiplies
  laps — every main merge ripples through base AND head, and the two branches can resolve
  the same hunk differently (measured same evening: the #285/#307 stack needed dual
  resolutions twice). Collapse stacks early: merge the head into its base as soon as the
  head is approved, then drive the base alone.
- Boundary: board/report rendering + desk policy ONLY — no Next-up score input ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)),
  no auto-merge (merge stays human; MERGE-NOW is a ranked surface, not an act), no
  statusgen change (statusgen has no gh access; PR state is deskboard's domain).
- mm/17's Awaiting-Age is the same instrument one gate later (merged→verified); this is
  approved→merged. Together they cover the two decay windows the DORA lead-time hides.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD (fixture gh, matching deskboard's existing test harness): APPROVED-at-head +
   CI-green → MERGE-NOW, ranked first; APPROVED-then-new-push → RE-REVIEW (not MERGE-NOW);
   approved-age computed from the review timestamp; age > threshold → decay banner line;
   threshold flag parse (default 20m).
2. Implement in tools/desk/cmd/deskboard (classifier + ranking + banner + flag).
3. Amend pr-review-desk SKILL.md per facts (drain-before-pull + the stacked-PR corollary
   rule; staged diff → apply last → stopgap commit).
4. README row; lint green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `(rc=0; for t in MergeNow ApprovedAge; do out=$(go test ./tools/desk/cmd/deskboard/... -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` | exit 0, prints nothing — both named test groups EXIST and pass; covers the five Task-1 cases. Exit status is captured (`tr=$?`) and asserted BEFORE the `--- PASS` check, so a FAILING test in the group also goes red — the previous pipeline form discarded `go test`'s status and passed on a red suite |
| 2 | `go test ./tools/desk/... -count=1 && go vet ./tools/desk/...` | exit 0 (no existing classifier case regressed) |
| 3 | `grep -c "MERGE-NOW" ~/.claude/skills/pr-review-desk/SKILL.md` | ≥1 (after the staged diff is applied) |
| 4 | `grep -ci "drain-before-pull\|before dispatching any new review" ~/.claude/skills/pr-review-desk/SKILL.md` | ≥1 |
| 5 | `grep -ci "collapse stacks" ~/.claude/skills/pr-review-desk/SKILL.md` | ≥1 |
| 6 | `git -C ~/.claude log --oneline -1 -- skills/pr-review-desk/SKILL.md` | one commit dated the implementation day |
| 7 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

Verified 2026-07-25 glm-5.2-verifier against merged main `75c8941c` (non-implementer; fresh `/private/tmp` worktree off `origin/main`).

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskboard/... -count=1 -run 'MergeNow\|ApprovedAge' -v` | 0 | 5 tests PASS — TestMergeNow_ApprovedAge, TestMergeNow_DecayBanner, TestMergeNow_ApprovedThenPush_ReReview, TestMergeNow_RankedFirst, TestMergeNow_ThresholdFlagParse (the five Task-1 cases) |
| 2 | `go test ./tools/desk/... -count=1 && go vet ./tools/desk/...` | 0 / 0 | 17 packages ok; vet clean (no classifier regression) |
| 3 | `grep -c "MERGE-NOW" ~/.claude/skills/pr-review-desk/SKILL.md` | 0 | count = 9 (≥1) |
| 4 | `grep -ci "drain-before-pull\|before dispatching any new review" ~/.claude/skills/pr-review-desk/SKILL.md` | 0 | count = 3 (≥1) |
| 5 | `grep -ci "collapse stacks" ~/.claude/skills/pr-review-desk/SKILL.md` | 0 | count = 1 (≥1) |
| 6 | `git -C ~/.claude log --oneline -1 -- skills/pr-review-desk/SKILL.md` | 0 | `882ca24 mm/11 …` (2026-07-17) is now `-1` head; implementation-day commit `c8ebe96 mm/20 …` (2026-07-16) exists — row intent (SKILL.md change committed in the impl window) satisfied by `c8ebe96` |
| 7 | `go run ./tools/statusgen --root . --lint` | 0 | lint passes (NOTICEs are pre-existing — findings/verification-debt — none block) |

**Risk-bearing value.** `mergeNowThreshold = 20 * time.Minute` @ `../assay-toolkit/tools/desk/cmd/deskboard/main.go:73` was returned `NAMED, NOT DERIVED`, but is a **reversible alarm-threshold knob** — flag-overridable (`--merge-now-threshold`), banner-only, on a read-only ranking surface (no merge act, no on-chain/canton write). Per the risk-value procedure step 3, reversible alarm thresholds rank last and need no derivation (out of scope by design); not risk-bearing, so no derivation required and no question-issue filed. The MERGE-NOW gate `approvedAtHead && ciGreen && !mergeConflict` (`board.go:388`) and `mergeConflict = DIRTY||BLOCKED` (`board.go:663`) are DERIVED from the read-only-surface contract, covered by `TestMergeNow_ApprovedThenPush_ReReview` and the dirty-merge fixture.

## Review
Gate: model. Reviewer confirms: (a) MERGE-NOW is a ranked read-only surface — no merge
act, no Next-up score input; (b) the approved-at-head check keys on the CURRENT head sha
(a post-approval push must demote to RE-REVIEW); (c) the skill diff kept the existing
dispatch rules intact.
