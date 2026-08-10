---
brief: methodology/14
title: CLAUDE.md + skill-description diet, with a placement rule
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (process desk)
sources: ["process-desk measurement 2026-07-08: CLAUDE.md 342 lines/~3.8k words after 11 same-day commits; canton-auth-doctor description 239 words (~4x cap)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "spec §13 (views are generated, sources are edited — prose rules have no checker)"]
---

# Brief 14 — CLAUDE.md + skill-description diet

## Context
files: CLAUDE.md; .claude/skills/*/SKILL.md frontmatter descriptions (esp. canton-auth-doctor); possibly docs/streams/methodology/README.md conventions (absorbing moved content)
facts:
- Cost model: CLAUDE.md loads into EVERY session AND every subagent (today: ~30 dispatches × ~5k tokens). Skill DESCRIPTIONS load every session; skill BODIES only on invoke (bodies are the cheap place for detail).
- Trajectory problem: 11 CLAUDE.md commits on 2026-07-08 alone (+~70 lines), mostly methodology rules added by the process desk.
- Redundancy: streams rules now live in 4 places (CLAUDE.md, design spec, stream-README conventions, author-brief wrapper) with no checker over prose divergence.
- Constraint learned the hard way: rules must be RESIDENT to be followed — pointers get skipped by wandering agents. Diet = compress wording + dedupe + relocate detail, NEVER rules→pointers-only.
- Tooling: use the claude-md-improver plugin skill for the audit pass; description caps per writing-skills guidance (<500 chars ideal).
- methodology/07 (toolkit extraction) will MOVE the portable methodology conventions out — coordinate: this brief compresses and dedupes; 07 relocates. Don't do 07's job here.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the PLACEMENT RULE first (a short section in CLAUDE.md itself): what earns residence where — CLAUDE.md = rules every session needs, stated once, compressed; stream-README conventions = rules only stream-workers need; skill wrapper = rules only that skill's users need; docs/spec = rationale, incidents, narrative. New-learning default is NOT CLAUDE.md.
2. Diet pass on CLAUDE.md via claude-md-improver: dedupe the 4-way overlaps to single residence + minimal cross-reference; compress verbose rules (keep meaning, cut narrative); move incident stories to docs (keep the rule + one incident pointer). Target: ≥25% word reduction with zero rule loss (list every removed/moved item in the report).
3. Compress skill descriptions to trigger-only within guidance caps — canton-auth-doctor especially (keep trigger coverage, cut process summary; per writing-skills, descriptions must never summarize workflow).
4. Verify no rule was lost: the report maps every deleted line to its new home or its compressed replacement.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `wc -w CLAUDE.md` | < 3000 (relaxed from ≤2850 by human:<name>, 2026-07-10; still well below the 2026-07-08 baseline of ~3800) |
| 2 | `python3 -c "import re;s=open('.claude/skills/canton-auth-doctor/SKILL.md').read();print(len(re.search(r'^---\n(.*?)\n---',s,re.S).group(1).split()))"` | ≤ 120 |
| 3 | `grep -c "Placement rule\|placement rule" CLAUDE.md` | ≥1 |
| 4 | `statusgen --root . --check` | exit 0 (link checker validates all surviving CLAUDE.md paths) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
Verifier run (independent, non-implementer — opus-verifier, merged main; Verify row 1 threshold relaxed to <3000 by human:<name> 2026-07-10):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `wc -w CLAUDE.md` | 0 | 2907 words — **< 3000** ✓ (was flagged FAIL vs the old ≤2850 cap; human:<name> relaxed the cap to <3000 on 2026-07-10, resolving #280) | 2026-07-10 | opus-verifier |
| 2 | canton-auth-doctor SKILL.md frontmatter word count | 0 | 119 ≤ 120 ✓ | 2026-07-10 | opus-verifier |
| 3 | `grep -c "Placement rule\|placement rule" CLAUDE.md` | 0 | 1 (≥1) ✓ | 2026-07-10 | opus-verifier |
| 4 | `go run ./tools/statusgen --root . --check` | 0 | exit 0 (surviving CLAUDE.md paths validate) ✓ | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — all four rows pass under the amended row-1 threshold (<3000). CLAUDE.md at 2907 words is within cap; frontmatter diet, placement rule, and link-check all hold.

## Review
Gate: model (from frontmatter). Reviewer must specifically check the "no rule lost"
mapping in the report — a diet that silently drops a rule is worse than no diet.
Reviewer records verdict + date in the stream README table.
