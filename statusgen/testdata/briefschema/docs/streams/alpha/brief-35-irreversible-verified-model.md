---
brief: alpha/35
title: Irreversible brief marked verified on a model-only sign-off
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: irreversible verified, model sign-off only"]
---

# Brief 35 — irreversible, verified, model sign-off only

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-08 | glm-verifier |

## Review
Gate: human — but this irreversible brief was marked verified with no human in the Reviewed column.
