---
stream: example-app
repo: example-org/example-app
status: active
priority: P2
track: platform
---

# Example-app — flow-instruments fixture (graph-execution/07)

Fixture tree for `TestFlow` / `TestFlowRefusesOpenInterval` (graph-execution/07) and
Verify rows 3–7 of `docs/streams/graph-execution/brief-07-flow-instruments.md`. Not a
real stream. `example-app/01` clears first; `example-app/02` depends on it and starts
work 3 days later (its `eligible_to_start` is `measured`); `example-app/03` has no
dependency and is left mid-`in-progress` with no closing historian record, so its
`active_work_time` must read `could-not-check`, never a fabricated duration.

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [One](brief-01-a.md) | 0 | S | done | — | — |
| 02 | [Two](brief-02-b.md) | 0 | S | verified | — | — |
| 03 | [Three](brief-03-c.md) | 0 | S | in-progress | — | — |
