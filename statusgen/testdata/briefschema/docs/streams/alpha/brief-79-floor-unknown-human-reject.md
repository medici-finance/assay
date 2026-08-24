---
brief: alpha/79
title: Risk-flagged brief whose Verified runner is an unresolvable human token
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified, human verifier"]
---

# Brief 79 — malformed human runner (names no login) (#280)

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | human:alex |

## Review
Gate: human (customer). The Verified cell reads `human:` — a human token that
names no login at all. It must FAIL the floor loud: a token naming no human is not
gate-satisfied (this is the forgery boundary the leaver-principle leniency keeps).
