---
brief: vf/01
title: Human-gated brief verified at a local tier only (below the verifier floor)
wave: 0
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: yes, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by fixture
sources: ["fixture: gate:human, verified, Verified cell names a below-floor runner"]
---

# Brief 01 — verified below the floor

The routine drain ran at a local tier and flipped the row `verified`. No
floor-tier re-verify stamp has landed, so a human done close must be REFUSED,
not flipped: the flip would land on main red under the methodology/19 floor.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./...` | exit 0 |

## Evidence
<!-- contract comment -->

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./...` | 0 | ok | 2026-07-08 | sonnet-verifier |

**VERIFY: PASS** (model) — all rows green.

## Review
Gate: human (regulatory).
