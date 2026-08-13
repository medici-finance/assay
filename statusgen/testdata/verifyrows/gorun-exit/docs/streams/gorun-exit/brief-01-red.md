---
brief: gorun-exit/01
title: Rows asserting a specific non-zero exit through go run
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [493]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: #493 — go run flattens every failure to exit 1"]
---

# Red — every row asserts a code `go run` cannot deliver

Measured in #493: a `main` calling `os.Exit(5)` run through `go run` prints
`exit status 5` to stderr and exits **1**. So each row below reports 1 whatever
the tool did — including when the package did not compile at all.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go run ./tools/desk/cmd/repohardenguard --repo x; echo $?` | rc=5 (refused) |
| 2 | `go run ./cmd/deskpost ready --pr 1` | exit status 6 (could-not-check) |
| 3 | `go run ./cmd/deskboard --repo unlisted` | exit code 3 |
| 4 | `go run ./cmd/deskclaim take --item x; echo $?` | `$? = 4` (rate-limited) |

## Review
Gate: model.
