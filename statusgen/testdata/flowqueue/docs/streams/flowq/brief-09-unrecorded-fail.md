---
brief: flowq/09
title: Unrecorded fail
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
# Unrecorded fail

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `go test ./...` | exit 0 |


## Evidence

**VERIFY: FAIL** - no outcome record names this brief

