---
brief: example:app:example-app:02
title: Fixture brief two — depends on 01, starts 3 days after it clears
why: >-
  Fixture-only: the historian records example-app/01 reaching `verified` on
  2026-08-04 and example-app/02 first going `in-progress` on 2026-08-07 — a 3
  day eligible_to_start delay, above the fixture's 1-day resolution, so it
  reports `measured` (Verify row 3).
wave: 0
depends: ["example-app/01"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution/07 fixture
sources: ["fixture: TestFlow / Verify row 3"]
---

# Brief 02 — fixture, verified, gated (by depends:) on brief 01
