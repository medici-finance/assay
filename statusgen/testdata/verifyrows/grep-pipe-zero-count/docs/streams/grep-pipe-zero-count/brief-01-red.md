---
brief: grep-pipe-zero-count/01
title: A grep piped into a counter, expected to count zero
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1862]
schema: brief-v1
authored: 2026-10-11 by fixture
sources: ["fixture: #1862 — verifyrun runs every row under pipefail, and grep exits 1 on no match"]
---

# Red — the success path exits 1

Every row here pipes a grep into a later stage and expects a count of zero. When
the property holds (nothing matches) grep exits 1 and, under `pipefail`, so does
the row; when a match is present grep exits 0 and the bare count is decided by
exit status alone. Row 5 is a non-leading `grep -v` stage, row 6 a `git grep`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -rn 'oldname' docs/ \| wc -l` | `0` |
| 2 | `ls docs/ \| grep -E 'draft' \| wc -l` | prints `0` |
| 3 | `test -d docs && grep -rn 'oldname' docs/ \| grep -v 'CHANGELOG' \| wc -l` | output is `0` |
| 4 | `egrep -rn 'old.name' docs/ \| tr -d ' ' \| wc -l` | 0 — nothing left |
| 5 | `cat docs/list.txt \| grep -v 'keep' \| sort \| wc -l` | output is `0` |
| 6 | `git grep -n 'oldname' -- docs/ \| wc -l` | **0** |

## Review
Gate: model.
