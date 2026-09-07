---
schema: brief-v1
brief: rerun/04
title: Implemented brief is out of scope
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-07-25 by Fable session (test)
sources: ["s"]
---

# Brief 04

Same disagreement shape as brief 01, but the row is still `implemented` — the
check reads `verified`/`done` rows only, so this raises nothing.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `manual source check` | 0 | ok | 2026-07-27 | k3-verifier |
| 2 | `grep -c foo article.md` | 0 | 3 | 2026-07-29 | opus-verifier |
| 3 | `grep -ci bar article.md` | 0 | 1 | 2026-07-29 | opus-verifier |
