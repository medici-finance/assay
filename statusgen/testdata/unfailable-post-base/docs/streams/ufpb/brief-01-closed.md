---
brief: ufpb/01
title: Two unfailable rows replayed from recorded incidents
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [262, 493]
schema: brief-v1
authored: 2026-10-08 by fixture
sources: ["fixture: #262 — grep alternation without -E", "fixture: #493 — go run flattens the exit code"]
---

# Closed — the rows pass whatever the tree holds

Row 1 replays #262: a basic-regex grep reads `|` as an ordinary character, so
the row counts its own line. Row 2 replays #493: `go run` exits 1 for every
non-zero program status, so the row cannot tell one refusal from another.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c "drain\|refuse\|watchdog" docs/contract.md` | ≥3 |
| 2 | `go run ./cmd/probe --strict` | exit code 3 |

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | replayed row 1 | 0 | 3 | 2026-10-08 | fixture-verifier |
| 2 | replayed row 2 | 1 | exit status 3 | 2026-10-08 | fixture-verifier |

## Review
Gate: model.
