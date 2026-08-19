---
brief: gatehydration/02
title: Blocked downstream brief
wave: 1
depends: [gatehydration/01]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
why: |
  A still-todo brief that depends on brief 01, giving brief 01 a blockedCount of
  one so the unblocks term of the gate score is non-zero in the fixture.
authored: 2026-08-15 by test-fixture
sources: ["issue 266"]
---
# Blocked downstream brief

Depends on `gatehydration/01`. It is `todo`, so it is not itself scored by
`--gate-scores`, but it makes brief 01 unblock exactly one open item.
