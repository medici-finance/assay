---
brief: multiroot-fixture/01
title: Fixture brief at done — exercises the Done/Evidence path
why: >-
  A second root whose briefs are all todo would never render an Awaiting row, a
  Done row, or a filled Evidence section, so a bug that dropped those sections for
  a non-primary root would pass the fixture unnoticed. This brief supplies the
  done end of the lifecycle.
wave: 0
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

# Fixture brief 01 — done

## Context
files: none — this brief is fixture data, not work.
facts:
- Synthetic. Nothing here describes real work, and nothing outside
  `statusgen/testdata/stubroot/` depends on it.

## Task
Exist at `done` with a dated Verified cell, a dated Reviewed cell, and a filled
Evidence section, so the stub root's board renders its own Done section.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c multiroot-fixture statusgen/testdata/stubroot/STATUS.md` | a count greater than 0 |

## Evidence
| # | Result |
|---|--------|
| 1 | Fixture row — the stub board names this stream because this stream is the only one under the stub root. |
