---
brief: af/02
title: Model-gated verified brief — App approval at a stale commit
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: model-gated, verified — App approval at a stale commit"]
---

# Brief 02 — model-gated, verified (App approval at a stale commit)

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go vet ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go vet ./...` | 0 | ok | 2026-07-08 | fixture-verifier |

## Review
Gate: model.
