---
brief: assay:assay:demo4:b-03
title: Pinned
why: "A brief-v4 brief pinning a dependency revision the snapshot does not carry."
wave: 2
depends: ["demo4:b-01@1"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v4
version: 1
id: 33333333-3333-4333-8333-333333333333
authored: 2026-11-01 fixture
sources: ["fixture: pinned"]
---

# Brief 03 — Pinned

## Context
files:
facts:
- nothing

## Task
1. Nothing.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |
