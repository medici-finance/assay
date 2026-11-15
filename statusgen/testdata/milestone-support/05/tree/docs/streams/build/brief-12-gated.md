---
brief: assay:assay:build:b-12
title: Gated
why: "Contributes to build m-03 at revision 2 and needs stream-y m-01 before it starts."
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v4
version: 2
id: 00000000-0000-4000-8000-000000000512
requires-milestones: ["stream-y:m-01@1"]
reader-requirements:
  schema: reader-requirements-v1
  required:
    - operation: syntax
      schemas: [brief-v4]
      semantics: []
authored: 2026-11-01 fixture
sources: ["fixture: gated"]
---

# Brief 12 — Gated

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
