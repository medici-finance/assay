---
brief: ch/02
title: Human verified whose latest verdict is FAIL
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: gate:human, verified, an earlier PASS followed by a later FAIL run"]
---

# Brief 02 — Human verified whose latest verdict is FAIL

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
| 2 | `go test ./integration/...` | 0 | ok | 2026-07-08 | opus-verifier |

**VERIFY: PASS** (model) — all rows green.

Re-run after a later merge:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 1 | TestFoo failed | 2026-07-10 | opus-verifier |
| 2 | `go test ./integration/...` | 0 | ok | 2026-07-10 | opus-verifier |

**VERIFY: FAIL** (model) — row 1 red.

## Review
Gate: human (regulatory).
