---
schema: brief-v1
brief: dg/06
title: Risk-gated in-progress citing a design record that does not exist
wave: 0
depends: []
unblocks: []
effort: S
gate: human
gate-why: touches a risk surface (fixture)
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
authored: 2026-09-10 by designgate fixture
sources: ["fixture"]
design: DR-does-not-exist-x
---

# Brief 06

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | rc 0 |
