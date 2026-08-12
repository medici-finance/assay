# Brief 02 — done over a routed UNRUN live row

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | live end-to-end: a real settlement lands on the cluster | observed on the ledger |

## Evidence

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |
| 2 | live settlement | UNRUN | routed to follow-up ur/12 — needs the demo cluster window | 2026-07-08 | sonnet-verifier |
