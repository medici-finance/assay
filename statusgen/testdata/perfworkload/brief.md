---
brief: {{stream}}/{{num}}
title: Synthetic workload brief {{num}}
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-01-01 by synthetic-fixture
sources: [synthetic-fixture]
---
# Synthetic workload brief

## Context
Credential-free offline workload. Measures the existing parser and evaluator.

## Task
One synthetic unit.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |

## Evidence
