---
schema: brief-v1
brief: mp/01
title: App-run Evidence rows the landing tool stamps
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-10-14 by fixture
sources: ["fixture: deskevidence on-behalf-of stamp"]
---

# Brief 01

Evidence rows landed by a shared App identity, before and after the landing tool stamps
the on-behalf-of principal into their Runner cells.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `go vet ./...` | exit 0 |
| 3 | `go build ./...` | exit 0 |

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-10-14 | example-app[bot] @ abc1234 (forge-identity) |
| 2 | `go vet ./...` | 0 | ok | 2026-10-14 | example-app[bot] @ abc1234 |
| 3 | `go build ./...` | 0 | ok | 2026-10-14 | human:operator |
