---
brief: ch/12
title: Human verified, superseded hold left unstruck
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, an earlier-run hold a later run resolved but nobody struck through"]
---

# Brief 12 — Human verified, superseded hold left unstruck

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `go test ./integration/...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | opus-verifier |
| 2 | `go test ./integration/...` | — | could-not-check — no runner online | 2026-07-08 | opus-verifier |

Row 2 awaits an online runner.

Online runner, same table:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | opus-verifier |
| 2 | `go test ./integration/...` | 0 | ok | 2026-07-10 | opus-verifier |

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (regulatory).
