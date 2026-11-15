---
brief: lp/07
title: PASS run without a sha whose rows disagree on the date
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

# Brief 07 — PASS run without a sha whose rows disagree on the date

The PASS run's rows do not read as one run, so the strict read refuses for a
reason other than the missing `@ sha`; the stamp has no run to name and the
close refuses.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `go vet ./...` | exit 0 |

## Evidence
### Verifier run — 2026-07-20

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-20 | opus-verifier |
| 2 | `go vet ./...` | 0 | ok | 2026-07-21 | opus-verifier |

**VERIFY: PASS** — all rows green.

## Review
Gate: human (regulatory).
