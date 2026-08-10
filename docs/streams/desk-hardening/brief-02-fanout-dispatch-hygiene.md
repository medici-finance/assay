---
brief: desk-hardening/02
title: Fanout dispatch hygiene — claim-at-dispatch, branch-from-fresh-main, foreign-commit self-check
why: >
  Parallel fanout against one board and one checkout produces two structural failures with real
  cost: two workers implement the same brief (nothing claims a brief between board-read and PR),
  and workers cut branches from a sibling's HEAD instead of fresh main, dragging other PRs'
  unreviewed commits into their diffs (three of six PRs in one batch, five branch-hygiene
  failures in another session — including a fake single-parent "merge"). Both launder unreviewed
  code toward main and waste reviews. The remedy is a claim mechanism plus a one-command
  self-check every worker runs before opening a PR.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [27, 22, 72]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#27 (nothing claims a brief at dispatch — two workers built issue-loop/07)"
  - "assay-toolkit#22 (workers cut branches from each other — 3 of 6 PRs carried foreign commits)"
  - "assay-toolkit#72 (non-main bases + a fake single-parent 'merge' — 5x in one session)"
  - "cross-ref: desk-hardening/07 (ID-collision half of the branch-hygiene failures)"
exec-tier: strong
exec-tier-why: "cross-component protocol design (dispatcher + worker + board/CI) where a wrong claim/base rule strands or contaminates work."
consumers:
  - "[oit] .claude/skills/batch-fanout/SKILL.md: fixed-here (worker dispatch + worktree protocol)"
  - "[oit] .claude/skills/the-desk/SKILL.md: fixed-here (board claim write at dispatch)"
  - "[oit] deskboard.go / a CI check: follow-up (merge-base intersection flag — optional deliverable 3)"
---

# Brief 02 — Fanout dispatch hygiene

## Context
files:
- `[oit]` `../oit/.claude/skills/batch-fanout/SKILL.md` (worker dispatch + branch/worktree protocol + pre-PR self-check)
- `[oit]` `../oit/.claude/skills/the-desk/SKILL.md` (claim-at-dispatch board write)
- `[oit]` optionally `deskboard.go` or a `.github/workflows/` check (merge-base intersection)
out-of-repo files: none (SKILL.md files are in-repo to oit, cross-repo to THIS stream)
facts:
- the race (#27): between "read the board" and "the PR appears" the brief looks unclaimed; window is minutes-to-an-hour; `in-progress` lifecycle state exists but nothing sets it at dispatch
- the contamination (#22): workers create the worktree from local HEAD or a sibling branch, not fresh `origin/main`; `git worktree add <path> origin/main --detach` after a fetch is the correct primitive, not `-b <branch>` off HEAD
- the self-check (#22, one command, caught all three): `git log --oneline origin/main..HEAD` — any commit the worker did not write ⇒ stop, re-cut
- #72: "merge, never rebase; stay current" is under-enforced; a single-parent hand-edited "rebase" commit masqueraded as a merge (oit#259, 353 commits behind); assert two parents on any merge commit
- cross-repo: these are oit skill/tooling changes; carry a sibling oit draft PR + SHA per the cross-repo-pairing rule

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Claim-at-dispatch (candidate approaches — pick one, record why):**
   (a) *Board claim* — the dispatcher flips the row to `in-progress` (worker + branch) and lands
   it on main BEFORE the worker starts; the only option that also tells a *human* the brief is
   taken. (b) *Branch-existence claim* — worker pushes an empty named branch as step 1; every
   worker checks `git ls-remote --heads origin '*<stream>-<NN>*'` first; no board write but
   relies on the naming convention. (c) *Dispatcher-side dedup* — batch-fanout records what it
   handed out this run; closes the intra-batch case only, not cross-session. Recommend (a) as
   primary (per #27); (c) as a cheap intra-batch backstop.
2. **Branch-from-fresh-main protocol** in `batch-fanout`: every feature branch is cut from a
   freshly-fetched `origin/main` (worktree `--detach` primitive); to stay current use a real
   two-parent `git merge origin/main`, never a hand-edited "rebase".
3. **Pre-PR worker self-check** (mandatory, in the skill): run `git log --oneline
   origin/main..HEAD`; if it lists a commit the worker did not author, STOP and re-cut. Assert
   any merge commit has two parents.
4. **(Optional, deliverable 3) board/CI intersection check:** flag any PR whose merge-base
   commit set intersects another open PR's head commits — computable from what the board
   already fetches; would have caught all three of #22 before a reviewer was dispatched.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci 'origin/main..HEAD\|foreign commit\|did not author' <batch-fanout SKILL.md>` | exit 0; ≥ 1 (self-check documented) |
| 2 | `grep -ci 'claim\|in-progress\|ls-remote' <the-desk or batch-fanout SKILL.md>` | exit 0; ≥ 1 (claim mechanism documented) |
| 3 | `grep -ci 'two-parent\|two parents\|--detach\|merge, never rebase' <batch-fanout SKILL.md>` | exit 0; ≥ 1 |
| 4 | positive control: on a branch deliberately stacked on a sibling head, run the documented self-check | it reports the foreign commit (non-empty output) — proving the check sees the defect |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Cross-repo: verify the
sibling oit draft PR exists and is referenced from the tracking PR.
