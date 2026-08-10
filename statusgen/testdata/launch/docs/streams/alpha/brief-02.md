---
brief: alpha/02
title: An in-progress dependency
why: fixture — implemented dep that itself depends on alpha/01 (transitive test)
wave: 0
depends: ["alpha/01"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-01 by fixture
sources: ["fixture"]
---

# Brief 02 — in-progress dep

This is an implemented dependency that itself depends on alpha/01, providing a
transitive dependency for the launch view test.
