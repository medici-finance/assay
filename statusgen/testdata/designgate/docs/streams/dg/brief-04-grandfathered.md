---
schema: brief-v1
brief: dg/04
title: Risk-gated in-progress authored before the cutover (grandfathered)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
gate-why: touches a risk surface (fixture)
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
authored: 2026-08-01 by designgate fixture
sources: ["fixture"]
---

# Brief 04

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | rc 0 |
