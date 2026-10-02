---
brief: ch/05
title: Human verified, hold routed to a follow-up
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, the held row is routed to a follow-up with a reference"]
---

# Brief 05 — Human verified, hold routed to a follow-up

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
| 2 | `go test ./integration/...` | — | HELD — deferred to follow-up brief ch/09 | 2026-07-10 | opus-verifier |

**VERIFY: PASS** (model) — row 1 green; row 2 routed.

## Review
Gate: human (regulatory).
