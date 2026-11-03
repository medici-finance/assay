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
id: 00000000-0000-4000-8000-000000000403
reader-requirements:
  schema: reader-requirements-v1
  required:
    - operation: syntax
      schemas: [brief-v4]
      semantics: []
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
