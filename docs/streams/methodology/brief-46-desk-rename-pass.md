---
brief: methodology/46
title: 'Desk rename pass — batch-fanout → worker-desk + issue-loop → intake-desk (land the four-loop taxonomy in one coordinated pass)'
why: >-
  The skill-naming convention and F-intake-desk-rename settled two desk renames — batch-fanout → worker-desk (name the
  function, not the fanout mechanism) and issue-loop → intake-desk (the generic front door). Left
  half-done, the desks read inconsistently and a half-renamed state (some refs old, some new) is worse
  than either. One coordinated pass renames both and re-points every reference, so the four automation
  loops the-desk coordinates land coherently and legibly at once: intake-desk → worker-desk →
  pr-review-desk → verify-desk.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by Opus desk session (human:<name> directive)
sources:
  - "human:<name> 2026-07-17: 'yes. issue the brief' — approving a single desk-rename brief covering both renames"
  - "human:<name> 2026-07-17: batch-fanout should be the worker-desk (function, not mechanism); `the-desk` is the coordinator, not a loop"
  - "../assay-toolkit/docs/skill-naming.md (PR #87) — the naming convention: <function>-desk for the loops; batch-fanout → worker-desk is the one rename it prescribes"
  - "[F-intake-desk-rename](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-17-issue-desk-becomes-generic-intake-desk.md) — issue-loop → intake-desk (the generic front door)"
  - "freshness-checked 2026-07-17 @ origin/main: skills live at `.claude/skills/batch-fanout/` and `.claude/skills/issue-loop/`; CLAUDE.md references `issue-loop/10` (line ~101); batch-fanout is not named in CLAUDE.md"

---

# Brief 46 — Desk rename pass

## Context
files (rename + re-point — this is a SHARED-VALUE change; a skill/stream name many things reference):
- `.claude/skills/batch-fanout/` → **`.claude/skills/worker-desk/`** (dir + SKILL.md `name: worker-desk`).
- `.claude/skills/issue-loop/` → **`.claude/skills/intake-desk/`** (dir + SKILL.md `name: intake-desk`).
- **All by-name refs**: grep `batch-fanout`, `issue-loop`, `issue-desk` across `.claude/skills/**`,
  `CLAUDE.md`, `docs/streams/*/README.md`, and briefs that cite the skill PATH; re-point each. Known
  sites: `CLAUDE.md` (`issue-loop/10` ref ~L101), pr-review-desk/the-desk/verify-desk skills, the loops
  reference. (Stream-brief typed IDs like `issue-loop/NN` in prose are register IDs, NOT skill paths —
  leave the stream/brief IDs; rename the SKILL refs. If the stream directories are also renamed, that
  is a larger change — keep it OUT of this brief unless trivial, and state the decision.)

facts:
- **Rename, not re-mechanism.** No skill BEHAVIOR changes — only the dir name, the `name:` field, and
  refs. Invocation triggers stay in the descriptions (batch-fanout's "fan out the next batch",
  issue-loop's issue/intake triggers) so invocation keeps working; add the new names as aliases where
  natural.
- **The taxonomy this lands:** `the-desk` (coordinator) orchestrates four loops — **intake-desk**
  (triage inbound → work) → **worker-desk** (implement) → **pr-review-desk** (review) → **verify-desk**
  (verify). Record this loop set in the loops reference / CLAUDE.md if a canonical list exists.

## Ground rules
- NEVER push to main / trigger workflows / merge. Branch + draft PR; stop at `implemented`.
- In-repo skills (`.claude/skills/`) — no out-of-repo declaration; do NOT edit any `~/.claude` copy.
- **Sequencing (hard — check before renaming):** the skill FILES are targets of in-flight briefs —
  `methodology/40` and `methodology/42,43` edit the batch-fanout skill; `issue-loop/13` broadens the
  intake-desk skill. Land this rename **after** those skill-editing briefs implement, OR coordinate the
  path change with whichever is open. `issue-loop/13` also carries an intake rename — whichever of {this
  brief, issue-loop/13} lands first does the rename; the other skips it (resolve F-intake-desk-rename wherever the
  intake rename actually lands). Also coordinate with the CLAUDE.md diet (PR #689) if both edit CLAUDE.md.
  NEEDS_CONTEXT if an open PR is mid-edit on a target skill file.

## Task
1. Rename both skill dirs + set each `name:` field (`worker-desk`, `intake-desk`).
2. Re-point every by-name SKILL ref (grep the three old names; update refs to skills/paths; leave
   register/stream typed IDs). Keep invocation triggers working.
3. Record the four-loop taxonomy where a canonical desk/loop list lives.
4. Resolve F-intake-desk-rename (if this brief lands the intake rename).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -d .claude/skills/worker-desk && test -d .claude/skills/intake-desk && ! test -d .claude/skills/batch-fanout && ! test -d .claude/skills/issue-loop` | both renamed, old dirs gone |
| 2 | `grep -m1 '^name: worker-desk' .claude/skills/worker-desk/SKILL.md && grep -m1 '^name: intake-desk' .claude/skills/intake-desk/SKILL.md` | both `name:` fields set |
| 3 | `grep -rniE -e 'skills/batch-fanout' -e 'skills/issue-loop' .claude/ CLAUDE.md` | empty — no stale skill-path refs |
| 4 | `test -d ../oit/.claude/skills && test -f ../oit/CLAUDE.md && grep -rniE -e 'batch-fanout' -e 'issue-desk' ../oit/.claude/skills/ ../oit/CLAUDE.md > /tmp/m46r4.txt && test -s /tmp/m46r4.txt && { grep -viE -e 'worker-desk' -e 'intake-desk' -e 'alias' /tmp/m46r4.txt \|\| true; }` | exit 0, and the printed residue empty (or each remaining hit justified in Evidence — e.g. a historical note). Re-anchored 2026-08-03, same reasoning as issue-loop/13 r4: the skills this brief renames live in the **sibling** repo (`../oit/.claude/skills/`) and this repo has no `CLAUDE.md`, so the bare form scanned assay-toolkit's own one-skill directory and a nonexistent file and its "empty" Expect was satisfied by an almost-empty corpus. `test -d`/`test -f` plus `test -s` on the raw capture make a missing corpus exit **1 before any grep runs**; the residue is printed for judgement, so `\|\| true` keeps the exit status meaning "the corpus was really scanned". Rows 1–3 and 5 keep their bare paths — they assert the post-rename dir layout and are out of this sweep's scope |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
