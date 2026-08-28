---
brief: vg/12
title: Non-irreversible human-gated brief at implemented with model verify pass
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-18 by fixture
sources: ["fixture: non-irreversible human-gate, implemented, VERIFY: PASS + Date/Runner table"]
gate-why: >-
  Changes an integrity-check surface; the human confirms the recorded model verify
  pass before the brief is accepted as done.
why: >-
  A non-irreversible gate:human brief with a recorded pass must still raise a
  sign-off issue at implemented — the class this fixture proves is now emitted.
---

# Brief 12 — non-irreversible, implemented, VERIFY: PASS with a Date/Runner row

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
Verifier run (independent, non-implementer — glm-verifier):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | 5 tests PASS | 2026-07-18 | glm-verifier |

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (integrity surface; regulatory risk).
