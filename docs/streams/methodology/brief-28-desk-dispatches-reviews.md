---
brief: methodology/28
title: The coordinator desk DISPATCHES reviews as issues — never runs code/security review inline
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-10: reviews run inline in the desk keep triggering things — hand off to another session as an issue", "memory: model-tier-downgrade-hazard", "memory: dispatch-neutral-wording", "the-desk skill role-split (Review = pr-review-desk own window)", "freshness-checked 2026-07-10 @ 88c14869"]
why: >-
  Running /code-review and especially /security-review inside the coordinator (the-desk)
  session repeatedly trips the dual-use classifier and the silent tier-downgrade hazard, and
  fragments the coordination work with review churn. The desk's job is to ARBITRATE and
  DISPATCH, not to be the reviewer; reviews belong in a separate session that can carry the
  security framing without degrading the coordinator.
---

# Brief 28 — The coordinator desk dispatches reviews as issues, never runs them inline

## Context
files: `~/.claude/skills/the-desk/SKILL.md` (out-of-repo — apply the edit live AND paste the
diff into the PR for the record; the in-repo record is this brief). Reference the existing
role-split table in that skill (Review = `pr-review-desk` in its own window) and the
review-gate tooling section (lines ~109-125).
facts:
- The-desk already SAYS review is a separate window (`pr-review-desk`), but its operating
  rules still let the coordinator run `/code-review` / `/review <PR#>` / `/security-review`
  inline (review-gate tooling section). That inline path is the problem human:<name> names.
- Two concrete harms (both in memory): (1) `/security-review` inline trips the dual-use
  classifier / can silently downgrade the coordinator's tier mid-task (model-tier-downgrade-
  hazard) — the desk then does its synthesis/arbitration work degraded; (2) it fragments the
  coordination window with review churn.
- The neutral-wording rule (dispatch-neutral-wording) already exists for HOW to phrase a
  dispatched review; this brief governs WHERE the review runs (a different session), which is
  the stronger protection — the coordinator never holds the security frame at all.
- The handoff unit is a GITHUB ISSUE (consistent with the inbound issue-loop direction,
  INTAKE [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md)): the desk files a `review-request` issue naming the PR/diff + review type
  (code / security / both) + the risk basis; a separate review session picks it up, runs the
  actual review skill, posts the verdict to the PR, and closes the issue. The desk never runs
  the review skill in its own session.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Amend `~/.claude/skills/the-desk/SKILL.md`: add a hard operating rule — **the coordinator
   desk never runs `/code-review`, `/review`, or `/security-review` in its own session.** When
   a review is needed (working diff pre-PR, an open PR, or a retroactive review of merged code
   like the daml-01 case), the desk FILES a `review-request` issue and stops; a review session
   (pr-review-desk window, or a fresh dispatched session for one-off/retroactive reviews) runs
   the skill and posts the verdict.
2. Define the `review-request` issue shape (document it in the skill): title
   `review-request: <target> — <type>`; body carries the PR number or the exact diff locator
   (branch / merged-commit range for retroactive), the review type(s), and the risk basis
   (why security is or isn't required — ties to the risk-classed-review gate, issue #216). A
   `review-request` label so it is distinguishable from work issues (and, per [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md), excluded
   from the issue-loop work-scanner — it is a dispatch token, not a work item).
3. Update the review-gate tooling section wording so "the desk runs /code-review" becomes "the
   desk dispatches the review to a review session via a review-request issue"; keep the
   built-in-skills-not-homegrown-prompts rule and the post-to-PR / redaction rules intact
   (they now bind the review SESSION, not the desk).
4. Cross-reference: this is the WHERE half; dispatch-neutral-wording is the HOW half; #216 is
   the WHAT-triggers-security-review half. Note all three in the skill so they compose.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "never runs" ~/.claude/skills/the-desk/SKILL.md` | ≥1 (the hard rule is present) |
| 2 | `grep -c "review-request" ~/.claude/skills/the-desk/SKILL.md` | ≥1 (the issue shape is documented) |
| 3 | the PR body contains the full before/after diff of the skill edit | present (out-of-repo file, recorded in-PR) |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS — glm-5.2-verifier, merged main `ae0086f0`, 2026-07-20

Isolated worktree off `origin/main`. `~/.claude/skills/the-desk/SKILL.md` is now a THIN POINTER
(sanctioned brief-22 relocation, authored *after* this brief) — the canonical skill body moved
in-repo to `../oit/.claude/skills/the-desk/SKILL.md`, which is where the methodology/28 rule + issue-shape
actually landed. Both paths reported per row.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -c "never runs" ~/.claude/skills/the-desk/SKILL.md` | 1 | `0` (thin-pointer stub) — **canonical** `grep -c "never runs" .claude/skills/the-desk/SKILL.md` → exit 0, `4`; the hard rule is present (in-repo SKILL.md: "HARD RULE — the coordinator desk never runs review skills inline (methodology/28)") |
| 2 | `grep -c "review-request" ~/.claude/skills/the-desk/SKILL.md` | 1 | `0` (stub) — **canonical** `grep -c "review-request" .claude/skills/the-desk/SKILL.md` → exit 0, `8`; the `review-request` dispatch-issue shape (title/body/label) is fully documented |
| 3 | PR before/after diff of the out-of-repo skill edit | — | informational; the substantive edit is now durably in-repo at `../oit/.claude/skills/the-desk/SKILL.md` tagged `(methodology/28)`, so the record is captured in-repo regardless of the PR-body snapshot |
| 4 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only, no lint failures) |

**VERIFY: PASS** — substantive goal MET on the canonical in-repo skill: the coordinator-desk hard rule
("never runs `/code-review`/`/review`/`/security-review` inline") plus the fully-specified `review-request`
dispatch-issue shape are present, live, and load-bearing. Caveat (not a failure): Verify rows 1–2 as
literally written point at `~/.claude/skills/the-desk/SKILL.md`, which now returns `0`/exit 1 because that
file was reduced to a thin pointer by the *sanctioned* brief-22 relocation authored after this brief — a
**stale/mis-specified Verify path, not an implementation defect**. Re-point rows 1–2 to
`../oit/.claude/skills/the-desk/SKILL.md` on any future edit of the brief's Verify table.

## Review
Gate: model. Reviewer confirms the inline-review path is actually CLOSED (not merely
discouraged) and the review-request handoff is fully specified — and, fittingly, this brief's
own review is dispatched, not run in the coordinator session.
