---
brief: held-b
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
effort: S
---

# held-b

## Context

files:
- src/b.go

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./src -run T02` | exit 0 |

## Evidence
<!-- appended at verification time -->
