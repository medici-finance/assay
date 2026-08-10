---
brief: alpha/72
title: exec-tier strong with why
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by fixture
sources: ["fixture: exec-tier strong with why"]
exec-tier: strong
exec-tier-why: "requires cross-component reasoning (question 2) — the change touches both the parser and the emitter"
why: "Prevents a model-tier mismatch at the most expensive gate — a cheap-tier implementation of a subtle cross-component change wastes review cycles and risks silent errors."
---

# Brief 72 — exec-tier strong, why present
