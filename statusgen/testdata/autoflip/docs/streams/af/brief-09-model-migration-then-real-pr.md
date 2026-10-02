---
brief: af/09
title: Model-gated verified brief — bulk migration touched it AFTER its real delivery
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: model-gated, verified — migration PR is newest, real delivery PR is older )"]
---

# Brief 09 — model-gated, verified (migration PR newest, real delivery older)

Regression fixture for B1: the NEWEST commit touching this
brief resolves to the same bulk brief-migration PR as af/08 (refused as a
candidate), but an OLDER commit in the same scan window resolves to a genuine
single-brief delivery PR — one file outside `docs/streams/**` plus exactly one
`Brief:` trailer, App-approved at its own merged head. The refusal of the
migration candidate must not be a dead end: the resolver keeps walking and
flips this brief citing the REAL delivering PR, never the migration.

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
