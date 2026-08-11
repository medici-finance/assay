# Brief 03 — Unbacked done: bare Verified cell

Evidence is filled, but the README row's Verified cell ("not-dated") is not
a "YYYY-MM-DD <runner>" dated+attributed cell — this row must render as
`done*` even though Evidence looks fine.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |
