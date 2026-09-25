---
brief: ch/11
title: Human verified, superseded hold struck through
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, an earlier-run hold struck through after a later run executed the row green"]
---

# Brief 11 — Human verified, superseded hold struck through

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
| 2 | `go test ./integration/...` | — | ~~could-not-check — no runner online~~ superseded by the 2026-07-10 run below | 2026-07-08 | opus-verifier |

Row 2 awaits an online runner.

Online runner, same table:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | opus-verifier |
| 2 | `go test ./integration/...` | 0 | ok | 2026-07-10 | opus-verifier |

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (regulatory).
