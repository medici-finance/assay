---
brief: ch/10
title: Human verified, FAIL followed by a prose PASS mention
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, a strict FAIL followed only by a prose mention of a future PASS"]
---

# Brief 10 — Human verified, FAIL followed by a prose PASS mention

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
| 2 | `go test ./integration/...` | 1 | TestBar failed | 2026-07-10 | opus-verifier |

**VERIFY: FAIL** — row 2 red.

Re-run pending; will record VERIFY: PASS once green.

## Review
Gate: human (regulatory).
