---
brief: example:app:example-a:02
title: "Fixture brief two — carries the gates edge under test"
why: >-
  Fixture-only: this is the brief the milestone test flips from held to
  eligible by editing ONLY its gates: on: target — no code path keyed on the
  stream name.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
gates:
  - on: "example-a/01"
    type: ordering-gate
    reason: "example-a/01 must be in force before example-a/02 may start"
authored: 2026-09-16 by graph-execution/01 fixture
sources: ["fixture: TestEligibilityDeclarationChangesDispatch"]
---

# Brief 02 — fixture, gated on example-a/01
