---
brief: issue-loop/12
title: Issue desk self-dispatch — the issue-loop desk fans out its own issue-placeholders (claims-locked)
why: >-
  As shipped in brief-11 the issue-loop desk was a *feeder*: it scanned issues onto Next-up and
  handed dispatch to batch-fanout. Run the issue desk alone and nothing gets fixed in parallel —
  issues pile up as placeholders until a second desk runs. human:<name>'s model is one window that fans out
  its own work, so the smart issue desk should triage AND dispatch its fixes (SDD: smart desk → cheap
  workers, behind the review gate). This gives it that authority, with a shared claims lock so it and
  batch-fanout never double-dispatch the same placeholder.
wave: 5
depends: ["issue-loop/11"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
sources: ["human:<name> 2026-07-16: chose fan-out option B (self-dispatch) over A (feeder) after the A/B fork was laid out", "human:<name> 2026-07-16: 'how does the worker see it?' → the worker's whole view is the GitHub issue (`gh issue view`, body = spec) + its own worktree; encoded as the dispatch template", "issue-loop/11 (the desk skill this amends — merged PR #576)", ".claude/skills/batch-fanout/SKILL.md (the CONSUMER — its claim key `<stream>--<NN>` and its Next-up dispatch, both edited here)", "issue-loop/01 (placeholder brief ID form `issue-loop/issue-<NN>` — the claim key derives from it)", "issue-loop/03 (await/unblock — the worker's ask-back channel on the issue)", "freshness-checked 2026-07-16 @ 958dcc50 (issue-loop skill on main says 'DISPATCH is not this desk's job'; batch-fanout dispatches the whole Next-up batch incl. issue-placeholders; placeholder ID = issue-loop/issue-<NN>; batch-fanout claim key = <stream>--<NN>.claim)"]
gate-why: not human-gated — a skill/operating-doc change. The new authority (dispatch cheap-tier workers) produces only draft PRs, which stay behind the existing review gate + human merge; no new irreversible/regulatory/customer/sensitive-data surface.
---

# Brief 12 — Issue desk self-dispatch

## Context
files: `../oit/.claude/skills/intake-desk/SKILL.md` (self-dispatch step + claim + worker template + role
framing), `../oit/.claude/skills/batch-fanout/SKILL.md` (skip issue-placeholders)
out-of-repo files: none — both skills are in-repo now (issue-loop via #576, batch-fanout via #570).

consumers (author-brief rule 6 — the shared value changed is **who dispatches issue-placeholders**;
enumerate who relies on the old "batch-fanout dispatches everything on Next-up" assumption):
- **batch-fanout** — must now EXCLUDE `issue-loop/issue-*` rows from its Next-up fan-out (they're the
  issue desk's). Fixed in this brief (a skip rule in its "Scope the batch" step). An unlisted
  batch-fanout that kept dispatching them is the double-dispatch this brief must prevent.
- **The claims lock** is the shared value's enforcement: both desks key on `<stream>--<NN>.claim`; the
  issue desk uses `issue-loop--issue-<NN>.claim`, the exact key batch-fanout would use if its skip
  were missed — so the noclobber lock is the belt-and-suspenders behind the skip rule.

facts:
- **The worker's entire view is the GitHub issue** (stream convention "the issue body IS the spec") +
  its own worktree. The dispatch prompt hands it `<repo>#<NN>` and "run `gh issue view <NN>` — its
  body is your spec"; it never reads the placeholder or Next-up. The issue is also its ask-back
  channel (brief-03 await/unblock: park a question on the issue with the `<!-- desk-automation -->`
  marker + `blocked: awaiting-issue-response`, then STOP).
- **Only worker-legible issues get a worker.** The routing test is the dispatch gate; thin/ambiguous
  issues get `question`-labelled or scoped, never a worker.
- **Claim key** = `issue-loop--issue-<NN>.claim` (from placeholder brief ID `issue-loop/issue-<NN>`,
  batch-fanout's `<stream>--<NN>` convention). Same key both desks would use → one shared lock.

## Ground rules
- NEVER git push / trigger workflows beyond the standing branch+draft-PR authorization.
- Stop at `implemented` — do not set verified/done.
- NEEDS_CONTEXT over guessing.

## Task
1. issue-loop skill: flip the issue-lane DISPATCH step from "not this desk's job" to a claims-locked
   fan-out (claim under the shared key → dispatch one worker per claimed issue → the worker-view
   template above → routing-test gate); update the role framing (batch-fanout owns non-issue Next-up;
   this desk fans out its own issue-placeholders).
2. batch-fanout skill: add the skip rule — exclude `issue-loop/issue-*` rows from the Next-up batch.

## Verify (executable — flow-level, not just site-local)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'issue-loop--issue-<NN>.claim' .claude/skills/intake-desk/SKILL.md` | ≥1 — issue desk claims under the shared key |
| 2 | `grep -icE 'exclude .*issue-loop/issue|skip .*issue-loop/issue' .claude/skills/batch-fanout/SKILL.md` | ≥1 — batch-fanout skips issue-placeholders |
| 3 | flow: the claim KEY matches between the two skills — `grep -oE 'issue-loop--issue-<NN>' .claude/skills/intake-desk/SKILL.md .claude/skills/batch-fanout/SKILL.md \| sort -u \| wc -l` | ≥1 shared key string present in both concerns (one shared lock, no double-dispatch) |
| 4 | `! grep -q "DISPATCH is not this desk's job" .claude/skills/intake-desk/SKILL.md` | exit 0 — the old feeder rule is gone |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- filled by a non-implementer at verify time -->

Non-implementer verifier run (glm-5.2-verifier, merged main `bfba03ca`, 2026-07-17). **VERIFY:
PASS — all 5 rows.**

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `grep -c 'issue-loop--issue-<NN>.claim' .claude/skills/intake-desk/SKILL.md` | 0 | `2` (≥1 — issue desk claims under the shared key) |
| 2 | `grep -icE 'exclude .*issue-loop/issue\|skip .*issue-loop/issue' .claude/skills/batch-fanout/SKILL.md` | 0 | `1` (≥1 — batch-fanout skips issue-placeholders) |
| 3 | `grep -oE 'issue-loop--issue-<NN>' .claude/skills/{issue-loop,batch-fanout}/SKILL.md \| sort -u \| wc -l` | 0 | same key string in BOTH files (issue-loop=2, batch-fanout=1) → one shared lock, no double-dispatch (`wc -l`=2 only because `grep -oE` prefixes filename; load-bearing fact — key present in both — holds) |
| 4 | `! grep -q "DISPATCH is not this desk's job" .claude/skills/intake-desk/SKILL.md` | 0 | old feeder phrase absent (count 0); new self-dispatch DISPATCH step at L108 |
| 5 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 (advisory NOTICEs only) |

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
