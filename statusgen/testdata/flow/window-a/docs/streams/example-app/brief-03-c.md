---
brief: example:app:example-app:03
title: Fixture brief three — open in-progress interval, no closing record
why: >-
  Fixture-only: the historian's last record for example-app/03 is
  `to: "in-progress"` with nothing after it — the interval is OPEN.
  active_work_time must read could-not-check, never a fabricated duration
  (TestFlowRefusesOpenInterval, Verify row 2).
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution/07 fixture
sources: ["fixture: TestFlowRefusesOpenInterval / Verify row 2"]
---

# Brief 03 — fixture, open in-progress interval
