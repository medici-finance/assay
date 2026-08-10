---
brief: methodology/32
title: Worker PR watch-loop — monitor mergeable state alongside reviews; rectify CONFLICTING immediately
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [212]
schema: brief-v1
authored: 2026-07-10 by Fable authoring session (issue #212)
sources: ["issue #212", "docs/streams/desk-tools/brief-06-cutover.md (the #212 skill-wiring facts, pulled forward here)", "live pattern 2026-07-10: a worker session's Monitor on gh pr view --json state,mergeable,reviews (memory: worker-listens-on-own-pr)"]
why: >-
  Workers treat reviewer findings as their only wake-up event; mergeable state is never
  watched, so CONFLICTING PRs sit until a human notices — three incidents on 2026-07-10
  alone (#137, #155, #202). The fix is pure skill text and needs none of the desk-tools
  binaries, but it was parked behind desk-tools/06 (wave 2, gate:human, six unstarted
  deps). Pull the text half forward; the tool half (deskboard mergeState) stays in desk-tools.
---

# Brief 32 — Worker PR watch-loop: mergeable alongside reviews

## Context
files: CLAUDE.md (PR-review-loop step 5 — repo file, rides the PR)
out-of-repo files: ~/.claude/skills/batch-fanout/SKILL.md
facts:
- The proven watch-loop shape (live 2026-07-10): after opening the draft PR, the worker arms a
  Monitor (or polls every turn) on `gh pr view <n> --json state,mergeable,reviews` — ONE loop,
  three event sources: new review → work the findings; `mergeable: CONFLICTING` →
  `git fetch origin && git merge origin/main`, resolve, push, reply on the PR that a
  conflict-resolution merge landed (board correctly classifies RE-REVIEW since own files
  changed); `state: MERGED|CLOSED` → STOP immediately.
- Squash-merge awareness (verbatim intent from #212): a sibling/stacked PR landing via squash
  makes same-content conflicts LIKELY; resolution is mechanical — take main's side, re-apply
  own edits. Say this in the skill so cheap-tier workers don't flail.
- batch-fanout worker-essentials bullet to amend is the one currently reading "keep the branch
  current with main … periodically while open and always right before signalling DESK-READY";
  DESK-READY is retired terminology (#125/methodology-17) — fix it in the same rewrite ("right
  before review").
- CLAUDE.md is word-budgeted: origin/main is at ~2846 words vs the ≤2850 cap (methodology/14).
  The step-5 edit must be NET-NEUTRAL — rewrite "periodically and ALWAYS right before review"
  into wording that names the concrete trigger (watch `mergeable`; on CONFLICTING resolve+push
  immediately; always merge right before review), compensating by trimming within step 5.
- Out-of-repo protocol (rule 7 / #221): stage the batch-fanout SKILL.md edit as a diff in the
  PR body; apply to the live file only as the LAST step before `implemented`; then commit the
  applied edit in the ~/.claude stopgap git repo. Dispatch note: max ONE out-of-repo brief in
  flight across ALL streams — methodology/30 (pr-review-desk edit) and desk-tools/06 compete
  for the same slot; check for in-flight out-of-repo PRs before dispatching this one.
- desk-tools/06 amendment landed with this authoring set: 06 no longer claims #212 — it only
  re-verifies this rule survived its skill rewiring and wires deskboard `prs` mergeState as
  the preferred sensor post-cutover.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Amend `~/.claude/skills/batch-fanout/SKILL.md` worker-prompt essentials: replace the
   keep-current bullet with the full watch-loop mandate (facts #1 + #2): arm a Monitor/poll on
   `gh pr view <n> --json state,mergeable,reviews` while the PR is open; the three
   event-to-action pairs; the squash-merge note; keep-current merge always right before review
   (drop "DESK-READY"). Merge-never-rebase stays.
2. Amend CLAUDE.md PR-review-loop step 5 per facts — net-neutral wording that adds the
   CONFLICTING trigger; verify the word budget.
3. Stage the out-of-repo edit per facts (diff in PR body, apply last, commit in ~/.claude
   stopgap repo).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'CONFLICTING' ~/.claude/skills/batch-fanout/SKILL.md` | ≥1 |
| 2 | `grep -ci 'squash' ~/.claude/skills/batch-fanout/SKILL.md` | ≥1 |
| 3 | `test -f ~/.claude/skills/batch-fanout/SKILL.md && ! grep -q 'DESK-READY' ~/.claude/skills/batch-fanout/SKILL.md` | exit 0 (the retired DESK-READY token is gone from the skill). Guarded by `test -f ~/.claude/skills/batch-fanout/SKILL.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |
| 4 | `grep -q 'CONFLICTING' CLAUDE.md && wc -w < CLAUDE.md` | ≤2850 |
| 5 | `grep '^issues:' docs/streams/desk-tools/brief-06-cutover.md` | `issues: [209, 210]` (06 no longer claims #212) |
| 6 | `git -C ~/.claude log --oneline -1 -- skills/batch-fanout/SKILL.md` | one commit dated the implementation day |
| 7 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s), date, runner). -->
Verifier run (independent, non-implementer — opus-verifier, merged main `0174b912`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -c 'CONFLICTING' ~/.claude/skills/batch-fanout/SKILL.md` | 0 | 1 (≥1) | 2026-07-12 | opus-verifier |
| 2 | `grep -ci 'squash' ~/.claude/skills/batch-fanout/SKILL.md` | 0 | 1 (≥1) | 2026-07-12 | opus-verifier |
| 3 | `grep -c 'DESK-READY' ~/.claude/skills/batch-fanout/SKILL.md` | 1 | 0 (removed) | 2026-07-12 | opus-verifier |
| 4 | `grep -q 'CONFLICTING' CLAUDE.md && wc -w < CLAUDE.md` | 0 | 2844 (≤2850; CONFLICTING present) | 2026-07-12 | opus-verifier |
| 5 | `grep '^issues:' docs/streams/desk-tools/brief-06-cutover.md` | 0 | `issues: [209, 210]` | 2026-07-12 | opus-verifier |
| 6 | `git -C ~/.claude log --oneline -1 -- skills/batch-fanout/SKILL.md` | 0 | committed to the stopgap repo (latest touch `f79b361`, 2026-07-12) — the watch-loop edit is live | 2026-07-12 | opus-verifier |
| 7 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-12 | opus-verifier |

**VERIFY: PASS** — the worker PR watch-loop (mergeable-alongside-reviews, CONFLICTING/squash handling, DESK-READY removed) is in the batch-fanout skill and committed to the `~/.claude` stopgap repo; CLAUDE.md stays under the 2850 cap.

## Review
Gate: model. Reviewer confirms: (a) the CLAUDE.md edit is net-neutral vs the 2850 cap, (b) the
batch-fanout rewrite kept merge-never-rebase and worktree/one-PR rules intact, (c) desk-tools/06
no longer claims to close #212.
