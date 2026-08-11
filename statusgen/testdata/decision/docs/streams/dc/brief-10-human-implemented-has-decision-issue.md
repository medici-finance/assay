---
brief: dc/10
title: Human-gated implemented brief that already carries a decision-issue (backfilled — must not emit)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
decision-issue: 999
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: human-gated, implemented, already has decision-issue #999 backfilled"]
---

# Brief 10 — human-gated, implemented, decision-issue already assigned

This brief already carries `decision-issue: 999` so it MUST NOT receive a second
issue from --decision-issues. The dedup guard (G1, decisionissues.go:162) skips
briefs whose DecisionIssue field is non-zero.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

## Review
Gate: human (regulatory + irreversible).
Decision recorded in #999 — option A was chosen and the implementation was accepted.
