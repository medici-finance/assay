---
brief: dc/07
title: Human-gated verified brief with gate-why rationale
wave: 0
depends: []
unblocks: [dc/10]
effort: M
gate: human
gate-why: Exposes customer PII in a new query path; a wrong ACL leaks data to every user.
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: human-gated, verified, gate-why"]
---

# Brief 07 — human-gated, verified, with gate-why

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
PR: [#57](https://github.com/example-org/tracker/pull/57) (merged 2026-07-10)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | glm-verifier |

## Review
Gate: human (customer + sensitive-data).
