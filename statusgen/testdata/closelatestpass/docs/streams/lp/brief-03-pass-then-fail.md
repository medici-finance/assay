---
brief: lp/03
title: PASS run, later FAIL run
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-18 by fixture
sources: ["fixture: gate:human at implemented; which Evidence run the one-step close stamps"]
---

# Brief 03 — PASS run, later FAIL run

The one-step implemented→done close stamps the Verified cell from the latest
strict PASS run in Evidence, never from an earlier run.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
### Verifier run — 2026-07-20

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-20 | opus-verifier @ 2222222bbbbb |

**VERIFY: PASS** — all rows green.

### Verifier run — 2026-07-22

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 1 | FAIL | 2026-07-22 | opus-verifier @ 3333333ccccc |

**VERIFY: FAIL** — row 1 exited 1.

## Review
Gate: human (regulatory).
