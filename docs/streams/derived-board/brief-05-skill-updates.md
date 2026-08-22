---
brief: derived-board/05
title: desk skills — reference the brief, never flip the cell (author-brief, worker-desk, pr-review-desk, verify-desk; public copies)
why: >-
  The skills currently tell workers to "stop at implemented" and, since 2026-08-22, to
  flip the README cell themselves. Under derived cells that instruction becomes wrong:
  a hand edit to the generated table is a lint PROBLEM. The skills must say the new thing
  — put the trailer in the PR, the board follows — or the first worker after cutover
  bounces on its own PR.
wave: 1
depends: ["derived-board/01", "derived-board/02"]
unblocks: ["derived-board/07"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §2, §3"
  - "plugins/assay/skills/worker-desk/SKILL.md §Board claim + the Board row bullet (PR #69, 2026-08-22) — the text this supersedes"
  - "freshness-checked 2026-08-22 @ f78ea24 — PR #69 open; the private twin lives in the private toolkit"
consumers:
  - "private toolkit .claude/skills/{author-brief,worker-desk,pr-review-desk,verify-desk}/SKILL.md: follow-up derived-board/07 (overlay re-stage with the rollout)"
---

# Brief 05 — skill updates

## Context
files:
- `plugins/assay/skills/author-brief/SKILL.md` — template block: `schema: brief-v2`, the
  reserved keys, the "no status: key" note; README structure: the generated-markers table,
  `board: generated` in stream frontmatter; Working-method step 6 rewritten (status is
  derived; author the `why:` and the edges, not the cell).
- `plugins/assay/skills/worker-desk/SKILL.md` — §Board claim and the Board row bullet
  replaced: the PR body carries `Brief: <stream>/<NN>` (deskpr refuses otherwise); the
  worker NEVER edits the README table; `in-progress` appears when the draft PR opens,
  `implemented` when it merges.
- `plugins/assay/skills/pr-review-desk/SKILL.md` — bounce rule: a PR whose diff touches
  a generated table region, or whose body lacks the trailer, is bounced with the one-line
  reason; no reviewer edits the board.
- `plugins/assay/skills/verify-desk/SKILL.md` — `verified`/`done` unchanged in mechanism
  (witness + Evidence row); remove any instruction to edit the README cell by hand;
  the verifier's deliverable is the witness, the board follows.

facts:
- Public-tree content must be self-contained — no private issue numbers or slugs in the
  skill text.
- The private twins are consumers of these files (overlay re-stage in 07); this brief
  edits the public copies only.
- Keep the phrase "stop at `implemented`" where it means "never set verified/done"; it is
  no longer an instruction to edit anything.

## Ground rules
- NEVER git push / trigger workflows. Commit on the feature branch only.
- Stop at `implemented`.
- Do not re-describe the derivation in the skills; link `docs/brief-rules.md` rule 30.

## Task
1. Edit the four skills as above; keep each diff minimal and quotable.
2. Add one sentence to each skill's top-of-file contract line stating the board is
   derived (so a reader of the first screen knows).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'Brief: <stream>/<NN>' plugins/assay/skills/worker-desk/SKILL.md` | ≥ 2 |
| 2 | `! grep -n -E -e 'flips? the (brief.s )?row' -e 'flips? its (brief.s )?row' -e 'Board row — part of the deliverable' plugins/assay/skills/worker-desk/SKILL.md` | exit 0 (the hand-flip instruction is gone) |
| 3 | `grep -c 'schema: brief-v2' plugins/assay/skills/author-brief/SKILL.md` | `1` |
| 4 | `grep -c -E -e 'generated table' -e 'statusgen:briefs:begin' plugins/assay/skills/pr-review-desk/SKILL.md` | ≥ 1 |
| 5 | `! grep -n -i -E -e 'edit the (stream )?README.*verified' -e 'edit the (stream )?README.*done' plugins/assay/skills/verify-desk/SKILL.md` | exit 0 |
| 6 | `! grep -rn -E '[a-z-]+#[0-9]{3,}' plugins/assay/skills/author-brief/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md` | exit 0 (self-contained: no private issue refs) |
| 7 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
