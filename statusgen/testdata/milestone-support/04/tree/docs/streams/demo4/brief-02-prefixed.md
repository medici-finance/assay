---
brief: assay:assay:demo4:b-02
title: Prefixed
why: "Proves an unversioned dependency binds to the snapshot while the milestone start requirement still holds the work."
wave: 2
depends: ["demo4:b-01"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v4
version: 1
id: 22222222-2222-4222-8222-222222222222
requires-milestones: ["demo4:m-01@1"]
reader-requirements:
  schema: reader-requirements-v1
  required:
    - operation: syntax
      schemas: [brief-v4]
      semantics: []
authored: 2026-11-01 fixture
sources: ["fixture: prefixed"]
---

# Brief 02 — Prefixed

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
