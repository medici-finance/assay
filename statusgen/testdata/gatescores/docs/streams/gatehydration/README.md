---
stream: gatehydration
status: active
priority: P1
track: product
serves: assay
---
# Gate-scores hydration test stream

Fixture for the end-to-end `--gate-scores` test (issue 266). Brief 01 is
`implemented` (so it is scored), carries `value: high`, and is depended on by the
still-`todo` brief 02. A correctly hydrated score is therefore
priorityWeight(P1)=2000 + valueWeightHigh=200 + unblocksWeight×1=500 = 2700 with
blockedCount 1. Without frontmatter hydration the value defaults to med and the
depends graph is empty, collapsing the same row to 2000 / blockedCount 0.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [High-value awaiting brief](./brief-01-high-value.md) | 0 | S | implemented | — | — |
| 02 | [Blocked downstream brief](./brief-02-blocked.md) | 1 | S | todo | — | — |
