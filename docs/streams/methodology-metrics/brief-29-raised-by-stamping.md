---
brief: methodology-metrics/29
title: 'raised-by:<desk> stamping — the filing desks label the issues they raise, so the by-desk metric is real'
why: >-
  brief-28's "which desk raised it" cut can only group issues that carry a `raised-by:<desk>` label;
  until the desks that FILE issues actually stamp it, every issue reads as `unattributed` and the cut
  is blind. This wires the convention into the four desks that file issues, so the by-desk metric
  stops being a placeholder and starts answering "is one loop generating all the churn?".
wave: 4
depends: ["methodology-metrics/28"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
sources: ["human:<name> 2026-07-16: 'measure which desk are they being raised by' — this makes the by-desk cut of mm/28 real", "methodology-metrics/28 (defines the `raised-by:<desk>` label + reads it — this brief makes the desks emit it)", ".claude/skills/{verify-desk,issue-loop,pr-review-desk,batch-fanout}/SKILL.md (the four consumers — each files issues; each stamps the label)", "CLAUDE.md bug-tracking rule + insight-routing (the file-issue sites being amended)", "freshness-checked 2026-07-16 @ 8a916655 (verify-desk skill files bug/FINDINGS/needs-decision issues (autonomy section, PR #594); issue-loop skill files bug + needs-decision + scoped→issue; pr-review-desk files out-of-scope issues; batch-fanout files insight issues — none stamp a source label today)"]
gate-why: not human-gated — adds one label to existing issue-filing steps in operating-manual skills; no new authority, no risk surface. All four skills are in-repo (.claude/skills/, not out-of-repo ~/.claude — no #221 serialization), and reviewable.
---

# Brief 29 — raised-by:<desk> stamping

## Context
files: `../oit/.claude/skills/verify-desk/SKILL.md`, `../oit/.claude/skills/intake-desk/SKILL.md`,
`../oit/.claude/skills/pr-review-desk/SKILL.md`, `../oit/.claude/skills/batch-fanout/SKILL.md` — each at its
file-an-issue step(s).

facts:
- **The label**: `raised-by:<desk>` — one of `raised-by:verify-desk`, `raised-by:issue-loop`,
  `raised-by:pr-review-desk`, `raised-by:batch-fanout`. mm/28 reads it for the by-desk cut; no label
  → `unattributed`.
- **Where each desk files** (add `--label "raised-by:<self>"` to the existing `gh issue create` at
  each site; keep the existing `bug`/`question`/etc. label too):
  - **verify-desk** — the VERIFY-FAIL `bug` issue + any FINDINGS/needs-decision it files (autonomy
    section) → `raised-by:verify-desk`.
  - **issue-loop** — issues it files from intake triage (`scoped → issue #NN`) + bug/needs-decision
    → `raised-by:issue-loop`.
  - **pr-review-desk** — the out-of-scope-discovery issue it files at review time →
    `raised-by:pr-review-desk`.
  - **batch-fanout** — the insight-routing issue it files → `raised-by:batch-fanout`.
- **Label existence**: `raised-by:*` are not GitHub defaults; the desk creates the label on first use
  (`gh label create "raised-by:<desk>" --force` is idempotent) or the metric tolerates a missing
  label (the create can 422 if it exists — ignore). State this in each skill's step so a first run
  doesn't fail.
- Retroactive: existing unlabeled issues stay `unattributed` — that's fine; the cut fills forward as
  desks file new issues. No back-fill required.

## Ground rules
- NEVER git push / trigger workflows beyond the standing branch+draft-PR flow.
- Stop at `implemented`.
- NEEDS_CONTEXT over guessing.

## Task
Add the `raised-by:<self>` label (idempotent create + apply) to each file-an-issue step in the four
skills above. One sentence per site; do not restructure the skills.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'raised-by:verify-desk' .claude/skills/verify-desk/SKILL.md` | ≥1 |
| 2 | `grep -c 'raised-by:issue-loop' .claude/skills/intake-desk/SKILL.md` | ≥1 |
| 3 | `grep -c 'raised-by:pr-review-desk' .claude/skills/pr-review-desk/SKILL.md` | ≥1 |
| 4 | `grep -c 'raised-by:batch-fanout' .claude/skills/batch-fanout/SKILL.md` | ≥1 |
| 5 | flow: the label string matches what mm/28 reads — `grep -rhoE 'raised-by:[a-z-]+' .claude/skills/ \| sort -u \| grep -c .` | ≥4 (the four desks) |
| 6 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
