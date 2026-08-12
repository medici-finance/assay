---
brief: dc/11
title: Human-gated implemented brief with an extremely long title that exceeds the GitHub issue-title limit of two hundred and fifty six characters by quite a significant margin — this deliberate overrun exercises the shared title truncation function to prove that the decision-issues emitter actually routes through it rather than raw concatenation
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: human-gated, implemented, long title > 256 runes"]
---

# Brief 11 — human-gated, implemented, long title

This brief's title exceeds 256 runes to prove that the decisionIssues emitter
routes through issueTitle() and truncates rather than raw concatenation (G6).

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

## Review
Gate: human (regulatory + irreversible).
