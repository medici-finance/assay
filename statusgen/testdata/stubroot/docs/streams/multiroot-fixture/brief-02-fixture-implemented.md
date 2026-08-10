---
brief: multiroot-fixture/02
title: Fixture brief at implemented — exercises the Awaiting queue
why: >-
  The Awaiting-verification queue, its gate scores and its historian-backed ages
  are per-root derived data; without a row at implemented under the second root
  there is nothing to prove those numbers came from the second root's own
  historian rather than the first root's.
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

# Fixture brief 02 — implemented

## Context
files: none — this brief is fixture data, not work.
facts:
- Synthetic. Sits at `implemented` so the stub root's Awaiting section is non-empty.

## Task
Exist at `implemented` so the stub root's "Awaiting verification / review" section
renders a row scored from the stub root's own historian.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'Awaiting verification' statusgen/testdata/stubroot/STATUS.md` | a count greater than 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
