---
brief: af/07
title: Model-gated verified brief — touched in a merge-committed PR's intermediate commit
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: model-gated, verified — brief file touched in an intermediate (non-head) commit of a PR merged as a real merge commit"]
---

# Brief 07 — model-gated, verified (touched in an intermediate commit of a merge-committed PR)

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go vet ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `go vet ./...` | pass exit=0 | sha256:abc123def456 | 2026-07-08 | fixture-verifier |

## Review
Gate: model.
