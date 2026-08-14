---
brief: moving-ref/02
title: The same comparisons pinned to something that cannot move
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [639]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the #639 fix — merge-base, an explicit SHA, or the PR's own base.sha"]
---

# Green — the negative control

A merge-base computation READS the moving ref precisely in order to pin it, so
it is the fix rather than the defect. All rows MUST stay silent.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --consumers --base $(git merge-base origin/main HEAD)` | exit 0 |
| 2 | `statusgen --root . --consumers --base c7cd7ef` | exit 0 |
| 3 | `statusgen --root . --consumers --base "$BASE_SHA"` | exit 0 |
| 4 | `git diff --stat HEAD` | no output |

## Review
Gate: model.
