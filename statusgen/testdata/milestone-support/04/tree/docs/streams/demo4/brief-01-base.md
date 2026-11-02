---
brief: assay:assay:demo4:01
title: Base
why: "A legacy brief-v2 dependency at revision 2."
wave: 1
depends: []
unblocks: ["demo4/02", "demo4/03"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 2
id: 00000000-0000-4000-8000-000000000401
authored: 2026-11-01 fixture
sources: ["fixture: base"]
---

# Brief 01 — Base

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
