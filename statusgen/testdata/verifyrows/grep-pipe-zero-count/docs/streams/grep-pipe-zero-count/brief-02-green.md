---
brief: grep-pipe-zero-count/02
title: The same zero-count checks written so they can decide
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1862]
schema: brief-v1
authored: 2026-10-11 by fixture
sources: ["fixture: the #1862 fix — tolerate only grep's no-match status, with a positive leg"]
---

# Green — the negative control

Every row here MUST stay silent. Rows 1-2 guard the grep so only its no-match
status is tolerated, row 3 negates the whole pipeline, row 4 expects a count of
at least one, row 5 has no grep ahead of a later stage, row 6 puts the grep last
in a pipeline of its own after a `&&`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -n "$(git ls-tree --name-only HEAD docs)" && { grep -rn 'oldname' docs/ \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` |
| 2 | `{ git grep -n 'oldname' -- docs/ \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` |
| 3 | `! grep -rn 'oldname' docs/ \| grep -v 'CHANGELOG'` | exit 0 |
| 4 | `grep -rn 'newname' docs/ \| wc -l` | ≥ 1 |
| 5 | `cat docs/list.txt \| sort \| wc -l` | 0 |
| 6 | `test -d docs && grep -rqn 'newname' docs/` | exit 0 |

## Review
Gate: model.
