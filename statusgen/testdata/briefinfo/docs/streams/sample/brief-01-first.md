---
brief: sample/01
title: First sample brief
wave: 1
depends: [sample/00]
unblocks: [sample/09]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [42]
schema: brief-v1
authored: 2026-09-02 by fixture
sources: ["fixture: brief-info happy path"]
exec-tier: strong
exec-tier-why: needs careful cross-file reasoning
---

# Brief 01 — first sample brief

Body.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
