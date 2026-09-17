---
schema: brief-v1
brief: mp/01
title: App-authored Evidence row with no on-behalf-of principal
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-09-16 by fixture
sources: ["fixture: multi-principal/01 Verify row 3"]
---

# Brief 01

An Evidence row landed by a shared App identity (`assay-verifier-app[bot]`) with no
on-behalf-of annotation naming the human principal behind the write.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-09-17 | assay-verifier-app[bot] |
