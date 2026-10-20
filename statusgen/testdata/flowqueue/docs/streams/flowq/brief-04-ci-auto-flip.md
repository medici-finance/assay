---
brief: flowq/04
title: Ci auto flip
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
# Ci auto flip

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `go test ./...` | exit 0 |


## Evidence

| # | Command | Exit | Output | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go test ./...` | 0 | ok | 2026-10-01 | verifier-model |

**VERIFY: PASS**

