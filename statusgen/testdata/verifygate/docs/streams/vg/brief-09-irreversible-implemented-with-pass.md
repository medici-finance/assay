---
brief: vg/09
title: Irreversible brief at implemented with model verify pass
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-17 by fixture
sources: ["fixture: irreversible, implemented, VERIFY: PASS"]
gate-why: >-
  Moves customer collateral on-ledger with no undo; sign-off confirms the model
  verify pass covers all critical paths.
why: >-
  Irreversible briefs need a human sign-off before they can advance past
  implemented, but the verify-gate machinery only fires at verified — this brief
  tests the chicken-and-egg fix.
---

# Brief 09 — irreversible, implemented, VERIFY: PASS with UNRUN rows

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |
| 2 | `go vet ./...` | exit 0 |
| 3 | live: submit deposit with idempotency key, re-submit identical | exactly ONE vault created |

## Evidence
Verifier run (independent, non-implementer — opus-verifier):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | 5 tests PASS | 2026-07-17 | opus-verifier |
| 2 | `go vet ./...` | 0 | clean | 2026-07-17 | opus-verifier |
| 3 | live deposit double-submit | — | UNRUN — mutating money-path; awaits human/live check | 2026-07-17 | opus-verifier |

**VERIFY: PASS** (model) — deterministic idempotency key wired; row 3 UNRUN (live mutating deposit, human gate).

## Review
Gate: human (irreversible + customer).
