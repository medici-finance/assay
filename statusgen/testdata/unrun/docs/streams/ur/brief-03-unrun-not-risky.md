# Brief 03 — done over an UNRUN non-risk-bearing row

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `wc -l README.md` | a number |

## Evidence

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |
| 2 | `wc -l README.md` | UNRUN | cosmetic; skipped | 2026-07-08 | sonnet-verifier |
