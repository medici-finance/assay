---
brief: alpha/53
title: Below-floor run cured by a strong-tier re-run in Evidence (floor accepts)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified, below-floor run cured by a strong-tier re-run"]
---

# Brief 53 — risk-flagged, cheap run cured by a strong-tier re-run

Row 1 was first run at a below-floor tier (`haiku-verifier`), then genuinely
re-run by an above-floor runner (`opus-verifier`) recorded in a second Evidence
table. The floor is satisfied PER ROW, so the cured row does not fail — this is
the legitimate cheap-then-strong-re-run shape, and keeping it a pass is what
stops the Evidence read from becoming a new false rejection.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

Implementer run:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | haiku-verifier |

Independent strong-tier re-run:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-10 | opus-verifier |

## Review
Gate: human (customer). Row 1 was re-run at strong tier — the floor is cleared
for the row it protects.
