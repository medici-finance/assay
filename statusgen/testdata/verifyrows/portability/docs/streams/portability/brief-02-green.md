---
brief: portability/02
title: The same rows written to mean one thing on both platforms
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [650]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the #650 fix — pipes instead of process substitution; -E instead of -P; sed -i ''"]
---

# Green — the negative control

The pipe form of row 1 returned the real counts (18/13/4/7/10) where the process
substitution returned 0. All rows MUST stay silent.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `sed -n '/3.2/,/3.3/p' contract.md \| grep -cE 'watchdog'` | ≥ 18 |
| 2 | `grep -cE 'v[0-9]+' versions.txt` | ≥ 1 |
| 3 | `sed -i '' 's/old/new/' docs/brief-rules.md && echo ok` | ok |
| 4 | `cd statusgen && pwd -P` | an absolute path |
| 5 | `wc -c < STATUS.md` | ≥ 1000 |

## Review
Gate: model.
