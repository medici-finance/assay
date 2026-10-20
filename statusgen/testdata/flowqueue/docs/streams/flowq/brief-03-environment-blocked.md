---
brief: flowq/03
title: Environment blocked
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
# Environment blocked

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:cluster | `kubectl get pods` | exit 0 |


## Evidence


