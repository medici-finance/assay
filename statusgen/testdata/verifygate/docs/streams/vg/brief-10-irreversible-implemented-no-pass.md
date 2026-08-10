---
brief: vg/10
title: Irreversible brief at implemented without model verify pass
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by fixture
sources: ["fixture: irreversible, implemented, no VERIFY: PASS"]
---

# Brief 10 — irreversible, implemented, NO verify pass

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

## Review
Gate: human (irreversible).
