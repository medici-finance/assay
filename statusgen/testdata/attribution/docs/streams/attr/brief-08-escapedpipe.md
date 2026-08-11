---
schema: brief-v1
brief: attr/08
title: Every Evidence row is implementer-attributed, one row has an escaped pipe
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-07-08 by Fable session (test)
sources: ["s"]
---

# Brief 08

Regression fixture for assay#443: an Evidence table whose every
Runner cell reads `implementer (k3)`, shaped after the real-world table that
fired the gate open (example-repo#1748,
oracle-retention/brief-03-bounded-page-read.md). Row 10's Command cell
carries an escaped pipe (`\|`), which is what defeated the non-escape-aware
column split: `splitRow` cut the row at the escaped pipe too, so every row's
Runner text was read from the wrong cell — one that never says
"implementer" — and the all-implementer table passed as independently
backed.

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go build ./...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 2 | `go test ./...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 3 | `go vet ./...` | 0 | clean | 2026-07-08 | implementer (k3) |
| 4 | `gofmt -l .` | 0 | clean | 2026-07-08 | implementer (k3) |
| 5 | `go test -run TestBoundedRead ./...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 6 | `go test -race ./...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 7 | `go test -run TestPageBounds ./...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 8 | `go build ./cmd/...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 9 | `go test ./oracle/...` | 0 | ok | 2026-07-08 | implementer (k3) |
| 10 | `grep -ciE "arm64\|amd64\|x86" out.log` | 0 | 0 | 2026-07-08 | implementer (k3) |
| 11 | `go test ./retention/...` | 0 | ok | 2026-07-08 | implementer (k3) |
