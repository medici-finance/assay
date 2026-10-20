---
brief: flowq/08
title: Corrupt evidence
wave: 0
depends: []
unblocks: []
effort: S
value: med
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
why: |
  Queue contract fixture.
authored: 2026-10-18 by test-fixture
sources: []
---
# Corrupt evidence

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `go test ./...` | exit 0 |


## Evidence

<!-- an unterminated comment hides every row after it
| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |

