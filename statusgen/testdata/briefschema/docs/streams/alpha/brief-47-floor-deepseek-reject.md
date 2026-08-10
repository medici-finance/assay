---
brief: alpha/47
title: Risk-flagged brief verified by a deepseek runner (floor must reject)
wave: 0
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by fixture
sources: ["fixture: risk-flagged, verified by a below-floor deepseek runner"]
---

# Brief 47 — risk-flagged, verified by deepseek

`deepseek` is a named below-floor family: strong enough for mechanical work,
not strong enough to sign off a risk-flagged brief's verification.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |

## Evidence
<!-- contract -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test` | 0 | ok | 2026-07-09 | reviewer |

## Review
Gate: human (customer). Verified below the floor — the floor should reject this.
