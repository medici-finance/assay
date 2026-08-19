---
brief: gatehydration/01
title: High-value awaiting brief
wave: 0
depends: []
unblocks: [gatehydration/02]
effort: S
value: high
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
why: |
  Exercises the gate-score value weight end-to-end so a regression that drops
  frontmatter hydration (issue 266) is caught by a test rather than by a
  mis-prioritized deskboard.
authored: 2026-08-15 by test-fixture
sources: ["issue 266"]
---
# High-value awaiting brief

An `implemented` brief with `value: high` that brief 02 depends on. Its
gate-score must reflect both the value weight and the unblocks term once its
frontmatter is hydrated.
