# Brief 05 — done, live row silently absent from Evidence

The Evidence table below stops at row 1. Nothing anywhere in this file says
"UNRUN", "deferred", or anything else about row 2 — it is simply not mentioned.
That silence is the whole test: a state a verifier can decline to enter proves
nothing about the briefs that do not carry it, so the derivation must flag row 2
from the coverage gap alone.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | live end-to-end: a real settlement lands on the cluster | observed on the ledger |

## Evidence

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |
