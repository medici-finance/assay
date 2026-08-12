---
brief: alpha/43
title: Irreversible brief verified by a below-floor runner (floor defers to #159)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: irreversible, verified, cheap runner but human-reviewed"]
---

# Brief 43 — irreversible, below-floor-verified, human-reviewed

The verifier floor exempts irreversible briefs: #159's stricter human-at-verified
rule governs them via the Reviewed cell, so the below-floor Verified runner here
must NOT also trip the floor.

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
Gate: human (irreversible). Human-reviewed at the verified stage per #159.
