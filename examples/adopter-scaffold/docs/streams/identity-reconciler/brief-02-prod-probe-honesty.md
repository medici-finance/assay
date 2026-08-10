---
brief: identity-reconciler/02
title: Prod Canton-half probe honesty
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by scaffold example
sources: ["needs-fixing Cluster 4"]
---

# Brief 02 — Prod Canton-half probe honesty

## Context
files: internal/identity/result.go (new)
facts:
- Placeholder scaffold brief demonstrating the brief-v1 format for an adopter.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Placeholder.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer. -->

## Review
Gate: model (from frontmatter).
