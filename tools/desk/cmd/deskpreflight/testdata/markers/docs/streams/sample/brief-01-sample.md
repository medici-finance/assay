---
brief: sample/01
title: Sample brief
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by fixture
sources: ["fixture — deskpreflight markers tree"]
---

# Brief 01 — sample

A fixture brief so the stream is non-empty and the board lints clean; the only
seeded defect in this tree is the conflict-marker file at the root.

## Task
Nothing — this is a fixture.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |
