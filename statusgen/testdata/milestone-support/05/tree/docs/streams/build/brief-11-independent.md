---
brief: assay:assay:build:b-11
title: Independent
why: "Shows that contributing to a milestone adds none of that milestone's prerequisites as a start gate."
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v4
version: 1
id: 00000000-0000-4000-8000-000000000511
reader-requirements:
  schema: reader-requirements-v1
  required:
    - operation: syntax
      schemas: [brief-v4]
      semantics: []
authored: 2026-11-01 fixture
sources: ["fixture: independent"]
---

# Brief 11 — Independent

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
