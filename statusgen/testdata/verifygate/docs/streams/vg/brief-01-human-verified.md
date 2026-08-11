---
brief: vg/01
title: Human-gated verified brief (eligible for a verify-gate issue)
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: human-gated, verified, eligible"]
---

# Brief 01 — human-gated, verified

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->
PR: [#42](https://github.com/example-org/tracker/pull/42) (merged 2026-07-08)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | glm-verifier |

## Review
Gate: human (regulatory + irreversible).
