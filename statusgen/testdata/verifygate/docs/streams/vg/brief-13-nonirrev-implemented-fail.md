---
brief: vg/13
title: Non-irreversible human-gated brief at implemented with a recorded FAIL
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-18 by fixture
sources: ["fixture: non-irreversible human-gate, implemented, no strict VERIFY: PASS marker"]
---

# Brief 13 — non-irreversible, implemented, no recorded pass

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
Verifier run (independent, non-implementer — glm-verifier):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 1 | 1 test FAIL | 2026-07-18 | glm-verifier |

**VERIFY: FAIL** — row 1 red; not eligible for a sign-off issue.

## Review
Gate: human (integrity surface; regulatory risk).
