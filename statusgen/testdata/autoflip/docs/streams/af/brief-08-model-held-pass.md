---
brief: af/08
title: Model-gated verified brief — App approved at merged head, but Evidence's own PASS entry is HELD
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-16 by fixture
sources: ["fixture: model-gated, verified, App approved at merged head, Evidence's PASS entry is HELD"]
---

# Brief 08 — model-gated, verified, App approved at head, but Evidence's PASS entry is HELD

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go vet ./...` | exit 0 |

## Evidence
<!-- contract comment -->

**VERIFY: PASS** — offline rows pass; row 4 is HELD pending a runner slot.

## Review
Gate: model.
