---
schema: brief-v1
brief: mp/01
title: App-authored Evidence row with an unrecognised on-behalf-of principal
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
authored: 2026-09-16 by fixture
sources: ["fixture: multi-principal/01 Verify row 4"]
---

# Brief 01

An Evidence row landed by a shared App identity carries an on-behalf-of annotation, but
the named login is not in this repo's roster human map.

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-09-16 | assay-verifier-app[bot] @ abc1234 (on-behalf-of human:ghost) |
