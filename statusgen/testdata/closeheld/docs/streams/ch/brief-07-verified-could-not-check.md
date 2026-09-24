---
brief: ch/07
title: Human verified over a could-not-check row
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, PASS marker contradicted by an un-routed could-not-check row"]
---

# Brief 07 — Human verified over a could-not-check row

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
| 2 | `go test ./integration/...` | — | could-not-check: the integration host was unreachable | 2026-07-10 | opus-verifier |

**VERIFY: PASS** (model) — row 1 green.

## Review
Gate: human (regulatory).
