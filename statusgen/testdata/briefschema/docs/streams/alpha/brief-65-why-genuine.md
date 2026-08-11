---
brief: alpha/65
title: Add a retry budget to the settlement poller
why: |
  Customers occasionally see a stuck settlement when the participant node
  hiccups mid-poll, and today the only recovery is a manual restart from an
  operator. That is a support burden and an unnecessary trust hit for a
  process that should be self-healing within a few seconds.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by fixture
sources: ["fixture: genuine why"]
---

# Brief 65 — genuine why

Body.
