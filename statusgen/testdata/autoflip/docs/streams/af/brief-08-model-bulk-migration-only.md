---
brief: af/08
title: Model-gated verified brief — only a bulk brief-migration PR resolves
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: model-gated, verified — only a bulk-migration PR resolves )"]
---

# Brief 08 — model-gated, verified (only a bulk-migration PR resolves)

Regression fixture for B1: the only commit touching this brief
resolves to a bulk brief-migration/reformat PR (docs/streams/**-only diff, many
Brief:/Authors: trailers) — shaped like the motivating migration. Even though
that PR is merged and carries an App approval at its head, it must never be
accepted as THIS brief's delivering PR: it never touched this brief's actual
content. With no other candidate in the commit window, the resolver reports
could-not-check, not a flip.

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
