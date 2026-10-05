---
schema: brief-v1
brief: check/01
title: Check
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-08-01 by fixture
sources: [fixture]
---
# Check
## Context
files: src/check.go
## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |
## Evidence
| # | Result | Date | Runner |
|---|--------|------|--------|
| 1 | FAIL | 2026-08-01 | independent-verifier |

**VERIFY: FAIL**
