---
brief: dc/08
title: Human-gated done brief with decision recorded in the body (silent)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
gate-why: fixture — regulatory exposure requires a human call.
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
decision-issue: 88
schema: brief-v1
authored: 2026-07-13 by fixture
sources: ["fixture: human-gated, done, decision reflected"]
---

# Brief 08 — human-gated, done, decision recorded

Decision: option A chosen via #88 (closed 2026-07-13) — proceed as specified.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
PR: [#88](https://github.com/example-org/tracker/issues/88)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-13 | glm-verifier |

## Review
Gate: human (regulatory).
