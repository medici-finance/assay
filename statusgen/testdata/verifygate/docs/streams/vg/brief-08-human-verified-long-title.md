---
brief: vg/08
title: Human-gated verified brief with an extremely long title that exceeds the GitHub issue-title limit of two hundred and fifty six characters by quite a significant margin — this deliberate overrun exercises the shared title truncation function to prove that the verify-issues emitter actually routes through it rather than raw concatenation
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: yes, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by fixture
sources: ["fixture: human-gated, verified, long title > 256 runes"]
---

# Brief 08 — human-gated, verified, long title

This brief's title exceeds 256 runes to prove that the verifyIssues emitter
routes through issueTitle() and truncates rather than raw concatenation (G6).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->
PR: [#99](https://github.com/example-org/tracker/pull/99) (merged 2026-07-10)

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-10 | glm-verifier |

## Review
Gate: human (regulatory + irreversible).
