---
brief: af/03
title: Human-gated verified brief — the model path must never touch it
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: human-gated, verified — the model path must never reach it"]
---

# Brief 03 — human-gated, verified

This brief's merge PR carries the SAME App approval at the SAME merged head as
af/01. If the model path ever reaches a `gate: human` brief, this row flips and
the test fails — that is the point of the fixture.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go vet ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |

## Review
Gate: human (regulatory).
