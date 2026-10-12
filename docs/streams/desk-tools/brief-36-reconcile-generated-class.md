---
brief: assay:assay:desk-tools:36
title: "The scheduled board reconcile's PR class becomes generated output — auto-approve lane (default) or direct-CI single-writer, per the driver's ruling on #2688"
why: >-
  The scheduled board reconcile (statusgen reconcile --apply, landing as
  `chore(board): reconcile` PRs by app/assay-board-writer) writes DERIVED state: Status-cell
  flips each witnessed by a merged PR carrying the brief's `Brief:` trailer, plus regen of
  the generated tables. It is the same artifact class as STATUS.md regen — which lands by
  CI single-writer with no PR at all — yet it currently consumes a dispatched reviewer slot
  and an authorization-needed human gate on every cron tick, reviewing content a reviewer
  cannot improve: the flips are derived, and the tool already self-holds any row that would
  lint. The driver ruled on #2688 (2026-10-11): the class is generated output; rule it into
  the auto-approve lane, or convert it to direct-CI single-writer — mechanism choice
  delegated to this brief. The residual risks the old review nominally covered (stale
  source reads, a wrong witness) are guarded better by the tooling than by a model pass:
  the reconcile's wouldApply lint, the derived-source discipline, and branch protection.
  This is a control-scope change (review-coverage narrowing) and exists ONLY because the
  human ruling on #2688 authorizes it.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [2688]
schema: brief-v2
authored: 2026-10-11 by a house authoring session at the driver's direction (ruling recorded on assay#2688)
sources:
  - "assay#2688 — the driver's ruling this brief implements: generated class, auto-approve lane or direct-CI, mechanism delegated. This brief cannot weaken the ruling's guards (self-hold, witness discipline, branch protection)."
  - ".github/workflows/ — the reconcile PR's own workflow (the wouldApply/reconcile machinery, #2440/#2441 era) is where the class's landing path lives today; the auto-lane's membership config and R-8 enactment line (docs/streams/issue-flow/rulings.md) are where a lane opt-in lands."
  - "CLAUDE.md-level auto-lane doctrine: the reviewer App merges driver-opted-in areas (never stream briefs) only via deskautolane merge, inert until R-8's Sign-off line holds an Enact: R-8 acceptance — never by hand, never on a session's own reading of lane membership."
  - "freshness-checked 2026-10-11 @ a666ceb1e (origin/main) — brief-36 free; #2651 (the class's first instance) still open under standing rules; the reconcile PR class has zero prior merges."
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument: the mechanism choice, the lane/config
  edit, the R-8 enactment line, and the runsheet leg must compose with the auto-lane's
  own inertness rules — and the Verify rows must prove lane membership the same way
  deskautolane itself reads it, not by prose.
consumers:
  - "the auto-approve lane's membership config (wherever deskautolane reads opted-in areas — the worker locates it and cites the exact file here at implementation): fixed-here (the board-reconcile PR class: match on title `chore(board): reconcile` + author app/assay-board-writer, scoped to nothing broader)"
  - "docs/streams/issue-flow/rulings.md: fixed-here (the R-8 sign-off/enactment line for this area, citing #2688)"
  - "the reconcile workflow / ruleset: fixed-here ONLY if mechanism B (direct-CI) is chosen — a human runsheet carries any .github/workflows/ or ruleset change (the worker App lacks the workflows scope); under mechanism A no workflow file changes"
  - "docs/runsheets/reconcile-lane-hunk.md (planned): fixed-here under mechanism B (the exact workflow/ruleset diff + one-line apply command)"
version: 1
id: 7e2c4a18-d50b-4f61-9a3e-64c8f2d1b7a3
---

# Brief 36 — The reconcile PR class becomes generated output

## Context

The driver ruled on #2688: the scheduled board reconcile's PRs are generated output; they
belong in the auto-approve lane or on a direct-CI single-writer path. Open #2651 — the
class's first instance — merges under the standing rules regardless of this brief; the
ruling governs the class from this brief's landing forward.

**Mechanism (the ruled fork, default A):**

- **A (default) — auto-approve lane.** The reconcile PR class joins deskautolane's
  driver-opted-in areas: membership matched on title + author (nothing broader), the R-8
  enactment line added citing #2688. The PR form stays (audit trail), the reviewer slot and
  the authorization-needed human gate stop consuming on this class. Choose A unless the
  lane machinery makes the membership match awkward — it is the smaller change and touches
  no workflow file.
- **B — direct-CI single-writer.** The reconcile lands on main the way STATUS.md regen
  does: the workflow pushes, no PR. Requires the ruleset bypass for the board-writer
  identity and a main-guard carve-out; every workflow/ruleset edit rides a human runsheet
  (the worker App cannot push them). Choose B only if A is structurally awkward — record
  the reason in the PR body.

**Boundaries this brief holds:**

1. **Nothing weakens the class's own guards** — the reconcile's wouldApply self-hold, the
   merged-PR witness discipline, and branch protection all stay exactly as they are.
2. **The lane membership is narrow** — title `chore(board): reconcile` AND author
   `app/assay-board-writer`, never a prefix or an owner match that could catch sibling
   PRs. If the lane config cannot express this precisely, that is a reason to pick B, not
   to widen the match.
3. **Never stream briefs** — the auto-lane's standing exclusion; this brief changes the
   reconcile class only.
4. **#2651 itself is out of scope** — it merges under standing rules; the first
   post-landing reconcile tick is the ruling's caught-by.

## Task

**Task 1 — locate and read the lane machinery.** Find where deskautolane reads its
opted-in areas and how the R-8 enactment line binds (docs/streams/issue-flow/rulings.md);
cite both exact paths in the PR body. Decide A vs B per the Context fork and say which,
with the reason, in the PR body's first line.

**Task 2 (A) — the lane membership + enactment.** Add the reconcile class to the lane
config (narrow match, boundary 2) and the R-8 sign-off line citing #2688.

**Task 2 (B, only if A is awkward) — the runsheet.** Author
docs/runsheets/reconcile-lane-hunk.md (planned) with the exact workflow/ruleset diff and
one-line apply command; the worker touches zero workflow files.

**Task 3 — changelog fragment.** changelog/desk-tools-36.md (planned), one bullet.

files:
- the lane config + rulings.md (Task 2A; exact paths cited at implementation)
- docs/runsheets/reconcile-lane-hunk.md (planned) — Task 2B only
- changelog/desk-tools-36.md (planned) — Task 3

## Verify (executable — no prose-only DoD items)

Row 1 is discriminating under either mechanism; rows 2–3 apply per mechanism chosen; row 4
passes today and must still pass. Run from the repo root.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -rq '2688' docs/streams/issue-flow/rulings.md && echo ok` | prints `ok`: the R-8 enactment line cites the ruling. FAILS today: #2688 is referenced nowhere in rulings.md. |
| 2 | check (mechanism A) | `deskautolane --help > /dev/null 2>&1 && grep -rqF 'chore(board): reconcile' <the lane-config path cited in the PR body> && echo ok` | prints `ok`: the lane config carries the narrow title match. FAILS today: no membership entry exists. If mechanism B was chosen, this row is not applicable — say so in Evidence with the B reason. |
| 3 | check +flow | READ FROM THE FORGE after the first scheduled reconcile tick following this brief's merge: the reconcile PR either does not exist (B: direct-CI landing on main) or exists merged by the reviewer App via deskautolane (A) with no dispatched reviewer verdict and no authorization-needed label. | The ruling's caught-by: the class stops consuming a reviewer slot and a human gate. Could-not-check at authoring: no post-landing tick has run. |
| 4 | check +neighbour | `cd statusgen && go build -o "${TMPDIR:-/tmp}/dt36-sg" . && "${TMPDIR:-/tmp}/dt36-sg" --root .. --lint 2>&1 \| grep -q 'LINT: PASS' && echo ok` | prints `ok`: the tree lints clean. Passes today and must still pass. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer answers, in the verdict: (1) is the lane
membership exactly the ruled class (title + author), with no widening? (2) does the R-8
enactment line cite #2688 and leave the auto-lane inert until its own sign-off discipline
is satisfied? (3) are the class's own guards (self-hold, witness discipline, branch
protection) demonstrably untouched? (4) if mechanism B was chosen, does the runsheet carry
the exact diff, and does the worker's diff touch zero workflow files? (5) is #2651
untouched by this change?
