---
brief: lp/06
title: Verdict-first FAIL run, later PASS run without a sha
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

# Brief 06 — Verdict-first FAIL run, later PASS run without a sha

The earlier entry puts its verdict above its rows, so its rows sit inside the
span that ends at the later PASS. The later run's Runner cells lack `@ sha`,
so the strict read refuses and the stamp falls back; the fallback must still
name the PASS run, never the FAIL run's row.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
### Verifier run — 2026-07-18

**VERIFY: FAIL** — row 1 exited 1.

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 1 | FAIL | 2026-07-18 | opus-verifier |

### Verifier run — 2026-07-20

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-20 | opus-verifier |

**VERIFY: PASS** — all rows green.

## Review
Gate: human (regulatory).
