# Brief 01 — fixture

## Context

| # | Command | Expect |
|---|---------|--------|
| 9 | `go test ./pkg/parse/ -run '^TestAlpha$'` | outside Verify: never reported |

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./pkg/parse/ -run '^TestAlpha$' -count=1` | `ok` |
| 2 | `go test ./pkg/parse/ -run TestGamma -count=1` | `ok` |
| 3 | `go test ./pkg/parse/ -run TestAlphaBeta -count=1` | a longer name: not a match |
| 4 | `go test ./pkg/parse/ -run TestEpsilon -count=1` | kept: not reported |
| 5 | `go test ./pkg/parse/ -run TestBeta -count=1` | trailed, still vacuous: reported |

## Evidence

| 1 | `go test ./pkg/parse/ -run TestAlpha` | after Verify: never reported |
