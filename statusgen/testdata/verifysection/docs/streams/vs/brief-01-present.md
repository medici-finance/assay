---
brief: vs/01
title: A brief-v1 file with a well-formed Verify table
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: Verify section present"]
---

# Brief 01 — Verify present

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Review
Gate: model.
