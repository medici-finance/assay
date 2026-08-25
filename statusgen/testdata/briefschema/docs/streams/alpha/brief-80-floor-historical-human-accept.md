---
brief: alpha/80
title: Risk-flagged brief verified by a human no longer in the current login map
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified, historical human verifier"]
---

# Brief 80 — historical human runner not in today's map (#280)

The Verified cell names `human:bob`. `bob` is NOT in the current ASSAY_HUMAN_LOGIN_MAP
(only `alex` is) but IS recorded in ASSAY_FORMER_HUMAN_LOGIN_MAP as a departed human
who was confirmed at some past point. By the leaver principle a Verified cell records
who signed off THEN, so dropping `bob` from the current roster must NOT retroactively
fail this board — the floor must CLEAR for a human confirmed now OR historically.
Live-identity enforcement (that `bob` actually acted) is `--corroborate`'s job, and it
consults the CURRENT map only, not the former one.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | human:bob |

## Review
Gate: human (customer). A human (bob) verified it; bob has since left the map. A
historical human stamp still clears the capability floor.
