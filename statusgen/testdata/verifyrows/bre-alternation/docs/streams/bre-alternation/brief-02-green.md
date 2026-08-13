---
brief: bre-alternation/02
title: The same searches written without a pipe at all
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [262]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the #262 fix — separate -e patterns read identically in source and rendered form"]
---

# Green — the negative control

Separate `-e` patterns carry no pipe, so the source and the rendered page are
the same command. Row 4 keeps a literal pipe deliberately and says so with `-F`;
row 5 uses the bracket-class escape hatch. All five MUST stay silent.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -cE -e drain -e refuse -e watchdog docs/contract.md` | ≥3 |
| 2 | `grep -ciE -e arm64 -e amd64 S1-report.md` | ≥2 |
| 3 | `grep -c "gate-why" docs/brief-rules.md` | ≥1 |
| 4 | `grep -cF "a\|b" table.md` | ≥1 (a literal pipe, by intent) |
| 5 | `grep -cE "a[\|]b" table.md` | ≥1 (a literal pipe, by intent) |

## Review
Gate: model.
