---
brief: multiroot-fixture/03
title: Fixture brief at todo — exercises Next-up eligibility
why: >-
  Next-up is the board's whole point and the place a silently-dropped stream is
  invisible; the fixture needs at least one eligible todo brief so the stub root's
  Next-up section is populated from the stub root's own streams and can be
  distinguished from an empty board.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-25 by the multiroot fixture (assay-selfcontain/01)
sources: ["assay-selfcontain/01 task 5 (committed synthetic populated fixture)"]
---

# Fixture brief 03 — todo

## Context
files: none — this brief is fixture data, not work.
facts:
- Synthetic. Sits at `todo` so the stub root's Next-up section has a candidate.

## Task
Exist at `todo` so the stub root's Next-up section is computed from the stub root's
own eligible briefs.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'Next up' statusgen/testdata/stubroot/STATUS.md` | a count greater than 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
