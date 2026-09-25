---
brief: ch/09
title: Human verified, no verdict marker, un-routed HELD row
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, no verdict marker at all plus an un-routed HELD row"]
---

# Brief 09 — Human verified, no verdict marker, un-routed HELD row

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `go test ./integration/...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | opus-verifier |
| 2 | `go test ./integration/...` | — | HELD — no runner online | 2026-07-10 | opus-verifier |

Row 1 green.

## Review
Gate: human (regulatory).
