---
brief: moving-ref/01
title: Comparison bases that move independently of the tree under test
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [639]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: #639 — the identical --base origin/main run returned exit 1 then exit 2 as main advanced"]
---

# Red — the answer depends on when you ask

Every row here compares against a ref another branch controls, so the same
command on the same tree returns different answers as that ref advances. The
long ref name in row 3 is the same moving ref, not a pin.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --consumers --base origin/main` | exit 0 |
| 2 | `statusgen --root . --consumers --base=origin/main` | exit 0 |
| 3 | `statusgen --root . --consumers --base refs/remotes/origin/main` | exit 0 |
| 4 | `git log --oneline origin/main..HEAD \| wc -l` | 3 |

## Review
Gate: model.
