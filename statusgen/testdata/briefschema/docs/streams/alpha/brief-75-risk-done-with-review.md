---
brief: alpha/75
title: Risk-classed done brief with a security-review token in Reviewed
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by fixture
sources: ["fixture: risk-classed done, with security-review token"]
gate-why: "Irreversible on-ledger change; human must sign off before done."
---

# Brief 75 — risk-classed, done, security-review in Reviewed

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-16 | human:alex |

## Review
Gate: human.
