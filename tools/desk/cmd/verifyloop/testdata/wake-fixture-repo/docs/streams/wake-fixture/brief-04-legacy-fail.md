---
brief: legacy-fail
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
effort: S
---

# legacy-fail

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./src -run T04` | exit 0 |

## Evidence
<!-- appended at verification time -->
