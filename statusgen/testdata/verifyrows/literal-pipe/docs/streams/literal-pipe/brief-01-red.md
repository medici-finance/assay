---
brief: literal-pipe/01
title: RE2 literal-pipe in go test selectors, and the cell the raw pipe shredded
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [374]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: #374 — ledger-hardening/15 row 2 matched ZERO tests and exited 0"]
---

# Red — patterns that match no test, and a row cut in half

Rows 1-3 compile `\|` as a LITERAL pipe under RE2, so they match no test or
benchmark name, print "no tests to run", and exit 0. Row 4 shows the other half
of #374: a RAW pipe is a cell delimiter, so the command is cut at `'Dora` and
the cell after it is a fragment of the command masquerading as an expectation.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run 'Forged\|Sub\|Onboard\|UserStatus'` | PASS |
| 2 | `go test ./statusgen/ -run='Dora\|Weekly'` | PASS |
| 3 | `go test -bench 'Board\|Lint' ./... -benchtime=1x` | PASS |
| 4 | `go test ./statusgen/ -run 'Dora|Weekly|Artifact'` | PASS |

## Review
Gate: model.
