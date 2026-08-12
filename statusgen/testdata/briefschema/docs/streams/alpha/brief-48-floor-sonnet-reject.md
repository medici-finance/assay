---
brief: alpha/48
title: Risk-flagged brief verified by a sonnet runner (floor must reject)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified by a below-floor sonnet runner"]
---

# Brief 48 — risk-flagged, verified by sonnet

`sonnet` is a named below-floor family. It passed the floor under the previous
substring list purely because nobody had listed it; the floor is a capability
statement, and sonnet sits below it.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | reviewer |

## Review
Gate: human (customer). Verified below the floor — the floor should reject this.
