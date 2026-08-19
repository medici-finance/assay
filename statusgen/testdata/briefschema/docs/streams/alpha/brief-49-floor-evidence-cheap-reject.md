---
brief: alpha/49
title: Strong Verified cell but Evidence records a below-floor run (floor rejects)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified, above-floor cell but below-floor Evidence run"]
---

# Brief 49 — risk-flagged, cell clears but Evidence is below floor

The Verified cell names an above-floor runner (`k3-verifier`), so the cell-only
floor is satisfied. But the `## Evidence` section — the record of who actually
ran the row the floor protects — shows the row was run only by a below-floor
runner, with no strong-tier re-run curing it. The floor must read Evidence and
reject this.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | haiku-verifier |

## Review
Gate: human (customer). The cell says `k3-verifier`, but Evidence says the row
was actually run at a below-floor tier and never cured — the floor is not met.
