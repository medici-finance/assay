---
brief: vs/02
title: A brief-v1 file with no Verify section at all
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: Verify section missing"]
---

# Brief 02 — Verify missing

This brief-v1 file has no `## Verify` section, so the presence lint must flag it.

## Review
Gate: model.
