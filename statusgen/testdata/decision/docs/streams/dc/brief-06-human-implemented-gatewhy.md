---
brief: dc/06
title: Human-gated implemented brief with gate-why rationale
wave: 0
depends: []
unblocks: [dc/08, dc/09]
effort: M
gate: human
gate-why: Rewrites an immutable on-ledger invariant; wrong math forks balances with no fix.
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: human-gated, implemented, gate-why"]
---

# Brief 06 — human-gated, implemented, with gate-why

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `go vet ./...` | exit 0 |

## Evidence
PR: [#56](https://github.com/example-org/tracker/pull/56) (merged 2026-07-10)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | glm-verifier |

## Review
Gate: human (regulatory + irreversible). This brief rewrites core ledger math.
