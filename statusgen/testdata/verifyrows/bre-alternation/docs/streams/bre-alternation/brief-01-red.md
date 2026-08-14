---
brief: bre-alternation/01
title: Alternation in a grep with no -E — the rows match their own line
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [262]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: #262 — grep alternation without -E; #257's table returned 1,1,1,1,1,1,1"]
---

# Red — a basic regex reads `|` as an ordinary character

Each pattern below is searched as ONE long literal string. The only line in the
brief containing that string is the Verify row itself, so each row returns 1 and
exits 0 — and row 4, whose bar is `≥1`, reports PASS having measured nothing.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "drain\|refuse\|watchdog" docs/contract.md` | ≥3 |
| 2 | `grep -ci "arm64\|amd64" S1-report.md` | ≥2 |
| 3 | `grep -c "alpha\|beta\|gamma" doc.md` | ≥3 |
| 4 | `grep -c "the-desk\|verify-desk" skills.md` | ≥1 |

## Review
Gate: model.
