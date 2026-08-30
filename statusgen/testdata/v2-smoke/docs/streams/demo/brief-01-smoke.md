---
brief: smoke:sg:demo:01
title: Reserved-key smoke fixture
why: >-
  Exercises the brief-v2 reserved keys end to end so --lint proves it parses and
  validates the hierarchical id, the gates edge, and the identity fields.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 4f8c2d1a-9b3e-4c7a-8f21-0a1b2c3d4e5f
supersedes: []
gates:
  - on: "rec:ingest/06"
    type: ordering-gate
    reason: the ingest ordering must land before this demo can be sequenced
authored: 2026-08-22 by derived-board smoke fixture
sources: ["fixture: brief-v2 reserved-key smoke"]
---

# Brief 01 — reserved-key smoke

## Context
files:
- `docs/streams/graph-repos.yaml`

facts:
- A brief-v2 file with one reserved gates edge lints clean (rc=0) and the edge is
  reported reserved-not-gating.

## Task
1. Parse the reserved keys.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go run . --lint --root testdata/v2-smoke` | rc=0 with a reserved-not-gating notice |

## Evidence
<!-- appended at implementation time -->
