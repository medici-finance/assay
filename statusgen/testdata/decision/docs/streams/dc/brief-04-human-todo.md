---
brief: dc/04
title: Human-gated todo brief (not eligible)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: human-gated, todo"]
---

# Brief 04 — human-gated, todo

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

## Review
Gate: human (customer).
